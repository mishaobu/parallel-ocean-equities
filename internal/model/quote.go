package model

import "maps"

// LiveQuote is a short-lived market snapshot. Market-value fields are only
// populated when the quote can be paired with actual SEC shares outstanding.
type LiveQuote struct {
	Ticker                      string            `json:"ticker"`
	Price                       *float64          `json:"price,omitempty"`
	PreviousClose               *float64          `json:"previousClose,omitempty"`
	Change                      *float64          `json:"change,omitempty"`
	ChangePercent               *float64          `json:"changePercent,omitempty"`
	AsOf                        string            `json:"asOf,omitempty"`
	MarketState                 string            `json:"marketState,omitempty"`
	Currency                    string            `json:"currency,omitempty"`
	Exchange                    string            `json:"exchange,omitempty"`
	Source                      string            `json:"source,omitempty"`
	FieldSources                map[string]string `json:"fieldSources,omitempty"`
	Beta5YMonthly               *float64          `json:"beta5YMonthly,omitempty"`
	BetaBenchmark               string            `json:"betaBenchmark,omitempty"`
	Change52Week                *float64          `json:"change52Week,omitempty"`
	High52Week                  *float64          `json:"high52Week,omitempty"`
	Low52Week                   *float64          `json:"low52Week,omitempty"`
	MovingAverage50Day          *float64          `json:"movingAverage50Day,omitempty"`
	MovingAverage200Day         *float64          `json:"movingAverage200Day,omitempty"`
	AverageVolume3Month         *float64          `json:"averageVolume3Month,omitempty"`
	AverageVolume10Day          *float64          `json:"averageVolume10Day,omitempty"`
	TrailingAnnualDividendRate  *float64          `json:"trailingAnnualDividendRate,omitempty"`
	TrailingAnnualDividendYield *float64          `json:"trailingAnnualDividendYield,omitempty"`
	ForwardAnnualDividendRate   *float64          `json:"forwardAnnualDividendRate,omitempty"`
	ForwardAnnualDividendYield  *float64          `json:"forwardAnnualDividendYield,omitempty"`
	AverageDividendYield5Year   *float64          `json:"averageDividendYield5Year,omitempty"`
	ExDividendDate              string            `json:"exDividendDate,omitempty"`
	LastDividendDate            string            `json:"lastDividendDate,omitempty"`
	LastSplitFactor             string            `json:"lastSplitFactor,omitempty"`
	LastSplitDate               string            `json:"lastSplitDate,omitempty"`
	MarketCapB                  *float64          `json:"marketCapB,omitempty"`
	EnterpriseValueB            *float64          `json:"enterpriseValueB,omitempty"`
	SharesOutstandingB          *float64          `json:"sharesOutstandingB,omitempty"`
	ShareBasisAsOf              string            `json:"shareBasisAsOf,omitempty"`
	// HistoryCacheStatus is one of fresh, stale, or unavailable. The latter two
	// make degraded long-history calculations visible without exposing raw
	// upstream errors as telemetry labels.
	HistoryCacheStatus        string              `json:"historyCacheStatus,omitempty"`
	HistoryCacheAsOf          string              `json:"historyCacheAsOf,omitempty"`
	HistoryRefreshFailureKind string              `json:"historyRefreshFailureKind,omitempty"`
	HistoryRefreshFailed      bool                `json:"historyRefreshFailed,omitempty"`
	History                   []StatisticSnapshot `json:"history,omitempty"`
}

// StatisticSnapshot is one point-in-time quote-derived statistics observation.
// Numeric and text values use the stable metric keys consumed by the statistics
// explorer; Sources carries the exact per-field provenance for those same keys.
type StatisticSnapshot struct {
	AsOf       string             `json:"asOf"`
	Source     string             `json:"source,omitempty"`
	AsOfSource string             `json:"asOfSource,omitempty"`
	Numeric    map[string]float64 `json:"numeric,omitempty"`
	Text       map[string]string  `json:"text,omitempty"`
	Sources    map[string]string  `json:"sources,omitempty"`
}

