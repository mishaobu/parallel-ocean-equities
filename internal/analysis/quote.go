package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

const (
	yahooQuoteSource         = "Yahoo Finance chart API; Parallel Ocean calculations are labeled per field"
	yahooBackfillSource      = "Yahoo Finance 10-year daily chart API; Parallel Ocean point-in-time calculations are labeled per metric"
	yahooBackfillAsOfSource  = "Yahoo Finance daily chart timestamp for the last completed session represented in the calendar month"
	yahooDailyCloseSource    = "Yahoo Finance completed-session daily chart close"
	yahooHistoryCacheTTL     = 12 * time.Hour
	yahooHistoryStaleTTL     = 7 * 24 * time.Hour
	yahooHistoryFetchTimeout = 45 * time.Second
	yahooHistoryCacheLimit   = 256
	yahooHistoryConcurrency  = 4
)

var ErrNoQuoteProvider = errors.New("no live-quote provider configured")

// QuoteProvider supplies a short-lived market snapshot independently of the
// slower fundamentals and long-history refresh path.
type QuoteProvider interface {
	Quote(context.Context, string) (model.LiveQuote, error)
}

// QuoteAnalyzer joins a provider quote to persisted fundamental provenance.
type QuoteAnalyzer interface {
	Quote(context.Context, string, *model.Equity) (model.LiveQuote, error)
}

func (p *Pipeline) Quote(ctx context.Context, ticker string, existing *model.Equity) (model.LiveQuote, error) {
	provider, ok := p.Market.(QuoteProvider)
	if !ok || provider == nil {
		return model.LiveQuote{}, ErrNoQuoteProvider
	}
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	quote, err := provider.Quote(ctx, ticker)
	if err != nil {
		return model.LiveQuote{}, err
	}
	quote.Ticker = ticker
	// A provider-supplied market cap must not bypass the exact SEC share basis.
	quote.MarketCapB = nil
	quote.EnterpriseValueB = nil
	quote.SharesOutstandingB = nil
	quote.ShareBasisAsOf = ""
	if existing == nil || existing.Current.SharesOutstandingB == nil || *existing.Current.SharesOutstandingB <= 0 || existing.Current.SharesOutstandingAsOf == "" {
		return quote, nil
	}
	shares := *existing.Current.SharesOutstandingB
	quote.SharesOutstandingB = liveFloat(shares)
	quote.ShareBasisAsOf = existing.Current.SharesOutstandingAsOf
	setQuoteFieldSource(&quote, "sharesOutstandingB", "SEC CompanyFacts dei:EntityCommonStockSharesOutstanding instant fact")
	setQuoteFieldSource(&quote, "shareBasisAsOf", "SEC CompanyFacts fact end date")
	if quote.Source == "" {
		quote.Source = "SEC CompanyFacts shares outstanding"
	} else {
		quote.Source += " + SEC CompanyFacts shares outstanding"
	}
	if quote.Price == nil || *quote.Price <= 0 {
		return quote, nil
	}
	marketCap := *quote.Price * shares
	quote.MarketCapB = liveFloat(marketCap)
	setQuoteFieldSource(&quote, "marketCapB", "Parallel Ocean intraday estimate: latest Yahoo regular-market price snapshot x latest SEC actual shares outstanding")
	if existing.Valuation.NetDebtB != nil {
		quote.EnterpriseValueB = liveFloat(marketCap + *existing.Valuation.NetDebtB)
		setQuoteFieldSource(&quote, "enterpriseValueB", "Parallel Ocean estimate: live market cap + latest persisted net debt")
	}
	return quote, nil
}

func (c *CompositeMarket) Quote(ctx context.Context, ticker string) (model.LiveQuote, error) {
	if c == nil || len(c.providers) == 0 {
		return model.LiveQuote{}, ErrNoQuoteProvider
	}
	var failures []error
	found := false
	for _, provider := range c.providers {
		quoter, ok := provider.(QuoteProvider)
		if !ok || quoter == nil {
			continue
		}
		found = true
		quote, err := quoter.Quote(ctx, ticker)
		if err == nil && quote.Price != nil && *quote.Price > 0 {
			return quote, nil
		}
		if err != nil {
			failures = append(failures, err)
		} else {
			failures = append(failures, errors.New("provider returned no current price"))
		}
	}
	if !found {
		return model.LiveQuote{}, ErrNoQuoteProvider
	}
	if ctx.Err() != nil {
		return model.LiveQuote{}, ctx.Err()
	}
	return model.LiveQuote{}, errors.Join(failures...)
}

type yahooTradingPeriod struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

