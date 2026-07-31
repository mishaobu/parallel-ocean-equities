package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func TestLatestActualSharesOutstandingUsesLatestActualPeriod(t *testing.T) {
	response := companyFacts{Facts: map[string]map[string]factConcept{"dei": {
		"EntityCommonStockSharesOutstanding": {Units: map[string][]fact{"shares": {
			{End: "2024-12-31", Val: 1.5e9, Form: "10-K", Filed: "2025-02-01"},
			{End: "2025-03-31", Val: 1.4e9, Form: "10-Q", Filed: "2025-04-20"},
			{End: "2025-03-31", Val: 1.35e9, Form: "10-Q/A", Filed: "2025-04-30"},
			{End: "2025-06-30", Val: 1.3e9, Form: "8-K", Filed: "2025-07-15"},
			{Start: "2025-04-01", End: "2025-06-30", Val: 1.25e9, Form: "10-Q", Filed: "2025-07-20"},
			{End: "2025-09-30", Val: 1.2e9, Form: "10-Q", Filed: "2025-07-20"},
		}}},
	}}}

	shares, ok := latestActualSharesOutstanding(response)
	if !ok {
		t.Fatal("expected an actual shares-outstanding fact")
	}
	if shares.Val != 1.35e9 || shares.End != "2025-03-31" || shares.Filed != "2025-04-30" {
		t.Fatalf("unexpected shares-outstanding fact: %#v", shares)
	}
}

