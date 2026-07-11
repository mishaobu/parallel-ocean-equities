package analysis

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

var (
	operatingIncomeTags = []string{"OperatingIncomeLoss"}
	daTags              = []string{"DepreciationDepletionAndAmortization", "DepreciationAndAmortization", "Depreciation"}
	operatingCashTags   = []string{"NetCashProvidedByUsedInOperatingActivities", "NetCashProvidedByUsedInOperatingActivitiesContinuingOperations"}
	dividendTags        = []string{"PaymentsOfDividends", "PaymentsOfDividendsCommonStock", "PaymentsOfOrdinaryDividends"}
	shareTags           = []string{"WeightedAverageNumberOfDilutedSharesOutstanding"}
	cashTags            = []string{"CashAndCashEquivalentsAtCarryingValue", "CashCashEquivalentsRestrictedCashAndRestrictedCashEquivalents", "Cash"}
	investmentTags      = []string{"MarketableSecuritiesCurrent", "ShortTermInvestments"}
	currentDebtTags     = []string{"LongTermDebtCurrent", "DebtCurrent", "ShortTermBorrowings"}
	noncurrentDebtTags  = []string{"LongTermDebtNoncurrent"}
	totalDebtTags       = []string{"LongTermDebtAndCapitalLeaseObligationsIncludingCurrentMaturities", "DebtLongtermAndShorttermCombinedAmount", "DebtAndCapitalLeaseObligations", "LongTermDebt"}
	assetTags           = []string{"Assets"}
	liabilityTags       = []string{"Liabilities"}
	equityTags          = []string{"StockholdersEquity", "StockholdersEquityIncludingPortionAttributableToNoncontrollingInterest"}
)

type quarterFact struct {
	value         float64
	periodEnd     string
	filed         string
	accession     string
	form          string
	derived       bool
	fiscalYear    int
	fiscalQuarter string
}

func extractQuarterlies(response companyFacts, cik string) ([]model.QuarterlyPoint, error) {
	gaap := response.Facts["us-gaap"]
	if gaap == nil {
		return nil, fmt.Errorf("SEC response has no us-gaap facts")
	}

	revenue := durationQuarterFacts(gaap, revenueTags, "USD")
	anchors := revenue
	if len(anchors) == 0 {
		anchors = durationQuarterFacts(gaap, netIncomeTags, "USD")
	}
	if len(anchors) == 0 {
		return nil, fmt.Errorf("no quarterly SEC revenue or net-income facts found")
	}

	operatingIncome := durationQuarterFacts(gaap, operatingIncomeTags, "USD")
	netIncome := durationQuarterFacts(gaap, netIncomeTags, "USD")
	da := durationQuarterFacts(gaap, daTags, "USD")
	operatingCash := durationQuarterFacts(gaap, operatingCashTags, "USD")
	capex := durationQuarterFacts(gaap, capexTags, "USD")
	dividends := durationQuarterFacts(gaap, dividendTags, "USD")
	eps := durationQuarterFacts(gaap, epsTags, "USD/shares")
	shares := durationQuarterFactsMode(gaap, shareTags, "shares", false)
	splits := stockSplitEvents(gaap)
	normalizeQuarterSeriesForSplits(eps, splits, false)
	normalizeQuarterSeriesForSplits(shares, splits, true)

	cash := instantQuarterFacts(gaap, cashTags, "USD")
	investments := instantQuarterFacts(gaap, investmentTags, "USD")
	currentDebt := instantQuarterFacts(gaap, currentDebtTags, "USD")
	noncurrentDebt := instantQuarterFacts(gaap, noncurrentDebtTags, "USD")
	totalDebt := instantQuarterFacts(gaap, totalDebtTags, "USD")
	assets := instantQuarterFacts(gaap, assetTags, "USD")
	liabilities := instantQuarterFacts(gaap, liabilityTags, "USD")
	equity := instantQuarterFacts(gaap, equityTags, "USD")

	rows := make([]model.QuarterlyPoint, 0, len(anchors))
	for periodEnd, anchor := range anchors {
		if anchor.fiscalYear == 0 || anchor.fiscalQuarter == "" {
			continue
		}
		row := model.QuarterlyPoint{
			FiscalYear:    anchor.fiscalYear,
			FiscalQuarter: anchor.fiscalQuarter,
			PeriodEnd:     periodEnd,
			FiledAt:       anchor.filed,
			Accession:     anchor.accession,
			Form:          anchor.form,
			FilingURL:     filingIndexURL(cik, anchor.accession),
			Derived:       anchor.derived,
		}
		row.RevenueB = quarterBillions(revenue[periodEnd])
		row.EBITB = quarterBillions(operatingIncome[periodEnd])
		row.DAB = quarterBillions(da[periodEnd])
		row.NetIncomeB = quarterBillions(netIncome[periodEnd])
		row.OperatingCashB = quarterBillions(operatingCash[periodEnd])
		row.CapexB = quarterBillions(capex[periodEnd])
		row.DividendsB = quarterBillions(dividends[periodEnd])
		if len(dividends) == 0 {
			row.DividendsB = floatPtr(0)
		}
		row.DilutedEPS = quarterRaw(eps[periodEnd])
		if value := quarterRaw(shares[periodEnd]); value != nil {
			row.DilutedSharesB = floatPtr(*value / 1e9)
		}
		row.CashB = quarterBillions(cash[periodEnd])
		row.InvestmentsB = quarterBillions(investments[periodEnd])
		row.AssetsB = quarterBillions(assets[periodEnd])
		row.LiabilitiesB = quarterBillions(liabilities[periodEnd])
		row.EquityB = quarterBillions(equity[periodEnd])

		row.DebtB = addKnown(quarterBillions(currentDebt[periodEnd]), quarterBillions(noncurrentDebt[periodEnd]))
		if row.DebtB == nil {
			row.DebtB = quarterBillions(totalDebt[periodEnd])
		}
		liquidity := addKnown(row.CashB, row.InvestmentsB)
		if row.DebtB != nil && liquidity != nil {
			row.NetDebtB = floatPtr(*row.DebtB - *liquidity)
		}
		if row.EBITB != nil && row.DAB != nil {
			row.EBITDAB = floatPtr(*row.EBITB + *row.DAB)
		}
		if row.OperatingCashB != nil && row.CapexB != nil {
			row.FCFB = floatPtr(*row.OperatingCashB - *row.CapexB)
		}
		if row.LiabilitiesB == nil && row.AssetsB != nil && row.EquityB != nil {
			row.LiabilitiesB = floatPtr(*row.AssetsB - *row.EquityB)
		}
		if row.EquityB == nil && row.AssetsB != nil && row.LiabilitiesB != nil {
			row.EquityB = floatPtr(*row.AssetsB - *row.LiabilitiesB)
		}
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].PeriodEnd == rows[j].PeriodEnd {
			return rows[i].FiscalQuarter < rows[j].FiscalQuarter
		}
		return rows[i].PeriodEnd < rows[j].PeriodEnd
	})
	return rows, nil
}

