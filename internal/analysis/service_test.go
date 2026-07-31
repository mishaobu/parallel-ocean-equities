package analysis

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
	"github.com/mishaobu/parallel-ocean-equities/internal/store"
)

type fakeAnalyzer struct {
	err        error
	panicValue any
}

type fakeMacroAnalyzer struct {
	err     error
	started chan struct{}
	release chan struct{}
}

type fakeQuoteAnalyzer struct {
	calls int
}

type controlledQuoteAnalyzer struct {
	fakeQuoteAnalyzer
	calls      atomic.Int32
	started    chan struct{}
	startOnce  sync.Once
	release    chan struct{}
	panicValue any
}

type deadlineQuoteAnalyzer struct {
	deadline chan time.Time
}

func (a *deadlineQuoteAnalyzer) Analyze(_ context.Context, _ string, existing *model.Equity) (*model.Equity, error) {
	return existing, nil
}

func (a *deadlineQuoteAnalyzer) Quote(ctx context.Context, ticker string, _ *model.Equity) (model.LiveQuote, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return model.LiveQuote{}, errors.New("provider context has no deadline")
	}
	a.deadline <- deadline
	return model.LiveQuote{Ticker: ticker, Price: floatPtr(123), AsOf: "2026-07-31T15:30:00Z", Source: "fixture"}, nil
}

func (c *controlledQuoteAnalyzer) Quote(ctx context.Context, ticker string, _ *model.Equity) (model.LiveQuote, error) {
	c.calls.Add(1)
	c.startOnce.Do(func() { close(c.started) })
	select {
	case <-ctx.Done():
		return model.LiveQuote{}, ctx.Err()
	case <-c.release:
	}
	if c.panicValue != nil {
		panic(c.panicValue)
	}
	return model.LiveQuote{Ticker: ticker, Price: floatPtr(123), AsOf: "2026-07-31T15:30:00Z", Source: "fixture"}, nil
}

func (f *fakeQuoteAnalyzer) Analyze(_ context.Context, ticker string, existing *model.Equity) (*model.Equity, error) {
	if existing == nil {
		existing = &model.Equity{Ticker: ticker, Annuals: []model.AnnualPoint{}}
	}
	return existing, nil
}

func (f *fakeQuoteAnalyzer) Quote(_ context.Context, ticker string, _ *model.Equity) (model.LiveQuote, error) {
	f.calls++
	return model.LiveQuote{
		Ticker: ticker,
		Price:  floatPtr(float64(100 + f.calls)),
		AsOf:   "2026-07-31T15:30:00Z",
		Source: "fixture",
		History: []model.StatisticSnapshot{{
			AsOf:    "2026-06-30T16:00:00Z",
			Source:  "monthly fixture",
			Numeric: map[string]float64{"price": 99},
		}},
	}, nil
}

func (f fakeMacroAnalyzer) Analyze(ctx context.Context) (model.MacroSeries, error) {
	if f.started != nil {
		close(f.started)
	}
	if f.release != nil {
		select {
		case <-ctx.Done():
			return model.MacroSeries{}, ctx.Err()
		case <-f.release:
		}
	}
	if f.err != nil {
		return model.MacroSeries{}, f.err
	}
	return model.MacroSeries{Points: []model.MacroPoint{{Date: "2025-01-01", FedFunds: floatPtr(4.5)}}}, nil
}

func (f fakeAnalyzer) Analyze(_ context.Context, ticker string, _ *model.Equity) (*model.Equity, error) {
	if f.panicValue != nil {
		panic(f.panicValue)
	}
	if f.err != nil {
		return nil, f.err
	}
	return &model.Equity{
		Ticker:  ticker,
		Company: "NVIDIA Corporation",
		Annuals: []model.AnnualPoint{},
		Sources: []string{"test"},
	}, nil
}

