package analysis

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

const (
	defaultSnapshotRegularInterval   = 15 * time.Minute
	defaultSnapshotPostCloseInterval = 20 * time.Minute
	defaultSnapshotClosedInterval    = 2 * time.Hour
	defaultSnapshotRetryInterval     = 5 * time.Minute
	defaultSnapshotBusyInterval      = 2 * time.Minute
	defaultSnapshotDiscoveryInterval = time.Minute
	defaultSnapshotRequestTimeout    = 30 * time.Second
	defaultSnapshotConcurrency       = 1
)

// ScheduledQuoteProvider records the observability associated with a
// scheduler-initiated quote request in addition to retrieving the quote.
type ScheduledQuoteProvider interface {
	ScheduledQuote(context.Context, string) (model.LiveQuote, error)
}

// QuoteSnapshotTickerSource returns the current persisted watchlist. The
// scheduler periodically reconciles this list so additions and removals do not
// require a restart.
type QuoteSnapshotTickerSource interface {
	Tickers() []string
}

// QuoteSnapshotTimer and QuoteSnapshotClock keep the scheduling loop
// deterministic in tests without weakening production cancellation semantics.
type QuoteSnapshotTimer interface {
	C() <-chan time.Time
	Stop() bool
}

type QuoteSnapshotClock interface {
	Now() time.Time
	NewTimer(time.Duration) QuoteSnapshotTimer
}

type QuoteSnapshotSchedulerConfig struct {
	RegularInterval   time.Duration
	PostCloseInterval time.Duration
	ClosedInterval    time.Duration
	RetryInterval     time.Duration
	BusyInterval      time.Duration
	DiscoveryInterval time.Duration
	RequestTimeout    time.Duration
	InitialDelay      time.Duration
	InitialStagger    time.Duration
	Concurrency       int
	Clock             QuoteSnapshotClock
	Logger            *slog.Logger
}

// DefaultQuoteSnapshotSchedulerConfig is intentionally conservative: current
// quotes are sampled every 15 minutes during regular trading, once more after
// the close, and every two hours otherwise. Initial work is staggered to avoid
// a burst against the upstream provider after a deployment.
func DefaultQuoteSnapshotSchedulerConfig() QuoteSnapshotSchedulerConfig {
	return QuoteSnapshotSchedulerConfig{
		RegularInterval:   defaultSnapshotRegularInterval,
		PostCloseInterval: defaultSnapshotPostCloseInterval,
		ClosedInterval:    defaultSnapshotClosedInterval,
		RetryInterval:     defaultSnapshotRetryInterval,
		BusyInterval:      defaultSnapshotBusyInterval,
		DiscoveryInterval: defaultSnapshotDiscoveryInterval,
		RequestTimeout:    defaultSnapshotRequestTimeout,
		InitialDelay:      2 * time.Minute,
		InitialStagger:    5 * time.Second,
		Concurrency:       defaultSnapshotConcurrency,
	}
}

type QuoteSnapshotScheduler struct {
	provider ScheduledQuoteProvider
	tickers  QuoteSnapshotTickerSource
	config   QuoteSnapshotSchedulerConfig
}

