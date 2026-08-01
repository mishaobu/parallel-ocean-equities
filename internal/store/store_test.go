package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func TestOpenSeedsAndPersists(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.json")
	state := model.NewState()
	state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN", Status: "ready", Annuals: []model.AnnualPoint{}}
	writeJSON(t, seed, state)

	path := filepath.Join(dir, "data", "state.json")
	store, err := Open(path, seed, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state not persisted: %v", err)
	}
	if err := store.Add("msft"); err != nil {
		t.Fatal(err)
	}
	if err := store.Add("googl"); !errors.Is(err, ErrLimit) {
		t.Fatalf("expected ticker limit, got %v", err)
	}

	reopened, err := Open(path, seed, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Tickers(); len(got) != 2 || got[0] != "AMZN" || got[1] != "MSFT" {
		t.Fatalf("unexpected tickers: %v", got)
	}
}

func TestRecordQuoteSnapshotReplacesSameUTCDayAndPersists(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.json")
	state := model.NewState()
	state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN", Status: "ready", Annuals: []model.AnnualPoint{}}
	writeJSON(t, seed, state)
	path := filepath.Join(dir, "state.json")
	stateStore, err := Open(path, seed, 2)
	if err != nil {
		t.Fatal(err)
	}

	first := model.StatisticSnapshot{
		AsOf:       "2025-07-31T10:00:00-04:00",
		Source:     "fixture",
		AsOfSource: "exchange timestamp",
		Numeric:    map[string]float64{"price": 100},
		Text:       map[string]string{"market-state": "REGULAR"},
		Sources:    map[string]string{"price": "fixture price"},
	}
	history, err := stateStore.RecordQuoteSnapshot("amzn", first)
	if err != nil {
		t.Fatal(err)
	}
	first.Numeric["price"] = 999
	first.Text["market-state"] = "MUTATED"
	if len(history) != 1 || history[0].AsOf != "2025-07-31T14:00:00Z" || history[0].Numeric["price"] != 100 {
		t.Fatalf("first snapshot = %#v", history)
	}

	history, err = stateStore.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{AsOf: "2025-07-31T13:00:00Z", Numeric: map[string]float64{"price": 90}})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Numeric["price"] != 100 {
		t.Fatalf("earlier same-day observation replaced history: %#v", history)
	}

	history, err = stateStore.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{AsOf: "2025-07-31T15:00:00Z", Source: "later fixture", AsOfSource: "later exchange timestamp", Numeric: map[string]float64{"price": 110}, Sources: map[string]string{"price": "later price"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].AsOf != "2025-07-31T14:00:00Z" || history[0].Numeric["price"] != 110 || history[0].Text["market-state"] != "REGULAR" || history[0].Source != "" || history[0].Sources["price"] != "later price" {
		t.Fatalf("later same-day observation did not replace history: %#v", history)
	}

	history, err = stateStore.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{AsOf: "2025-07-31T15:00:00Z", Source: "corrected fixture", AsOfSource: "same exchange timestamp", Numeric: map[string]float64{"price": 112}, Sources: map[string]string{"price": "corrected price"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Numeric["price"] != 112 || history[0].Source != "" || history[0].Sources["price"] != "corrected price" {
		t.Fatalf("equal-timestamp correction did not replace history: %#v", history)
	}
	versionAfterCorrection := stateStore.Snapshot().Version
	history, err = stateStore.RecordQuoteSnapshot("AMZN", history[0])
	if err != nil {
		t.Fatal(err)
	}
	if stateStore.Snapshot().Version != versionAfterCorrection {
		t.Fatal("identical equal-timestamp snapshot rewrote persisted state")
	}

	history, err = stateStore.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{AsOf: "2025-08-01T03:00:00+02:00", Numeric: map[string]float64{"price": 111}})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[1].AsOf != "2025-08-01T01:00:00Z" {
		t.Fatalf("UTC-day ordering failed: %#v", history)
	}

	reopened, err := Open(path, seed, 2)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := reopened.Get("AMZN")
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.QuoteHistory) != 2 || persisted.QuoteHistory[0].Numeric["price"] != 112 || persisted.QuoteHistory[0].Sources["price"] != "corrected price" || persisted.QuoteHistory[0].AsOfSource != "exchange timestamp" || persisted.QuoteHistory[0].LatestObservationAsOf != "2025-07-31T15:00:00Z" {
		t.Fatalf("snapshot history was not persisted exactly: %#v", persisted.QuoteHistory)
	}
	updatedBeforeOutOfOrder := reopened.Snapshot().UpdatedAt
	history, err = reopened.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{
		AsOf:    "2025-07-31T14:30:00Z",
		Numeric: map[string]float64{"price": 999},
	})
	if err != nil {
		t.Fatal(err)
	}
	if history[0].Numeric["price"] != 112 || history[0].LatestObservationAsOf != "2025-07-31T15:00:00Z" || !reopened.Snapshot().UpdatedAt.Equal(updatedBeforeOutOfOrder) {
		t.Fatalf("persisted watermark admitted an out-of-order correction after restart: %#v", history)
	}
}

