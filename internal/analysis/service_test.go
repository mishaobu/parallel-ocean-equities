package analysis

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
	"github.com/mishaobu/parallel-ocean-equities/internal/store"
)

type fakeAnalyzer struct {
	err        error
	panicValue any
}

func (f fakeAnalyzer) Analyze(_ context.Context, ticker string, _ *model.Equity) (*model.Equity, error) {
	if f.panicValue != nil {
		panic(f.panicValue)
	}
	if f.err != nil {
		return nil, f.err
	}
	return &model.Equity{
		Ticker:  ticker,
		Company: "NVIDIA Corporation",
		Annuals: []model.AnnualPoint{},
		Sources: []string{"test"},
	}, nil
}

func TestServiceContainsAnalyzerPanic(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(state, fakeAnalyzer{panicValue: "provider failure"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Start(ctx, 1)

	if err := service.AddTicker("NVDA"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		equity := service.Snapshot().Tickers["NVDA"]
		if equity != nil && equity.Status == "error" {
			if service.Stats().RefreshFailures != 1 {
				t.Fatal("panic was not counted as a refresh failure")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("panicking refresh was not contained")
}

func TestServiceRefreshLifecycle(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "state.json"), "../../data/seed.json", 10)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(state, fakeAnalyzer{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Start(ctx, 1)

	if err := service.AddTicker("nvda"); err != nil {
		t.Fatal(err)
	}
	if service.Queue("NVDA") {
		t.Fatal("duplicate refresh should not be queued")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		equity := service.Snapshot().Tickers["NVDA"]
		if equity != nil && equity.Status == "ready" {
			if equity.Company != "NVIDIA Corporation" {
				t.Fatalf("unexpected result: %#v", equity)
			}
			if got := service.Stats().RefreshTotal; got != 1 {
				t.Fatalf("refresh total = %d, want 1", got)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("refresh did not complete")
}