type yahooQuoteResult struct {
	Meta struct {
		Symbol               string   `json:"symbol"`
		Currency             string   `json:"currency"`
		ExchangeName         string   `json:"exchangeName"`
		FullExchangeName     string   `json:"fullExchangeName"`
		MarketState          string   `json:"marketState"`
		RegularMarketPrice   *float64 `json:"regularMarketPrice"`
		RegularMarketTime    int64    `json:"regularMarketTime"`
		PreviousClose        *float64 `json:"previousClose"`
		ChartPreviousClose   *float64 `json:"chartPreviousClose"`
		FiftyTwoWeekHigh     *float64 `json:"fiftyTwoWeekHigh"`
		FiftyTwoWeekLow      *float64 `json:"fiftyTwoWeekLow"`
		CurrentTradingPeriod struct {
			Pre     yahooTradingPeriod `json:"pre"`
			Regular yahooTradingPeriod `json:"regular"`
			Post    yahooTradingPeriod `json:"post"`
		} `json:"currentTradingPeriod"`
	} `json:"meta"`
	Timestamps []int64 `json:"timestamp"`
	Indicators struct {
		Quote []struct {
			Close  []*float64 `json:"close"`
			High   []*float64 `json:"high"`
			Low    []*float64 `json:"low"`
			Volume []*float64 `json:"volume"`
		} `json:"quote"`
		AdjClose []struct {
			Close []*float64 `json:"adjclose"`
		} `json:"adjclose"`
	} `json:"indicators"`
	Events struct {
		Dividends map[string]yahooDividendEvent `json:"dividends"`
		Splits    map[string]yahooSplitEvent    `json:"splits"`
	} `json:"events"`
}

type yahooDividendEvent struct {
	Amount float64 `json:"amount"`
	Date   int64   `json:"date"`
}

type yahooSplitEvent struct {
	Date        int64   `json:"date"`
	Numerator   float64 `json:"numerator"`
	Denominator float64 `json:"denominator"`
	SplitRatio  string  `json:"splitRatio"`
}

type quoteObservation struct {
	date          time.Time
	close         float64
	adjustedClose float64
	high          float64
	low           float64
	volume        *float64
}

type dividendObservation struct {
	date   time.Time
	amount float64
}

type yahooHistoryCacheKey struct {
	provider *YahooMarket
	ticker   string
}

type yahooHistoryCacheEntry struct {
	result     yahooQuoteResult
	cachedAt   time.Time
	lastAccess time.Time
}

type yahooHistoryCall struct {
	done   chan struct{}
	result yahooQuoteResult
	err    error
}

// The process-wide cache is keyed by provider identity so test/custom Yahoo
// endpoints cannot contaminate one another. It bounds both retained tickers and
// concurrent cold loads because the public quote endpoint accepts arbitrary
// symbols.
var yahooHistoryCache = struct {
	sync.Mutex
	entries map[yahooHistoryCacheKey]yahooHistoryCacheEntry
	calls   map[yahooHistoryCacheKey]*yahooHistoryCall
}{
	entries: make(map[yahooHistoryCacheKey]yahooHistoryCacheEntry),
	calls:   make(map[yahooHistoryCacheKey]*yahooHistoryCall),
}

var yahooHistoryFetchSlots = make(chan struct{}, yahooHistoryConcurrency)

func (y *YahooMarket) Quote(ctx context.Context, ticker string) (model.LiveQuote, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	historyResult := make(chan struct {
		result yahooQuoteResult
		err    error
	}, 1)
	go func() {
		result, err := y.cachedQuoteHistory(ctx, ticker)
		historyResult <- struct {
			result yahooQuoteResult
			err    error
		}{result: result, err: err}
	}()

	// Meta plus five daily observations is enough for current price, market
	// state, and an exact previous-session fallback. Slow-moving statistics and
	// backfill come from the independently cached ten-year result below.
	currentQuery := url.Values{
		"range":                {"5d"},
		"interval":             {"1d"},
		"events":               {"history"},
		"includeAdjustedClose": {"true"},
		"includePrePost":       {"false"},
	}
	current, err := y.fetchQuoteResult(ctx, ticker, currentQuery)
	if err != nil {
		return model.LiveQuote{}, err
	}

	history := current
	select {
	case outcome := <-historyResult:
		if outcome.err == nil {
			history = outcome.result
		}
	case <-ctx.Done():
		return model.LiveQuote{}, ctx.Err()
	}

	quote, err := buildYahooLiveQuoteWithHistory(ticker, current, history, time.Now().UTC())
	if err != nil {
		return model.LiveQuote{}, err
	}
	quote.Beta5YMonthly = y.beta5YMonthly(ctx, ticker, history)
	if quote.Beta5YMonthly != nil {
		quote.BetaBenchmark = "SPY adjusted total return"
		setQuoteFieldSource(&quote, "beta5YMonthly", "Parallel Ocean calculation: covariance with SPY / SPY variance over 60 aligned completed monthly adjusted returns")
		setQuoteFieldSource(&quote, "betaBenchmark", "SPY adjusted closes from Yahoo Finance chart API")
	}
	return quote, nil
}

func (y *YahooMarket) cachedQuoteHistory(ctx context.Context, ticker string) (yahooQuoteResult, error) {
	key := yahooHistoryCacheKey{provider: y, ticker: strings.ToUpper(strings.TrimSpace(ticker))}
	now := time.Now().UTC()
	yahooHistoryCache.Lock()
	entry, hasEntry := yahooHistoryCache.entries[key]
	if hasEntry && now.Sub(entry.cachedAt) < yahooHistoryCacheTTL {
		entry.lastAccess = now
		yahooHistoryCache.entries[key] = entry
		yahooHistoryCache.Unlock()
		return entry.result, nil
	}
	if call, ok := yahooHistoryCache.calls[key]; ok {
		yahooHistoryCache.Unlock()
		select {
		case <-call.done:
			return call.result, call.err
		case <-ctx.Done():
			return yahooQuoteResult{}, ctx.Err()
		}
	}
	call := &yahooHistoryCall{done: make(chan struct{})}
	yahooHistoryCache.calls[key] = call
	yahooHistoryCache.Unlock()

	go y.loadQuoteHistory(key, call, entry, hasEntry)
	select {
	case <-call.done:
		return call.result, call.err
	case <-ctx.Done():
		return yahooQuoteResult{}, ctx.Err()
	}
}

