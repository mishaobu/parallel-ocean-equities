package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

var (
	ErrNotFound = errors.New("ticker not found")
	ErrLimit    = errors.New("watchlist limit reached")
)

type Store struct {
	mu         sync.RWMutex
	path       string
	maxTickers int
	state      model.State
}

func Open(path, seedPath string, maxTickers int) (*Store, error) {
	if maxTickers < 1 {
		return nil, errors.New("max tickers must be positive")
	}
	s := &Store{path: path, maxTickers: maxTickers, state: model.NewState()}
	if err := s.load(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load state: %w", err)
		}
		if err := s.load(seedPath); err != nil {
			return nil, fmt.Errorf("load seed: %w", err)
		}
		if err := s.saveLocked(); err != nil {
			return nil, fmt.Errorf("persist initial state: %w", err)
		}
	}
	return s, nil
}

func (s *Store) load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var state model.State
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	if state.Tickers == nil {
		state.Tickers = make(map[string]*model.Equity)
	}
	state.Version = model.StateVersion
	for ticker, equity := range state.Tickers {
		canonical := strings.ToUpper(strings.TrimSpace(ticker))
		equity.Ticker = canonical
		if equity.Status == "" {
			equity.Status = "ready"
		}
		if canonical != ticker {
			delete(state.Tickers, ticker)
			state.Tickers[canonical] = equity
		}
	}
	s.state = state
	return nil
}

func (s *Store) Snapshot() model.State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, _ := json.Marshal(s.state)
	var clone model.State
	_ = json.Unmarshal(data, &clone)
	return clone
}

func (s *Store) Get(ticker string) (*model.Equity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	equity, ok := s.state.Tickers[strings.ToUpper(ticker)]
	if !ok {
		return nil, ErrNotFound
	}
	data, _ := json.Marshal(equity)
	var clone model.Equity
	_ = json.Unmarshal(data, &clone)
	return &clone, nil
}

func (s *Store) Add(ticker string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ticker = strings.ToUpper(ticker)
	if _, exists := s.state.Tickers[ticker]; exists {
		return nil
	}
	if len(s.state.Tickers) >= s.maxTickers {
		return ErrLimit
	}
	s.state.Tickers[ticker] = &model.Equity{Ticker: ticker, Status: "queued", Annuals: []model.AnnualPoint{}}
	return s.saveLocked()
}

func (s *Store) Delete(ticker string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ticker = strings.ToUpper(ticker)
	if _, exists := s.state.Tickers[ticker]; !exists {
		return ErrNotFound
	}
	if len(s.state.Tickers) == 1 {
		return errors.New("watchlist must contain at least one ticker")
	}
	delete(s.state.Tickers, ticker)
	return s.saveLocked()
}

func (s *Store) SetRefreshing(ticker string) error {
	return s.update(ticker, func(equity *model.Equity) {
		equity.Status = "refreshing"
		equity.Error = ""
	})
}

func (s *Store) SetResult(ticker string, result *model.Equity) error {
	return s.update(ticker, func(equity *model.Equity) {
		result.Ticker = strings.ToUpper(ticker)
		result.Status = "ready"
		result.Error = ""
		result.UpdatedAt = time.Now().UTC()
		*equity = *result
	})
}

func (s *Store) SetError(ticker string, refreshErr error) error {
	return s.update(ticker, func(equity *model.Equity) {
		equity.Status = "error"
		equity.Error = refreshErr.Error()
		equity.UpdatedAt = time.Now().UTC()
	})
}

func (s *Store) Tickers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tickers := make([]string, 0, len(s.state.Tickers))
	for ticker := range s.state.Tickers {
		tickers = append(tickers, ticker)
	}
	sort.Strings(tickers)
	return tickers
}

func (s *Store) update(ticker string, mutate func(*model.Equity)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	equity, exists := s.state.Tickers[strings.ToUpper(ticker)]
	if !exists {
		return ErrNotFound
	}
	mutate(equity)
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	s.state.Version = model.StateVersion
	s.state.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
