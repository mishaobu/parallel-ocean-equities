package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
		quoteHistory := equity.QuoteHistory
		result.Ticker = strings.ToUpper(ticker)
		result.Status = "ready"
		result.Error = ""
		result.UpdatedAt = time.Now().UTC()
		*equity = *result
		equity.QuoteHistory = quoteHistory
	})
}

const (
	quoteHistoryYears = 10
	quoteHistoryLimit = 4000
)

// RecordQuoteSnapshot persists at most one quote-derived statistics snapshot
// per ticker and UTC day. A later observation replaces the same day's prior
// point; history is ordered oldest-first and bounded to ten years / 4,000 rows.
func (s *Store) RecordQuoteSnapshot(ticker string, snapshot model.StatisticSnapshot) ([]model.StatisticSnapshot, error) {
	return s.RecordQuoteSnapshots(ticker, nil, snapshot)
}

// RecordQuoteSnapshots seeds missing calendar months from a bounded historical
// backfill and records the current observation in one locked disk save. Any
// month already represented in persisted history is authoritative and is not
// replaced by a historical seed. The current point still follows the normal
// same-UTC-day rule: a later asOf replaces an existing observation, while an
// equal-timestamp provider correction replaces only when its payload changed.
func (s *Store) RecordQuoteSnapshots(ticker string, monthlyBackfill []model.StatisticSnapshot, current model.StatisticSnapshot) ([]model.StatisticSnapshot, error) {
	current, currentAt, err := validatedStatisticSnapshot(current)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		snapshot model.StatisticSnapshot
		observed time.Time
	}
	backfillByMonth := make(map[string]candidate, len(monthlyBackfill))
	for _, snapshot := range monthlyBackfill {
		normalized, observed, validateErr := validatedStatisticSnapshot(snapshot)
		if validateErr != nil {
			return nil, fmt.Errorf("quote history backfill: %w", validateErr)
		}
		month := observed.Format("2006-01")
		if prior, exists := backfillByMonth[month]; !exists || observed.After(prior.observed) {
			backfillByMonth[month] = candidate{snapshot: normalized, observed: observed}
		}
	}
	backfill := make([]candidate, 0, len(backfillByMonth))
	for _, row := range backfillByMonth {
		backfill = append(backfill, row)
	}
	sort.Slice(backfill, func(i, j int) bool { return backfill[i].observed.Before(backfill[j].observed) })

	s.mu.Lock()
	defer s.mu.Unlock()
	equity, exists := s.state.Tickers[strings.ToUpper(strings.TrimSpace(ticker))]
	if !exists {
		return nil, ErrNotFound
	}
	history := cloneStatisticSnapshots(equity.QuoteHistory)
	occupiedMonths := make(map[string]bool, len(history)+1)
	for _, snapshot := range history {
		observed, parseErr := time.Parse(time.RFC3339Nano, snapshot.AsOf)
		if parseErr == nil {
			occupiedMonths[observed.UTC().Format("2006-01")] = true
		}
	}
	// Never seed the current month: its separately derived current snapshot may
	// carry exact SEC-share-based market value fields that backfill must not.
	occupiedMonths[currentAt.Format("2006-01")] = true
	changed := false
	for _, row := range backfill {
		if !row.observed.Before(currentAt) {
			continue
		}
		month := row.observed.Format("2006-01")
		if occupiedMonths[month] {
			continue
		}
		var accepted bool
		history, accepted = mergeQuoteHistory(history, row.snapshot, row.observed)
		if accepted {
			changed = true
			occupiedMonths[month] = true
		}
	}
	var currentAccepted bool
	history, currentAccepted = mergeQuoteHistory(history, current, currentAt)
	changed = changed || currentAccepted
	if !changed {
		return cloneStatisticSnapshots(equity.QuoteHistory), nil
	}
	previousHistory := equity.QuoteHistory
	previousVersion := s.state.Version
	previousUpdatedAt := s.state.UpdatedAt
	equity.QuoteHistory = history
	if err := s.saveLocked(); err != nil {
		equity.QuoteHistory = previousHistory
		s.state.Version = previousVersion
		s.state.UpdatedAt = previousUpdatedAt
		return nil, err
	}
	return cloneStatisticSnapshots(history), nil
}

func validatedStatisticSnapshot(snapshot model.StatisticSnapshot) (model.StatisticSnapshot, time.Time, error) {
	observedAt, err := time.Parse(time.RFC3339Nano, snapshot.AsOf)
	if err != nil {
		return model.StatisticSnapshot{}, time.Time{}, fmt.Errorf("quote snapshot asOf must be RFC3339: %w", err)
	}
	if len(snapshot.Numeric) == 0 && len(snapshot.Text) == 0 {
		return model.StatisticSnapshot{}, time.Time{}, errors.New("quote snapshot contains no statistic values")
	}
	for key, value := range snapshot.Numeric {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return model.StatisticSnapshot{}, time.Time{}, fmt.Errorf("quote snapshot numeric value %q must be finite", key)
		}
	}
	observedAt = observedAt.UTC()
	snapshot.AsOf = observedAt.Format(time.RFC3339Nano)
	return cloneStatisticSnapshot(snapshot), observedAt, nil
}

type datedStatisticSnapshot struct {
	snapshot model.StatisticSnapshot
	observed time.Time
}

