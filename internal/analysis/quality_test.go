package analysis

import (
	"testing"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func TestEnrichQualityBuildsCashWorkingCapitalAndReturnMetrics(t *testing.T) {
	equity := &model.Equity{}
	for index := 0; index < 8; index++ {
		current := index >= 4
		ebit := 3.75
		inventory := 8.0
		receivables := 6.0
		payables := 5.0
		debt := 8.0
		equityValue := 37.0
		shares := 2.0
		if current {
			ebit = 5
			inventory = 10
			receivables = 8
			payables = 6
			debt = 10
			equityValue = 45
			shares = 2.2
		}
		equity.Quarterlies = append(equity.Quarterlies, model.QuarterlyPoint{
			PeriodEnd:      []string{"2024-03-31", "2024-06-30", "2024-09-30", "2024-12-31", "2025-03-31", "2025-06-30", "2025-09-30", "2025-12-31"}[index],
			RevenueB:       floatPtr(25),
			GrossProfitB:   floatPtr(10),
			EBITB:          floatPtr(ebit),
			OperatingCashB: floatPtr(6),
			FCFB:           floatPtr(5),
			NetIncomeB:     floatPtr(4),
			PretaxIncomeB:  floatPtr(5),
			IncomeTaxB:     floatPtr(1),
			StockCompB:     floatPtr(0.5),
			DilutedSharesB: floatPtr(shares),
			InventoryB:     floatPtr(inventory),
			ReceivablesB:   floatPtr(receivables),
			PayablesB:      floatPtr(payables),
			DebtB:          floatPtr(debt),
			CashB:          floatPtr(5),
			EquityB:        floatPtr(equityValue),
		})
	}

	enrichQuality(equity)
	assertClose(t, "cash conversion", equity.Quality.CashConversion, 1.5)
	assertClose(t, "gross margin", equity.Quality.GrossMargin, 0.4)
	assertClose(t, "operating margin", equity.Quality.OperatingMargin, 0.2)
	assertClose(t, "OCF margin", equity.Quality.OperatingCashMargin, 0.24)
	assertClose(t, "FCF margin", equity.Quality.FCFMargin, 0.2)
	assertClose(t, "inventory days", equity.Quality.InventoryDays, 9.0/60.0*365)
	assertClose(t, "receivable days", equity.Quality.ReceivableDays, 7.0/100.0*365)
	assertClose(t, "payable days", equity.Quality.PayableDays, 5.5/60.0*365)
	assertClose(t, "cash conversion cycle", equity.Quality.CashConversionCycleDays, (9.0/60.0+7.0/100.0-5.5/60.0)*365)
	assertClose(t, "ROIC", equity.Quality.ROIC, 16.0/45.0)
	assertClose(t, "incremental ROIC", equity.Quality.IncrementalROIC, 0.4)
	assertClose(t, "stock compensation", equity.Quality.StockCompToRevenue, 0.02)
	assertClose(t, "diluted share growth", equity.Quality.DilutedShareGrowth, 0.1)
	if len(equity.Qualities) != 5 {
		t.Fatalf("quality history points = %d, want 5", len(equity.Qualities))
	}
}

func TestQualityMetricsRejectNonpositiveComparisonDenominators(t *testing.T) {
	point := qualityPoint("2025-12-31", qualityInputs{
		revenue: floatPtr(100), operatingCash: floatPtr(-5), netIncome: floatPtr(-2),
		ebit: floatPtr(-4), equity: floatPtr(10), cash: floatPtr(2),
	}, qualityInputs{})
	if point.CashConversion != nil {
		t.Fatalf("cash conversion should be unavailable for a loss: %#v", point.CashConversion)
	}
	assertClose(t, "negative OCF margin", point.OperatingCashMargin, -0.05)
	assertClose(t, "negative ROIC", point.ROIC, -0.395)
}
