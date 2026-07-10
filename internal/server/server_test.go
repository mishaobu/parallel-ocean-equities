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

func (f *fakeService) Snapshot() model.State         { return f.state }
func (f *fakeService) Stats() analysis.Stats         { return analysis.Stats{} }
func (f *fakeService) DeleteTicker(string) error     { return nil }
func (f *fakeService) Queue(string) bool             { return true }
func (f *fakeService) RefreshAll() int               { return 1 }
func (f *fakeService) AddTicker(ticker string) error { f.added = ticker; return nil }

func TestBasePathAndTickerAPI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<main>equities</main>"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := model.NewState()
	state.UpdatedAt = time.Now()
	state.Tickers["AMZN"] = &model.Equity{Ticker: "AMZN", Status: "ready"}
	service := &fakeService{state: state}
	handler := New(service, Config{BasePath: "/equities", StaticDir: dir}).Handler()

	req := httptest.NewRequest(http.MethodGet, "/equities/", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !bytes.Contains(resp.Body.Bytes(), []byte("equities")) {
		t.Fatalf("static response: %d %s", resp.Code, resp.Body.String())
	}

	body, _ := json.Marshal(map[string]string{"ticker": "NVDA"})
	req = httptest.NewRequest(http.MethodPost, "/equities/api/tickers", bytes.NewReader(body))
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted || service.added != "NVDA" {
		t.Fatalf("add response: %d ticker=%s body=%s", resp.Code, service.added, resp.Body.String())
	}
}