func TestMergeQuoteHistoryPreservesRicherSameDaySnapshot(t *testing.T) {
	observed := time.Date(2026, 7, 31, 17, 0, 0, 0, time.UTC)
	prior := model.StatisticSnapshot{
		AsOf:       observed.Format(time.RFC3339),
		Source:     "rich history",
		AsOfSource: "exchange timestamp",
		Numeric: map[string]float64{
			"price":                  100,
			"moving-average-50d":     95,
			"moving-average-200d":    80,
			"average-volume-10d":     10_000,
			"average-volume-3m":      12_000,
			"beta-5y":                1.2,
			"trailing-dividend-rate": 0.5,
		},
		Text: map[string]string{"market-state": "REGULAR"},
		Sources: map[string]string{
			"price":                  "old price",
			"moving-average-50d":     "rich 50-day history",
			"moving-average-200d":    "rich 200-day history",
			"average-volume-10d":     "rich 10-day history",
			"average-volume-3m":      "rich 3-month history",
			"beta-5y":                "rich beta history",
			"trailing-dividend-rate": "rich dividend history",
			"market-state":           "old market state",
		},
	}

	corrected := model.StatisticSnapshot{
		AsOf:       prior.AsOf,
		Source:     "sparse correction",
		AsOfSource: "corrected exchange timestamp",
		Numeric:    map[string]float64{"price": 101},
		Text:       map[string]string{"market-state": "CLOSED"},
		Sources:    map[string]string{"price": "corrected price", "market-state": "corrected market state"},
	}
	history, accepted := mergeQuoteHistory([]model.StatisticSnapshot{prior}, corrected, observed)
	if !accepted || len(history) != 1 {
		t.Fatalf("equal-time correction was not accepted: accepted=%v history=%#v", accepted, history)
	}
	got := history[0]
	if got.Numeric["price"] != 101 || got.Sources["price"] != "corrected price" || got.Text["market-state"] != "CLOSED" {
		t.Fatalf("correction was not applied: %#v", got)
	}
	if got.AsOf != prior.AsOf || got.AsOfSource != prior.AsOfSource || got.Source != "" {
		t.Fatalf("mixed-age snapshot inherited correction aggregate metadata: %#v", got)
	}
	for _, key := range []string{"moving-average-50d", "moving-average-200d", "average-volume-10d", "average-volume-3m", "beta-5y", "trailing-dividend-rate"} {
		if got.Numeric[key] != prior.Numeric[key] || got.Sources[key] != prior.Sources[key] {
			t.Fatalf("rich field %q was degraded: %#v", key, got)
		}
	}

	later := observed.Add(30 * time.Second)
	laterSparse := model.StatisticSnapshot{
		AsOf:    later.Format(time.RFC3339),
		Numeric: map[string]float64{"price": 102},
	}
	history, accepted = mergeQuoteHistory(history, laterSparse, later)
	if !accepted || history[0].AsOf != prior.AsOf || history[0].Numeric["moving-average-200d"] != 80 {
		t.Fatalf("later sparse observation degraded same-day history: accepted=%v history=%#v", accepted, history)
	}
	if history[0].LatestObservationAsOf != later.Format(time.RFC3339) {
		t.Fatalf("latest sparse observation was not persisted as an ordering watermark: %#v", history[0])
	}
	if history[0].AsOfSource != prior.AsOfSource {
		t.Fatalf("retained aggregate timestamp lost its provenance: %q", history[0].AsOfSource)
	}
	if history[0].Source != "" {
		t.Fatalf("mixed-provider aggregate provenance was not cleared: %q", history[0].Source)
	}
	if _, exists := history[0].Sources["price"]; exists {
		t.Fatalf("stale price provenance survived a source-less correction: %#v", history[0].Sources)
	}

	unchanged, accepted := mergeQuoteHistory(history, laterSparse, later)
	if accepted || !model.StatisticSnapshotContentEqual(unchanged[0], history[0]) || unchanged[0].AsOf != history[0].AsOf {
		t.Fatalf("identical sparse observation rewrote mixed-age history: accepted=%v history=%#v", accepted, unchanged)
	}
	outOfOrderAt := observed.Add(15 * time.Second)
	outOfOrder := model.StatisticSnapshot{
		AsOf:       outOfOrderAt.Format(time.RFC3339),
		Source:     "out-of-order provider response",
		AsOfSource: "older exchange timestamp",
		Numeric:    map[string]float64{"price": 999},
	}
	unchanged, accepted = mergeQuoteHistory(history, outOfOrder, outOfOrderAt)
	if accepted || unchanged[0].Numeric["price"] != 102 || unchanged[0].LatestObservationAsOf != later.Format(time.RFC3339) {
		t.Fatalf("out-of-order sparse observation crossed the persisted watermark: accepted=%v history=%#v", accepted, unchanged)
	}

	completeAt := later.Add(30 * time.Second)
	complete := cloneStatisticSnapshot(history[0])
	complete.AsOf = completeAt.Format(time.RFC3339)
	complete.AsOfSource = "complete exchange timestamp"
	complete.Source = "complete refresh"
	complete.Numeric["price"] = 103
	complete.Sources["price"] = "complete price"
	history, accepted = mergeQuoteHistory(history, complete, completeAt)
	if !accepted || history[0].AsOf != complete.AsOf || history[0].AsOfSource != complete.AsOfSource || history[0].Source != complete.Source || history[0].LatestObservationAsOf != "" {
		t.Fatalf("complete same-day replacement did not advance aggregate metadata: accepted=%v history=%#v", accepted, history)
	}

	nextDay := completeAt.Add(24 * time.Hour)
	history, accepted = mergeQuoteHistory(history, model.StatisticSnapshot{AsOf: nextDay.Format(time.RFC3339), Numeric: map[string]float64{"price": 104}}, nextDay)
	if !accepted || len(history) != 2 {
		t.Fatalf("next-day observation was not added: accepted=%v history=%#v", accepted, history)
	}
	if _, inherited := history[1].Numeric["moving-average-200d"]; inherited {
		t.Fatalf("next-day sparse observation inherited a prior-day field: %#v", history[1])
	}
}

