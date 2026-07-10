package analysis

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestExtractAnnualsUsesLatestFilingAndMergesMetrics(t *testing.T) {
	response := companyFacts{
		EntityName: "Example Inc.",
		Facts: map[string]map[string]factConcept{
			"us-gaap": {
				"RevenueFromContractWithCustomerExcludingAssessedTax": {Units: map[string][]fact{"USD": {
					{Start: "2023-01-01", End: "2023-12-31", Val: 100e9, FP: "FY", Form: "10-K", Filed: "2024-02-01"},
					{Start: "2023-01-01", End: "2023-12-31", Val: 101e9, FP: "FY", Form: "10-K", Filed: "2025-02-01"},
				}}},
				"NetIncomeLoss": {Units: map[string][]fact{"USD": {
					{Start: "2023-01-01", End: "2023-12-31", Val: 20e9, FP: "FY", Form: "10-K", Filed: "2024-02-01"},
				}}},
				"EarningsPerShareDiluted": {Units: map[string][]fact{"USD/shares": {
					{Start: "2023-01-01", End: "2023-12-31", Val: 4.25, FP: "FY", Form: "10-K", Filed: "2024-02-01"},
				}}},
			},
		},
	}
	rows, err := extractAnnuals(response)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one annual row, got %d", len(rows))
	}
	if rows[0].RevenueB == nil || *rows[0].RevenueB != 101 {
		t.Fatalf("latest filing was not selected: %#v", rows[0].RevenueB)
	}
	if rows[0].NetIncomeB == nil || *rows[0].NetIncomeB != 20 {
		t.Fatalf("net income missing: %#v", rows[0].NetIncomeB)
	}
	if rows[0].DilutedEPS == nil || *rows[0].DilutedEPS != 4.25 {
		t.Fatalf("EPS missing: %#v", rows[0].DilutedEPS)
	}
}

func TestDecodeThetaRowsSupportsArrayAndEnvelope(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`[{"created":"2026-01-02T17:15:00.000","close":123.4}]`),
		[]byte(`{"response":[{"created":"2026-01-02T17:15:00.000","close":123.4}]}`),
	} {
		rows, err := decodeThetaRows(body)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].Close != 123.4 {
			t.Fatalf("unexpected rows: %#v", rows)
		}
	}
}

func TestCompositeMarketFiltersTypedNilProvider(t *testing.T) {
	var theta *ThetaMarket
	market := NewCompositeMarket(theta)
	_, _, err := market.History(context.Background(), "AMZN", time.Now().AddDate(-1, 0, 0), time.Now())
	if !errors.Is(err, ErrNoMarketProvider) {
		t.Fatalf("expected no provider error, got %v", err)
	}
}

func TestNormalizeEPSForStockSplits(t *testing.T) {
	gaap := map[string]factConcept{
		"StockholdersEquityNoteStockSplitConversionRatio1": {Units: map[string][]fact{"pure": {
			{End: "2021-06-03", Val: 4},
			{End: "2021-07-19", Val: 4},
			{End: "2024-05-31", Val: 10},
			{End: "2024-06-30", Val: 10},
		}}},
	}
	eps := map[string]fact{
		"old":    {Val: 40, Filed: "2020-02-01"},
		"middle": {Val: 20, Filed: "2022-02-01"},
		"new":    {Val: 2, Filed: "2025-02-01"},
	}
	normalizeEPSForSplits(eps, stockSplitEvents(gaap))
	if eps["old"].Val != 1 || eps["middle"].Val != 2 || eps["new"].Val != 2 {
		t.Fatalf("unexpected normalized EPS values: %#v", eps)
	}
}

func TestExtractAnnualsSupportsProductiveAssetCapex(t *testing.T) {
	response := companyFacts{Facts: map[string]map[string]factConcept{"us-gaap": {
		"Revenues":                          {Units: map[string][]fact{"USD": {{Start: "2024-01-01", End: "2024-12-31", Val: 100e9, FP: "FY", Form: "10-K", Filed: "2025-02-01"}}}},
		"PaymentsToAcquireProductiveAssets": {Units: map[string][]fact{"USD": {{Start: "2024-01-01", End: "2024-12-31", Val: 7e9, FP: "FY", Form: "10-K", Filed: "2025-02-01"}}}},
	}}}
	rows, err := extractAnnuals(response)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].CapexB == nil || *rows[0].CapexB != 7 {
		t.Fatalf("productive-asset capex not extracted: %#v", rows)
	}
}
