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

var tickerPattern = regexp.MustCompile(`^[A-Z][A-Z0-9.-]{0,9}$`)

type Analyzer interface {
	Analyze(context.Context, string, *model.Equity) (*model.Equity, error)
}

type Stats struct {
	RefreshTotal    int64     `json:"refreshTotal"`
	RefreshFailures int64     `json:"refreshFailures"`
	QueueDepth      int       `json:"queueDepth"`
	InFlight        int       `json:"inFlight"`
	LastRefresh     time.Time `json:"lastRefresh,omitempty"`
}

type Service struct {
	store    *store.Store
	analyzer Analyzer
	queue    chan string

	mu       sync.Mutex
	inflight map[string]struct{}
	last     time.Time
	total    atomic.Int64
	failures atomic.Int64
}

func NewService(state *store.Store, analyzer Analyzer) *Service {
	return &Service{
		store:    state,
		analyzer: analyzer,
		queue:    make(chan string, 64),
		inflight: make(map[string]struct{}),
	}
}

func (s *Service) Start(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	for range workers {
		go s.worker(ctx)
	}
}

func (s *Service) AddTicker(ticker string) error {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if !tickerPattern.MatchString(ticker) {
		return errors.New("ticker must be 1-10 letters, numbers, dots, or hyphens")
	}
	if err := s.store.Add(ticker); err != nil {
		return err
	}
	if !s.Queue(ticker) {
		return errors.New("ticker refresh is already queued")
	}
	return nil
}

func (s *Service) DeleteTicker(ticker string) error {
	return s.store.Delete(strings.ToUpper(strings.TrimSpace(ticker)))
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
	return queued
}

func (s *Service) Snapshot() model.State {
	return s.store.Snapshot()
}

func (s *Service) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Stats{
		RefreshTotal:    s.total.Load(),
		RefreshFailures: s.failures.Load(),
		QueueDepth:      len(s.queue),
		InFlight:        len(s.inflight),
		LastRefresh:     s.last,
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
	}
}