func durationQuarterFacts(gaap map[string]factConcept, tags []string, unit string) map[string]quarterFact {
	return durationQuarterFactsMode(gaap, tags, unit, true)
}

func durationQuarterFactsMode(gaap map[string]factConcept, tags []string, unit string, additive bool) map[string]quarterFact {
	facts := canonicalDurationFacts(gaap, tags, unit)
	direct := make(map[string]fact)
	cumulative := make([]fact, 0)
	annual := make([]fact, 0)

	for _, candidate := range facts {
		start, startErr := time.Parse("2006-01-02", candidate.Start)
		end, endErr := time.Parse("2006-01-02", candidate.End)
		if startErr != nil || endErr != nil {
			continue
		}
		days := int(end.Sub(start).Hours() / 24)
		if candidate.Form == "10-K" && candidate.FP == "FY" && days >= 300 && days <= 390 {
			annual = append(annual, candidate)
			continue
		}
		if candidate.Form != "10-Q" || quarterNumber(candidate.FP) == 0 {
			continue
		}
		if days >= 70 && days <= 120 {
			direct[candidate.End] = earlierFact(direct[candidate.End], candidate)
		} else if days > 120 && days <= 300 {
			cumulative = append(cumulative, candidate)
		}
	}

	out := make(map[string]quarterFact)
	for periodEnd, candidate := range direct {
		out[periodEnd] = fromFact(candidate, false)
	}

	for _, candidate := range cumulative {
		if _, exists := out[candidate.End]; exists {
			continue
		}
		previous, found := previousCumulativeFact(facts, candidate)
		if !found {
			continue
		}
		value := candidate.Val
		if additive {
			value -= previous.Val
		}
		derived := fromFact(candidate, true)
		derived.value = value
		out[candidate.End] = derived
	}

	for _, candidate := range annual {
		value := candidate.Val
		complete := true
		if additive {
			quarterEnds := quarterEndsWithin(out, candidate.Start, candidate.End)
			if len(quarterEnds) != 3 {
				complete = false
			} else {
				for _, periodEnd := range quarterEnds {
					value -= out[periodEnd].value
				}
			}
		}
		if complete {
			derived := fromFact(candidate, true)
			derived.value = value
			derived.fiscalQuarter = "Q4"
			out[candidate.End] = derived
		}
	}
	return out
}