func (y *YahooMarket) loadQuoteHistory(key yahooHistoryCacheKey, call *yahooHistoryCall, stale yahooHistoryCacheEntry, hasStale bool) {
	var result yahooQuoteResult
	var fetchErr error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				fetchErr = fmt.Errorf("Yahoo Finance history fetch panic: %v", recovered)
			}
		}()
		// The shared load is deliberately independent of any one browser request;
		// canceled waiters do not abort useful work for other callers.
		loadContext, cancel := context.WithTimeout(context.Background(), yahooHistoryFetchTimeout)
		defer cancel()
		select {
		case yahooHistoryFetchSlots <- struct{}{}:
			defer func() { <-yahooHistoryFetchSlots }()
		case <-loadContext.Done():
			fetchErr = loadContext.Err()
			return
		}
		query := url.Values{
			"range":                {"10y"},
			"interval":             {"1d"},
			"events":               {"div,splits"},
			"includeAdjustedClose": {"true"},
			"includePrePost":       {"false"},
		}
		result, fetchErr = y.fetchQuoteResult(loadContext, key.ticker, query)
	}()

	now := time.Now().UTC()
	yahooHistoryCache.Lock()
	if fetchErr == nil {
		pruneYahooHistoryCacheLocked(key)
		yahooHistoryCache.entries[key] = yahooHistoryCacheEntry{result: result, cachedAt: now, lastAccess: now}
	} else if hasStale && now.Sub(stale.cachedAt) <= yahooHistoryStaleTTL {
		result = stale.result
		fetchErr = nil
		stale.lastAccess = now
		yahooHistoryCache.entries[key] = stale
	} else if hasStale {
		delete(yahooHistoryCache.entries, key)
	}
	call.result = result
	call.err = fetchErr
	delete(yahooHistoryCache.calls, key)
	close(call.done)
	yahooHistoryCache.Unlock()
}

func pruneYahooHistoryCacheLocked(incoming yahooHistoryCacheKey) {
	if _, exists := yahooHistoryCache.entries[incoming]; exists || len(yahooHistoryCache.entries) < yahooHistoryCacheLimit {
		return
	}
	var oldestKey yahooHistoryCacheKey
	var oldestAccess time.Time
	for key, entry := range yahooHistoryCache.entries {
		access := entry.lastAccess
		if access.IsZero() {
			access = entry.cachedAt
		}
		if oldestAccess.IsZero() || access.Before(oldestAccess) {
			oldestKey = key
			oldestAccess = access
		}
	}
	delete(yahooHistoryCache.entries, oldestKey)
}

