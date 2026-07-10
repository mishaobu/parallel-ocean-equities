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

func assertClose(t *testing.T, label string, actual *float64, expected float64) {
	t.Helper()
	if actual == nil || math.Abs(*actual-expected) > 1e-9 {
		t.Fatalf("%s: expected %.12f, got %#v", label, expected, actual)
	}
}
