package analysis

import (
	"math"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func enrichValuation(equity *model.Equity) {
	quarters := equity.Quarterlies
	if len(quarters) < 4 {
		return
	}
	recent := quarters[len(quarters)-4:]
	previous := []model.QuarterlyPoint(nil)
	if len(quarters) >= 8 {
		previous = quarters[len(quarters)-8 : len(quarters)-4]
	}

	ttmRevenue := sumQuarterValues(recent, func(row model.QuarterlyPoint) *float64 { return row.RevenueB })
	ttmEBITDA := sumQuarterValues(recent, func(row model.QuarterlyPoint) *float64 { return row.EBITDAB })
	ttmEBIT := sumQuarterValues(recent, func(row model.QuarterlyPoint) *float64 { return row.EBITB })
	ttmFCF := sumQuarterValues(recent, func(row model.QuarterlyPoint) *float64 { return row.FCFB })
	ttmNetIncome := sumQuarterValues(recent, func(row model.QuarterlyPoint) *float64 { return row.NetIncomeB })
	ttmDividends := sumQuarterValues(recent, func(row model.QuarterlyPoint) *float64 { return row.DividendsB })
	ttmEPS := sumQuarterValues(recent, func(row model.QuarterlyPoint) *float64 { return row.DilutedEPS })
	shares := latestQuarterValue(recent, func(row model.QuarterlyPoint) *float64 { return row.DilutedSharesB })
	netDebt := latestQuarterValue(recent, func(row model.QuarterlyPoint) *float64 { return row.NetDebtB })

	valuation := model.ValuationMetrics{
		AsOf:           recent[len(recent)-1].PeriodEnd,
		TTMRevenueB:    ttmRevenue,
		TTMEBITDAB:     ttmEBITDA,
		TTMEBITB:       ttmEBIT,
		TTMFCFB:        ttmFCF,
		TTMNetIncomeB:  ttmNetIncome,
		TTMDividendsB:  ttmDividends,
		NetDebtB:       netDebt,
		DilutedSharesB: shares,
	}

	if ttmEPS != nil {
		equity.Current.TTMEPS = ttmEPS
	}
	if equity.Current.Price != nil && shares != nil && *shares > 0 {
		valuation.MarketCapB = floatPtr(*equity.Current.Price * *shares)
	}
	if valuation.MarketCapB != nil && netDebt != nil {
		valuation.EnterpriseValueB = floatPtr(*valuation.MarketCapB + *netDebt)
	}
	if equity.Current.Price != nil && ttmEPS != nil && *ttmEPS > 0 {
		valuation.PE = ratio(equity.Current.Price, ttmEPS)
		equity.Current.TrailingPE = valuation.PE
	}

	forecast := buildForecast(equity, recent, previous, ttmRevenue, ttmEBIT, ttmEBITDA, ttmFCF, ttmNetIncome, ttmDividends, ttmEPS)
	if forecast.ForwardEPS != nil {
		equity.Current.ForwardEPS = forecast.ForwardEPS
	}
	if equity.Current.Price != nil && forecast.ForwardEPS != nil && *forecast.ForwardEPS > 0 {
		valuation.ForwardPE = ratio(equity.Current.Price, forecast.ForwardEPS)
		equity.Current.ForwardPE = valuation.ForwardPE
	}

	valuation.EVToEBITDA = ratio(valuation.EnterpriseValueB, ttmEBITDA)
	valuation.ForwardEVToEBITDA = ratio(valuation.EnterpriseValueB, forecast.ForwardEBITDAB)
	valuation.EVToEBIT = ratio(valuation.EnterpriseValueB, ttmEBIT)
	valuation.ForwardEVToEBIT = ratio(valuation.EnterpriseValueB, forecast.ForwardEBITB)
	valuation.FCFToMarketCap = ratio(ttmFCF, valuation.MarketCapB)
	valuation.ForwardFCFToMarketCap = ratio(forecast.ForwardFCFB, valuation.MarketCapB)
	valuation.FCFToEV = ratio(ttmFCF, valuation.EnterpriseValueB)
	valuation.ForwardFCFToEV = ratio(forecast.ForwardFCFB, valuation.EnterpriseValueB)
	valuation.NetDebtToEBITDA = ratio(netDebt, ttmEBITDA)
	valuation.ForwardNetDebtToEBITDA = ratio(netDebt, forecast.ForwardEBITDAB)
	valuation.DividendToFCF = ratio(ttmDividends, ttmFCF)
	valuation.ForwardDividendToFCF = ratio(forecast.ForwardDividendsB, forecast.ForwardFCFB)

	equity.Valuation = valuation
	equity.Forecast = forecast
	equity.Models = buildValuationModels(valuation, forecast)
}

func buildForecast(
	equity *model.Equity,
	recent, previous []model.QuarterlyPoint,
	ttmRevenue, ttmEBIT, ttmEBITDA, ttmFCF, ttmNetIncome, ttmDividends, ttmEPS *float64,
) model.ForecastModel {
	growth := 0.05
	previousRevenue := sumQuarterValues(previous, func(row model.QuarterlyPoint) *float64 { return row.RevenueB })
	if ttmRevenue != nil && previousRevenue != nil && *previousRevenue > 0 {
		growth = clamp(*ttmRevenue / *previousRevenue - 1, -0.20, 0.40)
	}
	dividendGrowth := clamp(growth, 0, 0.20)
	forecast := model.ForecastModel{
		Horizon:        "next 12 months",
		Method:         "SEC trailing quarters + trend model; configured estimates override EPS and net income",
		RevenueGrowth:  floatPtr(growth),
		DividendGrowth: floatPtr(dividendGrowth),
	}
	if ttmRevenue != nil && *ttmRevenue != 0 {
		forecast.ForwardRevenueB = floatPtr(*ttmRevenue * (1 + growth))
		forecast.EBITMargin = ratio(ttmEBIT, ttmRevenue)
		forecast.EBITDAMargin = ratio(ttmEBITDA, ttmRevenue)
		forecast.FCFMargin = ratio(ttmFCF, ttmRevenue)
	}
	if forecast.ForwardRevenueB != nil {
		forecast.ForwardEBITB = product(forecast.ForwardRevenueB, forecast.EBITMargin)
		forecast.ForwardEBITDAB = product(forecast.ForwardRevenueB, forecast.EBITDAMargin)
		forecast.ForwardFCFB = product(forecast.ForwardRevenueB, forecast.FCFMargin)
	}
	if ttmNetIncome != nil {
		forecast.ForwardNetIncomeB = floatPtr(*ttmNetIncome * (1 + growth))
	}
	if ttmDividends != nil {
		forecast.ForwardDividendsB = floatPtr(*ttmDividends * (1 + dividendGrowth))
	}
	if ttmEPS != nil {
		forecast.ForwardEPS = floatPtr(*ttmEPS * (1 + growth))
	}

	if estimate := latestAnnualEstimate(equity.Annuals); estimate != nil {
		if estimate.NetIncomeB != nil {
			forecast.ForwardNetIncomeB = estimate.NetIncomeB
		}
		if estimate.DilutedEPS != nil {
			forecast.ForwardEPS = estimate.DilutedEPS
		}
	}
	if equity.Current.ForwardEPS != nil {
		forecast.ForwardEPS = equity.Current.ForwardEPS
	}
	return forecast
}

func buildValuationModels(valuation model.ValuationMetrics, forecast model.ForecastModel) model.ValuationModels {
	years := 5
	wacc := 0.09
	terminalGrowth := 0.03
	fcfGrowth := 0.08
	if forecast.RevenueGrowth != nil {
		fcfGrowth = clamp(*forecast.RevenueGrowth, -0.05, 0.20)
	}
	models := model.ValuationModels{
		ProjectionYears: years,
		FCFGrowth:       floatPtr(fcfGrowth),
		WACC:            floatPtr(wacc),
		TerminalGrowth:  floatPtr(terminalGrowth),
	}
	models.DCFValuePerShare = dcfValuePerShare(forecast.ForwardFCFB, valuation.NetDebtB, valuation.DilutedSharesB, years, fcfGrowth, wacc, terminalGrowth)

	targetMultiple := 15.0
	models.TargetEVToEBITDA = floatPtr(targetMultiple)
	models.MultipleValuePerShare = multipleValuePerShare(forecast.ForwardEBITDAB, valuation.NetDebtB, valuation.DilutedSharesB, targetMultiple)

	targetPE := 20.0
	models.TargetPE = floatPtr(targetPE)
	if forecast.ForwardEPS != nil && *forecast.ForwardEPS > 0 {
		models.EarningsValuePerShare = floatPtr(*forecast.ForwardEPS * targetPE)
	}
	return models
}

func dcfValuePerShare(fcf, netDebt, shares *float64, years int, growth, wacc, terminalGrowth float64) *float64 {
	if fcf == nil || shares == nil || *fcf <= 0 || *shares <= 0 || years < 1 || wacc <= terminalGrowth {
		return nil
	}
	presentValue := 0.0
	projected := *fcf
	for year := 1; year <= years; year++ {
		projected *= 1 + growth
		presentValue += projected / math.Pow(1+wacc, float64(year))
	}
	terminal := projected * (1 + terminalGrowth) / (wacc - terminalGrowth)
	presentValue += terminal / math.Pow(1+wacc, float64(years))
	if netDebt != nil {
		presentValue -= *netDebt
	}
	return floatPtr(presentValue / *shares)
}

func multipleValuePerShare(ebitda, netDebt, shares *float64, multiple float64) *float64 {
	if ebitda == nil || shares == nil || *ebitda <= 0 || *shares <= 0 || multiple <= 0 {
		return nil
	}
	equityValue := *ebitda * multiple
	if netDebt != nil {
		equityValue -= *netDebt
	}
	return floatPtr(equityValue / *shares)
}

func sumQuarterValues(rows []model.QuarterlyPoint, getter func(model.QuarterlyPoint) *float64) *float64 {
	if len(rows) == 0 {
		return nil
	}
	total := 0.0
	for _, row := range rows {
		value := getter(row)
		if value == nil {
			return nil
		}
		total += *value
	}
	return floatPtr(total)
}

func latestQuarterValue(rows []model.QuarterlyPoint, getter func(model.QuarterlyPoint) *float64) *float64 {
	for index := len(rows) - 1; index >= 0; index-- {
		if value := getter(rows[index]); value != nil {
			return value
		}
	}
	return nil
}

func latestAnnualEstimate(rows []model.AnnualPoint) *model.AnnualPoint {
	for index := len(rows) - 1; index >= 0; index-- {
		if rows[index].Estimate {
			return &rows[index]
		}
	}
	return nil
}

func ratio(numerator, denominator *float64) *float64 {
	if numerator == nil || denominator == nil || *denominator == 0 {
		return nil
	}
	return floatPtr(*numerator / *denominator)
}

func product(left, right *float64) *float64 {
	if left == nil || right == nil {
		return nil
	}
	return floatPtr(*left * *right)
}

func clamp(value, minimum, maximum float64) float64 {
	return math.Max(minimum, math.Min(maximum, value))
}
