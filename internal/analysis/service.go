package analysis

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
	"github.com/mishaobu/parallel-ocean-equities/internal/store"
)

var tickerPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9.-]{0,9}$`)

var ErrInvalidTicker = errors.New("ticker must be 1-10 letters, numbers, dots, or hyphens")

var (
	ErrQuoteUpstream    = errors.New("live quote upstream failed")
	ErrQuotePersistence = errors.New("live quote persistence failed")
)

const (
	defaultQuoteRequestTimeout = 20 * time.Second
	defaultMacroRetryDelay     = 5 * time.Minute
	maximumQuoteFutureSkew     = 5 * time.Minute
)

type Analyzer interface {
	Analyze(context.Context, string, *model.Equity) (*model.Equity, error)
}

type TickerPreview struct {
	Ticker         string `json:"ticker"`
	Company        string `json:"company"`
	InstrumentType string `json:"instrumentType"`
	CIK            string `json:"cik,omitempty"`
	Source         string `json:"source"`
}

type TickerPreviewer interface {
	PreviewTicker(context.Context, string) (TickerPreview, error)
}

type Stats struct {
	RefreshTotal     int64     `json:"refreshTotal"`
	RefreshFailures  int64     `json:"refreshFailures"`
	QueueDepth       int       `json:"queueDepth"`
	InFlight         int       `json:"inFlight"`
	LastRefresh      time.Time `json:"lastRefresh,omitempty"`
	MacroRefreshing  bool      `json:"macroRefreshing"`
	MacroLastRefresh time.Time `json:"macroLastRefresh,omitempty"`
	MacroFailures    int64     `json:"macroFailures"`

	SnapshotSchedulerRunning        bool                           `json:"snapshotSchedulerRunning"`
	ScheduledSnapshotInFlight       int                            `json:"scheduledSnapshotInFlight"`
	ScheduledSnapshotAttempts       int64                          `json:"scheduledSnapshotAttempts"`
	ScheduledSnapshotSuccesses      int64                          `json:"scheduledSnapshotSuccesses"`
	ScheduledSnapshotNoNewSession   int64                          `json:"scheduledSnapshotNoNewSession"`
	ScheduledSnapshotFailures       map[string]int64               `json:"scheduledSnapshotFailures"`
	HistoryRefreshFailures          map[string]int64               `json:"historyRefreshFailures"`
	BenchmarkHistoryRefreshFailures map[string]int64               `json:"benchmarkHistoryRefreshFailures"`
	ScheduledQuoteFieldsExpected    int                            `json:"scheduledQuoteFieldsExpected"`
	ScheduledSnapshotObservations   map[string]SnapshotObservation `json:"scheduledSnapshotObservations"`
}

type Health struct {
	TickerCount  int       `json:"tickerCount"`
	UpdatedAt    time.Time `json:"updatedAt,omitempty"`
	ShuttingDown bool      `json:"shuttingDown"`
}

type cachedQuote struct {
	quote    model.LiveQuote
	cachedAt time.Time
}

type quoteCall struct {
	done  chan struct{}
	quote model.LiveQuote
	err   error
}

type Service struct {
	store                 *store.Store
	analyzer              Analyzer
	queue                 chan string
	macro                 MacroAnalyzer
	macroQueue            chan struct{}
	quoteTTL              time.Duration
	quoteTimeout          time.Duration
	quoteCache            map[string]cachedQuote
	quoteCalls            map[string]*quoteCall
	quotePersistedAt      map[string]time.Time
	quoteFinalizeMu       sync.Mutex
	quoteWG               sync.WaitGroup
	workerWG              sync.WaitGroup
	snapshotMetrics       scheduledSnapshotMetrics
	benchmarkFailureIDs   map[string]struct{}
	benchmarkFailureOrder []string

	mu                   sync.Mutex
	inflight             map[string]struct{}
	macroInflight        bool
	macroRetryScheduled  bool
	macroRetryGeneration uint64
	lifecycleCtx         context.Context
	lifecycleCancel      context.CancelFunc
	shuttingDown         bool
	last                 time.Time
	macroLast            time.Time
	total                atomic.Int64
	failures             atomic.Int64
	macroFailures        atomic.Int64
}

func NewService(state *store.Store, analyzer Analyzer) *Service {
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	service := &Service{
		store:               state,
		analyzer:            analyzer,
		queue:               make(chan string, 64),
		macroQueue:          make(chan struct{}, 1),
		quoteTTL:            time.Minute,
		quoteTimeout:        defaultQuoteRequestTimeout,
		quoteCache:          make(map[string]cachedQuote),
		quoteCalls:          make(map[string]*quoteCall),
		quotePersistedAt:    make(map[string]time.Time),
		lifecycleCtx:        lifecycleCtx,
		lifecycleCancel:     lifecycleCancel,
		snapshotMetrics:     newScheduledSnapshotMetrics(),
		benchmarkFailureIDs: make(map[string]struct{}),
		inflight:            make(map[string]struct{}),
	}
	service.seedScheduledSnapshotObservations()
	return service
}

func (s *Service) WithMacro(analyzer MacroAnalyzer) *Service {
	s.macro = analyzer
	return s
}

// WithQuoteTTL primarily supports deployments that need a different quote
// cadence and deterministic cache tests. A non-positive duration disables the
// cache without disabling quote retrieval.
func (s *Service) WithQuoteTTL(ttl time.Duration) *Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quoteTTL = ttl
	s.quoteCache = make(map[string]cachedQuote)
	return s
}

// WithQuoteRequestTimeout configures the actual shared provider-work deadline,
// not only an individual caller's wait. A non-positive value restores the
// service default.
func (s *Service) WithQuoteRequestTimeout(timeout time.Duration) *Service {
	if timeout <= 0 {
		timeout = defaultQuoteRequestTimeout
	}
	s.mu.Lock()
	s.quoteTimeout = timeout
	s.mu.Unlock()
	return s
}

func (s *Service) Start(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	runCtx, runCancel := context.WithCancel(ctx)
	s.mu.Lock()
	previousCancel := s.lifecycleCancel
	s.lifecycleCtx = runCtx
	s.lifecycleCancel = runCancel
	s.shuttingDown = false
	workerCount := workers
	if s.macro != nil {
		workerCount++
	}
	// Register every worker before shutdown can observe the new lifecycle. This
	// keeps Add ordered before Wait even under an immediate termination signal.
	s.workerWG.Add(workerCount)
	s.mu.Unlock()
	previousCancel()
	for range workers {
		go func() {
			defer s.workerWG.Done()
			s.worker(runCtx)
		}()
	}
	if s.macro != nil {
		go func() {
			defer s.workerWG.Done()
			s.macroWorker(runCtx)
		}()
	}
}

func (s *Service) AddTicker(ticker string) error {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if !tickerPattern.MatchString(ticker) {
		return ErrInvalidTicker
	}
	if err := s.store.Add(ticker); err != nil {
		return err
	}
	if !s.Queue(ticker) {
		return errors.New("ticker refresh is already queued")
	}
	return nil
}

func (s *Service) PreviewTicker(ctx context.Context, ticker string) (TickerPreview, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if !tickerPattern.MatchString(ticker) {
		return TickerPreview{}, ErrInvalidTicker
	}
	previewer, ok := s.analyzer.(TickerPreviewer)
	if !ok {
		return TickerPreview{}, errors.New("ticker preview is unavailable")
	}
	return previewer.PreviewTicker(ctx, ticker)
}

func (s *Service) DeleteTicker(ticker string) error {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	s.quoteFinalizeMu.Lock()
	defer s.quoteFinalizeMu.Unlock()
	if err := s.store.Delete(ticker); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.quoteCache, ticker)
	delete(s.quotePersistedAt, ticker)
	delete(s.snapshotMetrics.observations, ticker)
	s.mu.Unlock()
	return nil
}

func (s *Service) Quote(ctx context.Context, ticker string) (model.LiveQuote, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if !tickerPattern.MatchString(ticker) {
		return model.LiveQuote{}, ErrInvalidTicker
	}
	now := time.Now().UTC()
	s.mu.Lock()
	if cached, ok := s.quoteCache[ticker]; ok && s.quoteTTL > 0 && now.Sub(cached.cachedAt) < s.quoteTTL {
		s.mu.Unlock()
		return cached.quote, nil
	}
	s.mu.Unlock()
	existing, err := s.store.Get(ticker)
	if err != nil {
		return model.LiveQuote{}, err
	}

	quoter, ok := s.analyzer.(QuoteAnalyzer)
	if !ok {
		return model.LiveQuote{}, ErrNoQuoteProvider
	}
	s.mu.Lock()
	// A prior request may have populated the cache while this caller cloned the
	// persisted equity. Recheck under the same lock used to register calls.
	now = time.Now().UTC()
	if cached, ok := s.quoteCache[ticker]; ok && s.quoteTTL > 0 && now.Sub(cached.cachedAt) < s.quoteTTL {
		s.mu.Unlock()
		return cached.quote, nil
	}
	if call, ok := s.quoteCalls[ticker]; ok {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return model.LiveQuote{}, ctx.Err()
		case <-call.done:
			return call.quote, call.err
		}
	}
	call := &quoteCall{done: make(chan struct{})}
	if s.shuttingDown {
		s.mu.Unlock()
		return model.LiveQuote{}, context.Canceled
	}
	s.quoteCalls[ticker] = call
	s.quoteWG.Add(1)
	s.mu.Unlock()

	go func() {
		defer s.quoteWG.Done()
		s.executeQuoteCall(ticker, call, quoter, existing)
	}()
	select {
	case <-ctx.Done():
		return model.LiveQuote{}, ctx.Err()
	case <-call.done:
		return call.quote, call.err
	}
}

func (s *Service) executeQuoteCall(ticker string, call *quoteCall, quoter QuoteAnalyzer, existing *model.Equity) {
	var quote model.LiveQuote
	var quoteErr error
	finalizeLocked := false
	defer func() {
		if recovered := recover(); recovered != nil {
			quote = model.LiveQuote{}
			quoteErr = fmt.Errorf("live quote provider panicked: %v", recovered)
		}
		if finalizeLocked {
			defer s.quoteFinalizeMu.Unlock()
		}
		s.finishQuoteCall(ticker, call, quote, quoteErr)
	}()
	// The shared request outlives any one HTTP caller. Each waiter still observes
	// its own context while the provider work is bounded independently.
	quoteCtx, cancel := context.WithTimeout(s.quoteLifecycleContext(), s.quoteRequestTimeout())
	defer cancel()
	quote, quoteErr = quoter.Quote(quoteCtx, ticker, existing)
	if quoteErr != nil {
		quoteErr = fmt.Errorf("%w: %w", ErrQuoteUpstream, quoteErr)
		return
	}
	observedAt, parseErr := time.Parse(time.RFC3339Nano, quote.AsOf)
	if parseErr != nil {
		quote = model.LiveQuote{}
		quoteErr = fmt.Errorf("%w: provider timestamp is not RFC3339", ErrQuoteUpstream)
		return
	}
	if observedAt.After(time.Now().UTC().Add(maximumQuoteFutureSkew)) {
		quote = model.LiveQuote{}
		quoteErr = fmt.Errorf("%w: provider timestamp exceeds allowed future skew", ErrQuoteUpstream)
		return
	}
	// Serialize the exact-share rebase, persistence, and cache publication with
	// fundamentals commits. A refresh that completed while provider work was in
	// flight therefore wins before this observation can become durable/current.
	s.quoteFinalizeMu.Lock()
	finalizeLocked = true
	latest, latestErr := s.store.Get(ticker)
	if latestErr != nil {
		quote = model.LiveQuote{}
		quoteErr = latestErr
		return
	}
	quote = rebaseQuoteToEquity(quote, latest)
	existing = latest
	if s.quotePersistenceDue(ticker, time.Now().UTC()) && quoteObservationNeedsPersistence(existing.QuoteHistory, quote) {
		history, err := s.store.RecordQuoteSnapshots(ticker, quote.History, model.NewStatisticSnapshot(quote))
		if err != nil {
			quote = model.LiveQuote{}
			quoteErr = fmt.Errorf("%w: persist live quote statistics: %w", ErrQuotePersistence, err)
			return
		}
		quote.History = history
		if !statisticSnapshotHistoriesEqual(existing.QuoteHistory, history) {
			s.markQuotePersisted(ticker, time.Now().UTC())
		}
	} else {
		quote.History = existing.QuoteHistory
	}
}

func (s *Service) quoteLifecycleContext() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lifecycleCtx
}

// Shutdown prevents new shared provider calls, cancels the service-owned child
// lifecycle, and waits for every registered worker and quote finalizer.
func (s *Service) BeginShutdown() {
	s.mu.Lock()
	s.shuttingDown = true
	s.macroRetryScheduled = false
	s.macroRetryGeneration++
	cancel := s.lifecycleCancel
	s.mu.Unlock()
	cancel()
}

func (s *Service) Shutdown(ctx context.Context) error {
	s.BeginShutdown()
	done := make(chan struct{})
	go func() {
		s.quoteWG.Wait()
		s.workerWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) quoteRequestTimeout() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.quoteTimeout
}

func quoteObservationNeedsPersistence(history []model.StatisticSnapshot, quote model.LiveQuote) bool {
	now := time.Now().UTC()
	maximumObservedAt := now.Add(maximumQuoteFutureSkew)
	if !quoteHasProviderObservationAt(quote, now) {
		return false
	}
	observedAt, _ := time.Parse(time.RFC3339Nano, quote.AsOf)
	candidate := model.NewStatisticSnapshot(quote)
	latestIndex := -1
	var latestAt time.Time
	for index := range history {
		priorAt, valid := statisticSnapshotObservationTime(history[index], maximumObservedAt)
		if !valid {
			continue
		}
		if latestIndex < 0 || priorAt.After(latestAt) || priorAt.Equal(latestAt) && index > latestIndex {
			latestIndex = index
			latestAt = priorAt
		}
	}
	if latestIndex < 0 || observedAt.After(latestAt) {
		return true
	}
	if observedAt.Before(latestAt) {
		return false
	}
	// Closing values and filing-enriched market values can be corrected without
	// changing the provider market timestamp.
	return !model.StatisticSnapshotContentEqual(history[latestIndex], candidate)
}

// statisticSnapshotObservationTime returns the ordering time for a persisted
// daily row. A sparse same-day merge keeps its conservative aggregate AsOf but
// carries the latest accepted provider time as an internal ordering watermark.
// Invalid watermarks are ignored exactly as the store's sanitizer ignores them.
func statisticSnapshotObservationTime(snapshot model.StatisticSnapshot, maximumObservedAt time.Time) (time.Time, bool) {
	aggregateAt, err := time.Parse(time.RFC3339Nano, snapshot.AsOf)
	if err != nil {
		return time.Time{}, false
	}
	aggregateAt = aggregateAt.UTC()
	if aggregateAt.After(maximumObservedAt) {
		return time.Time{}, false
	}
	if snapshot.LatestObservationAsOf == "" {
		return aggregateAt, true
	}
	latestAt, err := time.Parse(time.RFC3339Nano, snapshot.LatestObservationAsOf)
	if err != nil {
		return aggregateAt, true
	}
	latestAt = latestAt.UTC()
	if latestAt.Before(aggregateAt) || latestAt.After(maximumObservedAt) || latestAt.Format("2006-01-02") != aggregateAt.Format("2006-01-02") {
		return aggregateAt, true
	}
	return latestAt, true
}

func statisticSnapshotHistoriesEqual(left, right []model.StatisticSnapshot) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].AsOf != right[index].AsOf || !model.StatisticSnapshotContentEqual(left[index], right[index]) {
			return false
		}
	}
	return true
}

func quoteHasProviderObservation(quote model.LiveQuote) bool {
	return quoteHasProviderObservationAt(quote, time.Now().UTC())
}

func quoteHasProviderObservationAt(quote model.LiveQuote, now time.Time) bool {
	// A request-time fallback is useful for serving a current response but is
	// not evidence of a new provider market observation and must not create a
	// durable time-series point or scheduled-snapshot success.
	if strings.Contains(strings.ToLower(quote.FieldSources["asOf"]), "request time fallback") {
		return false
	}
	observedAt, err := time.Parse(time.RFC3339Nano, quote.AsOf)
	return err == nil && !observedAt.After(now.Add(maximumQuoteFutureSkew))
}

const quotePersistenceInterval = 15 * time.Minute

func (s *Service) quotePersistenceDue(ticker string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	lastPersisted, ok := s.quotePersistedAt[ticker]
	return !ok || now.Sub(lastPersisted) >= quotePersistenceInterval
}

func (s *Service) markQuotePersisted(ticker string, observed time.Time) {
	s.mu.Lock()
	s.quotePersistedAt[ticker] = observed
	s.mu.Unlock()
}

func (s *Service) finishQuoteCall(ticker string, call *quoteCall, quote model.LiveQuote, quoteErr error) {
	s.mu.Lock()
	if quoteErr == nil {
		s.quoteCache[ticker] = cachedQuote{quote: quote, cachedAt: time.Now().UTC()}
	}
	call.quote = quote
	call.err = quoteErr
	delete(s.quoteCalls, ticker)
	close(call.done)
	s.mu.Unlock()
}

func (s *Service) Queue(ticker string) bool {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	s.mu.Lock()
	if _, exists := s.inflight[ticker]; exists {
		s.mu.Unlock()
		return false
	}
	s.inflight[ticker] = struct{}{}
	s.mu.Unlock()

	select {
	case s.queue <- ticker:
		return true
	default:
		s.mu.Lock()
		delete(s.inflight, ticker)
		s.mu.Unlock()
		return false
	}
}

func (s *Service) RefreshAll() int {
	queued := 0
	for _, ticker := range s.store.Tickers() {
		if s.Queue(ticker) {
			queued++
		}
	}
	s.QueueMacro()
	return queued
}

func (s *Service) QueueMacro() bool {
	if s.macro == nil {
		return false
	}
	s.mu.Lock()
	if s.macroInflight {
		s.mu.Unlock()
		return false
	}
	s.macroInflight = true
	s.mu.Unlock()

	select {
	case s.macroQueue <- struct{}{}:
		return true
	default:
		s.mu.Lock()
		s.macroInflight = false
		s.mu.Unlock()
		return false
	}
}

func (s *Service) Snapshot() model.State {
	return s.store.Snapshot()
}

func (s *Service) Health() Health {
	tickerCount, updatedAt := s.store.Metadata()
	s.mu.Lock()
	shuttingDown := s.shuttingDown
	s.mu.Unlock()
	return Health{TickerCount: tickerCount, UpdatedAt: updatedAt, ShuttingDown: shuttingDown}
}

// Tickers returns the normalized watchlist without cloning the full state.
func (s *Service) Tickers() []string {
	return s.store.Tickers()
}

func (s *Service) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshotFailures := cloneFailureCounts(s.snapshotMetrics.failures)
	historyFailures := cloneFailureCounts(s.snapshotMetrics.historyRefreshFailures)
	benchmarkHistoryFailures := cloneFailureCounts(s.snapshotMetrics.benchmarkHistoryRefreshFailures)
	observations := make(map[string]SnapshotObservation, len(s.snapshotMetrics.observations))
	for ticker, observation := range s.snapshotMetrics.observations {
		observations[ticker] = observation
	}
	return Stats{
		RefreshTotal:     s.total.Load(),
		RefreshFailures:  s.failures.Load(),
		QueueDepth:       len(s.queue),
		InFlight:         len(s.inflight),
		LastRefresh:      s.last,
		MacroRefreshing:  s.macroInflight,
		MacroLastRefresh: s.macroLast,
		MacroFailures:    s.macroFailures.Load(),

		SnapshotSchedulerRunning:        s.snapshotMetrics.schedulerRunning,
		ScheduledSnapshotInFlight:       s.snapshotMetrics.inflight,
		ScheduledSnapshotAttempts:       s.snapshotMetrics.attempts,
		ScheduledSnapshotSuccesses:      s.snapshotMetrics.successes,
		ScheduledSnapshotNoNewSession:   s.snapshotMetrics.noNewSession,
		ScheduledSnapshotFailures:       snapshotFailures,
		HistoryRefreshFailures:          historyFailures,
		BenchmarkHistoryRefreshFailures: benchmarkHistoryFailures,
		ScheduledQuoteFieldsExpected:    expectedScheduledQuoteFieldCount,
		ScheduledSnapshotObservations:   observations,
	}
}

func (s *Service) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		select {
		case <-ctx.Done():
			return
		case ticker := <-s.queue:
			s.refresh(ctx, ticker)
		}
	}
}

func (s *Service) macroWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		select {
		case <-ctx.Done():
			return
		case <-s.macroQueue:
			s.refreshMacro(ctx)
		}
	}
}

func (s *Service) refreshMacro(parent context.Context) {
	failed := false
	defer func() {
		if recovered := recover(); recovered != nil {
			failed = true
			s.macroFailures.Add(1)
			_ = s.store.SetMacroError(fmt.Errorf("macro analysis failed unexpectedly: %v", recovered))
		}
		s.mu.Lock()
		s.macroInflight = false
		s.macroLast = time.Now().UTC()
		s.mu.Unlock()
		if failed {
			s.scheduleMacroRetry(parent)
		} else {
			s.cancelMacroRetry()
		}
	}()

	ctx, cancel := context.WithTimeout(parent, 8*time.Minute)
	defer cancel()
	var series model.MacroSeries
	var err error
	if incremental, ok := s.macro.(IncrementalMacroAnalyzer); ok {
		series, err = incremental.AnalyzeWithPrevious(ctx, s.store.Snapshot().Macro)
	} else {
		series, err = s.macro.Analyze(ctx)
	}
	if err != nil {
		failed = true
		s.macroFailures.Add(1)
		_ = s.store.SetMacroError(err)
		return
	}
	if err := s.store.SetMacro(series); err != nil {
		failed = true
		s.macroFailures.Add(1)
	}
}

func (s *Service) scheduleMacroRetry(parent context.Context) {
	s.mu.Lock()
	if s.shuttingDown || s.macroRetryScheduled {
		s.mu.Unlock()
		return
	}
	s.macroRetryScheduled = true
	s.macroRetryGeneration++
	generation := s.macroRetryGeneration
	s.mu.Unlock()

	go func() {
		timer := time.NewTimer(defaultMacroRetryDelay)
		defer timer.Stop()
		select {
		case <-parent.Done():
			return
		case <-timer.C:
		}
		s.mu.Lock()
		if s.shuttingDown || !s.macroRetryScheduled || s.macroRetryGeneration != generation {
			s.mu.Unlock()
			return
		}
		s.macroRetryScheduled = false
		s.mu.Unlock()
		s.QueueMacro()
	}()
}

func (s *Service) cancelMacroRetry() {
	s.mu.Lock()
	s.macroRetryScheduled = false
	s.macroRetryGeneration++
	s.mu.Unlock()
}

func (s *Service) refresh(parent context.Context, ticker string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			s.total.Add(1)
			s.failures.Add(1)
			_ = s.store.SetError(ticker, fmt.Errorf("analysis failed unexpectedly: %v", recovered))
		}
		s.mu.Lock()
		delete(s.inflight, ticker)
		s.last = time.Now().UTC()
		s.mu.Unlock()
	}()

	if err := s.store.SetRefreshing(ticker); err != nil {
		s.total.Add(1)
		s.failures.Add(1)
		return
	}
	existing, err := s.store.Get(ticker)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()

	result, err := s.analyzer.Analyze(ctx, ticker, existing)
	s.total.Add(1)
	if err != nil {
		s.failures.Add(1)
		_ = s.store.SetError(ticker, err)
		return
	}
	s.quoteFinalizeMu.Lock()
	defer s.quoteFinalizeMu.Unlock()
	if err := s.store.SetResult(ticker, result); err != nil {
		s.failures.Add(1)
		return
	}
	s.mu.Lock()
	delete(s.quoteCache, ticker)
	s.mu.Unlock()
}
