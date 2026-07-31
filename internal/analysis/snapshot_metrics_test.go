package analysis

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
	"github.com/mishaobu/parallel-ocean-equities/internal/store"
)

type snapshotMetricsAnalyzer struct {
	mu    sync.Mutex
	quote model.LiveQuote
	err   error
}

func (a *snapshotMetricsAnalyzer) Analyze(_ context.Context, _ string, existing *model.Equity) (*model.Equity, error) {
	return existing, nil
}

func (a *snapshotMetricsAnalyzer) Quote(_ context.Context, _ string, _ *model.Equity) (model.LiveQuote, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.quote, a.err
}

func (a *snapshotMetricsAnalyzer) setError(err error) {
	a.mu.Lock()
	a.err = err
	a.mu.Unlock()
}

func TestScheduledQuoteRecordsCoverageFailuresAndNoNewSession(t *testing.T) {
	state, err := store.Open(filepath.Join(t.TempDir(), "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	observedAt := time.Date(2026, time.July, 31, 15, 30, 0, 0, time.UTC)
	cachedAt := observedAt.Add(-13 * time.Hour)
	analyzer := &snapshotMetricsAnalyzer{quote: completeScheduledQuote(observedAt, cachedAt)}
	service := NewService(state, analyzer).WithQuoteTTL(0)
	service.SetSnapshotSchedulerRunning(true)

	if _, err := service.ScheduledQuote(context.Background(), "amzn"); err != nil {
		t.Fatal(err)
	}
	first := service.Stats()
	observation := first.ScheduledSnapshotObservations["AMZN"]
	if !first.SnapshotSchedulerRunning || first.ScheduledSnapshotAttempts != 1 || first.ScheduledSnapshotSuccesses != 1 || first.ScheduledSnapshotNoNewSession != 0 || first.ScheduledSnapshotInFlight != 0 {
		t.Fatalf("unexpected first counters: %+v", first)
	}
	if observation.MarketState != "regular" || observation.QuoteFieldsPresent != expectedScheduledQuoteFieldCount || !observation.LastObservation.Equal(observedAt) || !observation.HistoryCacheAsOf.Equal(cachedAt) || observation.HistoryCacheStatus != "stale" || observation.BenchmarkHistoryCacheStatus != "stale" || !observation.BenchmarkHistoryCacheAsOf.Equal(cachedAt) {
		t.Fatalf("unexpected first observation: %+v", observation)
	}
	if first.HistoryRefreshFailures[SnapshotFailureThrottled] != 1 {
		t.Fatalf("history throttling was not counted: %+v", first.HistoryRefreshFailures)
	}
	if first.BenchmarkHistoryRefreshFailures[SnapshotFailureUpstream] != 1 {
		t.Fatalf("benchmark history failure was not counted separately: %+v", first.BenchmarkHistoryRefreshFailures)
	}
	versionAfterFirst := service.Snapshot().Version
	lastSuccess := observation.LastSuccess

	// The provider timestamp is unchanged. This is a healthy poll, but neither a
	// new observation nor a durable store mutation.
	analyzer.mu.Lock()
	analyzer.quote.HistoryRefreshFailed = false
	analyzer.quote.BenchmarkHistoryRefreshFailed = false
	analyzer.mu.Unlock()
	time.Sleep(time.Millisecond)
	if _, err := service.ScheduledQuote(context.Background(), "AMZN"); err != nil {
		t.Fatal(err)
	}
	second := service.Stats()
	secondObservation := second.ScheduledSnapshotObservations["AMZN"]
	if second.ScheduledSnapshotAttempts != 2 || second.ScheduledSnapshotSuccesses != 1 || second.ScheduledSnapshotNoNewSession != 1 {
		t.Fatalf("unchanged quote counters: %+v", second)
	}
	if !secondObservation.LastSuccess.Equal(lastSuccess) || !secondObservation.LastHealthyCheck.After(observation.LastHealthyCheck) {
		t.Fatalf("unchanged quote timestamps: first=%+v second=%+v", observation, secondObservation)
	}
	if service.Snapshot().Version != versionAfterFirst {
		t.Fatalf("unchanged provider observation mutated store version: before=%d after=%d", versionAfterFirst, service.Snapshot().Version)
	}

	analyzer.setError(errors.New("Yahoo Finance quote HTTP 429: rate limited"))
	if _, err := service.ScheduledQuote(context.Background(), "AMZN"); err == nil {
		t.Fatal("expected throttled quote to fail")
	}
	analyzer.setError(context.DeadlineExceeded)
	if _, err := service.ScheduledQuote(context.Background(), "AMZN"); err == nil {
		t.Fatal("expected timed-out quote to fail")
	}
	failed := service.Stats()
	if failed.ScheduledSnapshotAttempts != 4 || failed.ScheduledSnapshotFailures[SnapshotFailureThrottled] != 1 || failed.ScheduledSnapshotFailures[SnapshotFailureTimeout] != 1 {
		t.Fatalf("bounded failure counters: %+v", failed)
	}

	// Stats returns defensive copies of its maps.
	failed.ScheduledSnapshotFailures[SnapshotFailureOther] = 99
	delete(failed.ScheduledSnapshotObservations, "AMZN")
	fresh := service.Stats()
	if fresh.ScheduledSnapshotFailures[SnapshotFailureOther] != 0 || fresh.ScheduledSnapshotObservations["AMZN"].LastObservation.IsZero() {
		t.Fatal("Stats exposed mutable internal maps")
	}
}

func TestScheduledSnapshotFailureClassificationIsBounded(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{fmt.Errorf("%w: disk full", ErrQuotePersistence), SnapshotFailurePersist},
		{fmt.Errorf("%w: HTTP 503", ErrQuoteUpstream), SnapshotFailureUpstream},
		{errors.New("too many requests"), SnapshotFailureThrottled},
		{context.DeadlineExceeded, SnapshotFailureTimeout},
		{errors.New("unexpected invariant"), SnapshotFailureOther},
	}
	for _, test := range tests {
		if got := classifyQuoteFailure(test.err); got != test.want {
			t.Errorf("classifyQuoteFailure(%q) = %q, want %q", test.err, got, test.want)
		}
	}
	if got := normalizeFailureKind("raw provider detail"); got != SnapshotFailureOther {
		t.Fatalf("unbounded failure kind normalized to %q", got)
	}
}

func TestScheduledBenchmarkFailureCountsSharedRefreshAttemptOnce(t *testing.T) {
	state, err := store.Open(filepath.Join(t.TempDir(), "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	quote := completeScheduledQuote(time.Now().UTC().Truncate(time.Second), time.Time{})
	quote.HistoryRefreshFailed = false
	quote.BenchmarkHistoryRefreshFailureID = "shared-spy-refresh-1"
	analyzer := &snapshotMetricsAnalyzer{quote: quote}
	service := NewService(state, analyzer).WithQuoteTTL(0)

	for _, ticker := range []string{"AMZN", "MSFT"} {
		if _, err := service.ScheduledQuote(context.Background(), ticker); err != nil {
			t.Fatal(err)
		}
	}
	if got := service.Stats().BenchmarkHistoryRefreshFailures[SnapshotFailureUpstream]; got != 1 {
		t.Fatalf("one coalesced benchmark failure counted %d times", got)
	}

	analyzer.mu.Lock()
	analyzer.quote.BenchmarkHistoryRefreshFailureID = "shared-spy-refresh-2"
	analyzer.mu.Unlock()
	if _, err := service.ScheduledQuote(context.Background(), "GOOGL"); err != nil {
		t.Fatal(err)
	}
	if got := service.Stats().BenchmarkHistoryRefreshFailures[SnapshotFailureUpstream]; got != 2 {
		t.Fatalf("distinct benchmark refresh failure was not counted: %d", got)
	}

	service.mu.Lock()
	for index := 0; index < benchmarkFailureDedupeLimit+5; index++ {
		service.recordBenchmarkHistoryFailureLocked(fmt.Sprintf("bounded-%d", index), SnapshotFailureUpstream)
	}
	retained := len(service.benchmarkFailureIDs)
	service.mu.Unlock()
	if retained != benchmarkFailureDedupeLimit {
		t.Fatalf("benchmark failure dedupe retained %d IDs, want %d", retained, benchmarkFailureDedupeLimit)
	}
}

func TestServiceSeedsSnapshotFreshnessFromPersistedQuoteHistory(t *testing.T) {
	state, err := store.Open(filepath.Join(t.TempDir(), "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	observedAt := time.Date(2026, time.July, 30, 20, 0, 0, 0, time.UTC)
	quote := completeScheduledQuote(observedAt, time.Time{})
	if _, err := state.RecordQuoteSnapshot("AMZN", model.NewStatisticSnapshot(quote)); err != nil {
		t.Fatal(err)
	}

	service := NewService(state, &snapshotMetricsAnalyzer{})
	observation := service.Stats().ScheduledSnapshotObservations["AMZN"]
	if !observation.LastObservation.Equal(observedAt) || !observation.LastSuccess.Equal(observedAt) || observation.QuoteFieldsPresent != expectedScheduledQuoteFieldCount || observation.HistoryCacheStatus != "unavailable" {
		t.Fatalf("persisted freshness was not restored: %+v", observation)
	}
}

func TestQuoteObservationPersistenceRejectsFallbackAndAcceptsTimestampedCorrections(t *testing.T) {
	prior := model.StatisticSnapshot{AsOf: "2026-07-31T15:30:00Z", Numeric: map[string]float64{"price": 100}}
	duplicate := model.LiveQuote{AsOf: prior.AsOf}
	price := 100.0
	duplicate.Price = &price
	if quoteObservationNeedsPersistence([]model.StatisticSnapshot{prior}, duplicate) {
		t.Fatal("duplicate provider timestamp advanced")
	}
	correctedPrice := 101.0
	correction := model.LiveQuote{AsOf: prior.AsOf, Price: &correctedPrice}
	if !quoteObservationNeedsPersistence([]model.StatisticSnapshot{prior}, correction) {
		t.Fatal("equal-timestamp payload correction was not accepted")
	}
	newer := model.LiveQuote{AsOf: "2026-07-31T15:31:00Z"}
	if !quoteObservationNeedsPersistence([]model.StatisticSnapshot{prior}, newer) {
		t.Fatal("newer provider timestamp did not advance")
	}
	newer.FieldSources = map[string]string{"asOf": "Parallel Ocean request time fallback"}
	if quoteObservationNeedsPersistence([]model.StatisticSnapshot{prior}, newer) {
		t.Fatal("request-time fallback advanced durable history")
	}
}

func TestScheduledQuoteTreatsRequestTimeFallbackAsNoNewSession(t *testing.T) {
	state, err := store.Open(filepath.Join(t.TempDir(), "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	quote := completeScheduledQuote(time.Now().UTC(), time.Time{})
	quote.FieldSources["asOf"] = "Parallel Ocean request time fallback"
	service := NewService(state, &snapshotMetricsAnalyzer{quote: quote}).WithQuoteTTL(0)
	versionBefore := service.Snapshot().Version

	if _, err := service.ScheduledQuote(context.Background(), "AMZN"); err != nil {
		t.Fatal(err)
	}
	stats := service.Stats()
	observation := stats.ScheduledSnapshotObservations["AMZN"]
	if stats.ScheduledSnapshotSuccesses != 0 || stats.ScheduledSnapshotNoNewSession != 1 || !observation.LastSuccess.IsZero() || !observation.LastObservation.IsZero() || observation.LastHealthyCheck.IsZero() {
		t.Fatalf("request-time fallback telemetry: stats=%+v observation=%+v", stats, observation)
	}
	if service.Snapshot().Version != versionBefore {
		t.Fatal("request-time fallback mutated durable state")
	}
}

func completeScheduledQuote(observedAt, cachedAt time.Time) model.LiveQuote {
	value := func(number float64) *float64 { return &number }
	quote := model.LiveQuote{
		Ticker:                             "AMZN",
		Price:                              value(210),
		PreviousClose:                      value(209),
		Change:                             value(1),
		ChangePercent:                      value(1.0 / 209),
		AsOf:                               observedAt.Format(time.RFC3339),
		MarketState:                        "REGULAR",
		Currency:                           "USD",
		Exchange:                           "NasdaqGS",
		Source:                             "fixture",
		FieldSources:                       map[string]string{"asOf": "fixture provider timestamp"},
		Change52Week:                       value(0.2),
		High52Week:                         value(220),
		Low52Week:                          value(150),
		MovingAverage50Day:                 value(205),
		MovingAverage200Day:                value(190),
		AverageVolume3Month:                value(40_000_000),
		AverageVolume10Day:                 value(35_000_000),
		HistoryCacheStatus:                 "stale",
		HistoryRefreshFailureKind:          SnapshotFailureThrottled,
		HistoryRefreshFailed:               true,
		BenchmarkHistoryCacheStatus:        "stale",
		BenchmarkHistoryRefreshFailureKind: SnapshotFailureUpstream,
		BenchmarkHistoryRefreshFailed:      true,
	}
	if !cachedAt.IsZero() {
		quote.HistoryCacheAsOf = cachedAt.Format(time.RFC3339)
		quote.BenchmarkHistoryCacheAsOf = cachedAt.Format(time.RFC3339)
	}
	return quote
}