func TestMergeQuoteHistoryClearsSharedAggregateSourceForSparseObservation(t *testing.T) {
	priorAt := time.Date(2026, 7, 31, 17, 0, 0, 0, time.UTC)
	prior := model.StatisticSnapshot{
		AsOf:       priorAt.Format(time.RFC3339),
		Source:     "shared provider",
		AsOfSource: "exchange timestamp",
		Numeric:    map[string]float64{"price": 100, "moving-average-200d": 80},
	}
	laterAt := priorAt.Add(time.Minute)
	later := model.StatisticSnapshot{
		AsOf:       laterAt.Format(time.RFC3339),
		Source:     "shared provider",
		AsOfSource: "later exchange timestamp",
		Numeric:    map[string]float64{"price": 101},
	}

	history, accepted := mergeQuoteHistory([]model.StatisticSnapshot{prior}, later, laterAt)
	if !accepted || len(history) != 1 {
		t.Fatalf("sparse same-provider observation was not accepted: accepted=%v history=%#v", accepted, history)
	}
	if got := history[0]; got.AsOf != prior.AsOf || got.AsOfSource != prior.AsOfSource || got.Source != "" || got.LatestObservationAsOf != later.AsOf || got.Numeric["price"] != 101 || got.Numeric["moving-average-200d"] != 80 {
		t.Fatalf("mixed-age aggregate provenance was not made conservative: %#v", got)
	}
}

