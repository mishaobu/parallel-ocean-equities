package analysis

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func TestOfficialCountryClientOverlaysEuroAreaAndFlagsOtherStaleSeries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ons" {
			fmt.Fprint(w, "\"Title\",\"Example\"\n\"2024 MAY\",\"100\"\n\"2025 MAY\",\"104\"\n")
			return
		}
		if r.URL.Path == "/ecb/BSI/M.U2.Y.V.M30.X.1.U2.2300.Z01.E" {
			fmt.Fprint(w, "TIME_PERIOD,OBS_VALUE\n2024-05,100\n2025-05,110\n")
			return
		}
		fmt.Fprint(w, `{"value":{"0":100,"1":104},"dimension":{"time":{"category":{"index":{"2024-05":0,"2025-05":1}}}}}`)
	}))
	defer server.Close()
	client := NewOfficialCountryClient(server.Client())
	client.eurostatURL = server.URL
	client.ecbURL = server.URL + "/ecb"
	client.onsURL = server.URL + "/ons"
	client.now = func() time.Time { return time.Date(2025, time.June, 1, 0, 0, 0, 0, time.UTC) }
	countries := []model.CountrySeries{
		{Code: "EA", Points: []model.CountryPoint{{Date: "2024-05-01"}}},
		{Code: "GB", Points: []model.CountryPoint{{Date: "2024-05-01"}}},
		{Code: "CN", Points: []model.CountryPoint{{Date: "2020-01-01", Inflation: floatPtr(2), InflationDate: "2020-01-01"}}},
	}
	got, warnings := client.Enrich(context.Background(), countries)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	ea := got[0].Points[len(got[0].Points)-1]
	assertClose(t, "official inflation", ea.Inflation, 104)
	assertClose(t, "official money growth", ea.MoneyGrowth, 10)
	gb := got[1].Points[len(got[1].Points)-1]
	assertClose(t, "ONS inflation", gb.Inflation, 104)
	assertClose(t, "ONS industrial growth", gb.IndustrialGrowth, 4)
	if !containsString(got[2].Warnings, "CN industrial unavailable") || !containsString(got[2].Warnings, "CN inflation stale") {
		t.Fatalf("CN warnings = %v", got[2].Warnings)
	}
}

func TestONSPeriodMonthParsesMonthlyAndQuarterlyRows(t *testing.T) {
	if got := onsPeriodMonth("2026 MAR"); got != "2026-03" {
		t.Fatalf("monthly period = %q", got)
	}
	if got := onsPeriodMonth("2026 Q1"); got != "2026-03" {
		t.Fatalf("quarterly period = %q", got)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.Contains(value, target) {
			return true
		}
	}
	return false
}
