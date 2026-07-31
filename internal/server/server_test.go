package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/analysis"
	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

type fakeService struct {
	state model.State
	added string
	quote model.LiveQuote
	stats analysis.Stats
}

func (f *fakeService) Snapshot() model.State {
	data, _ := json.Marshal(f.state)
	var state model.State
	_ = json.Unmarshal(data, &state)
	return state
}
func (f *fakeService) Health() analysis.Health {
	return analysis.Health{TickerCount: len(f.state.Tickers), UpdatedAt: f.state.UpdatedAt}
}
func (f *fakeService) Stats() analysis.Stats         { return f.stats }
func (f *fakeService) DeleteTicker(string) error     { return nil }
func (f *fakeService) Queue(string) bool             { return true }
func (f *fakeService) RefreshAll() int               { return 1 }
func (f *fakeService) AddTicker(ticker string) error { f.added = ticker; return nil }
func (f *fakeService) Quote(_ context.Context, ticker string) (model.LiveQuote, error) {
	quote := f.quote
	quote.Ticker = ticker
	return quote, nil
}
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
	state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN", Status: "ready", Quarterlies: []model.QuarterlyPoint{{FiscalYear: 2026, FiscalQuarter: "Q1"}}, Prices: []model.PricePoint{{Date: "2026-01-01", Close: 1}}, QuoteHistory: []model.StatisticSnapshot{{AsOf: "2026-07-30T14:30:00Z", Numeric: map[string]float64{"price": 209}}}, Valuations: []model.ValuationPoint{{Date: "2026-01-01", PE: floatPtr(20)}}}
	state.Macro = model.MacroSeries{
		Points:    []model.MacroPoint{{Date: "2026-01-01", Inflation: floatPtr(3), CoreInflation: floatPtr(2.5)}},
		Countries: []model.CountrySeries{{Code: "US", Name: "United States"}},
		Assets:    []model.AssetSeries{{Symbol: "SPY", Label: "US large cap"}},
		Options:   model.OptionsSeries{Snapshots: []model.OptionSnapshot{{Ticker: "SPY", AsOf: "2026-01-01"}}},
	}
	service := &fakeService{state: state, quote: model.LiveQuote{Price: floatPtr(210.25), AsOf: "2026-07-31T14:30:00Z", Source: "fixture", History: []model.StatisticSnapshot{{AsOf: "2026-07-31T14:30:00Z", Source: "fixture", Numeric: map[string]float64{"price": 210.25, "market-cap": 3150}}}}}
	handler := New(service, Config{BasePath: "/equities", StaticDir: dir, MonetaryPath: "/monetary", MonetaryStaticDir: monetaryDir, MacroPath: "/macro", MacroStaticDir: macroDir, AdminToken: "test-admin"}).Handler()

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
	if bytes.Contains(resp.Body.Bytes(), []byte("quoteHistory")) {
		t.Fatalf("overview should omit quote history: %s", resp.Body.String())
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

	req = httptest.NewRequest(http.MethodGet, "/equities/api/tickers/AMZN/quote", nil)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !bytes.Contains(resp.Body.Bytes(), []byte(`"price":210.25`)) || !bytes.Contains(resp.Body.Bytes(), []byte(`"source":"fixture"`)) || !bytes.Contains(resp.Body.Bytes(), []byte(`"history"`)) || !bytes.Contains(resp.Body.Bytes(), []byte(`"market-cap":3150`)) {
		t.Fatalf("ticker quote response: %d %s", resp.Code, resp.Body.String())
	}
	if resp.Header().Get("Cache-Control") != "private, max-age=60" {
		t.Fatalf("unexpected quote cache control: %q", resp.Header().Get("Cache-Control"))
	}
	req = httptest.NewRequest(http.MethodGet, "/equities/api/tickers/AMZN/quote?history=0", nil)
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !bytes.Contains(resp.Body.Bytes(), []byte(`"price":210.25`)) || bytes.Contains(resp.Body.Bytes(), []byte(`"history"`)) {
		t.Fatalf("ticker quote without history: %d %s", resp.Code, resp.Body.String())
	}

	body, _ := json.Marshal(map[string]string{"ticker": "NVDA"})
	req = httptest.NewRequest(http.MethodPost, "/equities/api/tickers", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-admin")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted || service.added != "NVDA" {
		t.Fatalf("add response: %d ticker=%s body=%s", resp.Code, service.added, resp.Body.String())
	}
}