func canonicalDurationFacts(gaap map[string]factConcept, tags []string, unit string) []fact {
	byPeriod := make(map[string]fact)
	for _, tag := range tags {
		concept, ok := gaap[tag]
		if !ok {
			continue
		}
		tagPeriods := make(map[string]fact)
		for _, candidate := range concept.Units[unit] {
			if candidate.Start == "" || candidate.End == "" || (candidate.Form != "10-Q" && candidate.Form != "10-K") {
				continue
			}
			key := candidate.Start + "/" + candidate.End
			current, exists := tagPeriods[key]
			if !exists || candidate.Filed < current.Filed {
				tagPeriods[key] = candidate
			}
		}
		for key, candidate := range tagPeriods {
			if _, exists := byPeriod[key]; !exists {
				byPeriod[key] = candidate
			}
		}
	}
	out := make([]fact, 0, len(byPeriod))
	for _, candidate := range byPeriod {
		out = append(out, candidate)
	}
	return out
}

func instantQuarterFacts(gaap map[string]factConcept, tags []string, unit string) map[string]quarterFact {
	byPeriod := make(map[string]fact)
	for _, tag := range tags {
		concept, ok := gaap[tag]
		if !ok {
			continue
		}
		tagPeriods := make(map[string]fact)
		for _, candidate := range concept.Units[unit] {
			if candidate.Start != "" || candidate.End == "" || (candidate.Form != "10-Q" && candidate.Form != "10-K") {
				continue
			}
			if candidate.FP != "Q1" && candidate.FP != "Q2" && candidate.FP != "Q3" && candidate.FP != "FY" {
				continue
			}
			current, exists := tagPeriods[candidate.End]
			if !exists || candidate.Filed < current.Filed {
				tagPeriods[candidate.End] = candidate
			}
		}
		for periodEnd, candidate := range tagPeriods {
			if _, exists := byPeriod[periodEnd]; !exists {
				byPeriod[periodEnd] = candidate
			}
		}
	}
	out := make(map[string]quarterFact)
	for periodEnd, candidate := range byPeriod {
		out[periodEnd] = fromFact(candidate, false)
	}
	return out
}

func previousCumulativeFact(facts []fact, candidate fact) (fact, bool) {
	previous := fact{}
	for _, other := range facts {
		if other.Start != candidate.Start || other.End >= candidate.End {
			continue
		}
		if previous.End == "" || other.End > previous.End {
			previous = other
		}
	}
	return previous, previous.End != ""
}

func quarterEndsWithin(values map[string]quarterFact, start, end string) []string {
	periods := make([]string, 0, 3)
	for periodEnd := range values {
		if periodEnd >= start && periodEnd < end {
			periods = append(periods, periodEnd)
		}
	}
	sort.Strings(periods)
	return periods
}

func earlierFact(current, candidate fact) fact {
	if current.End == "" || candidate.Filed < current.Filed {
		return candidate
	}
	return current
}

func fromFact(value fact, derived bool) quarterFact {
	quarter := value.FP
	if quarter == "FY" {
		quarter = "Q4"
	}
	return quarterFact{
		value:         value.Val,
		periodEnd:     value.End,
		filed:         value.Filed,
		accession:     value.Accn,
		form:          value.Form,
		derived:       derived,
		fiscalYear:    value.FY,
		fiscalQuarter: quarter,
	}
}

func quarterNumber(period string) int {
	switch period {
	case "Q1":
		return 1
	case "Q2":
		return 2
	case "Q3":
		return 3
	default:
		return 0
	}
}

func quarterBillions(value quarterFact) *float64 {
	if value.periodEnd == "" {
		return nil
	}
	return floatPtr(value.value / 1e9)
}

func quarterRaw(value quarterFact) *float64 {
	if value.periodEnd == "" {
		return nil
	}
	return floatPtr(value.value)
}

func addKnown(left, right *float64) *float64 {
	if left == nil && right == nil {
		return nil
	}
	value := 0.0
	if left != nil {
		value += *left
	}
	if right != nil {
		value += *right
	}
	return floatPtr(value)
}

func filingIndexURL(cik, accession string) string {
	if cik == "" || accession == "" {
		return ""
	}
	return fmt.Sprintf(
		"https://www.sec.gov/Archives/edgar/data/%s/%s/%s-index.html",
		strings.TrimLeft(cik, "0"),
		strings.ReplaceAll(accession, "-", ""),
		accession,
	)
}

func normalizeQuarterSeriesForSplits(values map[string]quarterFact, splits []stockSplit, shares bool) {
	for key, value := range values {
		for _, split := range splits {
			if value.filed == "" || value.filed >= split.date {
				continue
			}
			if shares {
				value.value *= split.ratio
			} else {
				value.value /= split.ratio
			}
		}
		values[key] = value
	}
}
