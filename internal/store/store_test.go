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
		AsOf:       "2026-07-31T10:00:00-04:00",
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
	if len(history) != 1 || history[0].AsOf != "2026-07-31T14:00:00Z" || history[0].Numeric["price"] != 100 {
		t.Fatalf("first snapshot = %#v", history)
	}

	history, err = stateStore.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{AsOf: "2026-07-31T13:00:00Z", Numeric: map[string]float64{"price": 90}})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Numeric["price"] != 100 {
		t.Fatalf("earlier same-day observation replaced history: %#v", history)
	}

	history, err = stateStore.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{AsOf: "2026-07-31T15:00:00Z", Source: "later fixture", AsOfSource: "later exchange timestamp", Numeric: map[string]float64{"price": 110}, Sources: map[string]string{"price": "later price"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Numeric["price"] != 110 || history[0].Source != "later fixture" {
		t.Fatalf("later same-day observation did not replace history: %#v", history)
	}

	history, err = stateStore.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{AsOf: "2026-07-31T15:00:00Z", Source: "corrected fixture", AsOfSource: "same exchange timestamp", Numeric: map[string]float64{"price": 112}, Sources: map[string]string{"price": "corrected price"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Numeric["price"] != 112 || history[0].Source != "corrected fixture" {
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

	history, err = stateStore.RecordQuoteSnapshot("AMZN", model.StatisticSnapshot{AsOf: "2026-08-01T03:00:00+02:00", Numeric: map[string]float64{"price": 111}})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[1].AsOf != "2026-08-01T01:00:00Z" {
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
	if len(persisted.QuoteHistory) != 2 || persisted.QuoteHistory[0].Numeric["price"] != 112 || persisted.QuoteHistory[0].Sources["price"] != "corrected price" || persisted.QuoteHistory[0].AsOfSource != "same exchange timestamp" {
		t.Fatalf("snapshot history was not persisted exactly: %#v", persisted.QuoteHistory)
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
	if err := store.SetMacroError(errors.New("FRED unavailable")); err != nil {
		t.Fatal(err)
	}
	got := store.Snapshot().Macro
	if len(got.Points) != 1 || got.Error != "FRED unavailable" {
		t.Fatalf("unexpected macro state: %#v", got)
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
