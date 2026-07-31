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
	store            *store.Store
	analyzer         Analyzer
	queue            chan string
	macro            MacroAnalyzer
	macroQueue       chan struct{}
	quoteTTL         time.Duration
	quoteCache       map[string]cachedQuote
	quoteCalls       map[string]*quoteCall
	quotePersistedAt map[string]time.Time

	mu            sync.Mutex
	inflight      map[string]struct{}
	macroInflight bool
	last          time.Time
	macroLast     time.Time
	total         atomic.Int64
	failures      atomic.Int64
	macroFailures atomic.Int64
}

func NewService(state *store.Store, analyzer Analyzer) *Service {
	return &Service{
		store:            state,
		analyzer:         analyzer,
		queue:            make(chan string, 64),
		macroQueue:       make(chan struct{}, 1),
		quoteTTL:         time.Minute,
		quoteCache:       make(map[string]cachedQuote),
		quoteCalls:       make(map[string]*quoteCall),
		quotePersistedAt: make(map[string]time.Time),
		inflight:         make(map[string]struct{}),
	}
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

func (s *Service) Start(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	for range workers {
		go s.worker(ctx)
	}
	if s.macro != nil {
		go s.macroWorker(ctx)
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
	if err := s.store.Delete(ticker); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.quoteCache, ticker)
	delete(s.quotePersistedAt, ticker)
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
	s.quoteCalls[ticker] = call
	s.mu.Unlock()

	go s.executeQuoteCall(ticker, call, quoter, existing)
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
	defer func() {
		if recovered := recover(); recovered != nil {
			quote = model.LiveQuote{}
			quoteErr = fmt.Errorf("live quote provider panicked: %v", recovered)
		}
		s.finishQuoteCall(ticker, call, quote, quoteErr)
	}()
	// The shared request outlives any one HTTP caller. Each waiter still observes
	// its own context while the provider work is bounded independently.
	quoteCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	quote, quoteErr = quoter.Quote(quoteCtx, ticker, existing)
	if quoteErr != nil {
		return
	}
	if s.quotePersistenceDue(ticker, time.Now().UTC()) {
		history, err := s.store.RecordQuoteSnapshots(ticker, quote.History, model.NewStatisticSnapshot(quote))
		if err != nil {
			quote = model.LiveQuote{}
			quoteErr = fmt.Errorf("persist live quote statistics: %w", err)
			return
		}
		quote.History = history
		s.markQuotePersisted(ticker, time.Now().UTC())
	} else {
		quote.History = existing.QuoteHistory
	}
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

func (s *Service) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Stats{
		RefreshTotal:     s.total.Load(),
		RefreshFailures:  s.failures.Load(),
		QueueDepth:       len(s.queue),
		InFlight:         len(s.inflight),
		LastRefresh:      s.last,
		MacroRefreshing:  s.macroInflight,
		MacroLastRefresh: s.macroLast,
		MacroFailures:    s.macroFailures.Load(),
	}
}

func (s *Service) worker(ctx context.Context) {
	for {
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
		case <-s.macroQueue:
			s.refreshMacro(ctx)
		}
	}
}

func (s *Service) refreshMacro(parent context.Context) {
	defer func() {
		if recovered := recover(); recovered != nil {
			s.macroFailures.Add(1)
			_ = s.store.SetMacroError(fmt.Errorf("macro analysis failed unexpectedly: %v", recovered))
		}
		s.mu.Lock()
		s.macroInflight = false
		s.macroLast = time.Now().UTC()
		s.mu.Unlock()
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
		s.macroFailures.Add(1)
		_ = s.store.SetMacroError(err)
		return
	}
	if err := s.store.SetMacro(series); err != nil {
		s.macroFailures.Add(1)
	}
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

	_ = s.store.SetRefreshing(ticker)
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
	if err := s.store.SetResult(ticker, result); err != nil {
		s.failures.Add(1)
		return
	}
	s.mu.Lock()
	delete(s.quoteCache, ticker)
	s.mu.Unlock()
}
