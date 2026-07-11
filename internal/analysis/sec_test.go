package analysis

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func TestExtractQuarterliesBuildsDiscreteQ4AndBalanceSheet(t *testing.T) {
	response := companyFacts{Facts: map[string]map[string]factConcept{"us-gaap": {
		"RevenueFromContractWithCustomerExcludingAssessedTax": durationConcept(
			fact{Start: "2024-01-01", End: "2024-03-31", Val: 100e9, Accn: "0000000001-24-000001", FY: 2024, FP: "Q1", Form: "10-Q", Filed: "2024-04-20"},
			fact{Start: "2024-01-01", End: "2024-03-31", Val: 100e9, Accn: "0000000001-25-000001", FY: 2025, FP: "Q1", Form: "10-Q", Filed: "2025-04-20"},
			fact{Start: "2024-04-01", End: "2024-06-30", Val: 120e9, Accn: "0000000001-24-000002", FY: 2024, FP: "Q2", Form: "10-Q", Filed: "2024-07-20"},
			fact{Start: "2024-07-01", End: "2024-09-30", Val: 130e9, Accn: "0000000001-24-000003", FY: 2024, FP: "Q3", Form: "10-Q", Filed: "2024-10-20"},
			fact{Start: "2024-01-01", End: "2024-12-31", Val: 500e9, Accn: "0000000001-25-000004", FY: 2024, FP: "FY", Form: "10-K", Filed: "2025-02-20"},
		),
		"OperatingIncomeLoss": durationConcept(
			quarterDuration(2024, "Q1", "2024-01-01", "2024-03-31", 20e9),
			quarterDuration(2024, "Q2", "2024-04-01", "2024-06-30", 25e9),
			quarterDuration(2024, "Q3", "2024-07-01", "2024-09-30", 30e9),
			annualDuration(2024, 120e9),
		),
		"DepreciationDepletionAndAmortization": durationConcept(
			quarterDuration(2024, "Q1", "2024-01-01", "2024-03-31", 5e9),
			quarterDuration(2024, "Q2", "2024-04-01", "2024-06-30", 6e9),
			quarterDuration(2024, "Q3", "2024-07-01", "2024-09-30", 7e9),
			annualDuration(2024, 30e9),
		),
		"NetCashProvidedByUsedInOperatingActivities": durationConcept(
			quarterDuration(2024, "Q1", "2024-01-01", "2024-03-31", 30e9),
			quarterDuration(2024, "Q2", "2024-04-01", "2024-06-30", 35e9),
			quarterDuration(2024, "Q3", "2024-07-01", "2024-09-30", 40e9),
			annualDuration(2024, 160e9),
		),
		"PaymentsToAcquirePropertyPlantAndEquipment": durationConcept(
			quarterDuration(2024, "Q1", "2024-01-01", "2024-03-31", 10e9),
			quarterDuration(2024, "Q2", "2024-04-01", "2024-06-30", 11e9),
			quarterDuration(2024, "Q3", "2024-07-01", "2024-09-30", 12e9),
			annualDuration(2024, 50e9),
		),
		"WeightedAverageNumberOfDilutedSharesOutstanding": {Units: map[string][]fact{"shares": {
			quarterDuration(2024, "Q1", "2024-01-01", "2024-03-31", 1e9),
			quarterDuration(2024, "Q2", "2024-04-01", "2024-06-30", 1e9),
			quarterDuration(2024, "Q3", "2024-07-01", "2024-09-30", 1e9),
			annualDuration(2024, 1e9),
		}}},
		"CashAndCashEquivalentsAtCarryingValue": instantConcept(2024, "FY", 40e9),
		"MarketableSecuritiesCurrent":           instantConcept(2024, "FY", 10e9),
		"LongTermDebtCurrent":                   instantConcept(2024, "FY", 5e9),
		"LongTermDebtNoncurrent":                instantConcept(2024, "FY", 25e9),
		"Assets":                                instantConcept(2024, "FY", 300e9),
		"StockholdersEquity":                    instantConcept(2024, "FY", 180e9),
	}}}

	rows, err := extractQuarterlies(response, "0000000001")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("expected four quarters, got %d", len(rows))
	}
	q4 := rows[3]
	assertFloat(t, "Q4 revenue", q4.RevenueB, 150)
	assertFloat(t, "Q4 EBITDA", q4.EBITDAB, 57)
	assertFloat(t, "Q4 FCF", q4.FCFB, 38)
	assertFloat(t, "Q4 diluted shares", q4.DilutedSharesB, 1)
	assertFloat(t, "Q4 net debt", q4.NetDebtB, -20)
	assertFloat(t, "Q4 liabilities", q4.LiabilitiesB, 120)
	assertFloat(t, "Q4 dividends", q4.DividendsB, 0)
	if !q4.Derived || q4.Form != "10-K" || !strings.Contains(q4.FilingURL, "000000000125000004") {
		t.Fatalf("unexpected Q4 filing metadata: %#v", q4)
	}
	if rows[0].FiscalYear != 2024 || rows[0].Accession != "0000000001-24-000001" {
		t.Fatalf("comparative duplicate replaced the original quarter: %#v", rows[0])
	}
}