func TestRecordQuoteSnapshotPurgesPoisonedFutureHistory(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.json")
	now := time.Now().UTC().Truncate(time.Second)
	validPrior := now.AddDate(-9, 0, 0)
	poisonedFuture := now.AddDate(20, 0, 0)
	state := model.NewState()
	state.Tickers["AMZN"] = &model.Equity{
		Ticker: "AMZN",
		Status: "ready",
		QuoteHistory: []model.StatisticSnapshot{
			{AsOf: validPrior.Format(time.RFC3339), Numeric: map[string]float64{"price": 80}},
			{AsOf: poisonedFuture.Format(time.RFC3339), Numeric: map[string]float64{"price": 999}},
		},
	}
	writeJSON(t, seed, state)
	stateStore, err := Open(filepath.Join(dir, "state.json"), seed, 2)
	if err != nil {
		t.Fatal(err)
	}

	history, err := stateStore.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{
		AsOf:    now.Format(time.RFC3339),
		Numeric: map[string]float64{"price": 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].AsOf != validPrior.Format(time.RFC3339) || history[1].AsOf != now.Format(time.RFC3339) {
		t.Fatalf("future row poisoned retention: %#v", history)
	}
	for _, snapshot := range history {
		observedAt, parseErr := time.Parse(time.RFC3339Nano, snapshot.AsOf)
		if parseErr != nil || observedAt.After(now.Add(quoteHistoryFutureSkew)) {
			t.Fatalf("future row survived sanitization: %#v", history)
		}
	}

	_, err = stateStore.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{
		AsOf:    time.Now().UTC().Add(quoteHistoryFutureSkew + time.Minute).Format(time.RFC3339),
		Numeric: map[string]float64{"price": 101},
	})
	afterRejected := stateStore.Snapshot().Tickers["AMZN"].QuoteHistory
	if err == nil || len(afterRejected) != 2 || afterRejected[len(afterRejected)-1].Numeric["price"] != 100 {
		t.Fatalf("implausibly future current row mutated state: err=%v", err)
	}

	// Backfill is validated independently of the current-row clock guard. A
	// future-dated provider artifact is ignored relative to the valid current
	// point instead of making that valid point fail.
	history, err = stateStore.RecordQuoteSnapshots("AMZN", []model.StatisticSnapshot{{
		AsOf:    now.AddDate(20, 0, 0).Format(time.RFC3339),
		Numeric: map[string]float64{"price": 500},
	}}, model.StatisticSnapshot{
		AsOf:    now.Format(time.RFC3339),
		Numeric: map[string]float64{"price": 102},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[len(history)-1].Numeric["price"] != 102 {
		t.Fatalf("future backfill artifact displaced valid current row: %#v", history)
	}
}

func TestRecordQuoteSnapshotRollsBackOnSaveFailure(t *testing.T) {
	dir := t.TempDir()
	blockedPath := filepath.Join(dir, "state-directory")
	if err := os.Mkdir(blockedPath, 0o755); err != nil {
		t.Fatal(err)
	}
	state := model.NewState()
	state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN", Status: "ready"}
	stateStore := &Store{path: blockedPath, maxTickers: 2, state: state}
	if _, err := stateStore.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{AsOf: "2026-07-31T15:00:00Z", Numeric: map[string]float64{"price": 110}}); err == nil {
		t.Fatal("snapshot save unexpectedly succeeded over a directory")
	}
	if got := stateStore.state.Tickers["AMZN"].QuoteHistory; len(got) != 0 {
		t.Fatalf("failed persistence remained accepted in memory: %#v", got)
	}
}

func TestAllStoreMutationsUseCopyOnWriteOnSaveFailure(t *testing.T) {
	dir := t.TempDir()
	blockedPath := filepath.Join(dir, "state-directory")
	if err := os.Mkdir(blockedPath, 0o755); err != nil {
		t.Fatal(err)
	}
	state := model.NewState()
	state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN", Status: "ready"}
	state.Tickers["AAPL"] = &model.Equity{Ticker: "AAPL", Status: "ready"}
	stateStore := &Store{path: blockedPath, maxTickers: 3, state: state}

	if err := stateStore.Add("MSFT"); err == nil {
		t.Fatal("add unexpectedly succeeded over a directory")
	}
	if _, err := stateStore.Get("MSFT"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("failed add became visible in memory: %v", err)
	}
	if err := stateStore.Delete("AAPL"); err == nil {
		t.Fatal("delete unexpectedly succeeded over a directory")
	}
	if _, err := stateStore.Get("AAPL"); err != nil {
		t.Fatalf("failed delete removed ticker from memory: %v", err)
	}
	if err := stateStore.SetError("AMZN", errors.New("provider failed")); err == nil {
		t.Fatal("update unexpectedly succeeded over a directory")
	}
	got, err := stateStore.Get("AMZN")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "ready" || got.Error != "" {
		t.Fatalf("failed update changed memory: %#v", got)
	}
	if err := stateStore.SetMacroError(errors.New("FRED failed")); err == nil {
		t.Fatal("macro update unexpectedly succeeded over a directory")
	}
	if got := stateStore.Snapshot().Macro.Error; got != "" {
		t.Fatalf("failed macro update changed memory: %q", got)
	}
}

func TestRecordQuoteSnapshotsSeedsMissingMonthsAndPreservesExactCurrent(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.json")
	state := model.NewState()
	state.Tickers["AMZN"] = &model.Equity{
		Ticker: "AMZN",
		Status: "ready",
		QuoteHistory: []model.StatisticSnapshot{{
			AsOf:    "2026-01-15T16:00:00Z",
			Source:  "persisted exact observation",
			Numeric: map[string]float64{"price": 999, "market-cap": 3000},
		}},
	}
	writeJSON(t, seed, state)
	path := filepath.Join(dir, "state.json")
	stateStore, err := Open(path, seed, 2)
	if err != nil {
		t.Fatal(err)
	}
	backfill := []model.StatisticSnapshot{
		{AsOf: "2026-01-30T16:00:00Z", Source: "Yahoo backfill", Numeric: map[string]float64{"price": 101}},
		{AsOf: "2026-02-26T16:00:00Z", Source: "older February candidate", Numeric: map[string]float64{"price": 102}},
		{AsOf: "2026-02-27T16:00:00Z", Source: "last February session", Numeric: map[string]float64{"price": 103}},
		{AsOf: "2026-04-30T16:00:00Z", Source: "future candidate", Numeric: map[string]float64{"price": 104}},
	}
	current := model.StatisticSnapshot{AsOf: "2026-03-15T17:00:00Z", Source: "current exact", Numeric: map[string]float64{"price": 110, "market-cap": 3300}}
	history, err := stateStore.RecordQuoteSnapshots("amzn", backfill, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("history rows = %d, want persisted January + seeded February + current March: %#v", len(history), history)
	}
	if history[0].Numeric["price"] != 999 || history[0].Numeric["market-cap"] != 3000 || history[0].Source != "persisted exact observation" {
		t.Fatalf("backfill displaced authoritative January observation: %#v", history[0])
	}
	_, seededMarketCap := history[1].Numeric["market-cap"]
	if history[1].AsOf != "2026-02-27T16:00:00Z" || history[1].Numeric["price"] != 103 || seededMarketCap {
		t.Fatalf("bulk merge did not choose the last missing-month candidate: %#v", history[1])
	}
	if history[2].Numeric["market-cap"] != 3300 || history[2].Source != "current exact" {
		t.Fatalf("current exact market value was not preserved: %#v", history[2])
	}

	olderCurrent := model.StatisticSnapshot{AsOf: "2026-03-15T16:00:00Z", Source: "stale current", Numeric: map[string]float64{"price": 1}}
	history, err = stateStore.RecordQuoteSnapshots("AMZN", backfill, olderCurrent)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 || history[2].Numeric["market-cap"] != 3300 || history[2].Source != "current exact" {
		t.Fatalf("older current point displaced a later persisted observation: %#v", history)
	}
	reopened, err := Open(path, seed, 2)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := reopened.Get("AMZN")
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.QuoteHistory) != 3 || persisted.QuoteHistory[2].Numeric["market-cap"] != 3300 {
		t.Fatalf("bulk snapshot merge was not saved: %#v", persisted.QuoteHistory)
	}
}

