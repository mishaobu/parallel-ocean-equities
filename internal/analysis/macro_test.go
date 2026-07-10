package analysis

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestFREDClientBuildsDerivedMacroSeries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		w.Header().Set("Content-Type", "text/csv")
		fmt.Fprintf(w, "DATE,%s\n", id)
		switch id {
		case "CPIAUCSL":
			fmt.Fprintln(w, "2024-01-01,100\n2025-01-01,103")
		case "M1SL":
			fmt.Fprintln(w, "2024-01-01,1000\n2025-01-01,1100")
		case "M2SL":
			fmt.Fprintln(w, "2024-01-01,2000\n2025-01-01,2200")
		case "WALCL":
			fmt.Fprintln(w, "2025-01-01,7000000\n2025-01-29,8000000")
		case "FEDFUNDS":
			fmt.Fprintln(w, "2025-01-01,4.5")
		case "GS2":
			fmt.Fprintln(w, "2025-01-01,4.0")
		case "GS10":
			fmt.Fprintln(w, "2025-01-01,4.4")
		case "BAMLC0A0CM":
			fmt.Fprintln(w, "2025-01-01,1.1")
		default:
			fmt.Fprintln(w, "2025-01-01,1")
		}
	}))
	defer server.Close()

	client := NewFREDClient("test", server.Client())
	client.baseURL = server.URL
	series, err := client.Analyze(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Sources) != len(fredSeriesIDs) {
		t.Fatalf("sources = %v", series.Sources)
	}
	latest := series.Points[len(series.Points)-1]
	assertClose(t, "inflation", latest.Inflation, 3)
	assertClose(t, "real policy rate", latest.RealPolicyRate, 1.5)
	assertClose(t, "yield curve", latest.YieldCurve, 0.4)
	assertClose(t, "real 10Y", latest.Real10Y, 3.4)
	assertClose(t, "M1 growth", latest.M1Growth, 10)
	assertClose(t, "M2 growth", latest.M2Growth, 10)
	assertClose(t, "Fed assets log", latest.LogFedAssets, math.Log10(8000))
}

func TestFREDClientKeepsOptionalSeriesFailureAsWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "VIXCLS" {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintf(w, "DATE,%s\n2025-01-01,1\n", id)
	}))
	defer server.Close()
	client := NewFREDClient("test", server.Client())
	client.baseURL = server.URL
	series, err := client.Analyze(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Warnings) != 1 || !strings.Contains(series.Warnings[0], "VIXCLS") {
		t.Fatalf("warnings = %v", series.Warnings)
	}
	if len(series.Points) != 1 || series.Points[0].VIX != nil {
		t.Fatalf("unexpected points: %#v", series.Points)
	}
}

func TestDecodeFREDCSVUsesLastObservationInMonth(t *testing.T) {
	values, err := decodeFREDCSV(strings.NewReader("DATE,VALUE\n2025-01-02,1\n2025-01-31,2\n2025-02-01,.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if values["2025-01"] != 2 {
		t.Fatalf("January = %v, want 2", values["2025-01"])
	}
}

func TestFREDClientRetriesTransientFailure(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) < 3 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		fmt.Fprintln(w, "DATE,VALUE\n2025-01-01,4.5")
	}))
	defer server.Close()
	client := NewFREDClient("test", server.Client())
	client.baseURL = server.URL

	values, err := client.fetch(context.Background(), "FEDFUNDS")
	if err != nil {
		t.Fatal(err)
	}
	if values["2025-01"] != 4.5 || requests.Load() != 3 {
		t.Fatalf("values=%v requests=%d", values, requests.Load())
	}
}
