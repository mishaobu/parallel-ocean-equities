package analysis

import (
	"sort"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

const defaultCashTaxRate = 0.21

type qualityInputs struct {
	revenue, grossProfit, ebit, operatingCash, fcf, netIncome *float64
	pretaxIncome, incomeTax, stockComp                        *float64
	shares, inventory, receivables, payables                  *float64
	debt, cash, investments, equity                           *float64
}

func enrichQuality(equity *model.Equity) {
	equity.Qualities = nil
	quarters := equity.Quarterlies
	quarterlyByDate := make(map[string]model.QualityPoint, len(quarters))
	for index := 3; index < len(quarters); index++ {
		window := quarters[index-3 : index+1]
		if !consecutiveQuarterWindow(window) {
			continue
		}
		trailing := quarterQualityInputs(window)
		var previous qualityInputs
		if index >= 7 && consecutiveQuarterWindow(quarters[index-7:index-3]) {
			previous = quarterQualityInputs(quarters[index-7 : index-3])
		}
		date := availableDate(quarters[index].FiledAt, quarters[index].PeriodEnd)
		quarterlyByDate[date] = qualityPoint(date, trailing, previous)
	}
	quarterlyPoints := make([]model.QualityPoint, 0, len(quarterlyByDate))
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
		var previous qualityInputs
		if index > 0 {
			previous = annualQualityInputs(actualAnnuals[index-1])
		}
		equity.Qualities = append(equity.Qualities, qualityPoint(date, annualQualityInputs(row), previous))
	}
	equity.Qualities = append(equity.Qualities, quarterlyPoints...)
	sort.Slice(equity.Qualities, func(i, j int) bool { return equity.Qualities[i].Date < equity.Qualities[j].Date })

	if len(quarters) < 4 || !consecutiveQuarterWindow(quarters[len(quarters)-4:]) {
		return
	}
	recent := quarterQualityInputs(quarters[len(quarters)-4:])
	var previous qualityInputs
	if len(quarters) >= 8 {
		previous = quarterQualityInputs(quarters[len(quarters)-8 : len(quarters)-4])
	}
	point := qualityPoint(quarters[len(quarters)-1].PeriodEnd, recent, previous)
	equity.Quality = model.QualityMetrics{
		AsOf:                    quarters[len(quarters)-1].PeriodEnd,
		CashConversion:          point.CashConversion,
		GrossMargin:             point.GrossMargin,
		OperatingMargin:         point.OperatingMargin,
		OperatingCashMargin:     point.OperatingCashMargin,
		FCFMargin:               point.FCFMargin,
		InventoryDays:           point.InventoryDays,
		ReceivableDays:          point.ReceivableDays,
		PayableDays:             point.PayableDays,
		CashConversionCycleDays: point.CashConversionCycleDays,
		ROIC:                    point.ROIC,
		IncrementalROIC:         point.IncrementalROIC,
		StockCompToRevenue:      point.StockCompToRevenue,
		DilutedShareGrowth:      point.DilutedShareGrowth,
	}
}

func qualityPoint(date string, current, previous qualityInputs) model.QualityPoint {
	point := model.QualityPoint{
		Date:                date,
		CashConversion:      positiveRatio(current.operatingCash, current.netIncome),
		GrossMargin:         positiveRatio(current.grossProfit, current.revenue),
		OperatingMargin:     positiveRatio(current.ebit, current.revenue),
		OperatingCashMargin: positiveRatio(current.operatingCash, current.revenue),
		FCFMargin:           positiveRatio(current.fcf, current.revenue),
		StockCompToRevenue:  positiveRatio(current.stockComp, current.revenue),
		DilutedShareGrowth:  growthRatio(current.shares, previous.shares),
	}
	cogs := subtractKnown(current.revenue, current.grossProfit)
	point.InventoryDays = daysOutstanding(averageBalance(current.inventory, previous.inventory), cogs)
	point.ReceivableDays = daysOutstanding(averageBalance(current.receivables, previous.receivables), current.revenue)
	point.PayableDays = daysOutstanding(averageBalance(current.payables, previous.payables), cogs)
	if point.InventoryDays != nil && point.ReceivableDays != nil && point.PayableDays != nil {
		point.CashConversionCycleDays = floatPtr(*point.InventoryDays + *point.ReceivableDays - *point.PayableDays)
	}

	currentNOPAT := nopat(current)
	previousNOPAT := nopat(previous)
	currentCapital := investedCapital(current)
	previousCapital := investedCapital(previous)
	if currentNOPAT != nil {
		capital := averageBalance(currentCapital, previousCapital)
		point.ROIC = positiveRatio(currentNOPAT, capital)
	}
	if currentNOPAT != nil && previousNOPAT != nil && currentCapital != nil && previousCapital != nil {
		deltaCapital := *currentCapital - *previousCapital
		if deltaCapital > 0 {
			point.IncrementalROIC = floatPtr((*currentNOPAT - *previousNOPAT) / deltaCapital)
		}
	}
	return point
}

