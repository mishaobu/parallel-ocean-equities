package analysis

import (
	"math"
	"sort"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func enrichValuationHistory(equity *model.Equity, prices []model.PricePoint) {
	equity.Valuations = nil
	if len(prices) == 0 {
		return
	}
	sort.Slice(prices, func(i, j int) bool { return prices[i].Date < prices[j].Date })
	quarters := equity.Quarterlies
	quarterlyByDate := make(map[string]model.ValuationPoint, len(quarters))
	for index := 3; index < len(quarters); index++ {
		trailing := quarters[index-3 : index+1]
		if !consecutiveQuarterWindow(trailing) {
			continue
		}
		date := availableDate(quarters[index].FiledAt, quarters[index].PeriodEnd)
		price, ok := priceOnOrBefore(prices, date)
		if !ok || price <= 0 {
			continue
		}
		var forward []model.QuarterlyPoint
		if index+4 < len(quarters) && consecutiveQuarterWindow(quarters[index+1:index+5]) {
			forward = quarters[index+1 : index+5]
		}
		quarterlyByDate[date] = historicalValuationPoint(date, price, trailing, forward)
	}
	quarterlyPoints := make([]model.ValuationPoint, 0, len(quarterlyByDate))
	for _, point := range quarterlyByDate {
		quarterlyPoints = append(quarterlyPoints, point)
	}
	sort.Slice(quarterlyPoints, func(i, j int) bool { return quarterlyPoints[i].Date < quarterlyPoints[j].Date })

	firstQuarterlyDate := ""
	if len(quarterlyPoints) > 0 {
		firstQuarterlyDate = quarterlyPoints[0].Date
	}
	actualAnnuals := make([]model.AnnualPoint, 0, len(equity.Annuals))
	for _, row := range equity.Annuals {
		if !row.Estimate {
			actualAnnuals = append(actualAnnuals, row)
		}
	}
	for index, row := range actualAnnuals {
		date := availableDate(row.FiledAt, row.PeriodEnd)
		if date == "" || (firstQuarterlyDate != "" && date >= firstQuarterlyDate) {
			continue
		}
		price, ok := priceOnOrBefore(prices, date)
		if !ok || price <= 0 {
			continue
		}
		var forward *model.AnnualPoint
		if index+1 < len(actualAnnuals) {
			forward = &actualAnnuals[index+1]
		}
		equity.Valuations = append(equity.Valuations, historicalAnnualValuationPoint(date, price, row, forward))
	}
	equity.Valuations = append(equity.Valuations, quarterlyPoints...)
	sort.Slice(equity.Valuations, func(i, j int) bool { return equity.Valuations[i].Date < equity.Valuations[j].Date })

	currentDate := equity.Current.PriceAsOf
	if currentDate == "" || equity.Current.Price == nil {
		return
	}
	if len(equity.Valuations) > 0 && currentDate <= equity.Valuations[len(equity.Valuations)-1].Date {
		return
	}
	valuation := equity.Valuation
	equity.Valuations = append(equity.Valuations, model.ValuationPoint{
		Date:                     currentDate,
		PE:                       valuation.PE,
		EVToEBITDA:               valuation.EVToEBITDA,
		EVToEBIT:                 valuation.EVToEBIT,
		OperatingCashToMarketCap: valuation.OperatingCashToMarketCap,
		FCFToMarketCap:           valuation.FCFToMarketCap,
		FCFToEV:                  valuation.FCFToEV,
		NetDebtToEBITDA:          valuation.NetDebtToEBITDA,
		DividendToFCF:            valuation.DividendToFCF,
	})
}

func availableDate(filedAt, periodEnd string) string {
	if filedAt != "" {
		return filedAt
	}
	return periodEnd
}

func normalizePerShareInputs(equity *model.Equity) {
	type shareProxy struct {
		date   string
		shares float64
	}
	var reference float64
	for index := len(equity.Quarterlies) - 1; index >= 0; index-- {
		if shares := equity.Quarterlies[index].DilutedSharesB; shares != nil && *shares > 0 {
			reference = *shares
			break
		}
	}
	if reference == 0 {
		return
	}
	proxies := make([]shareProxy, 0, len(equity.Annuals)+len(equity.Quarterlies))
	for _, row := range equity.Quarterlies {
		if row.DilutedSharesB != nil && plausibleShareCount(*row.DilutedSharesB, reference) {
			proxies = append(proxies, shareProxy{date: row.PeriodEnd, shares: *row.DilutedSharesB})
		}
	}
	for _, row := range equity.Annuals {
		if row.NetIncomeB == nil || row.DilutedEPS == nil || *row.DilutedEPS == 0 {
			continue
		}
		shares := math.Abs(*row.NetIncomeB / *row.DilutedEPS)
		if plausibleShareCount(shares, reference) {
			proxies = append(proxies, shareProxy{date: row.PeriodEnd, shares: shares})
		}
	}
	if len(proxies) == 0 {
		return
	}
	nearest := func(date string) float64 {
		best := proxies[0]
		bestDistance := dateDistance(date, best.date)
		for _, candidate := range proxies[1:] {
			distance := dateDistance(date, candidate.date)
			if distance < bestDistance {
				best = candidate
				bestDistance = distance
			}
		}
		return best.shares
	}

	for index := range equity.Annuals {
		row := &equity.Annuals[index]
		if row.NetIncomeB == nil || row.DilutedEPS == nil || *row.DilutedEPS == 0 {
			continue
		}
		impliedShares := math.Abs(*row.NetIncomeB / *row.DilutedEPS)
		if !plausibleShareCount(impliedShares, reference) {
			row.DilutedEPS = floatPtr(*row.NetIncomeB / nearest(row.PeriodEnd))
		}
		if row.DilutedSharesB == nil || !plausibleShareCount(*row.DilutedSharesB, reference) {
			row.DilutedSharesB = floatPtr(nearest(row.PeriodEnd))
		}
	}
	for index := range equity.Quarterlies {
		row := &equity.Quarterlies[index]
		if row.DilutedSharesB == nil || !plausibleShareCount(*row.DilutedSharesB, reference) {
			row.DilutedSharesB = floatPtr(nearest(row.PeriodEnd))
		}
		if row.NetIncomeB == nil || row.DilutedSharesB == nil || *row.DilutedSharesB <= 0 {
			continue
		}
		impliedEPS := *row.NetIncomeB / *row.DilutedSharesB
		if row.DilutedEPS == nil || math.Abs(*row.DilutedEPS-impliedEPS) > math.Max(0.02, math.Abs(impliedEPS)*0.25) {
			row.DilutedEPS = floatPtr(impliedEPS)
		}
	}
}

func historicalAnnualValuationPoint(date string, price float64, trailing model.AnnualPoint, forward *model.AnnualPoint) model.ValuationPoint {
	shares := trailing.DilutedSharesB
	netDebt := trailing.NetDebtB
	var marketCap, enterpriseValue *float64
	if shares != nil && *shares > 0 {
		marketCap = floatPtr(price * *shares)
	}
	if marketCap != nil && netDebt != nil {
		enterpriseValue = floatPtr(*marketCap + *netDebt)
	}
	point := model.ValuationPoint{
		Date:                     date,
		PE:                       meaningfulMultiple(marketCap, trailing.NetIncomeB),
		EVToEBITDA:               meaningfulMultiple(enterpriseValue, trailing.EBITDAB),
		EVToEBIT:                 meaningfulMultiple(enterpriseValue, trailing.EBITB),
		OperatingCashToMarketCap: ratio(trailing.OperatingCashB, marketCap),
		FCFToMarketCap:           ratio(trailing.FCFB, marketCap),
		FCFToEV:                  ratio(trailing.FCFB, enterpriseValue),
		NetDebtToEBITDA:          ratio(netDebt, trailing.EBITDAB),
		DividendToFCF:            ratio(trailing.DividendsB, trailing.FCFB),
	}
	if forward == nil {
		return point
	}
	point.ForwardPE = meaningfulMultiple(marketCap, forward.NetIncomeB)
	point.ForwardEVToEBITDA = meaningfulMultiple(enterpriseValue, forward.EBITDAB)
	point.ForwardEVToEBIT = meaningfulMultiple(enterpriseValue, forward.EBITB)
	point.ForwardOperatingCashToMarketCap = ratio(forward.OperatingCashB, marketCap)
	point.ForwardFCFToMarketCap = ratio(forward.FCFB, marketCap)
	point.ForwardFCFToEV = ratio(forward.FCFB, enterpriseValue)
	point.ForwardNetDebtToEBITDA = ratio(netDebt, forward.EBITDAB)
	point.ForwardDividendToFCF = ratio(forward.DividendsB, forward.FCFB)
	return point
}

func plausibleShareCount(shares, reference float64) bool {
	return shares > 0 && shares >= reference/4 && shares <= reference*4
}

func dateDistance(left, right string) time.Duration {
	leftDate, leftErr := time.Parse("2006-01-02", left)
	rightDate, rightErr := time.Parse("2006-01-02", right)
	if leftErr != nil || rightErr != nil {
		return time.Duration(1<<63 - 1)
	}
	distance := leftDate.Sub(rightDate)
	if distance < 0 {
		return -distance
	}
	return distance
}

func historicalValuationPoint(date string, price float64, trailing, forward []model.QuarterlyPoint) model.ValuationPoint {
	shares := latestQuarterValue(trailing, func(row model.QuarterlyPoint) *float64 { return row.DilutedSharesB })
	netDebt := latestQuarterValue(trailing, func(row model.QuarterlyPoint) *float64 { return row.NetDebtB })
	var marketCap, enterpriseValue *float64
	if shares != nil && *shares > 0 {
		marketCap = floatPtr(price * *shares)
	}
	if marketCap != nil && netDebt != nil {
		enterpriseValue = floatPtr(*marketCap + *netDebt)
	}
	point := model.ValuationPoint{Date: date}
	trailingNetIncome := sumQuarterValues(trailing, func(row model.QuarterlyPoint) *float64 { return row.NetIncomeB })
	point.PE = meaningfulMultiple(marketCap, trailingNetIncome)
	trailingEBITDA := sumQuarterValues(trailing, func(row model.QuarterlyPoint) *float64 { return row.EBITDAB })
	trailingEBIT := sumQuarterValues(trailing, func(row model.QuarterlyPoint) *float64 { return row.EBITB })
	trailingOperatingCash := sumQuarterValues(trailing, func(row model.QuarterlyPoint) *float64 { return row.OperatingCashB })
	trailingFCF := sumQuarterValues(trailing, func(row model.QuarterlyPoint) *float64 { return row.FCFB })
	trailingDividends := sumQuarterValues(trailing, func(row model.QuarterlyPoint) *float64 { return row.DividendsB })
	point.EVToEBITDA = meaningfulMultiple(enterpriseValue, trailingEBITDA)
	point.EVToEBIT = meaningfulMultiple(enterpriseValue, trailingEBIT)
	point.OperatingCashToMarketCap = ratio(trailingOperatingCash, marketCap)
	point.FCFToMarketCap = ratio(trailingFCF, marketCap)
	point.FCFToEV = ratio(trailingFCF, enterpriseValue)
	point.NetDebtToEBITDA = ratio(netDebt, trailingEBITDA)
	point.DividendToFCF = ratio(trailingDividends, trailingFCF)

	if len(forward) != 4 {
		return point
	}
	forwardNetIncome := sumQuarterValues(forward, func(row model.QuarterlyPoint) *float64 { return row.NetIncomeB })
	point.ForwardPE = meaningfulMultiple(marketCap, forwardNetIncome)
	forwardEBITDA := sumQuarterValues(forward, func(row model.QuarterlyPoint) *float64 { return row.EBITDAB })
	forwardEBIT := sumQuarterValues(forward, func(row model.QuarterlyPoint) *float64 { return row.EBITB })
	forwardOperatingCash := sumQuarterValues(forward, func(row model.QuarterlyPoint) *float64 { return row.OperatingCashB })
	forwardFCF := sumQuarterValues(forward, func(row model.QuarterlyPoint) *float64 { return row.FCFB })
	forwardDividends := sumQuarterValues(forward, func(row model.QuarterlyPoint) *float64 { return row.DividendsB })
	point.ForwardEVToEBITDA = meaningfulMultiple(enterpriseValue, forwardEBITDA)
	point.ForwardEVToEBIT = meaningfulMultiple(enterpriseValue, forwardEBIT)
	point.ForwardOperatingCashToMarketCap = ratio(forwardOperatingCash, marketCap)
	point.ForwardFCFToMarketCap = ratio(forwardFCF, marketCap)
	point.ForwardFCFToEV = ratio(forwardFCF, enterpriseValue)
	point.ForwardNetDebtToEBITDA = ratio(netDebt, forwardEBITDA)
	point.ForwardDividendToFCF = ratio(forwardDividends, forwardFCF)
	return point
}

func meaningfulMultiple(numerator, denominator *float64) *float64 {
	if numerator == nil || denominator == nil || *denominator <= 0 {
		return nil
	}
	value := *numerator / *denominator
	if value <= 0 || value > 200 {
		return nil
	}
	return floatPtr(value)
}

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
	ttmOperatingCash := sumQuarterValues(recent, func(row model.QuarterlyPoint) *float64 { return row.OperatingCashB })
	ttmFCF := sumQuarterValues(recent, func(row model.QuarterlyPoint) *float64 { return row.FCFB })
	ttmNetIncome := sumQuarterValues(recent, func(row model.QuarterlyPoint) *float64 { return row.NetIncomeB })
	ttmDividends := sumQuarterValues(recent, func(row model.QuarterlyPoint) *float64 { return row.DividendsB })
	ttmEPS := sumQuarterValues(recent, func(row model.QuarterlyPoint) *float64 { return row.DilutedEPS })
	shares := latestQuarterValue(recent, func(row model.QuarterlyPoint) *float64 { return row.DilutedSharesB })
	netDebt := latestQuarterValue(recent, func(row model.QuarterlyPoint) *float64 { return row.NetDebtB })

	valuation := model.ValuationMetrics{
		AsOf:              recent[len(recent)-1].PeriodEnd,
		TTMRevenueB:       ttmRevenue,
		TTMEBITDAB:        ttmEBITDA,
		TTMEBITB:          ttmEBIT,
		TTMOperatingCashB: ttmOperatingCash,
		TTMFCFB:           ttmFCF,
		TTMNetIncomeB:     ttmNetIncome,
		TTMDividendsB:     ttmDividends,
		NetDebtB:          netDebt,
		DilutedSharesB:    shares,
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

	forecast := buildForecast(equity, recent, previous, ttmRevenue, ttmEBIT, ttmEBITDA, ttmOperatingCash, ttmFCF, ttmNetIncome, ttmDividends, ttmEPS)
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
	valuation.OperatingCashToMarketCap = ratio(ttmOperatingCash, valuation.MarketCapB)
	valuation.ForwardOperatingCashToMarketCap = ratio(forecast.ForwardOperatingCashB, valuation.MarketCapB)
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
	ttmRevenue, ttmEBIT, ttmEBITDA, ttmOperatingCash, ttmFCF, ttmNetIncome, ttmDividends, ttmEPS *float64,
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
		forecast.OperatingCashMargin = ratio(ttmOperatingCash, ttmRevenue)
		forecast.FCFMargin = ratio(ttmFCF, ttmRevenue)
	}
	if forecast.ForwardRevenueB != nil {
		forecast.ForwardEBITB = product(forecast.ForwardRevenueB, forecast.EBITMargin)
		forecast.ForwardEBITDAB = product(forecast.ForwardRevenueB, forecast.EBITDAMargin)
		forecast.ForwardOperatingCashB = product(forecast.ForwardRevenueB, forecast.OperatingCashMargin)
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
	if *numerator == 0 {
		return floatPtr(0)
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