func NewQuoteSnapshotScheduler(provider ScheduledQuoteProvider, tickers QuoteSnapshotTickerSource, config QuoteSnapshotSchedulerConfig) (*QuoteSnapshotScheduler, error) {
	if provider == nil {
		return nil, errors.New("quote snapshot provider is required")
	}
	if tickers == nil {
		return nil, errors.New("quote snapshot ticker source is required")
	}
	if config.RegularInterval == 0 {
		config.RegularInterval = defaultSnapshotRegularInterval
	}
	if config.PostCloseInterval == 0 {
		config.PostCloseInterval = defaultSnapshotPostCloseInterval
	}
	if config.ClosedInterval == 0 {
		config.ClosedInterval = defaultSnapshotClosedInterval
	}
	if config.RetryInterval == 0 {
		config.RetryInterval = defaultSnapshotRetryInterval
	}
	if config.BusyInterval == 0 {
		config.BusyInterval = defaultSnapshotBusyInterval
	}
	if config.DiscoveryInterval == 0 {
		config.DiscoveryInterval = defaultSnapshotDiscoveryInterval
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = defaultSnapshotRequestTimeout
	}
	if config.Concurrency == 0 {
		config.Concurrency = defaultSnapshotConcurrency
	}
	if config.RegularInterval < 0 || config.PostCloseInterval < 0 || config.ClosedInterval < 0 ||
		config.RetryInterval < 0 || config.BusyInterval < 0 || config.DiscoveryInterval < 0 || config.RequestTimeout < 0 ||
		config.InitialDelay < 0 || config.InitialStagger < 0 {
		return nil, errors.New("quote snapshot scheduler durations must not be negative")
	}
	if config.PostCloseInterval < quotePersistenceInterval {
		return nil, errors.New("quote snapshot post-close interval must cover the quote persistence interval")
	}
	if config.Concurrency < 1 {
		return nil, errors.New("quote snapshot scheduler concurrency must be positive")
	}
	if config.Clock == nil {
		config.Clock = realQuoteSnapshotClock{}
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	return &QuoteSnapshotScheduler{provider: provider, tickers: tickers, config: config}, nil
}

type quoteSnapshotSchedule struct {
	nextDue               time.Time
	inFlight              bool
	needsPostCloseConfirm bool
}

type quoteSnapshotResult struct {
	ticker      string
	marketState string
	err         error
}

type quoteSnapshotLifecycle interface {
	SetSnapshotSchedulerRunning(bool)
}

type quoteSnapshotLoadReporter interface {
	Stats() Stats
}

// Run polls the persisted watchlist until ctx is cancelled. It does not permit
// overlapping scheduler requests for a ticker and globally bounds provider
// concurrency. Any active request inherits both ctx cancellation and the
// configured per-request deadline.
func (s *QuoteSnapshotScheduler) Run(ctx context.Context) {
	lifecycle, _ := s.provider.(quoteSnapshotLifecycle)
	if lifecycle != nil {
		lifecycle.SetSnapshotSchedulerRunning(true)
		defer lifecycle.SetSnapshotSchedulerRunning(false)
	}

	requestCtx, cancelRequests := context.WithCancel(ctx)
	defer cancelRequests()
	var requests sync.WaitGroup
	defer requests.Wait()

	s.config.Logger.Info("quote snapshot scheduler started",
		"regularInterval", s.config.RegularInterval,
		"postCloseInterval", s.config.PostCloseInterval,
		"closedInterval", s.config.ClosedInterval,
		"concurrency", s.config.Concurrency,
	)
	defer s.config.Logger.Info("quote snapshot scheduler stopped")

	states := make(map[string]*quoteSnapshotSchedule)
	results := make(chan quoteSnapshotResult, s.config.Concurrency)
	running := 0
	discoveryDue := s.config.Clock.Now()

	for {
		now := s.config.Clock.Now()
		if !now.Before(discoveryDue) {
			s.reconcile(states, now)
			discoveryDue = now.Add(s.config.DiscoveryInterval)
		}

		if running < s.config.Concurrency && s.refreshBusy() {
			s.deferDueSnapshots(states, now)
		}
		for running < s.config.Concurrency {
			ticker := nextDueSnapshotTicker(states, now)
			if ticker == "" {
				break
			}
			states[ticker].inFlight = true
			running++
			requests.Add(1)
			go func(ticker string) {
				defer requests.Done()
				quoteCtx, quoteCancel := context.WithTimeout(requestCtx, s.config.RequestTimeout)
				defer quoteCancel()
				quote, err := s.provider.ScheduledQuote(quoteCtx, ticker)
				result := quoteSnapshotResult{ticker: ticker, marketState: quote.MarketState, err: err}
				select {
				case results <- result:
				case <-requestCtx.Done():
				}
			}(ticker)
		}

		delay := nextSnapshotWake(states, discoveryDue, now, running < s.config.Concurrency)
		timer := s.config.Clock.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			cancelRequests()
			return
		case result := <-results:
			timer.Stop()
			running--
			state, exists := states[result.ticker]
			if !exists {
				continue
			}
			state.inFlight = false
			s.applyResult(state, result, s.config.Clock.Now())
		case <-timer.C():
		}
	}
}