func TestRecordQuoteSnapshotsEnrichesOccupiedMonthWithMissingBeta(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.json")
	state := model.NewState()
	state.Tickers["AMZN"] = &model.Equity{
		Ticker: "AMZN",
		Status: "ready",
		QuoteHistory: []model.StatisticSnapshot{
			{AsOf: "2026-01-05T16:00:00Z", Numeric: map[string]float64{"price": 90}},
			{
				AsOf:       "2026-01-25T16:00:00Z",
				Source:     "persisted exact observation",
				AsOfSource: "persisted exchange timestamp",
				Numeric: map[string]float64{
					"price":              999,
					"market-cap":         3000,
					"moving-average-50d": 95,
					"numeric-only":       7,
				},
				Text: map[string]string{"market-state": "REGULAR", "text-only": "not reported"},
				Sources: map[string]string{
					"price":        "persisted exact price",
					"market-cap":   "persisted exact market cap",
					"market-state": "persisted market state",
				},
			},
		},
	}
	writeJSON(t, seed, state)
	path := filepath.Join(dir, "state.json")
	stateStore, err := Open(path, seed, 2)
	if err != nil {
		t.Fatal(err)
	}

	backfill := []model.StatisticSnapshot{{
		AsOf:       "2026-01-30T16:00:00Z",
		Source:     "Yahoo monthly backfill",
		AsOfSource: "Yahoo month-end timestamp",
		Numeric: map[string]float64{
			"price":              101,
			"moving-average-50d": 96,
			"beta-5y":            1.2,
			"text-only":          12,
		},
		Text: map[string]string{
			"market-state": "CLOSED",
			"currency":     "USD",
			"numeric-only": "seven",
		},
		Sources: map[string]string{
			"price":              "Yahoo backfill price",
			"moving-average-50d": "Yahoo backfill moving average",
			"beta-5y":            "60-month SPY beta",
			"market-state":       "Yahoo backfill market state",
			"currency":           "Yahoo currency",
			"text-only":          "conflicting numeric source",
			"numeric-only":       "conflicting text source",
		},
	}}
	current := model.StatisticSnapshot{AsOf: "2026-03-15T17:00:00Z", Source: "current exact", Numeric: map[string]float64{"price": 110}}
	history, err := stateStore.RecordQuoteSnapshots("amzn", backfill, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("history rows = %d, want two persisted January days + current March: %#v", len(history), history)
	}
	if _, exists := history[0].Numeric["beta-5y"]; exists {
		t.Fatalf("backfill enriched an older observation instead of the latest occupied month: %#v", history[0])
	}
	enriched := history[1]
	if enriched.AsOf != "2026-01-25T16:00:00Z" || enriched.AsOfSource != "persisted exchange timestamp" || enriched.Source != "persisted exact observation" {
		t.Fatalf("backfill replaced authoritative timestamp/source metadata: %#v", enriched)
	}
	if enriched.Numeric["price"] != 999 || enriched.Numeric["market-cap"] != 3000 || enriched.Numeric["moving-average-50d"] != 95 || enriched.Numeric["beta-5y"] != 1.2 {
		t.Fatalf("missing beta was not added without overwriting numeric values: %#v", enriched.Numeric)
	}
	if enriched.Text["market-state"] != "REGULAR" || enriched.Text["currency"] != "USD" {
		t.Fatalf("missing text was not added without overwriting existing text: %#v", enriched.Text)
	}
	if enriched.Text["text-only"] != "not reported" || enriched.Numeric["numeric-only"] != 7 {
		t.Fatalf("cross-type backfill displaced existing field types: numeric=%#v text=%#v", enriched.Numeric, enriched.Text)
	}
	if _, exists := enriched.Numeric["text-only"]; exists {
		t.Fatalf("cross-type backfill created a duplicate numeric field: %#v", enriched.Numeric)
	}
	if _, exists := enriched.Text["numeric-only"]; exists {
		t.Fatalf("cross-type backfill created a duplicate text field: %#v", enriched.Text)
	}
	if enriched.Sources["price"] != "persisted exact price" || enriched.Sources["market-state"] != "persisted market state" || enriched.Sources["beta-5y"] != "60-month SPY beta" || enriched.Sources["currency"] != "Yahoo currency" {
		t.Fatalf("field provenance was not merged conservatively: %#v", enriched.Sources)
	}
	if _, exists := enriched.Sources["moving-average-50d"]; exists {
		t.Fatalf("different backfill value falsely attributed an existing value: %#v", enriched.Sources)
	}
	if _, exists := enriched.Sources["text-only"]; exists {
		t.Fatalf("cross-type numeric backfill falsely attributed an existing text value: %#v", enriched.Sources)
	}
	if _, exists := enriched.Sources["numeric-only"]; exists {
		t.Fatalf("cross-type text backfill falsely attributed an existing numeric value: %#v", enriched.Sources)
	}

	reopened, err := Open(path, seed, 2)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := reopened.Get("AMZN")
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.QuoteHistory) != 3 || persisted.QuoteHistory[1].Numeric["beta-5y"] != 1.2 || persisted.QuoteHistory[1].Sources["beta-5y"] != "60-month SPY beta" {
		t.Fatalf("occupied-month beta enrichment was not durably persisted: %#v", persisted.QuoteHistory)
	}
}