func TestAdministrativeMutationsFailClosedAndAuthenticateBeforeLookup(t *testing.T) {
	state := model.NewState()
	state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN"}
	service := &fakeService{state: state}

	disabled := New(service, Config{}).Handler()
	req := httptest.NewRequest(http.MethodPost, "/internal/refresh", nil)
	resp := httptest.NewRecorder()
	disabled.ServeHTTP(resp, req)
	if resp.Code != http.StatusServiceUnavailable || resp.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("disabled admin endpoint: status=%d cache=%q body=%s", resp.Code, resp.Header().Get("Cache-Control"), resp.Body.String())
	}

	secured := New(service, Config{AdminToken: "secret"}).Handler()
	for _, target := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/internal/refresh"},
		{http.MethodPost, "/equities/api/tickers"},
		{http.MethodGet, "/equities/api/tickers/AAPL/preview"},
		{http.MethodPost, "/equities/api/tickers/DOES-NOT-EXIST/refresh"},
		{http.MethodDelete, "/equities/api/tickers/DOES-NOT-EXIST"},
	} {
		req = httptest.NewRequest(target.method, target.path, nil)
		resp = httptest.NewRecorder()
		secured.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized || resp.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("unauthenticated %s %s: status=%d headers=%v body=%s", target.method, target.path, resp.Code, resp.Header(), resp.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/internal/refresh", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp = httptest.NewRecorder()
	secured.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("authenticated refresh: status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestLivenessIsCheapAndReadinessReflectsTrackedState(t *testing.T) {
	service := &fakeService{state: model.NewState()}
	handler := New(service, Config{}).Handler()
	for _, check := range []struct {
		path string
		want int
	}{
		{"/healthz", http.StatusOK},
		{"/readyz", http.StatusServiceUnavailable},
	} {
		req := httptest.NewRequest(http.MethodGet, check.path, nil)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != check.want || resp.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: status=%d cache=%q body=%s", check.path, resp.Code, resp.Header().Get("Cache-Control"), resp.Body.String())
		}
	}
	service.state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN"}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("ready service: status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestStaticFallbackRejectsMissingAssetsButKeepsSPARoutes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<main>app</main>"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := New(&fakeService{state: model.NewState()}, Config{StaticDir: dir}).Handler()
	for _, path := range []string{"/equities/assets/missing.js", "/equities/missing.css", "/equities/favicon.ico"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusNotFound || strings.Contains(resp.Body.String(), "<main>app</main>") {
			t.Errorf("missing file %s: status=%d body=%q", path, resp.Code, resp.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/equities/compare", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "<main>app</main>") {
		t.Fatalf("SPA route: status=%d body=%q", resp.Code, resp.Body.String())
	}
}

func TestIfNoneMatchUsesWeakComparison(t *testing.T) {
	current := `"abc123"`
	for _, header := range []string{`"abc123"`, `W/"abc123"`, `"other", W/"abc123"`, `*`} {
		if !ifNoneMatch(header, current) {
			t.Errorf("expected %q to match %q", header, current)
		}
	}
	for _, header := range []string{"", `W/"other"`, `"abc12"`} {
		if ifNoneMatch(header, current) {
			t.Errorf("expected %q not to match %q", header, current)
		}
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

func TestMetricsExposeBoundedScheduledSnapshotTelemetry(t *testing.T) {
	now := time.Now().UTC()
	state := model.NewState()
	state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN"}
	state.Macro = model.MacroSeries{UpdatedAt: now.Add(-2 * time.Hour), LastSuccessAt: now.Add(-2 * time.Hour), LastAttemptAt: now.Add(-time.Hour), Error: "FRED unavailable"}
	service := &fakeService{
		state: state,
		stats: analysis.Stats{
			SnapshotSchedulerRunning:      true,
			ScheduledSnapshotInFlight:     2,
			ScheduledSnapshotAttempts:     9,
			ScheduledSnapshotSuccesses:    5,
			ScheduledSnapshotNoNewSession: 3,
			ScheduledSnapshotFailures: map[string]int64{
				analysis.SnapshotFailureThrottled: 1,
			},
			HistoryRefreshFailures: map[string]int64{
				analysis.SnapshotFailureThrottled: 2,
			},
			BenchmarkHistoryRefreshFailures: map[string]int64{
				analysis.SnapshotFailureUpstream: 1,
			},
			ScheduledQuoteFieldsExpected: 16,
			ScheduledSnapshotObservations: map[string]analysis.SnapshotObservation{
				"AMZN": {
					LastSuccess:                        now.Add(-25 * time.Minute),
					LastHealthyCheck:                   now.Add(-time.Minute),
					LastObservation:                    now.Add(-25 * time.Minute),
					MarketState:                        "regular",
					QuoteFieldsPresent:                 12,
					HistoryCacheStatus:                 "stale",
					HistoryCacheAsOf:                   now.Add(-13 * time.Hour),
					HistoryRefreshFailureKind:          analysis.SnapshotFailureThrottled,
					BenchmarkHistoryCacheStatus:        "stale",
					BenchmarkHistoryCacheAsOf:          now.Add(-14 * time.Hour),
					BenchmarkHistoryRefreshFailureKind: analysis.SnapshotFailureUpstream,
				},
			},
		},
	}
	handler := New(service, Config{}).Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	body := resp.Body.String()

	if resp.Code != http.StatusOK || resp.Header().Get("Content-Type") != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("metrics response: status=%d content-type=%q", resp.Code, resp.Header().Get("Content-Type"))
	}
	for _, expected := range []string{
		"equities_scheduled_snapshot_scheduler_running 1\n",
		"equities_scheduled_snapshot_inflight 2\n",
		"equities_scheduled_snapshot_attempts_total 9\n",
		"equities_scheduled_snapshot_successes_total 5\n",
		"equities_scheduled_snapshot_no_new_session_total 3\n",
		"equities_macro_degraded 1\n",
		"equities_macro_stale 0\n",
		`equities_scheduled_snapshot_failures_total{reason="throttled"} 1`,
		`equities_scheduled_snapshot_failures_total{reason="other"} 0`,
		`equities_scheduled_snapshot_history_refresh_failures_total{reason="throttled"} 2`,
		`equities_scheduled_snapshot_benchmark_history_refresh_failures_total{benchmark="SPY",reason="upstream"} 1`,
		`equities_scheduled_snapshot_benchmark_history_cache_status{benchmark="SPY",status="stale"} 1`,
		`equities_scheduled_snapshot_stale{ticker="AMZN",market_state="regular"} 1`,
		`equities_scheduled_snapshot_quote_fields_present{ticker="AMZN"} 12`,
		`equities_scheduled_snapshot_quote_fields_expected{ticker="AMZN"} 16`,
		`equities_scheduled_snapshot_quote_field_coverage_ratio{ticker="AMZN"} 0.75`,
		`equities_scheduled_snapshot_history_cache_status{ticker="AMZN",status="stale"} 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("metrics missing %q:\n%s", expected, body)
		}
	}
	if strings.Contains(body, "rate limited") || strings.Contains(body, "NaN") || strings.Contains(body, "+Inf") {
		t.Fatalf("metrics exposed raw errors or invalid values:\n%s", body)
	}
}

func TestPrometheusLabelValueEscapesSpecialCharacters(t *testing.T) {
	if got, want := prometheusLabelValue("A\\\"\nB"), `A\\\"\nB`; got != want {
		t.Fatalf("prometheusLabelValue = %q, want %q", got, want)
	}
}

func TestScheduledSnapshotStaleUsesMarketAwareClock(t *testing.T) {
	now := time.Now().UTC()
	oldObservation := now.Add(-time.Hour)
	if !scheduledSnapshotStale(analysis.SnapshotObservation{MarketState: "regular", LastObservation: oldObservation, LastHealthyCheck: now}, now) {
		t.Fatal("regular market should use provider observation freshness")
	}
	if scheduledSnapshotStale(analysis.SnapshotObservation{MarketState: "closed", LastObservation: oldObservation, LastHealthyCheck: now.Add(-time.Minute)}, now) {
		t.Fatal("closed market should accept a recent healthy no-new-session poll")
	}
	if !scheduledSnapshotStale(analysis.SnapshotObservation{MarketState: "closed", LastObservation: oldObservation, LastHealthyCheck: now.Add(-3 * time.Hour)}, now) {
		t.Fatal("closed market should become stale after the off-hours poll threshold")
	}
	if !scheduledSnapshotStale(analysis.SnapshotObservation{}, now) {
		t.Fatal("missing observations should always be stale")
	}
}