func TestSECAnalyzePopulatesOnlyExactSharesOutstanding(t *testing.T) {
	baseFacts := map[string]map[string]factConcept{"us-gaap": {
		"RevenueFromContractWithCustomerExcludingAssessedTax": durationConcept(
			fact{Start: "2024-01-01", End: "2024-12-31", Val: 100e9, Accn: "0000000001-25-000001", FY: 2024, FP: "FY", Form: "10-K", Filed: "2025-02-01"},
			fact{Start: "2025-01-01", End: "2025-03-31", Val: 25e9, Accn: "0000000001-25-000002", FY: 2025, FP: "Q1", Form: "10-Q", Filed: "2025-04-20"},
		),
		"WeightedAverageNumberOfDilutedSharesOutstanding": {Units: map[string][]fact{"shares": {
			{Start: "2024-01-01", End: "2024-12-31", Val: 1.3e9, Accn: "0000000001-25-000001", FY: 2024, FP: "FY", Form: "10-K", Filed: "2025-02-01"},
		}}},
	}}

	analyze := func(t *testing.T, facts map[string]map[string]factConcept) *model.Equity {
		t.Helper()
		payload, err := json.Marshal(companyFacts{EntityName: "Example Inc.", Facts: facts})
		if err != nil {
			t.Fatal(err)
		}
		httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(string(payload))),
			}, nil
		})}
		client := NewSECClient("test", "", httpClient)
		existing := &model.Equity{
			CIK: "0000000001",
			Current: model.CurrentMetrics{
				SharesOutstandingB:    floatPtr(99),
				SharesOutstandingAsOf: "2000-01-01",
			},
		}
		result, err := client.Analyze(context.Background(), "TEST", existing)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	t.Run("exact DEI instant fact", func(t *testing.T) {
		facts := cloneFactNamespaces(baseFacts)
		facts["dei"] = map[string]factConcept{
			"EntityCommonStockSharesOutstanding": {Units: map[string][]fact{"shares": {
				{End: "2025-04-10", Val: 1.25e9, Form: "10-Q", Filed: "2025-04-20"},
			}}},
		}
		result := analyze(t, facts)
		assertFloat(t, "actual shares outstanding", result.Current.SharesOutstandingB, 1.25)
		if result.Current.SharesOutstandingAsOf != "2025-04-10" {
			t.Fatalf("unexpected shares-outstanding as-of date: %q", result.Current.SharesOutstandingAsOf)
		}
		if result.Current.SharesOutstandingSource != deiActualSharesSource {
			t.Fatalf("unexpected shares-outstanding source: %q", result.Current.SharesOutstandingSource)
		}
	})

	t.Run("exact non-dimensional us-gaap instant fallback", func(t *testing.T) {
		facts := cloneFactNamespaces(baseFacts)
		facts["us-gaap"] = cloneFactConcepts(facts["us-gaap"])
		facts["us-gaap"]["CommonStockSharesOutstanding"] = factConcept{Units: map[string][]fact{"shares": {
			{End: "2025-03-31", Val: 1.25e9, Accn: "0000000001-25-000002", Form: "10-Q", Filed: "2025-04-20", Frame: "CY2025Q1I"},
		}}}
		result := analyze(t, facts)
		assertFloat(t, "us-gaap actual shares outstanding", result.Current.SharesOutstandingB, 1.25)
		if result.Current.SharesOutstandingAsOf != "2025-03-31" || result.Current.SharesOutstandingSource != usGAAPActualSharesSource {
			t.Fatalf("unexpected us-gaap share provenance: %#v", result.Current)
		}
	})

	t.Run("DEI remains authoritative", func(t *testing.T) {
		facts := cloneFactNamespaces(baseFacts)
		facts["us-gaap"] = cloneFactConcepts(facts["us-gaap"])
		facts["us-gaap"]["CommonStockSharesOutstanding"] = factConcept{Units: map[string][]fact{"shares": {
			{End: "2025-03-31", Val: 1.25e9, Form: "10-Q", Filed: "2025-04-20"},
		}}}
		facts["dei"] = map[string]factConcept{
			"EntityCommonStockSharesOutstanding": {Units: map[string][]fact{"shares": {
				{End: "2025-03-31", Val: 1.2e9, Form: "10-Q", Filed: "2025-04-20"},
			}}},
		}
		result := analyze(t, facts)
		assertFloat(t, "authoritative DEI shares outstanding", result.Current.SharesOutstandingB, 1.2)
		if result.Current.SharesOutstandingSource != deiActualSharesSource {
			t.Fatalf("us-gaap fallback displaced DEI: %#v", result.Current)
		}
	})

	t.Run("newer us-gaap instant supersedes stale DEI", func(t *testing.T) {
		facts := cloneFactNamespaces(baseFacts)
		facts["us-gaap"] = cloneFactConcepts(facts["us-gaap"])
		facts["us-gaap"]["CommonStockSharesOutstanding"] = factConcept{Units: map[string][]fact{"shares": {
			{End: "2025-03-31", Val: 1.25e9, Form: "10-Q", Filed: "2025-04-20"},
		}}}
		facts["dei"] = map[string]factConcept{
			"EntityCommonStockSharesOutstanding": {Units: map[string][]fact{"shares": {
				{End: "2024-12-31", Val: 1.3e9, Form: "10-K", Filed: "2025-02-01"},
			}}},
		}
		result := analyze(t, facts)
		assertFloat(t, "fresh us-gaap shares outstanding", result.Current.SharesOutstandingB, 1.25)
		if result.Current.SharesOutstandingSource != usGAAPActualSharesSource {
			t.Fatalf("stale DEI displaced newer us-gaap instant: %#v", result.Current)
		}
	})

	t.Run("implausible us-gaap fallback is rejected", func(t *testing.T) {
		facts := cloneFactNamespaces(baseFacts)
		facts["us-gaap"] = cloneFactConcepts(facts["us-gaap"])
		facts["us-gaap"]["CommonStockSharesOutstanding"] = factConcept{Units: map[string][]fact{"shares": {
			{End: "2025-03-31", Val: 9e9, Form: "10-Q", Filed: "2025-04-20"},
		}}}
		result := analyze(t, facts)
		if result.Current.SharesOutstandingB != nil || result.Current.SharesOutstandingAsOf != "" || result.Current.SharesOutstandingSource != "" {
			t.Fatalf("implausible us-gaap share fallback survived: %#v", result.Current)
		}
	})

	t.Run("no diluted-share fallback", func(t *testing.T) {
		result := analyze(t, cloneFactNamespaces(baseFacts))
		if result.Current.SharesOutstandingB != nil || result.Current.SharesOutstandingAsOf != "" {
			t.Fatalf("unexpected shares-outstanding fallback: %#v", result.Current)
		}
	})
}

