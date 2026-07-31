package analysis

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func TestQuoteSnapshotSchedulerAdaptiveCadenceAndPostCloseConfirmation(t *testing.T) {
	clock := newSnapshotTestClock(time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC))
	provider := newScriptedSnapshotProvider([]snapshotProviderResult{
		{quote: model.LiveQuote{Ticker: "AAPL", MarketState: "REGULAR", AsOf: "2026-07-31T14:00:00Z"}},
		{quote: model.LiveQuote{Ticker: "AAPL", MarketState: "POST", AsOf: "2026-07-31T20:00:00Z"}},
		{quote: model.LiveQuote{Ticker: "AAPL", MarketState: "CLOSED", AsOf: "2026-07-31T20:00:00Z"}},
	})
	config := snapshotTestConfig(clock)
	scheduler := mustSnapshotScheduler(t, provider, snapshotTickerList{"AAPL"}, config)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(done)
	}()

	waitSnapshotLifecycle(t, provider.lifecycle, true)
	waitSnapshotCall(t, provider.calls, "AAPL")
	waitSnapshotTimer(t, clock.created, config.RegularInterval)

	clock.Advance(config.RegularInterval - time.Second)
	assertNoSnapshotCall(t, provider.calls)
	clock.Advance(time.Second)
	waitSnapshotCall(t, provider.calls, "AAPL")
	waitSnapshotTimer(t, clock.created, config.PostCloseInterval)

	clock.Advance(config.PostCloseInterval)
	waitSnapshotCall(t, provider.calls, "AAPL")
	waitSnapshotTimer(t, clock.created, config.ClosedInterval)

	cancel()
	waitSnapshotDone(t, done)
	waitSnapshotLifecycle(t, provider.lifecycle, false)
}

func TestQuoteSnapshotSchedulerRetriesErrors(t *testing.T) {
	clock := newSnapshotTestClock(time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC))
	provider := newScriptedSnapshotProvider([]snapshotProviderResult{
		{err: errors.New("temporary upstream error")},
		{quote: model.LiveQuote{Ticker: "AAPL", MarketState: "REGULAR"}},
	})
	config := snapshotTestConfig(clock)
	scheduler := mustSnapshotScheduler(t, provider, snapshotTickerList{"AAPL"}, config)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(done)
	}()

	waitSnapshotCall(t, provider.calls, "AAPL")
	waitSnapshotTimer(t, clock.created, config.RetryInterval)
	clock.Advance(config.RetryInterval)
	waitSnapshotCall(t, provider.calls, "AAPL")
	waitSnapshotTimer(t, clock.created, config.RegularInterval)

	cancel()
	waitSnapshotDone(t, done)
}

func TestQuoteSnapshotSchedulerRetriesUnknownMarketState(t *testing.T) {
	now := time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC)
	config := snapshotTestConfig(newSnapshotTestClock(now))
	scheduler := mustSnapshotScheduler(t, newScriptedSnapshotProvider(nil), snapshotTickerList{"AAPL"}, config)
	state := &quoteSnapshotSchedule{}

	scheduler.applyResult(state, quoteSnapshotResult{ticker: "AAPL", marketState: "UNKNOWN"}, now)
	if !state.nextDue.Equal(now.Add(config.RetryInterval)) {
		t.Fatalf("unknown-state retry due = %s", state.nextDue)
	}
	scheduler.applyResult(state, quoteSnapshotResult{ticker: "AAPL", marketState: "PRE"}, now)
	if !state.nextDue.Equal(now.Add(config.RegularInterval)) || !state.needsPostCloseConfirm {
		t.Fatalf("pre-market cadence = %+v", state)
	}
}

