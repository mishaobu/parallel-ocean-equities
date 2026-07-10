package analysis

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func TestYahooMarketDecodesMonthlyClosesAtMonthEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("interval") != "1mo" || r.Header.Get("User-Agent") != "parallel-ocean-equities/1.0" {
			t.Fatalf("unexpected request: %s user-agent=%s", r.URL.String(), r.Header.Get("User-Agent"))
		}
		fmt.Fprint(w, `{"chart":{"result":[{"timestamp":[1704067200,1706745600,1709251200],"indicators":{"quote":[{"close":[100,null,120]}]}}],"error":null}}`)
	}))
	defer server.Close()
	provider := NewYahooMarket(server.Client())
	provider.baseURL = server.URL
	end := time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC)
	prices, source, err := provider.History(context.Background(), "AMZN", time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC), end)
	if err != nil {
		t.Fatal(err)
	}
	if source != "Yahoo Finance monthly closes" || len(prices) != 2 {
		t.Fatalf("source=%q prices=%v", source, prices)
	}
	if prices[0].Date != "2024-01-31" || prices[1].Date != "2024-03-01" {
		t.Fatalf("unexpected normalized dates: %v", prices)
	}
}

func TestCompositeMarketPrefersFirstProviderWithDecadeCoverage(t *testing.T) {
	short := &fakeMarketProvider{rows: []model.PricePoint{{Date: "2024-01-01", Close: 1}, {Date: "2026-01-01", Close: 2}}, source: "short"}
	long := &fakeMarketProvider{rows: []model.PricePoint{{Date: "2012-01-01", Close: 1}, {Date: "2026-01-01", Close: 2}}, source: "long"}
	unused := &fakeMarketProvider{err: fmt.Errorf("should not be called")}
	rows, source, err := NewCompositeMarket(short, long, unused).History(context.Background(), "AMZN", time.Time{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if source != "long" || len(rows) != 2 || unused.called {
		t.Fatalf("source=%q rows=%v unused.called=%v", source, rows, unused.called)
	}
}

type fakeMarketProvider struct {
	rows   []model.PricePoint
	source string
	err    error
	called bool
}

func (f *fakeMarketProvider) History(context.Context, string, time.Time, time.Time) ([]model.PricePoint, string, error) {
	f.called = true
	return f.rows, f.source, f.err
}