func TestActualSharesOutstandingRemainRawSECValues(t *testing.T) {
	const accession = "0000000001-24-000001"
	response := companyFacts{Facts: map[string]map[string]factConcept{
		"us-gaap": {
			"RevenueFromContractWithCustomerExcludingAssessedTax": durationConcept(
				fact{Start: "2024-01-01", End: "2024-03-31", Val: 25e9, Accn: accession, FY: 2024, FP: "Q1", Form: "10-Q", Filed: "2024-07-20"},
			),
			"WeightedAverageNumberOfDilutedSharesOutstanding": {Units: map[string][]fact{"shares": {
				// Filed after the split, so the issuer already presented this
				// duration value on the post-split basis.
				{Start: "2024-01-01", End: "2024-03-31", Val: 14e9, Accn: accession, FY: 2024, FP: "Q1", Form: "10-Q", Filed: "2024-07-20"},
			}}},
			"StockholdersEquityNoteStockSplitConversionRatio1": {Units: map[string][]fact{"pure": {
				{End: "2024-06-01", Val: 10, Form: "10-Q", Filed: "2024-07-20"},
			}}},
		},
		"dei": {
			"EntityCommonStockSharesOutstanding": {Units: map[string][]fact{"shares": {
				// The filing came after the split, but the instant fact predates
				// it and therefore still needs the 10:1 adjustment.
				{End: "2024-04-10", Val: 1.35e9, Accn: accession, Form: "10-Q", Filed: "2024-07-20"},
			}}},
		},
	}}

	quarters, err := extractQuarterlies(response, "0000000001")
	if err != nil {
		t.Fatal(err)
	}
	if len(quarters) != 1 {
		t.Fatalf("quarter count = %d", len(quarters))
	}
	assertFloat(t, "raw actual shares", quarters[0].SharesOutstandingB, 1.35)
	if quarters[0].SharesOutstandingAsOf != "2024-04-10" {
		t.Fatalf("share-basis date changed: %q", quarters[0].SharesOutstandingAsOf)
	}
}