func TestQuoteSnapshotSchedulerBoundsConcurrencyAndTickerOverlap(t *testing.T) {
	clock := newSnapshotTestClock(time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC))
	provider := newBlockingSnapshotProvider()
	config := snapshotTestConfig(clock)
	config.Concurrency = 2
	scheduler := mustSnapshotScheduler(t, provider, snapshotTickerList{"MSFT", "AAPL", "NVDA"}, config)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(done)
	}()

	first := waitSnapshotCallAny(t, provider.calls)
	second := waitSnapshotCallAny(t, provider.calls)
	if first == second {
		t.Fatalf("overlapping calls targeted the same ticker %q", first)
	}
	assertNoSnapshotCall(t, provider.calls)
	provider.releaseOne <- struct{}{}
	waitSnapshotCall(t, provider.calls, "NVDA")

	cancel()
	provider.releaseOne <- struct{}{}
	provider.releaseOne <- struct{}{}
	waitSnapshotDone(t, done)
	if maximum := provider.maximumActive(); maximum != config.Concurrency {
		t.Fatalf("maximum active calls = %d, want %d", maximum, config.Concurrency)
	}
}

func TestQuoteSnapshotSchedulerDefersForFundamentalsRefresh(t *testing.T) {
	clock := newSnapshotTestClock(time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC))
	provider := newScriptedSnapshotProvider([]snapshotProviderResult{{
		quote: model.LiveQuote{Ticker: "AAPL", MarketState: "REGULAR"},
	}})
	provider.setStats(Stats{InFlight: 1, QueueDepth: 1})
	config := snapshotTestConfig(clock)
	scheduler := mustSnapshotScheduler(t, provider, snapshotTickerList{"AAPL"}, config)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(done)
	}()

	waitSnapshotTimer(t, clock.created, config.BusyInterval)
	assertNoSnapshotCall(t, provider.calls)
	provider.setStats(Stats{})
	clock.Advance(config.BusyInterval)
	waitSnapshotCall(t, provider.calls, "AAPL")

	cancel()
	waitSnapshotDone(t, done)
}

func TestQuoteSnapshotSchedulerDoesNotDeferForIndependentMacroRefresh(t *testing.T) {
	clock := newSnapshotTestClock(time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC))
	provider := newScriptedSnapshotProvider([]snapshotProviderResult{{
		quote: model.LiveQuote{Ticker: "AAPL", MarketState: "REGULAR"},
	}})
	provider.setStats(Stats{MacroRefreshing: true})
	config := snapshotTestConfig(clock)
	scheduler := mustSnapshotScheduler(t, provider, snapshotTickerList{"AAPL"}, config)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(done)
	}()

	waitSnapshotCall(t, provider.calls, "AAPL")

	cancel()
	waitSnapshotDone(t, done)
}

func TestQuoteSnapshotSchedulerReconcilesPersistedTickers(t *testing.T) {
	clock := newSnapshotTestClock(time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC))
	provider := newBlockingSnapshotProvider()
	tickers := &mutableSnapshotTickerList{tickers: []string{"AAPL"}}
	config := snapshotTestConfig(clock)
	config.DiscoveryInterval = time.Minute
	scheduler := mustSnapshotScheduler(t, provider, tickers, config)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(done)
	}()
	waitSnapshotCall(t, provider.calls, "AAPL")
	// Consume the timer installed while AAPL is in flight, then release the
	// result so the next timer is known to use the reconciled scheduler state.
	waitSnapshotTimer(t, clock.created, config.DiscoveryInterval)
	tickers.set([]string{"MSFT"})
	provider.releaseOne <- struct{}{}
	waitSnapshotTimer(t, clock.created, config.DiscoveryInterval)
	clock.Advance(config.DiscoveryInterval)
	waitSnapshotCall(t, provider.calls, "MSFT")

	cancel()
	waitSnapshotDone(t, done)
}

func TestQuoteSnapshotSchedulerHonorsCancellation(t *testing.T) {
	clock := newSnapshotTestClock(time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC))
	provider := newBlockingSnapshotProvider()
	config := snapshotTestConfig(clock)
	scheduler := mustSnapshotScheduler(t, provider, snapshotTickerList{"AAPL"}, config)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(done)
	}()
	waitSnapshotCall(t, provider.calls, "AAPL")
	cancel()
	waitSnapshotDone(t, done)
}