func TestDurationQuarterFactsPreservesComparativePeriodsByEndDate(t *testing.T) {
	gaap := map[string]factConcept{
		"Depreciation": durationConcept(
			fact{Start: "2024-07-01", End: "2024-09-30", Val: 4e9, FY: 2026, FP: "Q1", Form: "10-Q", Filed: "2025-10-29"},
			fact{Start: "2024-10-01", End: "2024-12-31", Val: 5e9, FY: 2026, FP: "Q2", Form: "10-Q", Filed: "2026-01-28"},
			fact{Start: "2025-01-01", End: "2025-03-31", Val: 6e9, FY: 2026, FP: "Q3", Form: "10-Q", Filed: "2026-04-29"},
			fact{Start: "2024-07-01", End: "2025-06-30", Val: 22e9, FY: 2025, FP: "FY", Form: "10-K", Filed: "2025-07-30"},
			fact{Start: "2025-07-01", End: "2025-09-30", Val: 7e9, FY: 2026, FP: "Q1", Form: "10-Q", Filed: "2025-10-29"},
		),
	}
	values := durationQuarterFacts(gaap, daTags, "USD")
	assertQuarterFact(t, "comparative Q1", values["2024-09-30"], 4e9)
	assertQuarterFact(t, "derived Q4", values["2025-06-30"], 7e9)
	assertQuarterFact(t, "current Q1", values["2025-09-30"], 7e9)
}

func TestExtractQuarterliesUsesCombinedShortAndLongTermDebt(t *testing.T) {
	response := companyFacts{Facts: map[string]map[string]factConcept{"us-gaap": {
		"RevenueFromContractWithCustomerExcludingAssessedTax": durationConcept(
			quarterDuration(2024, "Q1", "2024-01-01", "2024-03-31", 100e9),
		),
		"CashAndCashEquivalentsAtCarryingValue":  instantConceptAt(2024, "Q1", "2024-03-31", 10e9),
		"DebtLongtermAndShorttermCombinedAmount": instantConceptAt(2024, "Q1", "2024-03-31", 50e9),
	}}}

	rows, err := extractQuarterlies(response, "0000000001")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one quarter, got %d", len(rows))
	}
	assertFloat(t, "combined debt", rows[0].DebtB, 50)
	assertFloat(t, "combined debt net debt", rows[0].NetDebtB, 40)
}

func durationConcept(values ...fact) factConcept {
	return factConcept{Units: map[string][]fact{"USD": values}}
}

func quarterDuration(year int, quarter, start, end string, value float64) fact {
	return fact{Start: start, End: end, Val: value, Accn: "0000000001-24-000001", FY: year, FP: quarter, Form: "10-Q", Filed: end}
}

func annualDuration(year int, value float64) fact {
	return fact{Start: "2024-01-01", End: "2024-12-31", Val: value, Accn: "0000000001-25-000004", FY: year, FP: "FY", Form: "10-K", Filed: "2025-02-20"}
}

func instantConcept(year int, period string, value float64) factConcept {
	return factConcept{Units: map[string][]fact{"USD": {{End: "2024-12-31", Val: value, Accn: "0000000001-25-000004", FY: year, FP: period, Form: "10-K", Filed: "2025-02-20"}}}}
}

func instantConceptAt(year int, period, end string, value float64) factConcept {
	return factConcept{Units: map[string][]fact{"USD": {{End: end, Val: value, Accn: "0000000001-24-000001", FY: year, FP: period, Form: "10-Q", Filed: end}}}}
}

func assertFloat(t *testing.T, label string, actual *float64, expected float64) {
	t.Helper()
	if actual == nil || *actual != expected {
		t.Fatalf("%s: expected %v, got %#v", label, expected, actual)
	}
}

func assertQuarterFact(t *testing.T, label string, actual quarterFact, expected float64) {
	t.Helper()
	if actual.periodEnd == "" || actual.value != expected {
		t.Fatalf("%s: expected %v, got %#v", label, expected, actual)
	}
}

func TestExtractAnnualsUsesFirstAvailableFilingAndMergesMetrics(t *testing.T) {
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
	if rows[0].RevenueB == nil || *rows[0].RevenueB != 100 || rows[0].FiledAt != "2024-02-01" {
		t.Fatalf("first available filing was not selected: %#v", rows[0])
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

func TestMergePredecessorFilingHistoryKeepsEarliestAvailability(t *testing.T) {
	annuals := mergeAnnualHistory(
		[]model.AnnualPoint{{PeriodEnd: "2010-12-31", FiledAt: "2011-02-01", RevenueB: floatPtr(10)}},
		[]model.AnnualPoint{{PeriodEnd: "2010-12-31", FiledAt: "2012-02-01", RevenueB: floatPtr(11)}, {PeriodEnd: "2011-12-31", FiledAt: "2012-02-01"}},
	)
	if len(annuals) != 2 || annuals[0].RevenueB == nil || *annuals[0].RevenueB != 10 {
		t.Fatalf("unexpected annual merge: %#v", annuals)
	}
	quarters := mergeQuarterlyHistory(
		[]model.QuarterlyPoint{{PeriodEnd: "2010-03-31", FiledAt: "2010-05-01"}},
		[]model.QuarterlyPoint{{PeriodEnd: "2010-03-31", FiledAt: "2011-05-01"}, {PeriodEnd: "2010-06-30", FiledAt: "2010-08-01"}},
	)
	if len(quarters) != 2 || quarters[0].FiledAt != "2010-05-01" {
		t.Fatalf("unexpected quarterly merge: %#v", quarters)
	}
}

func TestPolygonTickerLookup(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing bearer authorization: %q", r.Header.Get("Authorization"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"results":{"ticker":"NVDA","name":"Nvidia Corp","cik":"0001045810"}}`)),
		}, nil
	})}

	client := NewSECClient("test", "test-key", httpClient)
	client.polygonBaseURL = "https://polygon.test"
	company, err := client.lookupPolygon(context.Background(), "NVDA")
	if err != nil {
		t.Fatal(err)
	}
	if company.CIK != 1045810 || company.Ticker != "NVDA" || company.Title != "Nvidia Corp" {
		t.Fatalf("unexpected company: %#v", company)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
