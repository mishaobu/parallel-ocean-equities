package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/analysis"
	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

type fakeService struct {
	state model.State
	added string
}

func (f *fakeService) Snapshot() model.State {
	data, _ := json.Marshal(f.state)
	var state model.State
	_ = json.Unmarshal(data, &state)
	return state
}
func (f *fakeService) Stats() analysis.Stats         { return analysis.Stats{} }
func (f *fakeService) DeleteTicker(string) error     { return nil }
func (f *fakeService) Queue(string) bool             { return true }
func (f *fakeService) RefreshAll() int               { return 1 }
func (f *fakeService) AddTicker(ticker string) error { f.added = ticker; return nil }

func TestBasePathAndTickerAPI(t *testing.T) {
	dir := t.TempDir()
	monetaryDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<main>equities</main>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(monetaryDir, "index.html"), []byte("<main>monetary</main>"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := model.NewState()
	state.UpdatedAt = time.Now()
	state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN", Status: "ready", Quarterlies: []model.QuarterlyPoint{{FiscalYear: 2026, FiscalQuarter: "Q1"}}, Prices: []model.PricePoint{{Date: "2026-01-01", Close: 1}}}
	service := &fakeService{state: state}
	handler := New(service, Config{BasePath: "/equities", StaticDir: dir, MonetaryPath: "/monetary", MonetaryStaticDir: monetaryDir}).Handler()

	req := httptest.NewRequest(http.MethodGet, "/equities/", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !bytes.Contains(resp.Body.Bytes(), []byte("equities")) {
		t.Fatalf("static response: %d %s", resp.Code, resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monetary/dashboard", nil)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !bytes.Contains(resp.Body.Bytes(), []byte("monetary")) {
		t.Fatalf("monetary SPA fallback: %d %s", resp.Code, resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/equities/api/state", nil)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || bytes.Contains(resp.Body.Bytes(), []byte("quarterlies")) || !bytes.Contains(resp.Body.Bytes(), []byte("prices")) {
		t.Fatalf("overview should omit quarterlies and retain compact prices: %d %s", resp.Code, resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/equities/api/tickers/AMZN", nil)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !bytes.Contains(resp.Body.Bytes(), []byte("quarterlies")) || !bytes.Contains(resp.Body.Bytes(), []byte("prices")) {
		t.Fatalf("ticker detail should include raw histories: %d %s", resp.Code, resp.Body.String())
	}

	body, _ := json.Marshal(map[string]string{"ticker": "NVDA"})
	req = httptest.NewRequest(http.MethodPost, "/equities/api/tickers", bytes.NewReader(body))
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted || service.added != "NVDA" {
		t.Fatalf("add response: %d ticker=%s body=%s", resp.Code, service.added, resp.Body.String())
	}
}

func TestCompactPricesRetainsQuarterEnds(t *testing.T) {
	rows := compactPrices([]model.PricePoint{
		{Date: "2025-01-31", Close: 1},
		{Date: "2025-02-28", Close: 2},
		{Date: "2025-03-31", Close: 3},
		{Date: "2025-04-30", Close: 4},
	})
	if len(rows) != 2 || rows[0].Date != "2025-03-31" || rows[1].Date != "2025-04-30" {
		t.Fatalf("unexpected quarter-end rows: %+v", rows)
	}
}