func annualQualityInputs(row model.AnnualPoint) qualityInputs {
	return qualityInputs{
		revenue: row.RevenueB, grossProfit: row.GrossProfitB, ebit: row.EBITB, operatingCash: row.OperatingCashB,
		fcf: row.FCFB, netIncome: row.NetIncomeB, pretaxIncome: row.PretaxIncomeB, incomeTax: row.IncomeTaxB,
		stockComp: row.StockCompB, shares: row.DilutedSharesB, inventory: row.InventoryB,
		receivables: row.ReceivablesB, payables: row.PayablesB, debt: row.DebtB, cash: row.CashB,
		investments: row.InvestmentsB, equity: row.EquityB,
	}
}

func quarterQualityInputs(rows []model.QuarterlyPoint) qualityInputs {
	return qualityInputs{
		revenue:       sumQuarterValues(rows, func(row model.QuarterlyPoint) *float64 { return row.RevenueB }),
		grossProfit:   sumQuarterValues(rows, func(row model.QuarterlyPoint) *float64 { return row.GrossProfitB }),
		ebit:          sumQuarterValues(rows, func(row model.QuarterlyPoint) *float64 { return row.EBITB }),
		operatingCash: sumQuarterValues(rows, func(row model.QuarterlyPoint) *float64 { return row.OperatingCashB }),
		fcf:           sumQuarterValues(rows, func(row model.QuarterlyPoint) *float64 { return row.FCFB }),
		netIncome:     sumQuarterValues(rows, func(row model.QuarterlyPoint) *float64 { return row.NetIncomeB }),
		pretaxIncome:  sumQuarterValues(rows, func(row model.QuarterlyPoint) *float64 { return row.PretaxIncomeB }),
		incomeTax:     sumQuarterValues(rows, func(row model.QuarterlyPoint) *float64 { return row.IncomeTaxB }),
		stockComp:     sumQuarterValues(rows, func(row model.QuarterlyPoint) *float64 { return row.StockCompB }),
		shares:        latestQuarterValue(rows, func(row model.QuarterlyPoint) *float64 { return row.DilutedSharesB }),
		inventory:     latestQuarterValue(rows, func(row model.QuarterlyPoint) *float64 { return row.InventoryB }),
		receivables:   latestQuarterValue(rows, func(row model.QuarterlyPoint) *float64 { return row.ReceivablesB }),
		payables:      latestQuarterValue(rows, func(row model.QuarterlyPoint) *float64 { return row.PayablesB }),
		debt:          latestQuarterValue(rows, func(row model.QuarterlyPoint) *float64 { return row.DebtB }),
		cash:          latestQuarterValue(rows, func(row model.QuarterlyPoint) *float64 { return row.CashB }),
		investments:   latestQuarterValue(rows, func(row model.QuarterlyPoint) *float64 { return row.InvestmentsB }),
		equity:        latestQuarterValue(rows, func(row model.QuarterlyPoint) *float64 { return row.EquityB }),
	}
}

func positiveRatio(numerator, denominator *float64) *float64 {
	if numerator == nil || denominator == nil || *denominator <= 0 {
		return nil
	}
	return ratio(numerator, denominator)
}

func growthRatio(current, previous *float64) *float64 {
	if current == nil || previous == nil || *previous <= 0 {
		return nil
	}
	return floatPtr(*current / *previous - 1)
}

func subtractKnown(left, right *float64) *float64 {
	if left == nil || right == nil {
		return nil
	}
	return floatPtr(*left - *right)
}

func averageBalance(current, previous *float64) *float64 {
	if current == nil {
		return nil
	}
	if previous == nil {
		return current
	}
	return floatPtr((*current + *previous) / 2)
}

func daysOutstanding(balance, annualFlow *float64) *float64 {
	value := positiveRatio(balance, annualFlow)
	if value == nil {
		return nil
	}
	return floatPtr(*value * 365)
}

func nopat(input qualityInputs) *float64 {
	if input.ebit == nil {
		return nil
	}
	rate := defaultCashTaxRate
	if input.pretaxIncome != nil && input.incomeTax != nil && *input.pretaxIncome > 0 {
		rate = clamp(*input.incomeTax / *input.pretaxIncome, 0, 0.35)
	}
	return floatPtr(*input.ebit * (1 - rate))
}

func investedCapital(input qualityInputs) *float64 {
	if input.equity == nil || input.cash == nil {
		return nil
	}
	debt := 0.0
	if input.debt != nil {
		debt = *input.debt
	}
	liquidity := *input.cash
	if input.investments != nil {
		liquidity += *input.investments
	}
	value := *input.equity + debt - liquidity
	if value <= 0 {
		return nil
	}
	return floatPtr(value)
}