func TestExtractHistoricalSharesOutstandingRequiresMatchingFilingAccession(t *testing.T) {
	const (
		quarterAccession = "0000000001-24-000001"
		annualAccession  = "0000000001-25-000004"
	)

	gaap := map[string]factConcept{
		"RevenueFromContractWithCustomerExcludingAssessedTax": durationConcept(
			fact{Start: "2024-01-01", End: "2024-03-31", Val: 25e9, Accn: quarterAccession, FY: 2024, FP: "Q1", Form: "10-Q", Filed: "2024-04-20"},
			fact{Start: "2024-01-01", End: "2024-12-31", Val: 100e9, Accn: annualAccession, FY: 2024, FP: "FY", Form: "10-K", Filed: "2025-02-20"},
		),
		"WeightedAverageNumberOfDilutedSharesOutstanding": {Units: map[string][]fact{"shares": {
			{Start: "2024-01-01", End: "2024-03-31", Val: 1.4e9, Accn: quarterAccession, FY: 2024, FP: "Q1", Form: "10-Q", Filed: "2024-04-20"},
			{Start: "2024-01-01", End: "2024-12-31", Val: 1.3e9, Accn: annualAccession, FY: 2024, FP: "FY", Form: "10-K", Filed: "2025-02-20"},
		}}},
	}
	matchingFacts := []fact{
		{End: "2024-04-10", Val: 1.35e9, Accn: quarterAccession, Form: "10-Q", Filed: "2024-04-20"},
		{End: "2024-12-31", Val: 1.28e9, Accn: annualAccession, Form: "10-K", Filed: "2025-02-20"},
		{End: "2025-02-10", Val: 1.25e9, Accn: annualAccession, Form: "10-K", Filed: "2025-02-20"},
	}

	tests := []struct {
		name        string
		deiFacts    []fact
		usGAAPFacts []fact
		customFacts []fact
		wantAnnual  *float64
		wantQuarter *float64
	}{
		{
			name:        "exact filing accessions",
			deiFacts:    matchingFacts,
			wantAnnual:  floatPtr(1.25),
			wantQuarter: floatPtr(1.35),
		},
		{
			name: "different filing accessions",
			deiFacts: []fact{
				{End: "2024-04-10", Val: 9.35e9, Accn: "0000000001-24-999999", Form: "10-Q", Filed: "2024-04-20"},
				{End: "2025-02-10", Val: 9.25e9, Accn: "0000000001-25-999999", Form: "10-K", Filed: "2025-02-20"},
			},
		},
		{
			name:        "missing DEI fact",
			customFacts: matchingFacts,
		},
		{
			name:        "exact us-gaap fallback by accession",
			usGAAPFacts: matchingFacts,
			wantAnnual:  floatPtr(1.25),
			wantQuarter: floatPtr(1.35),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			facts := map[string]map[string]factConcept{"us-gaap": cloneFactConcepts(gaap)}
			if test.usGAAPFacts != nil {
				facts["us-gaap"]["CommonStockSharesOutstanding"] = factConcept{Units: map[string][]fact{"shares": test.usGAAPFacts}}
			}
			if test.deiFacts != nil {
				facts["dei"] = map[string]factConcept{
					"EntityCommonStockSharesOutstanding": {Units: map[string][]fact{"shares": test.deiFacts}},
				}
			}
			if test.customFacts != nil {
				facts["example"] = map[string]factConcept{
					"EntityCommonStockSharesOutstanding": {Units: map[string][]fact{"shares": test.customFacts}},
				}
			}
			response := companyFacts{Facts: facts}

			annuals, err := extractAnnuals(response)
			if err != nil {
				t.Fatal(err)
			}
			quarters, err := extractQuarterlies(response, "0000000001")
			if err != nil {
				t.Fatal(err)
			}
			if len(annuals) != 1 || len(quarters) != 1 {
				t.Fatalf("unexpected history lengths: annual=%d quarterly=%d", len(annuals), len(quarters))
			}

			assertFloat(t, "annual diluted shares", annuals[0].DilutedSharesB, 1.3)
			assertFloat(t, "quarterly diluted shares", quarters[0].DilutedSharesB, 1.4)
			if test.wantAnnual == nil {
				if annuals[0].SharesOutstandingB != nil || annuals[0].SharesOutstandingAsOf != "" {
					t.Fatalf("unexpected annual actual-share match: %#v", annuals[0])
				}
			} else {
				assertFloat(t, "annual actual shares", annuals[0].SharesOutstandingB, *test.wantAnnual)
				if annuals[0].SharesOutstandingAsOf != "2025-02-10" {
					t.Fatalf("unexpected annual actual-share date: %q", annuals[0].SharesOutstandingAsOf)
				}
				wantSource := deiActualSharesSource
				if test.usGAAPFacts != nil {
					wantSource = usGAAPActualSharesSource
				}
				if annuals[0].SharesOutstandingSource != wantSource {
					t.Fatalf("unexpected annual actual-share source: %q", annuals[0].SharesOutstandingSource)
				}
			}
			if test.wantQuarter == nil {
				if quarters[0].SharesOutstandingB != nil || quarters[0].SharesOutstandingAsOf != "" {
					t.Fatalf("unexpected quarterly actual-share match: %#v", quarters[0])
				}
			} else {
				assertFloat(t, "quarterly actual shares", quarters[0].SharesOutstandingB, *test.wantQuarter)
				if quarters[0].SharesOutstandingAsOf != "2024-04-10" {
					t.Fatalf("unexpected quarterly actual-share date: %q", quarters[0].SharesOutstandingAsOf)
				}
				wantSource := deiActualSharesSource
				if test.usGAAPFacts != nil {
					wantSource = usGAAPActualSharesSource
				}
				if quarters[0].SharesOutstandingSource != wantSource {
					t.Fatalf("unexpected quarterly actual-share source: %q", quarters[0].SharesOutstandingSource)
				}
			}
		})
	}
}