func TestSetResultPreservesQuoteHistory(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.json")
	state := model.NewState()
	state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN", Status: "ready", Annuals: []model.AnnualPoint{}}
	writeJSON(t, seed, state)
	stateStore, err := Open(filepath.Join(dir, "state.json"), seed, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stateStore.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{AsOf: "2026-07-31T15:00:00Z", Numeric: map[string]float64{"price": 110}}); err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SetResult("AMZN", &model.Equity{Company: "Amazon.com, Inc.", Annuals: []model.AnnualPoint{}}); err != nil {
		t.Fatal(err)
	}
	got, err := stateStore.Get("AMZN")
	if err != nil {
		t.Fatal(err)
	}
	if got.Company != "Amazon.com, Inc." || len(got.QuoteHistory) != 1 || got.QuoteHistory[0].Numeric["price"] != 110 {
		t.Fatalf("SEC refresh clobbered quote history: %#v", got)
	}
}

func TestMergeQuoteHistoryBoundsAgeAndRows(t *testing.T) {
	latest := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	history := make([]model.StatisticSnapshot, 0, 4200)
	for daysAgo := 4200; daysAgo > 0; daysAgo-- {
		observed := latest.AddDate(0, 0, -daysAgo)
		history = append(history, model.StatisticSnapshot{AsOf: observed.Format(time.RFC3339), Numeric: map[string]float64{"price": float64(daysAgo)}})
	}
	incoming := model.StatisticSnapshot{AsOf: latest.Format(time.RFC3339), Numeric: map[string]float64{"price": 200}}
	got, accepted := mergeQuoteHistory(history, incoming, latest)
	if !accepted {
		t.Fatal("latest snapshot was not accepted")
	}
	if len(got) == 0 || len(got) > quoteHistoryLimit {
		t.Fatalf("bounded rows = %d", len(got))
	}
	cutoff := latest.AddDate(-quoteHistoryYears, 0, 0)
	first, err := time.Parse(time.RFC3339Nano, got[0].AsOf)
	if err != nil {
		t.Fatal(err)
	}
	if first.Before(cutoff) {
		t.Fatalf("oldest row %s precedes cutoff %s", first, cutoff)
	}
	for index := 1; index < len(got); index++ {
		prior, _ := time.Parse(time.RFC3339Nano, got[index-1].AsOf)
		current, _ := time.Parse(time.RFC3339Nano, got[index].AsOf)
		if !current.After(prior) || current.Format("2006-01-02") == prior.Format("2006-01-02") {
			t.Fatalf("history not strictly UTC-day ordered at %d: %s then %s", index, prior, current)
		}
	}
}

