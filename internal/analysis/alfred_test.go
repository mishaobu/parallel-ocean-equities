package analysis

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func TestALFREDClientBuildsAndReusesPointInTimeQuarters(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		id := r.URL.Query().Get("id")
		vintage := r.URL.Query().Get("vintage_date")
		if vintage == "" || r.URL.Query().Get("coed") != vintage {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		fmt.Fprintf(w, "observation_date,%s_%s\n2023-11-01,100\n2024-11-01,104\n", id, vintage)
	}))
	defer server.Close()
	client := NewALFREDClient("test", server.Client())
	client.baseURL = server.URL
	client.now = func() time.Time { return time.Date(1994, time.January, 2, 0, 0, 0, 0, time.UTC) }

	series, err := client.Analyze(context.Background(), model.VintageSeries{})
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Points) != 1 || series.Points[0].VintageDate != "1993-12-31" {
		t.Fatalf("points = %#v", series.Points)
	}
	assertClose(t, "vintage inflation", series.Points[0].Inflation, 4)
	firstRequests := requests.Load()
	series, err = client.Analyze(context.Background(), series)
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != firstRequests {
		t.Fatalf("persisted quarter was refetched: %d -> %d", firstRequests, requests.Load())
	}
}

func TestLatestVintageGrowthUsesLatestComparableObservation(t *testing.T) {
	value, date := latestVintageGrowth(map[string]float64{"2023-10": 100, "2024-10": 103, "2024-11": 104})
	assertClose(t, "growth", value, 3)
	if date != "2024-10-01" {
		t.Fatalf("date = %q", date)
	}
}