func mergeQuoteHistory(history []model.StatisticSnapshot, incoming model.StatisticSnapshot, incomingAt time.Time) ([]model.StatisticSnapshot, bool) {
	byDay := make(map[string]datedStatisticSnapshot, len(history)+1)
	for _, snapshot := range history {
		observed, err := time.Parse(time.RFC3339Nano, snapshot.AsOf)
		if err != nil {
			continue
		}
		observed = observed.UTC()
		day := observed.Format("2006-01-02")
		current, exists := byDay[day]
		if !exists || observed.After(current.observed) {
			snapshot.AsOf = observed.Format(time.RFC3339Nano)
			byDay[day] = datedStatisticSnapshot{snapshot: cloneStatisticSnapshot(snapshot), observed: observed}
		}
	}

	incomingDay := incomingAt.Format("2006-01-02")
	if current, exists := byDay[incomingDay]; exists {
		if incomingAt.Before(current.observed) {
			return cloneStatisticSnapshots(history), false
		}
		incoming = mergeSameDayStatisticSnapshot(current.snapshot, incoming, incomingAt)
		if incomingAt.Equal(current.observed) && model.StatisticSnapshotContentEqual(current.snapshot, incoming) {
			return cloneStatisticSnapshots(history), false
		}
	}
	byDay[incomingDay] = datedStatisticSnapshot{snapshot: incoming, observed: incomingAt}

	latest := incomingAt
	for _, row := range byDay {
		if row.observed.After(latest) {
			latest = row.observed
		}
	}
	cutoff := latest.AddDate(-quoteHistoryYears, 0, 0)
	rows := make([]datedStatisticSnapshot, 0, len(byDay))
	for _, row := range byDay {
		if !row.observed.Before(cutoff) {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].observed.Before(rows[j].observed) })
	if len(rows) > quoteHistoryLimit {
		rows = rows[len(rows)-quoteHistoryLimit:]
	}
	out := make([]model.StatisticSnapshot, len(rows))
	for index, row := range rows {
		out[index] = cloneStatisticSnapshot(row.snapshot)
	}
	return out, true
}

// mergeSameDayStatisticSnapshot keeps a daily observation monotonic when a
// later provider response is degraded or sparse. Fields present in the newer
// response are authoritative corrections; absent fields retain the richest
// value already persisted for that same UTC day. Provenance is replaced with
// each corrected value and removed when the correction supplies none.
func mergeSameDayStatisticSnapshot(current, incoming model.StatisticSnapshot, incomingAt time.Time) model.StatisticSnapshot {
	merged := cloneStatisticSnapshot(current)
	merged.AsOf = incomingAt.UTC().Format(time.RFC3339Nano)
	// AsOf always comes from the incoming observation, so its provenance must
	// move with it (or be cleared) rather than describe the retained timestamp.
	merged.AsOfSource = incoming.AsOfSource
	if incoming.Source != "" {
		merged.Source = incoming.Source
	}

	for key, value := range incoming.Numeric {
		if merged.Numeric == nil {
			merged.Numeric = make(map[string]float64)
		}
		merged.Numeric[key] = value
		delete(merged.Text, key)
		mergeStatisticSource(&merged, incoming, key)
	}
	for key, value := range incoming.Text {
		if merged.Text == nil {
			merged.Text = make(map[string]string)
		}
		merged.Text[key] = value
		delete(merged.Numeric, key)
		mergeStatisticSource(&merged, incoming, key)
	}
	return merged
}

func mergeStatisticSource(merged *model.StatisticSnapshot, incoming model.StatisticSnapshot, key string) {
	if source := incoming.Sources[key]; source != "" {
		if merged.Sources == nil {
			merged.Sources = make(map[string]string)
		}
		merged.Sources[key] = source
		return
	}
	delete(merged.Sources, key)
}

func cloneStatisticSnapshots(history []model.StatisticSnapshot) []model.StatisticSnapshot {
	if history == nil {
		return nil
	}
	clone := make([]model.StatisticSnapshot, len(history))
	for index, snapshot := range history {
		clone[index] = cloneStatisticSnapshot(snapshot)
	}
	return clone
}

func cloneStatisticSnapshot(snapshot model.StatisticSnapshot) model.StatisticSnapshot {
	snapshot.Numeric = cloneFloatMap(snapshot.Numeric)
	snapshot.Text = cloneStringMap(snapshot.Text)
	snapshot.Sources = cloneStringMap(snapshot.Sources)
	return snapshot
}

func cloneFloatMap(source map[string]float64) map[string]float64 {
	if source == nil {
		return nil
	}
	clone := make(map[string]float64, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func (s *Store) SetError(ticker string, refreshErr error) error {
	return s.update(ticker, func(equity *model.Equity) {
		equity.Status = "error"
		equity.Error = refreshErr.Error()
		equity.UpdatedAt = time.Now().UTC()
	})
}

func (s *Store) SetMacro(series model.MacroSeries) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(series.Vintages.Points) == 0 && len(s.state.Macro.Vintages.Points) > 0 {
		series.Vintages = s.state.Macro.Vintages
	}
	if len(series.Options.Snapshots) == 0 && len(s.state.Macro.Options.Snapshots) > 0 {
		series.Options = s.state.Macro.Options
	}
	series.Error = ""
	if series.UpdatedAt.IsZero() {
		series.UpdatedAt = time.Now().UTC()
	}
	s.state.Macro = series
	return s.saveLocked()
}

func (s *Store) SetMacroError(refreshErr error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Macro.Error = refreshErr.Error()
	s.state.Macro.UpdatedAt = time.Now().UTC()
	return s.saveLocked()
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