// StatisticSnapshotContentEqual compares the payload attached to an already
// matched provider timestamp. Nil and empty maps are equivalent, which avoids
// rewriting persisted state for representational-only differences.
func StatisticSnapshotContentEqual(left, right StatisticSnapshot) bool {
	return left.Source == right.Source &&
		left.AsOfSource == right.AsOfSource &&
		maps.Equal(left.Numeric, right.Numeric) &&
		maps.Equal(left.Text, right.Text) &&
		maps.Equal(left.Sources, right.Sources)
}

// NewStatisticSnapshot converts every numeric or textual LiveQuote value into
// the generic metric-keyed history schema. Ticker, aggregate source, and as-of
// time remain snapshot metadata rather than chartable metric values.
func NewStatisticSnapshot(quote LiveQuote) StatisticSnapshot {
	snapshot := StatisticSnapshot{
		AsOf:       quote.AsOf,
		Source:     quote.Source,
		AsOfSource: quote.FieldSources["asOf"],
		Numeric:    make(map[string]float64),
		Text:       make(map[string]string),
		Sources:    make(map[string]string),
	}
	addNumeric := func(metricKey, fieldKey string, value *float64) {
		if value == nil {
			return
		}
		snapshot.Numeric[metricKey] = *value
		if source := quote.FieldSources[fieldKey]; source != "" {
			snapshot.Sources[metricKey] = source
		}
	}
	addText := func(metricKey, fieldKey, value string) {
		if value == "" {
			return
		}
		snapshot.Text[metricKey] = value
		if source := quote.FieldSources[fieldKey]; source != "" {
			snapshot.Sources[metricKey] = source
		}
	}

	addNumeric("price", "price", quote.Price)
	addNumeric("previous-close", "previousClose", quote.PreviousClose)
	addNumeric("change", "change", quote.Change)
	addNumeric("change-percent", "changePercent", quote.ChangePercent)
	addNumeric("beta-5y", "beta5YMonthly", quote.Beta5YMonthly)
	addNumeric("change-52-week", "change52Week", quote.Change52Week)
	addNumeric("high-52-week", "high52Week", quote.High52Week)
	addNumeric("low-52-week", "low52Week", quote.Low52Week)
	addNumeric("moving-average-50d", "movingAverage50Day", quote.MovingAverage50Day)
	addNumeric("moving-average-200d", "movingAverage200Day", quote.MovingAverage200Day)
	addNumeric("average-volume-3m", "averageVolume3Month", quote.AverageVolume3Month)
	addNumeric("average-volume-10d", "averageVolume10Day", quote.AverageVolume10Day)
	addNumeric("trailing-dividend-rate", "trailingAnnualDividendRate", quote.TrailingAnnualDividendRate)
	addNumeric("trailing-dividend-yield", "trailingAnnualDividendYield", quote.TrailingAnnualDividendYield)
	addNumeric("forward-dividend-rate", "forwardAnnualDividendRate", quote.ForwardAnnualDividendRate)
	addNumeric("forward-dividend-yield", "forwardAnnualDividendYield", quote.ForwardAnnualDividendYield)
	addNumeric("average-dividend-yield-5y", "averageDividendYield5Year", quote.AverageDividendYield5Year)
	addNumeric("market-cap", "marketCapB", quote.MarketCapB)
	addNumeric("enterprise-value", "enterpriseValueB", quote.EnterpriseValueB)
	addNumeric("shares-outstanding", "sharesOutstandingB", quote.SharesOutstandingB)

	addText("market-state", "marketState", quote.MarketState)
	addText("currency", "currency", quote.Currency)
	addText("exchange", "exchange", quote.Exchange)
	addText("beta-benchmark", "betaBenchmark", quote.BetaBenchmark)
	addText("ex-dividend-date", "exDividendDate", quote.ExDividendDate)
	addText("dividend-date", "lastDividendDate", quote.LastDividendDate)
	addText("last-split-factor", "lastSplitFactor", quote.LastSplitFactor)
	addText("last-split-date", "lastSplitDate", quote.LastSplitDate)
	addText("share-basis-as-of", "shareBasisAsOf", quote.ShareBasisAsOf)

	if len(snapshot.Numeric) == 0 {
		snapshot.Numeric = nil
	}
	if len(snapshot.Text) == 0 {
		snapshot.Text = nil
	}
	if len(snapshot.Sources) == 0 {
		snapshot.Sources = nil
	}
	return snapshot
}
