package server

import (
	"bytes"
	"context"
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
func (f *fakeService) PreviewTicker(_ context.Context, ticker string) (analysis.TickerPreview, error) {
	return analysis.TickerPreview{Ticker: ticker, Company: "Preview Co", InstrumentType: "US equity", Source: "test"}, nil
}

func TestBasePathAndTickerAPI(t *testing.T) {
	dir := t.TempDir()
	monetaryDir := t.TempDir()
	macroDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<main>equities</main>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(monetaryDir, "index.html"), []byte("<main>monetary</main>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(macroDir, "index.html"), []byte("<main>macro</main>"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := model.NewState()
	state.UpdatedAt = time.Now()
	state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN", Status: "ready", Quarterlies: []model.QuarterlyPoint{{FiscalYear: 2026, FiscalQuarter: "Q1"}}, Prices: []model.PricePoint{{Date: "2026-01-01", Close: 1}}, Valuations: []model.ValuationPoint{{Date: "2026-01-01", PE: floatPtr(20)}}}
	state.Macro = model.MacroSeries{
		Points:    []model.MacroPoint{{Date: "2026-01-01", Inflation: floatPtr(3), CoreInflation: floatPtr(2.5)}},
		Countries: []model.CountrySeries{{Code: "US", Name: "United States"}},
		Assets:    []model.AssetSeries{{Symbol: "SPY", Label: "US large cap"}},
		Options:   model.OptionsSeries{Snapshots: []model.OptionSnapshot{{Ticker: "SPY", AsOf: "2026-01-01"}}},
	}
	service := &fakeService{state: state}
	handler := New(service, Config{BasePath: "/equities", StaticDir: dir, MonetaryPath: "/monetary", MonetaryStaticDir: monetaryDir, MacroPath: "/macro", MacroStaticDir: macroDir}).Handler()

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
	etag := resp.Header().Get("ETag")
	if etag == "" {
		t.Fatal("state response has no ETag")
	}
	req = httptest.NewRequest(http.MethodGet, "/equities/api/state", nil)
	req.Header.Set("If-None-Match", etag)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotModified || resp.Body.Len() != 0 {
		t.Fatalf("conditional response: %d %s", resp.Code, resp.Body.String())
	}
	if bytes.Contains(resp.Body.Bytes(), []byte("coreInflation")) {
		t.Fatalf("equities overview should use compact macro fields: %s", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/monetary/api/state", nil)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !bytes.Contains(resp.Body.Bytes(), []byte("coreInflation")) || !bytes.Contains(resp.Body.Bytes(), []byte("valuations")) || bytes.Contains(resp.Body.Bytes(), []byte("quarterlies")) {
		t.Fatalf("monetary state should include full macro and compact equities: %d %s", resp.Code, resp.Body.String())
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte("countries")) || bytes.Contains(resp.Body.Bytes(), []byte("assets")) {
		t.Fatalf("monetary state should include countries but omit cross-assets: %s", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/macro/api/state", nil)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !bytes.Contains(resp.Body.Bytes(), []byte("countries")) || !bytes.Contains(resp.Body.Bytes(), []byte("assets")) {
		t.Fatalf("macro state should include countries and cross-assets: %d %s", resp.Code, resp.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/macro/api/state?view=options", nil)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !bytes.Contains(resp.Body.Bytes(), []byte("snapshots")) || bytes.Contains(resp.Body.Bytes(), []byte("assets")) || bytes.Contains(resp.Body.Bytes(), []byte("points")) {
		t.Fatalf("options scope: %d %s", resp.Code, resp.Body.String())
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

func floatPtr(value float64) *float64 { return &value }

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
