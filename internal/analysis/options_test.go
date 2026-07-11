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

func TestThetaOptionsClientBuildsBoundedSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v3/option/list/expirations":
			fmt.Fprint(w, `[{"symbol":"SPY","expiration":"2025-02-14"},{"symbol":"SPY","expiration":"2025-03-14"},{"symbol":"SPY","expiration":"2025-04-18"}]`)
		case "/v3/option/history/greeks/implied_volatility":
			if r.URL.Query().Get("interval") != "1h" || r.URL.Query().Get("strike_range") != "4" {
				t.Fatalf("unbounded IV query: %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"response":[{"contract":{"symbol":"SPY","expiration":"2025-02-14","strike":95,"right":"PUT"},"data":[{"timestamp":"2025-01-10T16:00:00.000","midpoint":1.2,"bid_implied_vol":0.24,"ask_implied_vol":0.26,"underlying_price":100}]},{"contract":{"symbol":"SPY","expiration":"2025-02-14","strike":100,"right":"PUT"},"data":[{"timestamp":"2025-01-10T16:00:00.000","midpoint":2.0,"bid_implied_vol":0.19,"ask_implied_vol":0.21,"underlying_price":100}]},{"contract":{"symbol":"SPY","expiration":"2025-02-14","strike":100,"right":"CALL"},"data":[{"timestamp":"2025-01-10T16:00:00.000","midpoint":2.2,"bid_implied_vol":0.18,"ask_implied_vol":0.20,"underlying_price":100}]},{"contract":{"symbol":"SPY","expiration":"2025-02-14","strike":105,"right":"CALL"},"data":[{"timestamp":"2025-01-10T16:00:00.000","midpoint":1.1,"bid_implied_vol":0.17,"ask_implied_vol":0.19,"underlying_price":100}]}]}`)
		case "/v3/stock/history/eod":
			fmt.Fprint(w, `[`)
			for index := 0; index < 22; index++ {
				if index > 0 {
					fmt.Fprint(w, ",")
				}
				fmt.Fprintf(w, `{"created":"2025-01-%02d","close":%d}`, index+1, 100+index%3)
			}
			fmt.Fprint(w, `]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewThetaOptionsClient(server.URL, []string{"SPY"}, server.Client())
	client.now = func() time.Time { return time.Date(2025, time.January, 11, 8, 0, 0, 0, time.UTC) }
	series, err := client.Analyze(context.Background(), model.OptionsSeries{})
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Snapshots) != 1 || len(series.Snapshots[0].Terms) != 3 {
		t.Fatalf("series = %#v", series)
	}
	if series.Snapshots[0].AsOf != "2025-01-10" {
		t.Fatalf("snapshot as-of = %q", series.Snapshots[0].AsOf)
	}
	assertClose(t, "ATM IV", series.Snapshots[0].ATMIV30D, 0.195)
	assertClose(t, "skew", series.Snapshots[0].Skew30D, 7)
	assertClose(t, "straddle move", series.Snapshots[0].Terms[0].StraddleMove, 4.2)
	if len(series.History) != 1 || series.History[0].Date != "2025-01-10" {
		t.Fatalf("history = %#v", series.History)
	}
}

func TestMergeOptionHistoryDeduplicatesTickerDate(t *testing.T) {
	old := 20.0
	fresh := 25.0
	rows := mergeOptionHistory([]model.OptionHistoryPoint{{Ticker: "SPY", Date: "2025-01-10", ATMIV30D: &old}}, []model.OptionSnapshot{{Ticker: "SPY", AsOf: "2025-01-10", ATMIV30D: &fresh}})
	if len(rows) != 1 || rows[0].ATMIV30D == nil || *rows[0].ATMIV30D != fresh {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestThetaOptionsClientSkipsMarketWindow(t *testing.T) {
	client := NewThetaOptionsClient("http://unused", []string{"SPY"}, nil)
	client.now = func() time.Time { return time.Date(2025, time.January, 13, 15, 0, 0, 0, time.UTC) }
	if _, err := client.Analyze(context.Background(), model.OptionsSeries{}); err == nil {
		t.Fatal("expected market-hours guard")
	}
}

func TestRealizedVolatilityRequiresFullWindow(t *testing.T) {
	if realizedVolatility([]model.PricePoint{{Date: "2025-01-01", Close: 100}}, 20) != nil {
		t.Fatal("short sample should not produce realized volatility")
	}
}
