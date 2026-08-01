package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"os"
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

type refreshDuringQuoteAnalyzer struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

type staticQuoteAnalyzer struct {
	quote model.LiveQuote
}

func (a *staticQuoteAnalyzer) Analyze(_ context.Context, _ string, existing *model.Equity) (*model.Equity, error) {
	return existing, nil
}

func (a *staticQuoteAnalyzer) Quote(_ context.Context, _ string, _ *model.Equity) (model.LiveQuote, error) {
	return a.quote, nil
}

func (a *refreshDuringQuoteAnalyzer) Analyze(_ context.Context, _ string, existing *model.Equity) (*model.Equity, error) {
	result := *existing
	result.Annuals = []model.AnnualPoint{{PeriodEnd: "2026-06-30", DilutedSharesB: floatPtr(12)}}
	result.Current.SharesOutstandingB = floatPtr(12)
	result.Current.SharesOutstandingAsOf = "2026-07-30"
	result.Valuation.NetDebtB = floatPtr(7)
	return &result, nil
}

func (a *refreshDuringQuoteAnalyzer) Quote(ctx context.Context, ticker string, _ *model.Equity) (model.LiveQuote, error) {
	a.calls.Add(1)
	close(a.started)
	select {
	case <-ctx.Done():
		return model.LiveQuote{}, ctx.Err()
	case <-a.release:
	}
	return model.LiveQuote{
		Ticker:                     ticker,
		Price:                      floatPtr(100),
		AsOf:                       "2026-07-31T17:00:00Z",
		Source:                     "fixture",
		StockSplitCoverageStart:    "2020-01-01",
		StockSplitCoverageComplete: true,
	}, nil
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

func TestServiceStaleSparseQuoteDoesNotSuppressNewerPersistence(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{
		AsOf:    "2026-07-31T14:00:00Z",
		Numeric: map[string]float64{"price": 100, "moving-average-200d": 80},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{
		AsOf:    "2026-07-31T15:00:00Z",
		Numeric: map[string]float64{"price": 101},
	}); err != nil {
		t.Fatal(err)
	}

	analyzer := &staticQuoteAnalyzer{quote: model.LiveQuote{Ticker: "AMZN", AsOf: "2026-07-31T14:30:00Z", Price: floatPtr(999)}}
	service := NewService(state, analyzer).WithQuoteTTL(0)
	if _, err := service.Quote(context.Background(), "AMZN"); err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	_, staleMarkedPersisted := service.quotePersistedAt["AMZN"]
	service.mu.Unlock()
	if staleMarkedPersisted {
		t.Fatal("rejected stale observation started the persistence throttle")
	}

	analyzer.quote = model.LiveQuote{Ticker: "AMZN", AsOf: "2026-07-31T15:01:00Z", Price: floatPtr(102)}
	if _, err := service.Quote(context.Background(), "AMZN"); err != nil {
		t.Fatal(err)
	}
	got := service.Snapshot().Tickers["AMZN"].QuoteHistory
	if len(got) == 0 || got[len(got)-1].Numeric["price"] != 102 || got[len(got)-1].LatestObservationAsOf != "2026-07-31T15:01:00Z" {
		t.Fatalf("newer observation was not persisted after stale rejection: %#v", got)
	}
}

func TestServiceRebasesQuoteAfterConcurrentFundamentalsRefresh(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &refreshDuringQuoteAnalyzer{started: make(chan struct{}), release: make(chan struct{})}
	service := NewService(state, analyzer).WithQuoteTTL(time.Minute)
	result := make(chan model.LiveQuote, 1)
	quoteErr := make(chan error, 1)
	go func() {
		quote, err := service.Quote(context.Background(), "AMZN")
		result <- quote
		quoteErr <- err
	}()
	<-analyzer.started
	service.refresh(context.Background(), "AMZN")
	close(analyzer.release)
	quote := <-result
	if err := <-quoteErr; err != nil {
		t.Fatal(err)
	}
	assertLiveFloat(t, "rebased shares", quote.SharesOutstandingB, 12)
	assertLiveFloat(t, "rebased market cap", quote.MarketCapB, 1200)
	assertLiveFloat(t, "rebased enterprise value", quote.EnterpriseValueB, 1207)
	if quote.ShareBasisAsOf != "2026-07-30" {
		t.Fatalf("stale share basis survived refresh: %+v", quote)
	}
	stored := service.Snapshot().Tickers["AMZN"].QuoteHistory
	latest := stored[len(stored)-1]
	if latest.Numeric["shares-outstanding"] != 12 || latest.Numeric["market-cap"] != 1200 || latest.Numeric["enterprise-value"] != 1207 {
		t.Fatalf("persisted quote did not use refreshed fundamentals: %+v", latest)
	}
	cached, err := service.Quote(context.Background(), "AMZN")
	if err != nil {
		t.Fatal(err)
	}
	if analyzer.calls.Load() != 1 || cached.SharesOutstandingB == nil || *cached.SharesOutstandingB != 12 {
		t.Fatalf("cache retained stale fundamentals: calls=%d quote=%+v", analyzer.calls.Load(), cached)
	}
}

func TestServiceRejectsImplausiblyFutureQuoteTimestamp(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	before := len(state.Snapshot().Tickers["AMZN"].QuoteHistory)
	service := NewService(state, &staticQuoteAnalyzer{quote: model.LiveQuote{Price: floatPtr(100), AsOf: "2099-01-01T00:00:00Z", Source: "fixture"}})
	if _, err := service.Quote(context.Background(), "AMZN"); !errors.Is(err, ErrQuoteUpstream) {
		t.Fatalf("future provider timestamp error = %v", err)
	}
	if after := len(state.Snapshot().Tickers["AMZN"].QuoteHistory); after != before {
		t.Fatalf("future provider timestamp changed history: before=%d after=%d", before, after)
	}
}

func TestServiceRecoversFromPersistedFutureQuoteTimestamp(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.json")
	now := time.Now().UTC().Truncate(time.Second)
	validPrior := now.Add(-24 * time.Hour)
	poisonedFuture := now.AddDate(20, 0, 0)
	seedState := model.NewState()
	seedState.Tickers["AMZN"] = &model.Equity{
		Ticker: "AMZN",
		Status: "ready",
		QuoteHistory: []model.StatisticSnapshot{
			{AsOf: validPrior.Format(time.RFC3339), Numeric: map[string]float64{"price": 90}},
			{AsOf: poisonedFuture.Format(time.RFC3339), Numeric: map[string]float64{"price": 999}},
		},
	}
	data, err := json.Marshal(seedState)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seed, data, 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := store.Open(filepath.Join(dir, "state.json"), seed, 10)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(state, &staticQuoteAnalyzer{quote: model.LiveQuote{
		Ticker:       "AMZN",
		Price:        floatPtr(100),
		AsOf:         now.Format(time.RFC3339),
		Source:       "fixture",
		FieldSources: map[string]string{"asOf": "fixture provider timestamp"},
	}})

	seeded := service.Stats().ScheduledSnapshotObservations["AMZN"]
	if !seeded.LastObservation.Equal(validPrior) {
		t.Fatalf("future row poisoned seeded freshness: %+v", seeded)
	}
	if _, err := service.Quote(context.Background(), "AMZN"); err != nil {
		t.Fatal(err)
	}
	history := service.Snapshot().Tickers["AMZN"].QuoteHistory
	if len(history) != 2 || history[0].AsOf != validPrior.Format(time.RFC3339) || history[1].AsOf != now.Format(time.RFC3339) {
		t.Fatalf("valid quote did not recover poisoned history: %#v", history)
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

func TestServiceShutdownCancelsAndDrainsSharedQuoteWork(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &controlledQuoteAnalyzer{started: make(chan struct{}), release: make(chan struct{})}
	service := NewService(state, analyzer)
	rootCtx, cancelRoot := context.WithCancel(context.Background())
	service.Start(rootCtx, 1)

	quoteDone := make(chan error, 1)
	go func() {
		_, quoteErr := service.Quote(context.Background(), "AMZN")
		quoteDone <- quoteErr
	}()
	<-analyzer.started
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), time.Second)
	defer cancelShutdown()
	if err := service.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown did not drain quote call: %v", err)
	}
	if err := <-quoteDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("provider call was not cancelled by service lifecycle: %v", err)
	}
	if _, err := service.Quote(context.Background(), "AMZN"); !errors.Is(err, context.Canceled) {
		t.Fatalf("new quote work started after shutdown: %v", err)
	}
	cancelRoot()
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