func (y *YahooMarket) fetchQuoteResult(ctx context.Context, ticker string, query url.Values) (yahooQuoteResult, error) {
	endpoint := y.baseURL + "/v8/finance/chart/" + url.PathEscape(ticker) + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return yahooQuoteResult{}, err
	}
	req.Header.Set("User-Agent", "parallel-ocean-equities/1.0")
	resp, err := y.http.Do(req)
	if err != nil {
		return yahooQuoteResult{}, fmt.Errorf("Yahoo Finance quote: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return yahooQuoteResult{}, fmt.Errorf("Yahoo Finance quote HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Chart struct {
			Result []yahooQuoteResult `json:"result"`
			Error  *struct {
				Description string `json:"description"`
			} `json:"error"`
		} `json:"chart"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&payload); err != nil {
		return yahooQuoteResult{}, fmt.Errorf("decode Yahoo Finance quote response: %w", err)
	}
	if payload.Chart.Error != nil {
		return yahooQuoteResult{}, fmt.Errorf("Yahoo Finance quote: %s", payload.Chart.Error.Description)
	}
	if len(payload.Chart.Result) == 0 {
		return yahooQuoteResult{}, errors.New("Yahoo Finance returned no quote")
	}
	return payload.Chart.Result[0], nil
}

func buildYahooLiveQuote(ticker string, result yahooQuoteResult, now time.Time) (model.LiveQuote, error) {
	return buildYahooLiveQuoteWithHistory(ticker, result, result, now)
}

func buildYahooLiveQuoteWithHistory(ticker string, current, history yahooQuoteResult, now time.Time) (model.LiveQuote, error) {
	currentObservations := yahooQuoteObservations(current)
	observations := mergeYahooQuoteObservations(yahooQuoteObservations(history), currentObservations)
	price := positiveLiveFloat(current.Meta.RegularMarketPrice)
	priceSource := "Yahoo Finance chart meta.regularMarketPrice"
	if price == nil && len(currentObservations) > 0 {
		price = liveFloat(currentObservations[len(currentObservations)-1].close)
		priceSource = "Yahoo Finance latest daily chart close fallback"
	}
	if price == nil {
		return model.LiveQuote{}, errors.New("Yahoo Finance returned no current price")
	}
	asOf := now.UTC()
	asOfSource := "Parallel Ocean request time fallback"
	if current.Meta.RegularMarketTime > 0 {
		asOf = time.Unix(current.Meta.RegularMarketTime, 0).UTC()
		asOfSource = "Yahoo Finance chart meta.regularMarketTime"
	} else if len(currentObservations) > 0 {
		asOf = currentObservations[len(currentObservations)-1].date.UTC()
		asOfSource = "Yahoo Finance latest daily chart timestamp fallback"
	}
	quote := model.LiveQuote{
		Ticker:       ticker,
		Price:        price,
		AsOf:         asOf.Format(time.RFC3339),
		MarketState:  yahooMarketState(current, now),
		Currency:     current.Meta.Currency,
		Exchange:     current.Meta.FullExchangeName,
		Source:       yahooQuoteSource,
		FieldSources: make(map[string]string),
	}
	setQuoteFieldSource(&quote, "price", priceSource)
	setQuoteFieldSource(&quote, "asOf", asOfSource)
	setQuoteFieldSource(&quote, "marketState", "Yahoo Finance chart meta.marketState or currentTradingPeriod fallback")
	setQuoteFieldSource(&quote, "currency", "Yahoo Finance chart meta.currency")
	setQuoteFieldSource(&quote, "exchange", "Yahoo Finance chart exchange metadata")
	if quote.Exchange == "" {
		quote.Exchange = current.Meta.ExchangeName
	}
	quote.PreviousClose = positiveLiveFloat(current.Meta.PreviousClose)
	if quote.PreviousClose != nil {
		setQuoteFieldSource(&quote, "previousClose", "Yahoo Finance chart meta.previousClose")
	} else if previous := previousSessionClose(observations, asOf); previous != nil {
		quote.PreviousClose = previous
		setQuoteFieldSource(&quote, "previousClose", "Yahoo Finance completed daily chart close fallback")
	}
	if quote.PreviousClose != nil {
		change := *quote.Price - *quote.PreviousClose
		quote.Change = liveFloat(change)
		quote.ChangePercent = liveFloat(change / *quote.PreviousClose)
		setQuoteFieldSource(&quote, "change", "Parallel Ocean calculation: price - previous close")
		setQuoteFieldSource(&quote, "changePercent", "Parallel Ocean calculation: change / previous close")
	}

	cutoff52 := asOf.AddDate(-1, 0, 0)
	if baseline := closeOnOrBefore(observations, cutoff52); baseline != nil {
		quote.Change52Week = liveFloat(*quote.Price / *baseline - 1)
		setQuoteFieldSource(&quote, "change52Week", "Parallel Ocean calculation from latest Yahoo price snapshot and daily close on or before one-year cutoff")
	}
	quote.High52Week = positiveLiveFloat(current.Meta.FiftyTwoWeekHigh)
	quote.Low52Week = positiveLiveFloat(current.Meta.FiftyTwoWeekLow)
	if quote.High52Week != nil {
		setQuoteFieldSource(&quote, "high52Week", "Yahoo Finance chart meta.fiftyTwoWeekHigh")
	}
	if quote.Low52Week != nil {
		setQuoteFieldSource(&quote, "low52Week", "Yahoo Finance chart meta.fiftyTwoWeekLow")
	}
	if quote.High52Week == nil || quote.Low52Week == nil {
		high, low := trailingHighLow(observations, cutoff52)
		if quote.High52Week == nil {
			quote.High52Week = high
			setQuoteFieldSource(&quote, "high52Week", "Parallel Ocean maximum Yahoo daily high over trailing year")
		}
		if quote.Low52Week == nil {
			quote.Low52Week = low
			setQuoteFieldSource(&quote, "low52Week", "Parallel Ocean minimum Yahoo daily low over trailing year")
		}
	}
	completed := completedDailyObservations(observations, quote.MarketState, asOf)
	quote.MovingAverage50Day = averageCloses(completed, 50)
	quote.MovingAverage200Day = averageCloses(completed, 200)
	quote.AverageVolume3Month = averageVolumesSince(completed, asOf.AddDate(0, -3, 0))
	quote.AverageVolume10Day = averageVolumes(completed, 10)
	if quote.MovingAverage50Day != nil {
		setQuoteFieldSource(&quote, "movingAverage50Day", "Parallel Ocean mean of 50 completed Yahoo daily closes")
	}
	if quote.MovingAverage200Day != nil {
		setQuoteFieldSource(&quote, "movingAverage200Day", "Parallel Ocean mean of 200 completed Yahoo daily closes")
	}
	if quote.AverageVolume3Month != nil {
		setQuoteFieldSource(&quote, "averageVolume3Month", "Parallel Ocean mean Yahoo volume over completed sessions in trailing three calendar months")
	}
	if quote.AverageVolume10Day != nil {
		setQuoteFieldSource(&quote, "averageVolume10Day", "Parallel Ocean mean Yahoo volume over 10 completed sessions")
	}

	dividends := yahooDividends(history, asOf)
	if len(dividends) > 0 {
		latest := dividends[len(dividends)-1]
		// Yahoo chart dividend events are dated on the ex-dividend session.
		// The chart feed does not expose the separate cash-payment date, so
		// LastDividendDate intentionally remains absent rather than inferred.
		quote.ExDividendDate = latest.date.Format("2006-01-02")
		setQuoteFieldSource(&quote, "exDividendDate", "Yahoo Finance chart dividend event date (ex-dividend session)")
		trailing := trailingDividendRate(dividends, asOf)
		if trailing > 0 {
			quote.TrailingAnnualDividendRate = liveFloat(trailing)
			quote.TrailingAnnualDividendYield = liveFloat(trailing / *quote.Price)
			setQuoteFieldSource(&quote, "trailingAnnualDividendRate", "Parallel Ocean sum of Yahoo dividend events over trailing 12 months")
			setQuoteFieldSource(&quote, "trailingAnnualDividendYield", "Parallel Ocean calculation: trailing dividend rate / latest Yahoo price snapshot")
		}
		if forward := inferredForwardDividendRate(dividends, asOf); forward != nil {
			quote.ForwardAnnualDividendRate = forward
			quote.ForwardAnnualDividendYield = liveFloat(*forward / *quote.Price)
			setQuoteFieldSource(&quote, "forwardAnnualDividendRate", "Parallel Ocean estimate: latest Yahoo dividend x conservatively inferred observed frequency")
			setQuoteFieldSource(&quote, "forwardAnnualDividendYield", "Parallel Ocean estimate: inferred forward dividend rate / latest Yahoo price snapshot")
		}
		quote.AverageDividendYield5Year = fiveYearAverageDividendYield(dividends, observations, asOf)
		if quote.AverageDividendYield5Year != nil {
			setQuoteFieldSource(&quote, "averageDividendYield5Year", "Parallel Ocean mean of five completed calendar-year dividend sums / year-end Yahoo closes")
		}
	}
	quote.LastSplitFactor, quote.LastSplitDate = yahooLatestSplit(history, asOf)
	if quote.LastSplitDate != "" {
		setQuoteFieldSource(&quote, "lastSplitFactor", "Yahoo Finance latest chart split event")
		setQuoteFieldSource(&quote, "lastSplitDate", "Yahoo Finance latest chart split event date")
	}
	quote.History = yahooMonthlyStatisticSnapshots(history, observations, quote.MarketState, asOf)
	return quote, nil
}

func mergeYahooQuoteObservations(history, current []quoteObservation) []quoteObservation {
	if len(history) == 0 {
		return current
	}
	rowsByDate := make(map[string]quoteObservation, len(history)+len(current))
	for _, row := range history {
		rowsByDate[row.date.UTC().Format("2006-01-02")] = row
	}
	for _, row := range current {
		key := row.date.UTC().Format("2006-01-02")
		if prior, ok := rowsByDate[key]; ok && row.adjustedClose == row.close && prior.adjustedClose > 0 {
			row.adjustedClose = prior.adjustedClose
		}
		rowsByDate[key] = row
	}
	rows := make([]quoteObservation, 0, len(rowsByDate))
	for _, row := range rowsByDate {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].date.Before(rows[j].date) })
	return rows
}

func yahooQuoteObservations(result yahooQuoteResult) []quoteObservation {
	if len(result.Indicators.Quote) == 0 {
		return nil
	}
	series := result.Indicators.Quote[0]
	var adjusted []*float64
	if len(result.Indicators.AdjClose) > 0 {
		adjusted = result.Indicators.AdjClose[0].Close
	}
	rows := make([]quoteObservation, 0, len(result.Timestamps))
	for index, timestamp := range result.Timestamps {
		if index >= len(series.Close) || series.Close[index] == nil || *series.Close[index] <= 0 {
			continue
		}
		row := quoteObservation{date: time.Unix(timestamp, 0).UTC(), close: *series.Close[index], adjustedClose: *series.Close[index]}
		if index < len(adjusted) && adjusted[index] != nil && *adjusted[index] > 0 {
			row.adjustedClose = *adjusted[index]
		}
		if index < len(series.High) && series.High[index] != nil && *series.High[index] > 0 {
			row.high = *series.High[index]
		} else {
			row.high = row.close
		}
		if index < len(series.Low) && series.Low[index] != nil && *series.Low[index] > 0 {
			row.low = *series.Low[index]
		} else {
			row.low = row.close
		}
		if index < len(series.Volume) && series.Volume[index] != nil && *series.Volume[index] >= 0 {
			row.volume = liveFloat(*series.Volume[index])
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].date.Before(rows[j].date) })
	return rows
}

func previousSessionClose(rows []quoteObservation, asOf time.Time) *float64 {
	if len(rows) == 0 {
		return nil
	}
	latest := len(rows) - 1
	if sameUTCDate(rows[latest].date, asOf) {
		latest--
	}
	if latest < 0 {
		return nil
	}
	return liveFloat(rows[latest].close)
}

func completedDailyObservations(rows []quoteObservation, marketState string, asOf time.Time) []quoteObservation {
	if len(rows) == 0 || !strings.EqualFold(marketState, "REGULAR") || !sameUTCDate(rows[len(rows)-1].date, asOf) {
		return rows
	}
	return rows[:len(rows)-1]
}

// yahooMonthlyStatisticSnapshots derives one observation from the last
// completed daily session represented in each calendar month. It intentionally
// contains no SEC share basis, market cap, or enterprise value: those belong to
// the separately persisted current quote after Pipeline enrichment.
func yahooMonthlyStatisticSnapshots(result yahooQuoteResult, observations []quoteObservation, marketState string, asOf time.Time) []model.StatisticSnapshot {
	completed := completedDailyObservations(observations, marketState, asOf)
	if len(completed) == 0 {
		return nil
	}
	monthEnds := make([]int, 0, len(completed)/20+1)
	for index, row := range completed {
		if index == len(completed)-1 || row.date.Format("2006-01") != completed[index+1].date.Format("2006-01") {
			monthEnds = append(monthEnds, index)
		}
	}

	snapshots := make([]model.StatisticSnapshot, 0, len(monthEnds))
	for _, index := range monthEnds {
		row := completed[index]
		throughSession := completed[:index+1]
		snapshot := model.StatisticSnapshot{
			AsOf:       row.date.UTC().Format(time.RFC3339),
			Source:     yahooBackfillSource,
			AsOfSource: yahooBackfillAsOfSource,
			Numeric:    make(map[string]float64),
			Text:       make(map[string]string),
			Sources:    make(map[string]string),
		}
		addNumeric := func(key string, value *float64, source string) {
			if value == nil {
				return
			}
			snapshot.Numeric[key] = *value
			snapshot.Sources[key] = source
		}
		addText := func(key, value, source string) {
			if value == "" {
				return
			}
			snapshot.Text[key] = value
			snapshot.Sources[key] = source
		}

		addNumeric("price", liveFloat(row.close), yahooDailyCloseSource)
		if index > 0 {
			previous := throughSession[index-1].close
			change := row.close - previous
			addNumeric("previous-close", liveFloat(previous), "Yahoo Finance previous completed-session daily chart close")
			addNumeric("change", liveFloat(change), "Parallel Ocean calculation: completed-session close - previous completed-session close")
			addNumeric("change-percent", liveFloat(change/previous), "Parallel Ocean calculation: completed-session change / previous completed-session close")
		}

		cutoff52Week := row.date.AddDate(-1, 0, 0)
		if historyCovers(throughSession, cutoff52Week) {
			if baseline := closeOnOrBefore(throughSession, cutoff52Week); baseline != nil {
				addNumeric("change-52-week", liveFloat(row.close / *baseline - 1), "Parallel Ocean calculation: completed-session close / Yahoo daily close on or before one-year cutoff - 1")
			}
			high, low := trailingHighLow(throughSession, cutoff52Week)
			addNumeric("high-52-week", high, "Parallel Ocean maximum Yahoo daily high over trailing year; daily close is used only where high is absent")
			addNumeric("low-52-week", low, "Parallel Ocean minimum Yahoo daily low over trailing year; daily close is used only where low is absent")
		}
		addNumeric("moving-average-50d", averageCloses(throughSession, 50), "Parallel Ocean mean of 50 completed Yahoo daily closes through snapshot session")
		addNumeric("moving-average-200d", averageCloses(throughSession, 200), "Parallel Ocean mean of 200 completed Yahoo daily closes through snapshot session")
		addNumeric("average-volume-10d", averageVolumes(throughSession, 10), "Parallel Ocean mean of 10 completed Yahoo daily volumes through snapshot session")
		cutoff3Month := row.date.AddDate(0, -3, 0)
		if historyCovers(throughSession, cutoff3Month) {
			addNumeric("average-volume-3m", averageVolumesSince(throughSession, cutoff3Month), "Parallel Ocean mean Yahoo daily volume over completed sessions in trailing three calendar months")
		}

		dividends := yahooDividends(result, row.date)
		if len(dividends) > 0 {
			latest := dividends[len(dividends)-1]
			addText("ex-dividend-date", latest.date.Format("2006-01-02"), "Yahoo Finance chart dividend event date (ex-dividend session)")
			if historyCovers(throughSession, cutoff52Week) {
				if trailing := trailingDividendRate(dividends, row.date); trailing > 0 {
					addNumeric("trailing-dividend-rate", liveFloat(trailing), "Parallel Ocean sum of Yahoo dividend events over trailing 12 months through snapshot session")
					addNumeric("trailing-dividend-yield", liveFloat(trailing/row.close), "Parallel Ocean calculation: trailing dividend rate / completed-session Yahoo close")
				}
			}
			if historyCovers(throughSession, row.date.AddDate(0, -18, 0)) {
				if forward := inferredForwardDividendRate(dividends, row.date); forward != nil {
					addNumeric("forward-dividend-rate", forward, "Parallel Ocean estimate: latest Yahoo dividend through snapshot date x conservatively inferred observed frequency")
					addNumeric("forward-dividend-yield", liveFloat(*forward/row.close), "Parallel Ocean estimate: inferred forward dividend rate / completed-session Yahoo close")
				}
			}
			addNumeric("average-dividend-yield-5y", fiveYearAverageDividendYield(dividends, throughSession, row.date), "Parallel Ocean mean of five completed calendar-year Yahoo dividend-event sums / year-end daily closes")
		}

		factor, splitDate := yahooLatestSplit(result, row.date)
		addText("last-split-factor", factor, "Yahoo Finance latest chart split event through snapshot session")
		addText("last-split-date", splitDate, "Yahoo Finance latest chart split event date through snapshot session")
		if len(snapshot.Text) == 0 {
			snapshot.Text = nil
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func historyCovers(rows []quoteObservation, cutoff time.Time) bool {
	return len(rows) > 0 && !rows[0].date.After(cutoff)
}

func sameUTCDate(left, right time.Time) bool {
	left = left.UTC()
	right = right.UTC()
	return left.Year() == right.Year() && left.Month() == right.Month() && left.Day() == right.Day()
}

func (y *YahooMarket) beta5YMonthly(ctx context.Context, ticker string, target yahooQuoteResult) *float64 {
	asOf := time.Now().UTC()
	if target.Meta.RegularMarketTime > 0 {
		asOf = time.Unix(target.Meta.RegularMarketTime, 0).UTC()
	}
	targetMonths := monthlyAdjustedCloses(yahooQuoteObservations(target), asOf)
	if longestRecentMonthlyRun(targetMonths) < 61 {
		return nil
	}
	if strings.EqualFold(ticker, "SPY") {
		return liveFloat(1)
	}
	benchmark, err := y.cachedQuoteHistory(ctx, "SPY")
	if err != nil {
		return nil
	}
	benchmarkMonths := monthlyAdjustedCloses(yahooQuoteObservations(benchmark), asOf)
	return alignedMonthlyBeta(targetMonths, benchmarkMonths)
}

func monthlyAdjustedCloses(rows []quoteObservation, asOf time.Time) map[string]float64 {
	currentMonth := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC)
	values := make(map[string]float64)
	for _, row := range rows {
		if !row.date.Before(currentMonth) {
			continue
		}
		values[row.date.Format("2006-01")] = row.adjustedClose
	}
	return values
}

func longestRecentMonthlyRun(values map[string]float64) int {
	keys := sortedMonthKeys(values)
	if len(keys) == 0 {
		return 0
	}
	run := 1
	for index := len(keys) - 1; index > 0; index-- {
		if monthNumber(keys[index])-monthNumber(keys[index-1]) != 1 {
			break
		}
		run++
	}
	return run
}

func alignedMonthlyBeta(target, benchmark map[string]float64) *float64 {
	common := make(map[string]float64)
	for month, value := range target {
		if _, ok := benchmark[month]; ok {
			common[month] = value
		}
	}
	keys := sortedMonthKeys(common)
	if len(keys) < 61 {
		return nil
	}
	start := len(keys) - 61
	for index := start + 1; index < len(keys); index++ {
		if monthNumber(keys[index])-monthNumber(keys[index-1]) != 1 {
			return nil
		}
	}
	targetReturns := make([]float64, 0, 60)
	benchmarkReturns := make([]float64, 0, 60)
	for index := start + 1; index < len(keys); index++ {
		previous, current := keys[index-1], keys[index]
		if target[previous] <= 0 || target[current] <= 0 || benchmark[previous] <= 0 || benchmark[current] <= 0 {
			return nil
		}
		targetReturns = append(targetReturns, target[current]/target[previous]-1)
		benchmarkReturns = append(benchmarkReturns, benchmark[current]/benchmark[previous]-1)
	}
	meanTarget, meanBenchmark := 0.0, 0.0
	for index := range targetReturns {
		meanTarget += targetReturns[index]
		meanBenchmark += benchmarkReturns[index]
	}
	meanTarget /= float64(len(targetReturns))
	meanBenchmark /= float64(len(benchmarkReturns))
	covariance, variance := 0.0, 0.0
	for index := range targetReturns {
		targetDelta := targetReturns[index] - meanTarget
		benchmarkDelta := benchmarkReturns[index] - meanBenchmark
		covariance += targetDelta * benchmarkDelta
		variance += benchmarkDelta * benchmarkDelta
	}
	if variance <= 1e-12 {
		return nil
	}
	return liveFloat(covariance / variance)
}

func sortedMonthKeys(values map[string]float64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func monthNumber(value string) int {
	parsed, err := time.Parse("2006-01", value)
	if err != nil {
		return 0
	}
	return parsed.Year()*12 + int(parsed.Month())
}

func yahooMarketState(result yahooQuoteResult, now time.Time) string {
	if state := strings.ToUpper(strings.TrimSpace(result.Meta.MarketState)); state != "" {
		return state
	}
	unix := now.Unix()
	period := result.Meta.CurrentTradingPeriod
	switch {
	case period.Pre.Start > 0 && unix >= period.Pre.Start && unix < period.Pre.End:
		return "PRE"
	case period.Regular.Start > 0 && unix >= period.Regular.Start && unix < period.Regular.End:
		return "REGULAR"
	case period.Post.Start > 0 && unix >= period.Post.Start && unix < period.Post.End:
		return "POST"
	case period.Regular.Start > 0:
		return "CLOSED"
	default:
		return "UNKNOWN"
	}
}

func closeOnOrBefore(rows []quoteObservation, date time.Time) *float64 {
	var value *float64
	for _, row := range rows {
		if row.date.After(date) {
			break
		}
		value = liveFloat(row.close)
	}
	return value
}

func trailingHighLow(rows []quoteObservation, cutoff time.Time) (*float64, *float64) {
	var high, low *float64
	for _, row := range rows {
		if row.date.Before(cutoff) {
			continue
		}
		if high == nil || row.high > *high {
			high = liveFloat(row.high)
		}
		if low == nil || row.low < *low {
			low = liveFloat(row.low)
		}
	}
	return high, low
}

func averageCloses(rows []quoteObservation, count int) *float64 {
	if len(rows) < count || count < 1 {
		return nil
	}
	total := 0.0
	for _, row := range rows[len(rows)-count:] {
		total += row.close
	}
	return liveFloat(total / float64(count))
}

func averageVolumes(rows []quoteObservation, count int) *float64 {
	values := make([]float64, 0, count)
	for index := len(rows) - 1; index >= 0 && len(values) < count; index-- {
		if rows[index].volume != nil {
			values = append(values, *rows[index].volume)
		}
	}
	if len(values) < count || count < 1 {
		return nil
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return liveFloat(total / float64(len(values)))
}

func averageVolumesSince(rows []quoteObservation, cutoff time.Time) *float64 {
	cutoff = cutoff.UTC()
	cutoff = time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, time.UTC)
	total := 0.0
	count := 0
	for _, row := range rows {
		if row.date.Before(cutoff) || row.volume == nil {
			continue
		}
		total += *row.volume
		count++
	}
	if count == 0 {
		return nil
	}
	return liveFloat(total / float64(count))
}

func yahooDividends(result yahooQuoteResult, asOf time.Time) []dividendObservation {
	rows := make([]dividendObservation, 0, len(result.Events.Dividends))
	for _, event := range result.Events.Dividends {
		date := time.Unix(event.Date, 0).UTC()
		if event.Date <= 0 || event.Amount <= 0 || date.After(asOf) {
			continue
		}
		rows = append(rows, dividendObservation{date: date, amount: event.Amount})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].date.Before(rows[j].date) })
	return rows
}

func trailingDividendRate(rows []dividendObservation, asOf time.Time) float64 {
	cutoff := asOf.AddDate(-1, 0, 0)
	total := 0.0
	for _, row := range rows {
		if !row.date.Before(cutoff) && !row.date.After(asOf) {
			total += row.amount
		}
	}
	return total
}

func inferredForwardDividendRate(rows []dividendObservation, asOf time.Time) *float64 {
	cutoff := asOf.AddDate(0, -18, 0)
	recent := make([]dividendObservation, 0, 8)
	for _, row := range rows {
		if !row.date.Before(cutoff) && !row.date.After(asOf) {
			recent = append(recent, row)
		}
	}
	if len(recent) < 2 {
		return nil
	}
	intervals := make([]float64, 0, len(recent)-1)
	for index := 1; index < len(recent); index++ {
		intervals = append(intervals, recent[index].date.Sub(recent[index-1].date).Hours()/24)
	}
	sort.Float64s(intervals)
	median := intervals[len(intervals)/2]
	frequency := 0
	switch {
	case median >= 24 && median <= 45:
		frequency = 12
	case median >= 65 && median <= 120:
		frequency = 4
	case median >= 145 && median <= 210:
		frequency = 2
	case median >= 300 && median <= 400:
		frequency = 1
	default:
		return nil
	}
	needed := frequency
	if needed > 4 {
		needed = 4
	}
	if len(recent) < needed {
		return nil
	}
	target := 365.25 / float64(frequency)
	check := intervals
	if len(check) > needed {
		check = check[len(check)-needed:]
	}
	for _, days := range check {
		if days < target*0.65 || days > target*1.35 {
			return nil
		}
	}
	latest := recent[len(recent)-1].amount
	for _, row := range recent[len(recent)-needed:] {
		if row.amount < latest*0.5 || row.amount > latest*1.5 {
			return nil
		}
	}
	return liveFloat(latest * float64(frequency))
}

func fiveYearAverageDividendYield(dividends []dividendObservation, prices []quoteObservation, asOf time.Time) *float64 {
	yields := make([]float64, 0, 5)
	for year := asOf.Year() - 5; year < asOf.Year(); year++ {
		start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(year, time.December, 31, 23, 59, 59, 0, time.UTC)
		dividend := 0.0
		for _, row := range dividends {
			if !row.date.Before(start) && !row.date.After(end) {
				dividend += row.amount
			}
		}
		close := closeOnOrBefore(prices, end)
		if dividend <= 0 || close == nil || *close <= 0 {
			return nil
		}
		yields = append(yields, dividend / *close)
	}
	if len(yields) != 5 {
		return nil
	}
	total := 0.0
	for _, value := range yields {
		total += value
	}
	return liveFloat(total / 5)
}

func yahooLatestSplit(result yahooQuoteResult, asOf time.Time) (string, string) {
	var latestDate time.Time
	factor := ""
	for _, event := range result.Events.Splits {
		date := time.Unix(event.Date, 0).UTC()
		if event.Date <= 0 || date.After(asOf) || !date.After(latestDate) {
			continue
		}
		latestDate = date
		factor = strings.TrimSpace(event.SplitRatio)
		if factor == "" && event.Numerator > 0 && event.Denominator > 0 {
			factor = fmt.Sprintf("%g:%g", event.Numerator, event.Denominator)
		}
	}
	if latestDate.IsZero() {
		return "", ""
	}
	return factor, latestDate.Format("2006-01-02")
}

func positiveLiveFloat(value *float64) *float64 {
	if value == nil || *value <= 0 {
		return nil
	}
	return liveFloat(*value)
}

func liveFloat(value float64) *float64 { return &value }

func setQuoteFieldSource(quote *model.LiveQuote, field, source string) {
	if quote == nil || field == "" || source == "" {
		return
	}
	if quote.FieldSources == nil {
		quote.FieldSources = make(map[string]string)
	}
	quote.FieldSources[field] = source
}
