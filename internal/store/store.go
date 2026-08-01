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
	return cloneState(s.state)
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
	next := cloneState(s.state)
	next.Tickers[ticker] = &model.Equity{Ticker: ticker, Status: "queued", Annuals: []model.AnnualPoint{}}
	return s.commitLocked(next)
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
	next := cloneState(s.state)
	delete(next.Tickers, ticker)
	return s.commitLocked(next)
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
	// Exchange timestamps slightly ahead of the application clock are tolerated,
	// but a poisoned far-future row must never become the retention anchor.
	quoteHistoryFutureSkew = 5 * time.Minute
)

// RecordQuoteSnapshot persists at most one quote-derived statistics snapshot
// per ticker and UTC day. Newer same-day fields correct the prior point without
// discarding fields absent from a sparse response; history is ordered
// oldest-first and bounded to ten years / 4,000 rows.
func (s *Store) RecordQuoteSnapshot(ticker string, snapshot model.StatisticSnapshot) ([]model.StatisticSnapshot, error) {
	return s.RecordQuoteSnapshots(ticker, nil, snapshot)
}

// RecordQuoteSnapshots seeds missing calendar months from a bounded historical
// backfill and records the current observation in one locked disk save. A month
// already represented in persisted history remains authoritative: backfill may
// only add fields and matching provenance that its latest persisted observation
// does not have, without changing any value, source, or timestamp already on
// disk. The current point still follows the normal same-UTC-day rule: newer
// fields correct the existing observation, while aggregate time only advances
// when the incoming payload covers every prior field. An equal-timestamp
// provider correction replaces only when its payload changed.
func (s *Store) RecordQuoteSnapshots(ticker string, monthlyBackfill []model.StatisticSnapshot, current model.StatisticSnapshot) ([]model.StatisticSnapshot, error) {
	current, currentAt, err := validatedStatisticSnapshot(current)
	if err != nil {
		return nil, err
	}
	if currentAt.After(time.Now().UTC().Add(quoteHistoryFutureSkew)) {
		return nil, errors.New("quote snapshot asOf exceeds allowed future skew")
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
	// Never seed the current month: its separately derived current snapshot may
	// carry exact SEC-share-based market value fields that backfill must not.
	currentMonth := currentAt.Format("2006-01")
	changed := false
	for _, row := range backfill {
		if !row.observed.Before(currentAt) {
			continue
		}
		month := row.observed.Format("2006-01")
		if month == currentMonth {
			continue
		}
		if index, occupied := latestSnapshotIndexInMonth(history, month); occupied {
			var enriched bool
			history[index], enriched = enrichStatisticSnapshotMissing(history[index], row.snapshot)
			changed = changed || enriched
			continue
		}
		var accepted bool
		history, accepted = mergeQuoteHistory(history, row.snapshot, row.observed)
		if accepted {
			changed = true
		}
	}
	var currentAccepted bool
	history, currentAccepted = mergeQuoteHistory(history, current, currentAt)
	changed = changed || currentAccepted
	if !changed {
		return cloneStatisticSnapshots(equity.QuoteHistory), nil
	}
	next := cloneState(s.state)
	next.Tickers[strings.ToUpper(strings.TrimSpace(ticker))].QuoteHistory = history
	if err := s.commitLocked(next); err != nil {
		return nil, err
	}
	return cloneStatisticSnapshots(history), nil
}

func latestSnapshotIndexInMonth(history []model.StatisticSnapshot, month string) (int, bool) {
	latestIndex := -1
	var latest time.Time
	for index, snapshot := range history {
		observed, err := time.Parse(time.RFC3339Nano, snapshot.AsOf)
		if err != nil {
			continue
		}
		observed = observed.UTC()
		if observed.Format("2006-01") != month || (latestIndex >= 0 && !observed.After(latest)) {
			continue
		}
		latestIndex = index
		latest = observed
	}
	return latestIndex, latestIndex >= 0
}

// enrichStatisticSnapshotMissing adds only absent backfill fields to an
// authoritative persisted observation. Existing values and provenance always
// win. Missing provenance for an existing field is filled only when the
// backfill carries the exact same value, avoiding false attribution.
func enrichStatisticSnapshotMissing(current, backfill model.StatisticSnapshot) (model.StatisticSnapshot, bool) {
	merged := cloneStatisticSnapshot(current)
	changed := false
	for key, value := range backfill.Numeric {
		existing, numericExists := merged.Numeric[key]
		_, textExists := merged.Text[key]
		if !numericExists && !textExists {
			if merged.Numeric == nil {
				merged.Numeric = make(map[string]float64)
			}
			merged.Numeric[key] = value
			changed = true
		}
		if source := backfill.Sources[key]; source != "" {
			if _, sourced := merged.Sources[key]; !sourced && !textExists && (!numericExists || existing == value) {
				if merged.Sources == nil {
					merged.Sources = make(map[string]string)
				}
				merged.Sources[key] = source
				changed = true
			}
		}
	}
	for key, value := range backfill.Text {
		existing, textExists := merged.Text[key]
		_, numericExists := merged.Numeric[key]
		if !textExists && !numericExists {
			if merged.Text == nil {
				merged.Text = make(map[string]string)
			}
			merged.Text[key] = value
			changed = true
		}
		if source := backfill.Sources[key]; source != "" {
			if _, sourced := merged.Sources[key]; !sourced && !numericExists && (!textExists || existing == value) {
				if merged.Sources == nil {
					merged.Sources = make(map[string]string)
				}
				merged.Sources[key] = source
				changed = true
			}
		}
	}
	return merged, changed
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
	// Merge ordering is store-owned metadata, never provider input.
	snapshot.LatestObservationAsOf = ""
	return cloneStatisticSnapshot(snapshot), observedAt, nil
}

type datedStatisticSnapshot struct {
	snapshot model.StatisticSnapshot
	observed time.Time
}

func mergeQuoteHistory(history []model.StatisticSnapshot, incoming model.StatisticSnapshot, incomingAt time.Time) ([]model.StatisticSnapshot, bool) {
	byDay := make(map[string]datedStatisticSnapshot, len(history)+1)
	sanitized := false
	maximumObservedAt := time.Now().UTC().Add(quoteHistoryFutureSkew)
	for _, snapshot := range history {
		aggregateAt, err := time.Parse(time.RFC3339Nano, snapshot.AsOf)
		if err != nil || aggregateAt.After(maximumObservedAt) {
			sanitized = true
			continue
		}
		aggregateAt = aggregateAt.UTC()
		observed := aggregateAt
		day := aggregateAt.Format("2006-01-02")
		if snapshot.LatestObservationAsOf != "" {
			latestObservation, latestErr := time.Parse(time.RFC3339Nano, snapshot.LatestObservationAsOf)
			latestObservation = latestObservation.UTC()
			if latestErr != nil || latestObservation.Before(observed) || latestObservation.After(maximumObservedAt) || latestObservation.Format("2006-01-02") != day {
				snapshot.LatestObservationAsOf = ""
				sanitized = true
			} else {
				snapshot.LatestObservationAsOf = latestObservation.Format(time.RFC3339Nano)
				observed = latestObservation
			}
		}
		current, exists := byDay[day]
		if !exists || observed.After(current.observed) {
			snapshot.AsOf = aggregateAt.Format(time.RFC3339Nano)
			byDay[day] = datedStatisticSnapshot{snapshot: cloneStatisticSnapshot(snapshot), observed: observed}
		}
	}

	incomingDay := incomingAt.Format("2006-01-02")
	acceptIncoming := true
	if current, exists := byDay[incomingDay]; exists {
		if incomingAt.Before(current.observed) {
			acceptIncoming = false
		} else {
			incoming = mergeSameDayStatisticSnapshot(current.snapshot, incoming, incomingAt)
			if incoming.AsOf == current.snapshot.AsOf && model.StatisticSnapshotContentEqual(current.snapshot, incoming) {
				acceptIncoming = false
			}
		}
	}
	if acceptIncoming {
		byDay[incomingDay] = datedStatisticSnapshot{snapshot: incoming, observed: incomingAt}
	}
	if !acceptIncoming && !sanitized {
		return cloneStatisticSnapshots(history), false
	}

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
	return out, acceptIncoming || sanitized
}

// mergeSameDayStatisticSnapshot keeps a daily observation monotonic when a
// later provider response is degraded or sparse. Fields present in the newer
// response are authoritative corrections; absent fields retain the richest
// value already persisted for that same UTC day. A mixed-age row keeps the
// prior aggregate timestamp and its provenance so retained fields never appear
// newer than they are. Mixed aggregate source is cleared; corrected fields
// continue to carry their incoming per-field source.
func mergeSameDayStatisticSnapshot(current, incoming model.StatisticSnapshot, incomingAt time.Time) model.StatisticSnapshot {
	merged := cloneStatisticSnapshot(current)
	if retainsPriorStatisticFields(current, incoming) {
		merged.Source = ""
		if latest := incomingAt.UTC().Format(time.RFC3339Nano); latest != merged.AsOf {
			merged.LatestObservationAsOf = latest
		} else {
			merged.LatestObservationAsOf = ""
		}
	} else {
		merged.AsOf = incomingAt.UTC().Format(time.RFC3339Nano)
		merged.AsOfSource = incoming.AsOfSource
		merged.Source = incoming.Source
		merged.LatestObservationAsOf = ""
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

func retainsPriorStatisticFields(current, incoming model.StatisticSnapshot) bool {
	for key := range current.Numeric {
		if _, numeric := incoming.Numeric[key]; numeric {
			continue
		}
		if _, text := incoming.Text[key]; !text {
			return true
		}
	}
	for key := range current.Text {
		if _, text := incoming.Text[key]; text {
			continue
		}
		if _, numeric := incoming.Numeric[key]; !numeric {
			return true
		}
	}
	return false
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
	now := time.Now().UTC()
	next := cloneState(s.state)
	if len(series.Vintages.Points) == 0 && len(next.Macro.Vintages.Points) > 0 {
		series.Vintages = next.Macro.Vintages
	}
	if len(series.Options.Snapshots) == 0 && len(next.Macro.Options.Snapshots) > 0 {
		series.Options = next.Macro.Options
	}
	series.Error = ""
	if series.UpdatedAt.IsZero() {
		series.UpdatedAt = now
	}
	series.LastAttemptAt = now
	series.LastSuccessAt = series.UpdatedAt
	next.Macro = series
	return s.commitLocked(next)
}

func (s *Store) SetMacroError(refreshErr error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneState(s.state)
	next.Macro.Error = refreshErr.Error()
	next.Macro.LastAttemptAt = time.Now().UTC()
	return s.commitLocked(next)
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

// Metadata returns the readiness-relevant state without serializing and
// cloning the full filing and market-history payload.
func (s *Store) Metadata() (int, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.state.Tickers), s.state.UpdatedAt
}

func (s *Store) update(ticker string, mutate func(*model.Equity)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneState(s.state)
	equity, exists := next.Tickers[strings.ToUpper(ticker)]
	if !exists {
		return ErrNotFound
	}
	mutate(equity)
	return s.commitLocked(next)
}

func (s *Store) saveLocked() error {
	return s.commitLocked(cloneState(s.state))
}

// commitLocked uses copy-on-write publication: callers build a complete next
// state, it is durably replaced on disk, and only then becomes visible to
// readers. A failed write therefore cannot leave memory ahead of the restart
// source of truth.
func (s *Store) commitLocked(next model.State) error {
	next.Version = model.StateVersion
	next.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o644); err != nil {
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return err
	}
	committed = true
	var durabilityErr error
	if directory, openErr := os.Open(dir); openErr == nil {
		syncErr := directory.Sync()
		closeErr := directory.Close()
		if syncErr != nil {
			durabilityErr = syncErr
		} else if closeErr != nil {
			durabilityErr = closeErr
		}
	} else {
		durabilityErr = openErr
	}
	// Rename has already published the new file. Keep in-memory state aligned
	// even when the final directory durability barrier reports an error.
	s.state = next
	return durabilityErr
}

func cloneState(state model.State) model.State {
	data, _ := json.Marshal(state)
	var clone model.State
	_ = json.Unmarshal(data, &clone)
	if clone.Tickers == nil {
		clone.Tickers = make(map[string]*model.Equity)
	}
	return clone
}
