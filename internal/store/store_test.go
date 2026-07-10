package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