func TestNewQuoteSnapshotSchedulerValidatesConfiguration(t *testing.T) {
	provider := newScriptedSnapshotProvider(nil)
	tickers := snapshotTickerList{"AAPL"}
	config := DefaultQuoteSnapshotSchedulerConfig()
	config.PostCloseInterval = quotePersistenceInterval - time.Second
	if _, err := NewQuoteSnapshotScheduler(provider, tickers, config); err == nil {
		t.Fatal("expected a post-close persistence-window error")
	}
	config = DefaultQuoteSnapshotSchedulerConfig()
	config.InitialDelay = -time.Second
	if _, err := NewQuoteSnapshotScheduler(provider, tickers, config); err == nil {
		t.Fatal("expected a negative-duration error")
	}
}

func snapshotTestConfig(clock QuoteSnapshotClock) QuoteSnapshotSchedulerConfig {
	return QuoteSnapshotSchedulerConfig{
		RegularInterval:   15 * time.Minute,
		PostCloseInterval: 20 * time.Minute,
		ClosedInterval:    2 * time.Hour,
		RetryInterval:     5 * time.Minute,
		BusyInterval:      2 * time.Minute,
		DiscoveryInterval: 24 * time.Hour,
		RequestTimeout:    time.Hour,
		Concurrency:       1,
		Clock:             clock,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func mustSnapshotScheduler(t *testing.T, provider ScheduledQuoteProvider, tickers QuoteSnapshotTickerSource, config QuoteSnapshotSchedulerConfig) *QuoteSnapshotScheduler {
	t.Helper()
	scheduler, err := NewQuoteSnapshotScheduler(provider, tickers, config)
	if err != nil {
		t.Fatalf("NewQuoteSnapshotScheduler: %v", err)
	}
	return scheduler
}

type snapshotTickerList []string

func (tickers snapshotTickerList) Tickers() []string {
	return append([]string(nil), tickers...)
}

type mutableSnapshotTickerList struct {
	mu      sync.Mutex
	tickers []string
}

func (tickers *mutableSnapshotTickerList) Tickers() []string {
	tickers.mu.Lock()
	defer tickers.mu.Unlock()
	return append([]string(nil), tickers.tickers...)
}

func (tickers *mutableSnapshotTickerList) set(next []string) {
	tickers.mu.Lock()
	tickers.tickers = append([]string(nil), next...)
	tickers.mu.Unlock()
}

type snapshotProviderResult struct {
	quote model.LiveQuote
	err   error
}

type scriptedSnapshotProvider struct {
	mu        sync.Mutex
	results   []snapshotProviderResult
	next      int
	stats     Stats
	calls     chan string
	lifecycle chan bool
}

func newScriptedSnapshotProvider(results []snapshotProviderResult) *scriptedSnapshotProvider {
	return &scriptedSnapshotProvider{
		results:   results,
		calls:     make(chan string, 16),
		lifecycle: make(chan bool, 4),
	}
}

func (p *scriptedSnapshotProvider) ScheduledQuote(_ context.Context, ticker string) (model.LiveQuote, error) {
	p.calls <- ticker
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.next >= len(p.results) {
		return model.LiveQuote{Ticker: ticker, MarketState: "CLOSED"}, nil
	}
	result := p.results[p.next]
	p.next++
	return result.quote, result.err
}

func (p *scriptedSnapshotProvider) SetSnapshotSchedulerRunning(running bool) {
	p.lifecycle <- running
}

func (p *scriptedSnapshotProvider) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stats
}

func (p *scriptedSnapshotProvider) setStats(stats Stats) {
	p.mu.Lock()
	p.stats = stats
	p.mu.Unlock()
}

type blockingSnapshotProvider struct {
	mu           sync.Mutex
	active       int
	maxActive    int
	activeTicker map[string]bool
	calls        chan string
	releaseOne   chan struct{}
}

func newBlockingSnapshotProvider() *blockingSnapshotProvider {
	return &blockingSnapshotProvider{
		activeTicker: make(map[string]bool),
		calls:        make(chan string, 16),
		releaseOne:   make(chan struct{}, 16),
	}
}

func (p *blockingSnapshotProvider) ScheduledQuote(ctx context.Context, ticker string) (model.LiveQuote, error) {
	p.mu.Lock()
	p.active++
	if p.active > p.maxActive {
		p.maxActive = p.active
	}
	if p.activeTicker[ticker] {
		p.mu.Unlock()
		return model.LiveQuote{}, errors.New("overlapping ticker call")
	}
	p.activeTicker[ticker] = true
	p.mu.Unlock()
	p.calls <- ticker

	select {
	case <-ctx.Done():
	case <-p.releaseOne:
	}
	p.mu.Lock()
	p.active--
	delete(p.activeTicker, ticker)
	p.mu.Unlock()
	return model.LiveQuote{Ticker: ticker, MarketState: "REGULAR"}, ctx.Err()
}

func (p *blockingSnapshotProvider) maximumActive() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.maxActive
}