func TestMacroErrorPreservesLastSuccessfulPoints(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.json")
	state := model.NewState()
	state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN", Status: "ready", Annuals: []model.AnnualPoint{}}
	writeJSON(t, seed, state)
	store, err := Open(filepath.Join(dir, "state.json"), seed, 2)
	if err != nil {
		t.Fatal(err)
	}
	series := model.MacroSeries{Points: []model.MacroPoint{{Date: "2025-01-01"}}}
	if err := store.SetMacro(series); err != nil {
		t.Fatal(err)
	}
	beforeFailure := store.Snapshot().Macro
	time.Sleep(time.Millisecond)
	if err := store.SetMacroError(errors.New("FRED unavailable")); err != nil {
		t.Fatal(err)
	}
	got := store.Snapshot().Macro
	if len(got.Points) != 1 || got.Error != "FRED unavailable" {
		t.Fatalf("unexpected macro state: %#v", got)
	}
	if !got.UpdatedAt.Equal(beforeFailure.UpdatedAt) || !got.LastSuccessAt.Equal(beforeFailure.LastSuccessAt) {
		t.Fatalf("failed attempt changed last-success freshness: before=%#v after=%#v", beforeFailure, got)
	}
	if !got.LastAttemptAt.After(beforeFailure.LastAttemptAt) {
		t.Fatalf("failed attempt was not recorded separately: before=%s after=%s", beforeFailure.LastAttemptAt, got.LastAttemptAt)
	}
}

func TestMacroRefreshPreservesOptionalEnrichmentWhenAdapterReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.json")
	state := model.NewState()
	writeJSON(t, seed, state)
	store, err := Open(filepath.Join(dir, "state.json"), seed, 2)
	if err != nil {
		t.Fatal(err)
	}
	previous := model.MacroSeries{
		Vintages: model.VintageSeries{Points: []model.VintagePoint{{Date: "2025-01-01", VintageDate: "2024-12-31"}}},
		Options:  model.OptionsSeries{Snapshots: []model.OptionSnapshot{{Ticker: "SPY"}}},
	}
	if err := store.SetMacro(previous); err != nil {
		t.Fatal(err)
	}
	if err := store.SetMacro(model.MacroSeries{Points: []model.MacroPoint{{Date: "2025-02-01"}}}); err != nil {
		t.Fatal(err)
	}
	got := store.Snapshot().Macro
	if len(got.Vintages.Points) != 1 || len(got.Options.Snapshots) != 1 || len(got.Points) != 1 {
		t.Fatalf("optional enrichment was not retained: %#v", got)
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := jsonMarshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

var jsonMarshal = func(value any) ([]byte, error) {
	return json.Marshal(value)
}