func cloneFactNamespaces(source map[string]map[string]factConcept) map[string]map[string]factConcept {
	clone := make(map[string]map[string]factConcept, len(source))
	for namespace, concepts := range source {
		clone[namespace] = concepts
	}
	return clone
}

func cloneFactConcepts(source map[string]factConcept) map[string]factConcept {
	clone := make(map[string]factConcept, len(source))
	for concept, facts := range source {
		clone[concept] = facts
	}
	return clone
}

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

func TestFiscalPeriodForEndUsesCompanyYearEndInsteadOfComparativeFilingMetadata(t *testing.T) {
	tests := []struct {
		end         string
		yearEnd     time.Month
		wantYear    int
		wantQuarter string
	}{
		{end: "2016-12-31", yearEnd: time.December, wantYear: 2016, wantQuarter: "Q4"},
		{end: "2017-09-30", yearEnd: time.December, wantYear: 2017, wantQuarter: "Q3"},
		{end: "2024-09-30", yearEnd: time.June, wantYear: 2025, wantQuarter: "Q1"},
		{end: "2025-06-30", yearEnd: time.June, wantYear: 2025, wantQuarter: "Q4"},
		{end: "2025-02-01", yearEnd: time.January, wantYear: 2025, wantQuarter: "Q4"},
		{end: "2025-01-04", yearEnd: time.December, wantYear: 2025, wantQuarter: "Q4"},
	}
	for _, test := range tests {
		year, quarter := fiscalPeriodForEnd(test.end, test.yearEnd)
		if year != test.wantYear || quarter != test.wantQuarter {
			t.Fatalf("%s year-end=%s: got FY%d %s, want FY%d %s", test.end, test.yearEnd, year, quarter, test.wantYear, test.wantQuarter)
		}
	}
}

