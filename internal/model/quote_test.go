package model

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNewStatisticSnapshotMapsEveryLiveQuoteStatistic(t *testing.T) {
	value := func(number float64) *float64 { return &number }
	quote := LiveQuote{
		Ticker:                      "AAPL",
		Price:                       value(211.18),
		PreviousClose:               value(209.05),
		Change:                      value(2.13),
		ChangePercent:               value(0.01019),
		AsOf:                        "2026-07-31T15:30:00.123Z",
		MarketState:                 "REGULAR",
		Currency:                    "USD",
		Exchange:                    "NMS",
		Source:                      "Yahoo Finance chart + SEC companyfacts",
		FieldSources:                map[string]string{"asOf": "Yahoo Finance chart timestamp", "movingAverage50Day": "Yahoo Finance completed-session closes / calculated", "marketCapB": "Yahoo Finance price + SEC shares outstanding / calculated", "lastDividendDate": "SEC companyfacts"},
		Beta5YMonthly:               value(1.21),
		BetaBenchmark:               "SPY",
		Change52Week:                value(0.17),
		High52Week:                  value(237.49),
		Low52Week:                   value(169.21),
		MovingAverage50Day:          value(205.12),
		MovingAverage200Day:         value(198.44),
		AverageVolume3Month:         value(54_000_000),
		AverageVolume10Day:          value(47_000_000),
		TrailingAnnualDividendRate:  value(1.04),
		TrailingAnnualDividendYield: value(0.0049),
		ForwardAnnualDividendRate:   value(1.08),
		ForwardAnnualDividendYield:  value(0.0051),
		AverageDividendYield5Year:   value(0.0062),
		ExDividendDate:              "2026-08-10",
		LastDividendDate:            "2026-08-13",
		LastSplitFactor:             "4:1",
		LastSplitDate:               "2020-08-31",
		MarketCapB:                  value(3_150),
		EnterpriseValueB:            value(3_120),
		SharesOutstandingB:          value(14.916),
		ShareBasisAsOf:              "2026-06-27",
	}

	snapshot := NewStatisticSnapshot(quote)
	if snapshot.AsOf != quote.AsOf || snapshot.Source != quote.Source || snapshot.AsOfSource != quote.FieldSources["asOf"] {
		t.Fatalf("snapshot metadata lost: %+v", snapshot)
	}
	if len(snapshot.Numeric) != 20 {
		t.Fatalf("numeric metrics = %d, want 20: %#v", len(snapshot.Numeric), snapshot.Numeric)
	}
	if len(snapshot.Text) != 9 {
		t.Fatalf("text metrics = %d, want 9: %#v", len(snapshot.Text), snapshot.Text)
	}
	if snapshot.Numeric["market-cap"] != 3_150 || snapshot.Numeric["moving-average-50d"] != 205.12 {
		t.Fatalf("numeric metric mapping failed: %#v", snapshot.Numeric)
	}
	if snapshot.Text["dividend-date"] != "2026-08-13" || snapshot.Text["share-basis-as-of"] != "2026-06-27" {
		t.Fatalf("text metric mapping failed: %#v", snapshot.Text)
	}
	if snapshot.Sources["moving-average-50d"] != quote.FieldSources["movingAverage50Day"] || snapshot.Sources["market-cap"] != quote.FieldSources["marketCapB"] {
		t.Fatalf("per-metric provenance mapping failed: %#v", snapshot.Sources)
	}

	encoded, err := json.Marshal(LiveQuote{Ticker: quote.Ticker, History: []StatisticSnapshot{snapshot}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"history"`)) || !bytes.Contains(encoded, []byte(`"market-cap":3150`)) {
		t.Fatalf("live quote JSON omitted history: %s", encoded)
	}
}

func TestNewStatisticSnapshotOmitsAbsentValues(t *testing.T) {
	snapshot := NewStatisticSnapshot(LiveQuote{AsOf: "2026-07-31T15:30:00Z"})
	if snapshot.Numeric != nil || snapshot.Text != nil || snapshot.Sources != nil {
		t.Fatalf("empty values should remain omitted: %+v", snapshot)
	}
}

func TestStatisticSnapshotContentEqualTreatsNilAndEmptyMapsEqually(t *testing.T) {
	left := StatisticSnapshot{AsOf: "2026-07-31T15:30:00Z", Source: "fixture"}
	right := StatisticSnapshot{
		AsOf:    "2026-07-31T11:30:00-04:00",
		Source:  "fixture",
		Numeric: map[string]float64{},
		Text:    map[string]string{},
		Sources: map[string]string{},
	}
	if !StatisticSnapshotContentEqual(left, right) {
		t.Fatal("timestamp formatting and nil/empty maps should not change matched snapshot content")
	}
	right.Numeric["price"] = 101
	if StatisticSnapshotContentEqual(left, right) {
		t.Fatal("numeric correction was treated as identical content")
	}
}