func (s *QuoteSnapshotScheduler) refreshBusy() bool {
	reporter, ok := s.provider.(quoteSnapshotLoadReporter)
	if !ok {
		return false
	}
	stats := reporter.Stats()
	return stats.InFlight > 0 || stats.QueueDepth > 0 || stats.MacroRefreshing
}

func (s *QuoteSnapshotScheduler) deferDueSnapshots(states map[string]*quoteSnapshotSchedule, now time.Time) {
	tickers := make([]string, 0, len(states))
	for ticker, state := range states {
		if !state.inFlight && !state.nextDue.After(now) {
			tickers = append(tickers, ticker)
		}
	}
	sort.Strings(tickers)
	for index, ticker := range tickers {
		states[ticker].nextDue = now.Add(s.config.BusyInterval + time.Duration(index)*s.config.InitialStagger)
	}
}

func (s *QuoteSnapshotScheduler) reconcile(states map[string]*quoteSnapshotSchedule, now time.Time) {
	tickers := s.tickers.Tickers()
	sort.Strings(tickers)
	present := make(map[string]struct{}, len(tickers))
	for index, rawTicker := range tickers {
		ticker := strings.ToUpper(strings.TrimSpace(rawTicker))
		if ticker == "" {
			continue
		}
		present[ticker] = struct{}{}
		if _, exists := states[ticker]; !exists {
			states[ticker] = &quoteSnapshotSchedule{
				nextDue: now.Add(s.config.InitialDelay + time.Duration(index)*s.config.InitialStagger),
			}
		}
	}
	for ticker := range states {
		if _, exists := present[ticker]; !exists {
			delete(states, ticker)
		}
	}
}

func (s *QuoteSnapshotScheduler) applyResult(state *quoteSnapshotSchedule, result quoteSnapshotResult, now time.Time) {
	if result.err != nil {
		state.nextDue = now.Add(s.config.RetryInterval)
		s.config.Logger.Warn("scheduled quote snapshot failed", "ticker", result.ticker, "error", result.err)
		return
	}
	if strings.EqualFold(strings.TrimSpace(result.marketState), "REGULAR") {
		state.needsPostCloseConfirm = true
		state.nextDue = now.Add(s.config.RegularInterval)
		return
	}
	if state.needsPostCloseConfirm {
		// The transition poll already observes the closing value. One delayed
		// confirmation ensures it is persisted after the service's same-day
		// snapshot throttle and accommodates a late-settling upstream close.
		state.needsPostCloseConfirm = false
		state.nextDue = now.Add(s.config.PostCloseInterval)
		return
	}
	state.nextDue = now.Add(s.config.ClosedInterval)
}

func nextDueSnapshotTicker(states map[string]*quoteSnapshotSchedule, now time.Time) string {
	due := make([]string, 0, len(states))
	for ticker, state := range states {
		if !state.inFlight && !state.nextDue.After(now) {
			due = append(due, ticker)
		}
	}
	if len(due) == 0 {
		return ""
	}
	sort.Strings(due)
	return due[0]
}

func nextSnapshotWake(states map[string]*quoteSnapshotSchedule, discoveryDue, now time.Time, capacityAvailable bool) time.Duration {
	next := discoveryDue
	if capacityAvailable {
		for _, state := range states {
			if !state.inFlight && state.nextDue.Before(next) {
				next = state.nextDue
			}
		}
	}
	if !next.After(now) {
		return 0
	}
	return next.Sub(now)
}

type realQuoteSnapshotClock struct{}

func (realQuoteSnapshotClock) Now() time.Time { return time.Now().UTC() }

func (realQuoteSnapshotClock) NewTimer(delay time.Duration) QuoteSnapshotTimer {
	return realQuoteSnapshotTimer{timer: time.NewTimer(delay)}
}

type realQuoteSnapshotTimer struct {
	timer *time.Timer
}

func (t realQuoteSnapshotTimer) C() <-chan time.Time { return t.timer.C }
func (t realQuoteSnapshotTimer) Stop() bool          { return t.timer.Stop() }