func TestExtractQuarterliesUsesCombinedShortAndLongTermDebt(t *testing.T) {
	response := companyFacts{Facts: map[string]map[string]factConcept{"us-gaap": {
		"RevenueFromContractWithCustomerExcludingAssessedTax": durationConcept(
			quarterDuration(2024, "Q1", "2024-01-01", "2024-03-31", 100e9),
		),
		"GrossProfit": durationConcept(quarterDuration(2024, "Q1", "2024-01-01", "2024-03-31", 40e9)),
		"IncomeLossFromContinuingOperationsBeforeIncomeTaxesMinorityInterestAndIncomeLossFromEquityMethodInvestments": durationConcept(quarterDuration(2024, "Q1", "2024-01-01", "2024-03-31", 20e9)),
		"IncomeTaxExpenseBenefit":                durationConcept(quarterDuration(2024, "Q1", "2024-01-01", "2024-03-31", 4e9)),
		"ShareBasedCompensation":                 durationConcept(quarterDuration(2024, "Q1", "2024-01-01", "2024-03-31", 1e9)),
		"CashAndCashEquivalentsAtCarryingValue":  instantConceptAt(2024, "Q1", "2024-03-31", 10e9),
		"DebtLongtermAndShorttermCombinedAmount": instantConceptAt(2024, "Q1", "2024-03-31", 50e9),
		"InventoryNet":                           instantConceptAt(2024, "Q1", "2024-03-31", 30e9),
		"AccountsReceivableNetCurrent":           instantConceptAt(2024, "Q1", "2024-03-31", 20e9),
		"AccountsPayableCurrent":                 instantConceptAt(2024, "Q1", "2024-03-31", 15e9),
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
	assertFloat(t, "gross profit", rows[0].GrossProfitB, 40)
	assertFloat(t, "pretax income", rows[0].PretaxIncomeB, 20)
	assertFloat(t, "income tax", rows[0].IncomeTaxB, 4)
	assertFloat(t, "stock compensation", rows[0].StockCompB, 1)
	assertFloat(t, "inventory", rows[0].InventoryB, 30)
	assertFloat(t, "receivables", rows[0].ReceivablesB, 20)
	assertFloat(t, "payables", rows[0].PayablesB, 15)
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
				"GrossProfit": {Units: map[string][]fact{"USD": {
					{Start: "2023-01-01", End: "2023-12-31", Val: 45e9, FP: "FY", Form: "10-K", Filed: "2024-02-01"},
				}}},
				"ShareBasedCompensation": {Units: map[string][]fact{"USD": {
					{Start: "2023-01-01", End: "2023-12-31", Val: 2e9, FP: "FY", Form: "10-K", Filed: "2024-02-01"},
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
	assertFloat(t, "annual gross profit", rows[0].GrossProfitB, 45)
	assertFloat(t, "annual stock compensation", rows[0].StockCompB, 2)
}

func TestExtractAnnualsIncludesCurrentBalanceSheetAmounts(t *testing.T) {
	response := companyFacts{Facts: map[string]map[string]factConcept{"us-gaap": {
		"Revenues":           {Units: map[string][]fact{"USD": {{Start: "2024-01-01", End: "2024-12-31", Val: 100e9, FP: "FY", Form: "10-K", Filed: "2025-02-01"}}}},
		"AssetsCurrent":      instantConcept(2024, "FY", 42e9),
		"LiabilitiesCurrent": instantConcept(2024, "FY", 21e9),
	}}}

	rows, err := extractAnnuals(response)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one annual row, got %d", len(rows))
	}
	assertFloat(t, "annual current assets", rows[0].CurrentAssetsB, 42)
	assertFloat(t, "annual current liabilities", rows[0].CurrentLiabilitiesB, 21)
}

func TestExtractQuarterliesIncludesCurrentBalanceSheetAmounts(t *testing.T) {
	response := companyFacts{Facts: map[string]map[string]factConcept{"us-gaap": {
		"Revenues": durationConcept(
			quarterDuration(2024, "Q1", "2024-01-01", "2024-03-31", 25e9),
		),
		"AssetsCurrent":      instantConceptAt(2024, "Q1", "2024-03-31", 12e9),
		"LiabilitiesCurrent": instantConceptAt(2024, "Q1", "2024-03-31", 6e9),
	}}}

	rows, err := extractQuarterlies(response, "0000000001")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one quarterly row, got %d", len(rows))
	}
	assertFloat(t, "quarterly current assets", rows[0].CurrentAssetsB, 12)
	assertFloat(t, "quarterly current liabilities", rows[0].CurrentLiabilitiesB, 6)
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

func TestNormalizeEPSAndSharesForReverseStockSplit(t *testing.T) {
	gaap := map[string]factConcept{
		"StockholdersEquityNoteStockSplitConversionRatio1": {Units: map[string][]fact{"pure": {
			{End: "2026-01-15", Val: 0.1},
		}}},
	}
	events := stockSplitEvents(gaap)
	if len(events) != 1 || events[0].ratio != 0.1 {
		t.Fatalf("reverse split event was not retained: %#v", events)
	}
	eps := map[string]fact{"old": {Val: 2, Filed: "2025-02-01"}}
	shares := map[string]fact{"old": {Val: 10, Filed: "2025-02-01"}}
	normalizeEPSForSplits(eps, events)
	normalizeAnnualSharesForSplits(shares, events)
	if eps["old"].Val != 20 || shares["old"].Val != 1 {
		t.Fatalf("reverse split normalization failed: eps=%#v shares=%#v", eps, shares)
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