func TestServiceContainsAnalyzerPanic(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(state, fakeAnalyzer{panicValue: "provider failure"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Start(ctx, 1)

	if err := service.AddTicker("NVDA"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		equity := service.Snapshot().Tickers["NVDA"]
		if equity != nil && equity.Status == "error" {
			if service.Stats().RefreshFailures != 1 {
				t.Fatal("panic was not counted as a refresh failure")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("panicking refresh was not contained")
}

func TestServiceCachesLiveQuoteForShortTTL(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &fakeQuoteAnalyzer{}
	service := NewService(state, analyzer).WithQuoteTTL(time.Minute)
	first, err := service.Quote(context.Background(), "amzn")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Quote(context.Background(), "AMZN")
	if err != nil {
		t.Fatal(err)
	}
	if analyzer.calls != 1 || first.Price == nil || second.Price == nil || *first.Price != *second.Price {
		t.Fatalf("quote cache miss: calls=%d first=%+v second=%+v", analyzer.calls, first, second)
	}
	stored := service.Snapshot().Tickers["AMZN"].QuoteHistory
	if len(first.History) != 2 || len(second.History) != 2 || len(stored) != 2 || stored[0].Numeric["price"] != 99 || stored[1].Numeric["price"] != *first.Price {
		t.Fatalf("quote history was not returned and persisted: first=%+v stored=%+v", first.History, stored)
	}
	service.WithQuoteTTL(0)
	service.markQuotePersisted("AMZN", time.Now().UTC().Add(-quotePersistenceInterval))
	third, err := service.Quote(context.Background(), "AMZN")
	if err != nil {
		t.Fatal(err)
	}
	if analyzer.calls != 2 || third.Price == nil || *third.Price == *second.Price {
		t.Fatalf("disabled cache should refetch: calls=%d second=%+v third=%+v", analyzer.calls, second, third)
	}
	correctedHistory := service.Snapshot().Tickers["AMZN"].QuoteHistory
	if len(correctedHistory) != 2 || correctedHistory[1].Numeric["price"] != *third.Price {
		t.Fatalf("equal-timestamp quote correction was not persisted: quote=%+v history=%+v", third, correctedHistory)
	}
}

func TestServiceQuoteRequestTimeoutConfiguresProviderWork(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &deadlineQuoteAnalyzer{deadline: make(chan time.Time, 1)}
	timeout := 3 * time.Second
	started := time.Now()
	service := NewService(state, analyzer).WithQuoteRequestTimeout(timeout)
	if _, err := service.Quote(context.Background(), "AMZN"); err != nil {
		t.Fatal(err)
	}
	deadline := <-analyzer.deadline
	if got := deadline.Sub(started); got < timeout-500*time.Millisecond || got > timeout+500*time.Millisecond {
		t.Fatalf("provider deadline offset = %s, want approximately %s", got, timeout)
	}
}

func TestServiceCoalescesQuoteWithoutLeaderCancellationOrPanicPoisoning(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &controlledQuoteAnalyzer{started: make(chan struct{}), release: make(chan struct{})}
	service := NewService(state, analyzer)
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderErr := make(chan error, 1)
	go func() {
		_, quoteErr := service.Quote(leaderCtx, "AMZN")
		leaderErr <- quoteErr
	}()
	<-analyzer.started
	waiter := make(chan error, 1)
	go func() {
		_, quoteErr := service.Quote(context.Background(), "AMZN")
		waiter <- quoteErr
	}()
	cancelLeader()
	if err := <-leaderErr; !errors.Is(err, context.Canceled) {
		t.Fatalf("leader cancellation = %v", err)
	}
	close(analyzer.release)
	if err := <-waiter; err != nil {
		t.Fatalf("shared quote should survive leader cancellation: %v", err)
	}
	if analyzer.calls.Load() != 1 {
		t.Fatalf("provider calls = %d, want 1", analyzer.calls.Load())
	}

	panicking := &controlledQuoteAnalyzer{started: make(chan struct{}), release: make(chan struct{}), panicValue: "boom"}
	close(panicking.release)
	panicService := NewService(state, panicking).WithQuoteTTL(0)
	for range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := panicService.Quote(ctx, "AMZN")
		cancel()
		if err == nil || !strings.Contains(err.Error(), "panicked") {
			t.Fatalf("provider panic was not contained: %v", err)
		}
	}
	if panicking.calls.Load() != 2 {
		t.Fatalf("panic left ticker call poisoned: calls=%d", panicking.calls.Load())
	}
}

func TestServiceRefreshLifecycle(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(state, fakeAnalyzer{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Start(ctx, 1)

	if err := service.AddTicker("nvda"); err != nil {
		t.Fatal(err)
	}
	if service.Queue("NVDA") {
		t.Fatal("duplicate refresh should not be queued")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		equity := service.Snapshot().Tickers["NVDA"]
		if equity != nil && equity.Status == "ready" {
			if equity.Company != "NVIDIA Corporation" {
				t.Fatalf("unexpected result: %#v", equity)
			}
			if got := service.Stats().RefreshTotal; got != 1 {
				t.Fatalf("refresh total = %d, want 1", got)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("refresh did not complete")
}

func TestServiceAcceptsNumericInternationalTicker(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(state, fakeAnalyzer{})
	if err := service.AddTicker("005930.ks"); err != nil {
		t.Fatal(err)
	}
	if service.Snapshot().Tickers["005930.KS"] == nil {
		t.Fatal("normalized international ticker was not persisted")
	}
}

func TestServiceRefreshAllQueuesMacroOnce(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	service := NewService(state, fakeAnalyzer{}).WithMacro(fakeMacroAnalyzer{started: started, release: release})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Start(ctx, 1)

	service.RefreshAll()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("macro refresh did not start")
	}
	if service.QueueMacro() {
		t.Fatal("duplicate macro refresh should not be queued")
	}
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := service.Snapshot()
		stats := service.Stats()
		if len(snapshot.Macro.Points) == 1 && !stats.MacroRefreshing && stats.QueueDepth == 0 && stats.InFlight == 0 {
			if snapshot.Macro.Points[0].FedFunds == nil || *snapshot.Macro.Points[0].FedFunds != 4.5 {
				t.Fatalf("unexpected macro state: %#v", snapshot.Macro)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("macro refresh did not complete")
}
