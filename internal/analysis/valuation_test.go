package analysis

import (
	"math"
	"testing"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func TestEnrichValuationBuildsOrderedActualAndForwardInputs(t *testing.T) {
	equity := &model.Equity{
		Annuals: []model.AnnualPoint{{FiscalYear: 2026, Estimate: true, NetIncomeB: floatPtr(20), DilutedEPS: floatPtr(3)}},
		Current: model.CurrentMetrics{Price: floatPtr(50)},
	}
	for index := 0; index < 8; index++ {
		revenue := 25.0
		if index >= 4 {
			revenue = 30
		}
		equity.Quarterlies = append(equity.Quarterlies, model.QuarterlyPoint{
			PeriodEnd:      []string{"2024-03-31", "2024-06-30", "2024-09-30", "2024-12-31", "2025-03-31", "2025-06-30", "2025-09-30", "2025-12-31"}[index],
			RevenueB:       floatPtr(revenue),
			EBITB:          floatPtr(6),
			EBITDAB:        floatPtr(7),
			FCFB:           floatPtr(5),
			NetIncomeB:     floatPtr(4),
			DividendsB:     floatPtr(1),
			DilutedEPS:     floatPtr(1),
			DilutedSharesB: floatPtr(2),
			NetDebtB:       floatPtr(10),
		})
	}

	enrichValuation(equity)
	assertClose(t, "market cap", equity.Valuation.MarketCapB, 100)
	assertClose(t, "enterprise value", equity.Valuation.EnterpriseValueB, 110)
	assertClose(t, "P/E", equity.Valuation.PE, 12.5)
	assertClose(t, "forward P/E", equity.Valuation.ForwardPE, 50.0/3.0)
	assertClose(t, "EV/EBITDA", equity.Valuation.EVToEBITDA, 110.0/28.0)
	assertClose(t, "forward EV/EBITDA", equity.Valuation.ForwardEVToEBITDA, 110.0/33.6)
	assertClose(t, "FCF/market cap", equity.Valuation.FCFToMarketCap, 0.20)
	assertClose(t, "forward FCF/market cap", equity.Valuation.ForwardFCFToMarketCap, 0.24)
	assertClose(t, "dividend/FCF", equity.Valuation.DividendToFCF, 0.20)
	assertClose(t, "forward dividend/FCF", equity.Valuation.ForwardDividendToFCF, 0.20)
	assertClose(t, "forecast revenue", equity.Forecast.ForwardRevenueB, 144)
	if equity.Models.DCFValuePerShare == nil || equity.Models.MultipleValuePerShare == nil || equity.Models.EarningsValuePerShare == nil {
		t.Fatalf("valuation models were not populated: %#v", equity.Models)
	}
	assertClose(t, "default EV/EBITDA target", equity.Models.TargetEVToEBITDA, 15)
	assertClose(t, "default multiple value", equity.Models.MultipleValuePerShare, 247)
	assertClose(t, "default P/E target", equity.Models.TargetPE, 20)
	assertClose(t, "default earnings value", equity.Models.EarningsValuePerShare, 60)
}

func TestEnrichValuationHistoryUsesTrailingAndRealizedForwardQuarters(t *testing.T) {
	equity := &model.Equity{Current: model.CurrentMetrics{Price: floatPtr(60), PriceAsOf: "2026-02-01"}}
	for index := 0; index < 8; index++ {
		equity.Quarterlies = append(equity.Quarterlies, model.QuarterlyPoint{
			PeriodEnd:      []string{"2024-03-31", "2024-06-30", "2024-09-30", "2024-12-31", "2025-03-31", "2025-06-30", "2025-09-30", "2025-12-31"}[index],
			EBITB:          floatPtr(5),
			EBITDAB:        floatPtr(6),
			FCFB:           floatPtr(4),
			NetIncomeB:     floatPtr(2),
			DividendsB:     floatPtr(1),
			DilutedEPS:     floatPtr(1),
			DilutedSharesB: floatPtr(2),
			NetDebtB:       floatPtr(8),
		})
	}
	equity.Valuation = model.ValuationMetrics{PE: floatPtr(15), ForwardPE: floatPtr(12)}
	prices := []model.PricePoint{{Date: "2024-12-30", Close: 40}, {Date: "2025-03-31", Close: 44}, {Date: "2026-02-01", Close: 60}}

	enrichValuationHistory(equity, prices)
	if len(equity.Valuations) != 6 {
		t.Fatalf("valuation points = %d, want 6", len(equity.Valuations))
	}
	first := equity.Valuations[0]
	assertClose(t, "historical P/E", first.PE, 10)
	assertClose(t, "realized forward P/E", first.ForwardPE, 10)
	assertClose(t, "historical EV/EBITDA", first.EVToEBITDA, 88.0/24.0)
	assertClose(t, "historical FCF/market cap", first.FCFToMarketCap, 16.0/80.0)
	latest := equity.Valuations[len(equity.Valuations)-1]
	if latest.Date != "2026-02-01" {
		t.Fatalf("latest valuation date = %s", latest.Date)
	}
	assertClose(t, "current P/E", latest.PE, 15)
	assertClose(t, "current forward P/E", latest.ForwardPE, 12)
}

func TestNormalizePerShareInputsRepairsInconsistentSECPerShareFacts(t *testing.T) {
	equity := &model.Equity{
		Annuals: []model.AnnualPoint{
			{PeriodEnd: "2018-12-31", NetIncomeB: floatPtr(30), DilutedEPS: floatPtr(0.1)},
			{PeriodEnd: "2019-12-31", NetIncomeB: floatPtr(36), DilutedEPS: floatPtr(3)},
		},
		Quarterlies: []model.QuarterlyPoint{
			{PeriodEnd: "2019-03-31", NetIncomeB: floatPtr(3), DilutedEPS: floatPtr(0.01)},
			{PeriodEnd: "2025-03-31", NetIncomeB: floatPtr(30), DilutedEPS: floatPtr(2.5), DilutedSharesB: floatPtr(12)},
		},
	}
	normalizePerShareInputs(equity)
	assertClose(t, "normalized annual EPS", equity.Annuals[0].DilutedEPS, 2.5)
	assertClose(t, "inferred quarterly shares", equity.Quarterlies[0].DilutedSharesB, 12)
	assertClose(t, "normalized quarterly EPS", equity.Quarterlies[0].DilutedEPS, 0.25)
}

func TestMeaningfulMultipleRejectsNonpositiveAndExtremeDenominators(t *testing.T) {
	assertClose(t, "meaningful multiple", meaningfulMultiple(floatPtr(100), floatPtr(2)), 50)
	if meaningfulMultiple(floatPtr(100), floatPtr(-2)) != nil {
		t.Fatal("negative denominator should not produce a valuation multiple")
	}
	if meaningfulMultiple(floatPtr(500), floatPtr(2)) != nil {
		t.Fatal("multiple above 200x should not be charted")
	}
}

func assertClose(t *testing.T, label string, actual *float64, expected float64) {
	t.Helper()
	if actual == nil || math.Abs(*actual-expected) > 1e-9 {
		t.Fatalf("%s: expected %.12f, got %#v", label, expected, actual)
	}
}