type snapshotTestClock struct {
	mu      sync.Mutex
	now     time.Time
	timers  map[*snapshotTestTimer]struct{}
	created chan time.Duration
}

func newSnapshotTestClock(now time.Time) *snapshotTestClock {
	return &snapshotTestClock{
		now:     now,
		timers:  make(map[*snapshotTestTimer]struct{}),
		created: make(chan time.Duration, 128),
	}
}

func (c *snapshotTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *snapshotTestClock) NewTimer(delay time.Duration) QuoteSnapshotTimer {
	c.mu.Lock()
	timer := &snapshotTestTimer{
		clock:    c,
		deadline: c.now.Add(delay),
		channel:  make(chan time.Time, 1),
		active:   true,
	}
	c.timers[timer] = struct{}{}
	now := c.now
	if delay <= 0 {
		timer.active = false
		delete(c.timers, timer)
	}
	c.mu.Unlock()
	c.created <- delay
	if delay <= 0 {
		timer.channel <- now
	}
	return timer
}

func (c *snapshotTestClock) Advance(delta time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(delta)
	now := c.now
	due := make([]*snapshotTestTimer, 0)
	for timer := range c.timers {
		if timer.active && !timer.deadline.After(now) {
			timer.active = false
			delete(c.timers, timer)
			due = append(due, timer)
		}
	}
	c.mu.Unlock()
	for _, timer := range due {
		timer.channel <- now
	}
}

type snapshotTestTimer struct {
	clock    *snapshotTestClock
	deadline time.Time
	channel  chan time.Time
	active   bool
}

func (t *snapshotTestTimer) C() <-chan time.Time { return t.channel }

func (t *snapshotTestTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	wasActive := t.active
	t.active = false
	delete(t.clock.timers, t)
	return wasActive
}

func waitSnapshotCall(t *testing.T, calls <-chan string, expected string) {
	t.Helper()
	if ticker := waitSnapshotCallAny(t, calls); ticker != expected {
		t.Fatalf("scheduled ticker = %q, want %q", ticker, expected)
	}
}

func waitSnapshotCallAny(t *testing.T, calls <-chan string) string {
	t.Helper()
	select {
	case ticker := <-calls:
		return ticker
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for scheduled quote call")
		return ""
	}
}

func assertNoSnapshotCall(t *testing.T, calls <-chan string) {
	t.Helper()
	select {
	case ticker := <-calls:
		t.Fatalf("unexpected scheduled quote call for %q", ticker)
	case <-time.After(25 * time.Millisecond):
	}
}

func waitSnapshotTimer(t *testing.T, timers <-chan time.Duration, expected time.Duration) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case delay := <-timers:
			if delay == expected {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for scheduler timer %s", expected)
		}
	}
}

func waitSnapshotLifecycle(t *testing.T, lifecycle <-chan bool, expected bool) {
	t.Helper()
	select {
	case running := <-lifecycle:
		if running != expected {
			t.Fatalf("scheduler running = %v, want %v", running, expected)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for scheduler lifecycle %v", expected)
	}
}

func waitSnapshotDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop after cancellation")
	}
}
