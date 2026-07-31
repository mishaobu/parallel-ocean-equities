package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func TestYahooMarketDecodesMonthlyClosesAtMonthEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("interval") != "1mo" || r.URL.Query().Get("events") != "history,splits" || r.Header.Get("User-Agent") != "parallel-ocean-equities/1.0" {
			t.Fatalf("unexpected request: %s user-agent=%s", r.URL.String(), r.Header.Get("User-Agent"))
		}
		fmt.Fprint(w, `{"chart":{"result":[{"timestamp":[1704067200,1706745600,1709251200],"events":{"splits":{"split":{"date":1707955200,"numerator":10,"denominator":1,"splitRatio":"10:1"}}},"indicators":{"quote":[{"close":[100,null,120]}],"adjclose":[{"adjclose":[90,null,115]}]}}],"error":null}}`)
	}))
	defer server.Close()
	provider := NewYahooMarket(server.Client())
	provider.baseURL = server.URL
	end := time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC)
	prices, source, basis, err := provider.HistoryWithPriceBasis(context.Background(), "AMZN", time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC), end)
	if err != nil {
		t.Fatal(err)
	}
	if source != "Yahoo Finance monthly close and adjusted close" || len(prices) != 2 {
		t.Fatalf("source=%q prices=%v", source, prices)
	}
	if prices[0].Date != "2024-01-31" || prices[1].Date != "2024-03-01" {
		t.Fatalf("unexpected normalized dates: %v", prices)
	}
	if prices[0].TotalReturnClose == nil || *prices[0].TotalReturnClose != 90 || prices[1].TotalReturnClose == nil || *prices[1].TotalReturnClose != 115 {
		t.Fatalf("adjusted closes missing: %v", prices)
	}
	if basis == nil || basis.Provider != "yahoo-finance" || basis.Adjustment != "split-adjusted" || !basis.SplitCoverageComplete || basis.SplitCoverageStart != "2024-01-01" || basis.SplitCoverageEnd != "2024-03-01" || len(basis.StockSplits) != 1 || basis.StockSplits[0].Date != "2024-02-15" || basis.StockSplits[0].Ratio != 10 {
		t.Fatalf("historical price basis is not bound to the response: %+v", basis)
	}
}

func TestYahooHistoricalPriceBasisValidatesEventsAndRetainsReverseSplits(t *testing.T) {
	start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	timestamps := []int64{start.Unix(), end.Unix()}
	reverse := map[string]yahooSplitEvent{
		"reverse": {Date: time.Date(2025, time.June, 2, 13, 30, 0, 0, time.UTC).Unix(), Numerator: 1, Denominator: 10},
	}
	basis := yahooHistoricalPriceBasis(timestamps, reverse, "Yahoo Finance monthly close and adjusted close")
	if basis == nil || len(basis.StockSplits) != 1 || basis.StockSplits[0].Date != "2025-06-02" || basis.StockSplits[0].Ratio != 0.1 {
		t.Fatalf("valid reverse split was not retained: %+v", basis)
	}

	conflicting := map[string]yahooSplitEvent{
		"one": {Date: time.Date(2025, time.June, 2, 13, 30, 0, 0, time.UTC).Unix(), Numerator: 2, Denominator: 1},
		"two": {Date: time.Date(2025, time.June, 2, 16, 0, 0, 0, time.UTC).Unix(), Numerator: 3, Denominator: 1},
	}
	if got := yahooHistoricalPriceBasis(timestamps, conflicting, "Yahoo Finance monthly close and adjusted close"); got != nil {
		t.Fatalf("conflicting same-day splits produced a trusted basis: %+v", got)
	}
	future := map[string]yahooSplitEvent{
		"future": {Date: end.AddDate(0, 0, 1).Unix(), Numerator: 2, Denominator: 1},
	}
	if got := yahooHistoricalPriceBasis(timestamps, future, "Yahoo Finance monthly close and adjusted close"); got != nil {
		t.Fatalf("post-coverage split produced a trusted basis: %+v", got)
	}
}

func TestYahooMarketBuildsLiveQuoteFromChartHistory(t *testing.T) {
	asOf := time.Date(2026, time.July, 31, 15, 30, 0, 0, time.UTC)
	start := asOf.AddDate(0, 0, -399)
	timestamps := make([]int64, 400)
	closes := make([]float64, 400)
	volumes := make([]float64, 400)
	for index := range timestamps {
		timestamps[index] = start.AddDate(0, 0, index).Unix()
		closes[index] = float64(index + 1)
		volumes[index] = float64(1000 + index)
	}
	dividends := map[string]any{}
	for index, days := range []int{-300, -210, -120, -30} {
		date := asOf.AddDate(0, 0, days).Unix()
		dividends[fmt.Sprintf("%d", index)] = map[string]any{"amount": 0.25, "date": date}
	}
	historyPayload := map[string]any{"chart": map[string]any{
		"result": []any{map[string]any{
			"meta": map[string]any{
				"symbol": "AAPL", "currency": "USD", "fullExchangeName": "NasdaqGS", "marketState": "REGULAR",
				"regularMarketPrice": 405.0, "regularMarketTime": asOf.Unix(), "previousClose": 400.0,
				"fiftyTwoWeekHigh": 420.0, "fiftyTwoWeekLow": 180.0,
			},
			"timestamp":  timestamps,
			"indicators": map[string]any{"quote": []any{map[string]any{"close": closes, "volume": volumes}}},
			"events": map[string]any{
				"dividends": dividends,
				"splits":    map[string]any{"split": map[string]any{"date": asOf.AddDate(-2, 0, 0).Unix(), "numerator": 4, "denominator": 1, "splitRatio": "4:1"}},
			},
		}},
		"error": nil,
	}}
	currentPayload := map[string]any{"chart": map[string]any{
		"result": []any{map[string]any{
			"meta": map[string]any{
				"symbol": "AAPL", "currency": "USD", "fullExchangeName": "NasdaqGS", "marketState": "REGULAR",
				"regularMarketPrice": 405.0, "regularMarketTime": asOf.Unix(), "previousClose": 400.0,
				"fiftyTwoWeekHigh": 420.0, "fiftyTwoWeekLow": 180.0,
			},
			"timestamp": timestamps[len(timestamps)-5:],
			"indicators": map[string]any{"quote": []any{map[string]any{
				"close": closes[len(closes)-5:], "volume": volumes[len(volumes)-5:],
			}}},
		}},
		"error": nil,
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("interval") != "1d" {
			http.Error(w, "unexpected quote interval: "+r.URL.String(), http.StatusBadRequest)
			return
		}
		var payload map[string]any
		switch r.URL.Query().Get("range") {
		case "5d":
			if r.URL.Query().Get("events") != "div,splits" {
				http.Error(w, "unexpected current quote request: "+r.URL.String(), http.StatusBadRequest)
				return
			}
			payload = currentPayload
		case "10y":
			if r.URL.Query().Get("events") != "div,splits" {
				http.Error(w, "unexpected history request: "+r.URL.String(), http.StatusBadRequest)
				return
			}
			payload = historyPayload
		default:
			http.Error(w, "unexpected quote request: "+r.URL.String(), http.StatusBadRequest)
			return
		}
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()
	provider := NewYahooMarket(server.Client())
	provider.baseURL = server.URL
	quote, err := provider.Quote(context.Background(), "AAPL")
	if err != nil {
		t.Fatal(err)
	}
	if quote.Ticker != "AAPL" || quote.Source != yahooQuoteSource || quote.MarketState != "REGULAR" || quote.Exchange != "NasdaqGS" || quote.Currency != "USD" {
		t.Fatalf("unexpected quote identity: %+v", quote)
	}
	if !quote.StockSplitCoverageComplete || quote.StockSplitCoverageStart == "" || len(quote.StockSplits) != 1 || quote.StockSplits[0].Ratio != 4 {
		t.Fatalf("split coverage missing: %+v", quote)
	}
	assertLiveFloat(t, "price", quote.Price, 405)
	assertLiveFloat(t, "previous close", quote.PreviousClose, 400)
	assertLiveFloat(t, "change", quote.Change, 5)
	assertLiveFloat(t, "change percent", quote.ChangePercent, 0.0125)
	assertLiveFloat(t, "50-day average", quote.MovingAverage50Day, 374.5)
	assertLiveFloat(t, "200-day average", quote.MovingAverage200Day, 299.5)
	assertLiveFloat(t, "10-day average volume", quote.AverageVolume10Day, 1393.5)
	assertLiveFloat(t, "3-month average volume", quote.AverageVolume3Month, 1353)
	assertLiveFloat(t, "trailing dividend rate", quote.TrailingAnnualDividendRate, 1)
	assertLiveFloat(t, "forward dividend rate", quote.ForwardAnnualDividendRate, 1)
	assertLiveFloat(t, "52-week high", quote.High52Week, 420)
	assertLiveFloat(t, "52-week low", quote.Low52Week, 180)
	if quote.AverageDividendYield5Year != nil {
		t.Fatalf("partial five-year history should not produce an average yield: %+v", quote)
	}
	if quote.ExDividendDate != asOf.AddDate(0, 0, -30).Format("2006-01-02") || quote.LastDividendDate != "" {
		t.Fatalf("unexpected dividend dates: %+v", quote)
	}
	if !strings.Contains(quote.FieldSources["forwardAnnualDividendRate"], "estimate") || !strings.Contains(quote.FieldSources["averageVolume10Day"], "completed") {
		t.Fatalf("calculated fields need explicit methodology: %+v", quote.FieldSources)
	}
	if quote.LastSplitFactor != "4:1" || quote.LastSplitDate != asOf.AddDate(-2, 0, 0).Format("2006-01-02") {
		t.Fatalf("unexpected split: %+v", quote)
	}
	if len(quote.History) == 0 || quote.History[len(quote.History)-1].AsOf != asOf.AddDate(0, 0, -1).Format(time.RFC3339) {
		t.Fatalf("monthly history should end at the last completed session: %+v", quote.History)
	}
}

func TestYahooQuoteCachesLongHistoryAndUsesRecentStaleDataOnRefreshError(t *testing.T) {
	asOf := time.Date(2026, time.July, 31, 15, 30, 0, 0, time.UTC)
	currentPayload, historyPayload := yahooQuoteCacheTestPayloads(asOf)
	var currentCalls atomic.Int32
	var historyCalls atomic.Int32
	var failHistory atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("range") {
		case "5d":
			currentCalls.Add(1)
			_ = json.NewEncoder(w).Encode(currentPayload)
		case "10y":
			historyCalls.Add(1)
			if failHistory.Load() {
				http.Error(w, "rate limited", http.StatusTooManyRequests)
				return
			}
			_ = json.NewEncoder(w).Encode(historyPayload)
		default:
			http.Error(w, "unexpected request: "+r.URL.String(), http.StatusBadRequest)
		}
	}))
	defer server.Close()
	provider := NewYahooMarket(server.Client())
	provider.baseURL = server.URL
	defer clearYahooHistoryCacheForProvider(provider)

	first, err := provider.Quote(context.Background(), "cache")
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.Quote(context.Background(), "CACHE")
	if err != nil {
		t.Fatal(err)
	}
	if currentCalls.Load() != 2 || historyCalls.Load() != 1 {
		t.Fatalf("requests current=%d history=%d, want current=2 history=1", currentCalls.Load(), historyCalls.Load())
	}
	if len(first.History) == 0 || len(second.History) == 0 {
		t.Fatalf("cached history disappeared: first=%+v second=%+v", first.History, second.History)
	}
	if first.HistoryCacheStatus != "fresh" || second.HistoryCacheStatus != "fresh" || first.HistoryCacheAsOf == "" || second.HistoryCacheAsOf != first.HistoryCacheAsOf {
		t.Fatalf("fresh cache metadata first=%+v second=%+v", first, second)
	}

	key := yahooHistoryCacheKey{provider: provider, ticker: "CACHE"}
	yahooHistoryCache.Lock()
	entry := yahooHistoryCache.entries[key]
	entry.cachedAt = time.Now().UTC().Add(-yahooHistoryCacheTTL - time.Minute)
	expiredCacheAsOf := entry.cachedAt.Format(time.RFC3339)
	yahooHistoryCache.entries[key] = entry
	yahooHistoryCache.Unlock()
	failHistory.Store(true)

	stale, err := provider.Quote(context.Background(), "CACHE")
	if err != nil {
		t.Fatalf("recent stale history should preserve a live quote during refresh failure: %v", err)
	}
	if currentCalls.Load() != 3 || historyCalls.Load() != 2 || len(stale.History) == 0 {
		t.Fatalf("stale fallback current=%d history=%d quote=%+v", currentCalls.Load(), historyCalls.Load(), stale)
	}
	if stale.HistoryCacheStatus != "stale" || stale.HistoryCacheAsOf != expiredCacheAsOf || stale.HistoryRefreshFailureKind != SnapshotFailureThrottled || !stale.HistoryRefreshFailed {
		t.Fatalf("stale fallback metadata = %+v", stale)
	}
	assertLiveFloat(t, "stale-fallback live price", stale.Price, 101)

	backoff, err := provider.Quote(context.Background(), "CACHE")
	if err != nil {
		t.Fatal(err)
	}
	if currentCalls.Load() != 4 || historyCalls.Load() != 2 {
		t.Fatalf("stale retry backoff did not suppress 10y fetch: current=%d history=%d", currentCalls.Load(), historyCalls.Load())
	}
	if backoff.HistoryCacheStatus != "stale" || backoff.HistoryCacheAsOf != expiredCacheAsOf || backoff.HistoryRefreshFailureKind != SnapshotFailureThrottled || backoff.HistoryRefreshFailed {
		t.Fatalf("backoff cache metadata = %+v", backoff)
	}

	// A cold-start history failure also gets a negative-cache backoff. Current
	// quote data remains available, while page traffic cannot retry the
	// expensive ten-year request on every one-minute live-quote cache expiry.
	cold, err := provider.Quote(context.Background(), "COLD")
	if err != nil {
		t.Fatalf("cold history failure should preserve a live quote: %v", err)
	}
	if currentCalls.Load() != 5 || historyCalls.Load() != 3 || cold.HistoryCacheStatus != "unavailable" || cold.HistoryRefreshFailureKind != SnapshotFailureThrottled || !cold.HistoryRefreshFailed {
		t.Fatalf("cold failure current=%d history=%d quote=%+v", currentCalls.Load(), historyCalls.Load(), cold)
	}
	coldBackoff, err := provider.Quote(context.Background(), "COLD")
	if err != nil {
		t.Fatal(err)
	}
	if currentCalls.Load() != 6 || historyCalls.Load() != 3 {
		t.Fatalf("cold retry backoff did not suppress 10y fetch: current=%d history=%d", currentCalls.Load(), historyCalls.Load())
	}
	if coldBackoff.HistoryCacheStatus != "unavailable" || coldBackoff.HistoryRefreshFailureKind != SnapshotFailureThrottled || coldBackoff.HistoryRefreshFailed {
		t.Fatalf("cold backoff metadata = %+v", coldBackoff)
	}
}

func TestYahooQuoteHistoryFetchIsSingleflight(t *testing.T) {
	asOf := time.Date(2026, time.July, 31, 15, 30, 0, 0, time.UTC)
	currentPayload, historyPayload := yahooQuoteCacheTestPayloads(asOf)
	var currentCalls atomic.Int32
	var historyCalls atomic.Int32
	historyStarted := make(chan struct{})
	releaseHistory := make(chan struct{})
	var startedOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("range") {
		case "5d":
			currentCalls.Add(1)
			_ = json.NewEncoder(w).Encode(currentPayload)
		case "10y":
			historyCalls.Add(1)
			startedOnce.Do(func() { close(historyStarted) })
			<-releaseHistory
			_ = json.NewEncoder(w).Encode(historyPayload)
		default:
			http.Error(w, "unexpected request: "+r.URL.String(), http.StatusBadRequest)
		}
	}))
	defer server.Close()
	provider := NewYahooMarket(server.Client())
	provider.baseURL = server.URL
	defer clearYahooHistoryCacheForProvider(provider)

	const callers = 12
	start := make(chan struct{})
	errorsByCaller := make(chan error, callers)
	var wait sync.WaitGroup
	wait.Add(callers)
	for range callers {
		go func() {
			defer wait.Done()
			<-start
			_, err := provider.Quote(context.Background(), "SYNC")
			errorsByCaller <- err
		}()
	}
	close(start)
	select {
	case <-historyStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("history request did not start")
	}
	close(releaseHistory)
	wait.Wait()
	close(errorsByCaller)
	for err := range errorsByCaller {
		if err != nil {
			t.Fatal(err)
		}
	}
	if historyCalls.Load() != 1 || currentCalls.Load() != callers {
		t.Fatalf("requests current=%d history=%d, want current=%d history=1", currentCalls.Load(), historyCalls.Load(), callers)
	}
}

func yahooQuoteCacheTestPayloads(asOf time.Time) (map[string]any, map[string]any) {
	currentPayload := map[string]any{"chart": map[string]any{
		"result": []any{map[string]any{
			"meta": map[string]any{
				"symbol": "CACHE", "currency": "USD", "fullExchangeName": "Test", "marketState": "REGULAR",
				"regularMarketPrice": 101.0, "regularMarketTime": asOf.Unix(), "previousClose": 100.0,
				"fiftyTwoWeekHigh": 110.0, "fiftyTwoWeekLow": 80.0,
			},
			"timestamp": []int64{asOf.AddDate(0, 0, -1).Unix(), asOf.Unix()},
			"indicators": map[string]any{"quote": []any{map[string]any{
				"close": []float64{100, 101}, "high": []float64{102, 103}, "low": []float64{98, 99}, "volume": []float64{1000, 1200},
			}}},
		}},
		"error": nil,
	}}
	historyPayload := map[string]any{"chart": map[string]any{
		"result": []any{map[string]any{
			"meta": map[string]any{"symbol": "CACHE", "regularMarketTime": asOf.Unix()},
			"timestamp": []int64{
				asOf.AddDate(0, -2, 0).Unix(), asOf.AddDate(0, -1, 0).Unix(), asOf.AddDate(0, 0, -1).Unix(),
			},
			"indicators": map[string]any{
				"quote": []any{map[string]any{
					"close": []float64{90, 95, 100}, "high": []float64{91, 96, 102}, "low": []float64{89, 94, 98}, "volume": []float64{800, 900, 1000},
				}},
				"adjclose": []any{map[string]any{"adjclose": []float64{89, 94, 100}}},
			},
		}},
		"error": nil,
	}}
	return currentPayload, historyPayload
}

func clearYahooHistoryCacheForProvider(provider *YahooMarket) {
	yahooHistoryCache.Lock()
	defer yahooHistoryCache.Unlock()
	for key := range yahooHistoryCache.entries {
		if key.provider == provider {
			delete(yahooHistoryCache.entries, key)
		}
	}
	for key := range yahooHistoryCache.calls {
		if key.provider == provider {
			delete(yahooHistoryCache.calls, key)
		}
	}
}

func TestYahooMonthlyStatisticSnapshotsArePointInTimeAndMonthly(t *testing.T) {
	asOf := time.Date(2026, time.July, 31, 18, 0, 0, 0, time.UTC)
	rows := make([]quoteObservation, 0, 2000)
	close := 100.0
	for date := time.Date(2019, time.January, 1, 16, 0, 0, 0, time.UTC); !date.After(asOf); date = date.AddDate(0, 0, 1) {
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		close += 0.05
		volume := 1_000_000 + float64(len(rows))
		rows = append(rows, quoteObservation{date: date, close: close, adjustedClose: close, high: close + 1, low: close - 1, volume: liveFloat(volume)})
	}
	result := yahooQuoteResult{}
	result.Events.Dividends = make(map[string]yahooDividendEvent)
	for year := 2019; year <= asOf.Year(); year++ {
		for _, month := range []time.Month{time.February, time.May, time.August, time.November} {
			date := time.Date(year, month, 15, 16, 0, 0, 0, time.UTC)
			if date.After(asOf) {
				continue
			}
			key := fmt.Sprintf("%d-%02d", year, month)
			result.Events.Dividends[key] = yahooDividendEvent{Amount: 0.25, Date: date.Unix()}
		}
	}
	result.Events.Splits = map[string]yahooSplitEvent{
		"split": {Date: time.Date(2020, time.August, 31, 16, 0, 0, 0, time.UTC).Unix(), Numerator: 4, Denominator: 1, SplitRatio: "4:1"},
	}

	stockSplits, complete := yahooStockSplitEvents(result, result, asOf)
	if !complete {
		t.Fatal("test split history was not valid")
	}
	snapshots := yahooMonthlyStatisticSnapshots(result, rows, "REGULAR", asOf, stockSplits)
	completed := completedDailyObservations(rows, "REGULAR", asOf)
	expectedByMonth := make(map[string]quoteObservation)
	for _, row := range completed {
		expectedByMonth[row.date.Format("2006-01")] = row
	}
	if len(snapshots) != len(expectedByMonth) {
		t.Fatalf("monthly snapshots = %d, want %d", len(snapshots), len(expectedByMonth))
	}
	seen := make(map[string]bool)
	for _, snapshot := range snapshots {
		observed, err := time.Parse(time.RFC3339Nano, snapshot.AsOf)
		if err != nil {
			t.Fatal(err)
		}
		month := observed.Format("2006-01")
		if seen[month] {
			t.Fatalf("duplicate month %s: %+v", month, snapshots)
		}
		seen[month] = true
		if want := expectedByMonth[month].date.Format(time.RFC3339); snapshot.AsOf != want {
			t.Fatalf("month %s asOf = %s, want last completed session %s", month, snapshot.AsOf, want)
		}
	}
	first := snapshots[0]
	if _, ok := first.Numeric["change-52-week"]; ok {
		t.Fatalf("range-boundary month must not claim a full 52-week calculation: %+v", first)
	}
	if _, ok := first.Numeric["moving-average-200d"]; ok {
		t.Fatalf("range-boundary month must not claim a 200-session average: %+v", first)
	}
	last := snapshots[len(snapshots)-1]
	for _, key := range []string{"price", "previous-close", "change", "change-percent", "change-52-week", "high-52-week", "low-52-week", "moving-average-50d", "moving-average-200d", "average-volume-10d", "average-volume-3m", "trailing-dividend-rate", "trailing-dividend-yield", "forward-dividend-rate", "forward-dividend-yield", "average-dividend-yield-5y"} {
		if _, ok := last.Numeric[key]; !ok {
			t.Fatalf("last monthly snapshot missing %q: %+v", key, last)
		}
	}
	if last.Text["ex-dividend-date"] != "2026-05-15" || last.Text["last-split-factor"] != "4:1" || last.Text["last-split-date"] != "2020-08-31" {
		t.Fatalf("event state was not point-in-time: %+v", last.Text)
	}
	if last.AsOfSource != yahooBackfillAsOfSource || last.Sources["price"] != yahooDailyCloseSource {
		t.Fatalf("backfill provenance missing: %+v", last)
	}
	for _, forbidden := range []string{"market-cap", "enterprise-value", "shares-outstanding"} {
		if _, ok := last.Numeric[forbidden]; ok {
			t.Fatalf("historical Yahoo snapshot must not contain SEC-share field %q: %+v", forbidden, last)
		}
	}
}

func TestYahooLiveQuoteNeverUsesRangeBoundaryAsPreviousClose(t *testing.T) {
	asOf := time.Date(2026, time.July, 31, 15, 30, 0, 0, time.UTC)
	price, boundary, close := 200.0, 25.0, 199.0
	result := yahooQuoteResult{Timestamps: []int64{asOf.Unix()}}
	result.Meta.RegularMarketPrice = &price
	result.Meta.RegularMarketTime = asOf.Unix()
	result.Meta.ChartPreviousClose = &boundary
	result.Meta.MarketState = "REGULAR"
	result.Indicators.Quote = append(result.Indicators.Quote, struct {
		Close  []*float64 `json:"close"`
		High   []*float64 `json:"high"`
		Low    []*float64 `json:"low"`
		Volume []*float64 `json:"volume"`
	}{Close: []*float64{&close}})
	quote, err := buildYahooLiveQuote("AAPL", result, asOf)
	if err != nil {
		t.Fatal(err)
	}
	if quote.PreviousClose != nil || quote.Change != nil || quote.ChangePercent != nil {
		t.Fatalf("10-year range boundary must not become daily previous close: %+v", quote)
	}
}

func TestPipelineLiveMarketCapRequiresExactSECShareBasis(t *testing.T) {
	market := &fakeMarketProvider{quote: model.LiveQuote{
		Price:                      floatPtr(200),
		AsOf:                       "2026-07-31T15:30:00Z",
		MarketCapB:                 floatPtr(9999),
		StockSplitCoverageStart:    "2016-08-01",
		StockSplitCoverageComplete: true,
		StockSplits:                []model.StockSplitEvent{{Date: "2026-07-01", Numerator: 10, Denominator: 1, Ratio: 10}},
	}}
	pipeline := &Pipeline{Market: market}
	equity := &model.Equity{
		Annuals: []model.AnnualPoint{{PeriodEnd: "2026-06-30", DilutedSharesB: floatPtr(15)}},
		Current: model.CurrentMetrics{
			SharesOutstandingB:      floatPtr(1.5),
			SharesOutstandingAsOf:   "2026-06-27",
			SharesOutstandingSource: usGAAPActualSharesSource,
		},
		Valuation: model.ValuationMetrics{NetDebtB: floatPtr(50)},
	}
	quote, err := pipeline.Quote(context.Background(), "aapl", equity)
	if err != nil {
		t.Fatal(err)
	}
	assertLiveFloat(t, "shares outstanding", quote.SharesOutstandingB, 15)
	assertLiveFloat(t, "market cap", quote.MarketCapB, 3000)
	assertLiveFloat(t, "enterprise value", quote.EnterpriseValueB, 3050)
	if quote.ShareBasisAsOf != "2026-06-27" {
		t.Fatalf("unexpected share basis: %+v", quote)
	}
	if strings.Count(quote.Source, exactSharesAggregateSource) != 1 || !strings.Contains(quote.FieldSources["sharesOutstandingB"], "us-gaap:CommonStockSharesOutstanding") || !strings.Contains(quote.FieldSources["sharesOutstandingB"], "split-adjusted x10") {
		t.Fatalf("split provenance is not exact/idempotent: %+v", quote)
	}
	quote = rebaseQuoteToEquity(quote, equity)
	if quote.SharesOutstandingB == nil || *quote.SharesOutstandingB != 15 || strings.Count(quote.Source, exactSharesAggregateSource) != 1 {
		t.Fatalf("rebase was not idempotent: %+v", quote)
	}

	withoutExactShares, err := pipeline.Quote(context.Background(), "AAPL", &model.Equity{Valuation: model.ValuationMetrics{NetDebtB: floatPtr(50)}})
	if err != nil {
		t.Fatal(err)
	}
	if withoutExactShares.MarketCapB != nil || withoutExactShares.EnterpriseValueB != nil || withoutExactShares.SharesOutstandingB != nil {
		t.Fatalf("market value must be absent without exact SEC shares: %+v", withoutExactShares)
	}
}

func TestYahooStockSplitEventsValidateAndMergeCurrentHistory(t *testing.T) {
	asOf := time.Date(2026, time.July, 31, 17, 0, 0, 0, time.UTC)
	history := yahooQuoteResult{}
	history.Events.Splits = map[string]yahooSplitEvent{
		"old": {Date: time.Date(2021, time.July, 20, 0, 0, 0, 0, time.UTC).Unix(), Numerator: 4, Denominator: 1},
	}
	current := yahooQuoteResult{}
	current.Events.Splits = map[string]yahooSplitEvent{
		"new": {Date: time.Date(2024, time.June, 10, 0, 0, 0, 0, time.UTC).Unix(), Numerator: 10, Denominator: 1},
	}
	events, ok := yahooStockSplitEvents(current, history, asOf)
	if !ok || len(events) != 2 || events[0].Date != "2021-07-20" || events[1].Date != "2024-06-10" {
		t.Fatalf("validated split events = %#v, ok=%v", events, ok)
	}
	quote := model.LiveQuote{
		AsOf:                       asOf.Format(time.RFC3339),
		StockSplits:                events,
		StockSplitCoverageStart:    "2020-01-01",
		StockSplitCoverageComplete: true,
	}
	adjusted, factor, ok := splitAdjustedActualShares(0.615, "2021-06-30", quote)
	if !ok || math.Abs(adjusted-24.6) > 1e-9 || factor != 40 {
		t.Fatalf("cumulative split adjustment = %v x%v, ok=%v", adjusted, factor, ok)
	}
	quote.StockSplitCoverageComplete = false
	if _, _, ok := splitAdjustedActualShares(0.615, "2021-06-30", quote); ok {
		t.Fatal("incomplete split coverage was accepted")
	}
	quote.StockSplitCoverageComplete = true
	quote.StockSplitCoverageStart = "2022-01-01"
	if _, _, ok := splitAdjustedActualShares(0.615, "2021-06-30", quote); ok {
		t.Fatal("pre-coverage share basis was accepted")
	}

	current.Events.Splits["conflict"] = yahooSplitEvent{Date: current.Events.Splits["new"].Date, Numerator: 5, Denominator: 1}
	if _, ok := yahooStockSplitEvents(current, history, asOf); ok {
		t.Fatal("conflicting same-day split events were accepted")
	}
}

func TestYahooStockSplitEventsDeduplicateSameRatioNearDates(t *testing.T) {
	asOf := time.Date(2026, time.July, 31, 17, 0, 0, 0, time.UTC)
	history := yahooQuoteResult{}
	history.Events.Splits = map[string]yahooSplitEvent{
		"canonical": {Date: time.Date(2018, time.May, 4, 0, 0, 0, 0, time.UTC).Unix(), Numerator: 50, Denominator: 1},
	}
	current := yahooQuoteResult{}
	current.Events.Splits = map[string]yahooSplitEvent{
		"duplicate":        {Date: time.Date(2018, time.May, 16, 0, 0, 0, 0, time.UTC).Unix(), Numerator: 100, Denominator: 2},
		"different-ratio":  {Date: time.Date(2018, time.May, 17, 0, 0, 0, 0, time.UTC).Unix(), Numerator: 2, Denominator: 1},
		"later-real-event": {Date: time.Date(2018, time.June, 1, 0, 0, 0, 0, time.UTC).Unix(), Numerator: 50, Denominator: 1},
	}
	events, ok := yahooStockSplitEvents(current, history, asOf)
	if !ok {
		t.Fatal("valid split events were rejected")
	}
	if len(events) != 3 || events[0].Date != "2018-05-04" || events[0].Ratio != 50 || events[1].Date != "2018-05-17" || events[1].Ratio != 2 || events[2].Date != "2018-06-01" || events[2].Ratio != 50 {
		t.Fatalf("near-date split canonicalization = %#v", events)
	}
	factor, date := latestValidatedStockSplit(events, time.Date(2018, time.May, 20, 0, 0, 0, 0, time.UTC))
	if factor != "2:1" || date != "2018-05-17" {
		t.Fatalf("latest canonical split = %s on %s", factor, date)
	}
}

func TestYahooBetaBenchmarkCacheDegradationIsObservable(t *testing.T) {
	now := time.Date(2026, time.July, 31, 17, 0, 0, 0, time.UTC)
	target := yahooHistoryMetadata{cacheStatus: "fresh", cachedAt: now}
	benchmark := yahooHistoryMetadata{cacheStatus: "stale", cachedAt: now.Add(-48 * time.Hour), refreshFailureKind: "throttled", refreshFailed: true, refreshFailureID: "spy-refresh-1"}
	quote := model.LiveQuote{}
	applyYahooHistoryMetadata(&quote, target, benchmark)
	if quote.HistoryCacheStatus != "fresh" || quote.HistoryCacheAsOf != now.Format(time.RFC3339) || quote.HistoryRefreshFailed {
		t.Fatalf("benchmark degradation contaminated target history: %+v", quote)
	}
	if quote.BenchmarkHistoryCacheStatus != "stale" || quote.BenchmarkHistoryCacheAsOf != benchmark.cachedAt.Format(time.RFC3339) || quote.BenchmarkHistoryRefreshFailureKind != "throttled" || !quote.BenchmarkHistoryRefreshFailed || quote.BenchmarkHistoryRefreshFailureID != benchmark.refreshFailureID {
		t.Fatalf("benchmark cache degradation was hidden: %+v", quote)
	}
}

func TestAlignedMonthlyBetaUsesFiveYearsOfAdjustedReturns(t *testing.T) {
	target := make(map[string]float64)
	benchmark := make(map[string]float64)
	date := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	targetPrice, benchmarkPrice := 100.0, 100.0
	target[date.Format("2006-01")] = targetPrice
	benchmark[date.Format("2006-01")] = benchmarkPrice
	for index := 1; index <= 60; index++ {
		marketReturn := 0.01
		if index%3 == 0 {
			marketReturn = -0.006
		}
		benchmarkPrice *= 1 + marketReturn
		targetPrice *= 1 + 2*marketReturn
		month := date.AddDate(0, index, 0).Format("2006-01")
		target[month] = targetPrice
		benchmark[month] = benchmarkPrice
	}
	beta := alignedMonthlyBeta(target, benchmark)
	assertLiveFloat(t, "five-year monthly beta", beta, 2)
	delete(benchmark, date.AddDate(0, 42, 0).Format("2006-01"))
	if beta := alignedMonthlyBeta(target, benchmark); beta != nil {
		t.Fatalf("gapped monthly history should not produce beta: %v", *beta)
	}
}

func TestRollingAlignedMonthlyBetasPopulateRecordedHistory(t *testing.T) {
	target := make(map[string]float64)
	benchmark := make(map[string]float64)
	date := time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC)
	targetPrice, benchmarkPrice := 100.0, 100.0
	for index := 0; index <= 72; index++ {
		month := date.AddDate(0, index, 0).Format("2006-01")
		if index > 0 {
			marketReturn := 0.01
			if index%3 == 0 {
				marketReturn = -0.006
			}
			benchmarkPrice *= 1 + marketReturn
			targetPrice *= 1 + 2*marketReturn
		}
		target[month] = targetPrice
		benchmark[month] = benchmarkPrice
	}
	values := rollingAlignedMonthlyBetas(target, benchmark)
	if len(values) != 13 {
		t.Fatalf("rolling beta points = %d, want 13: %#v", len(values), values)
	}
	for month, value := range values {
		if math.Abs(value-2) > 1e-12 {
			t.Fatalf("rolling beta for %s = %v, want 2", month, value)
		}
	}
	history := []model.StatisticSnapshot{
		{AsOf: "2023-12-29T21:00:00Z"},
		{AsOf: "2024-01-31T21:00:00Z", Numeric: map[string]float64{"price": 100}, Sources: map[string]string{"price": yahooDailyCloseSource}},
		{AsOf: "2025-01-31T21:00:00Z"},
	}
	addRollingBetaHistory(history, values)
	if _, ok := history[0].Numeric["beta-5y"]; ok {
		t.Fatalf("pre-window snapshot received beta: %+v", history[0])
	}
	if math.Abs(history[1].Numeric["beta-5y"]-2) > 1e-12 || history[1].Sources["beta-5y"] != betaCalculationSource || history[1].Numeric["price"] != 100 {
		t.Fatalf("beta history was not merged with provenance: %+v", history[1])
	}
	if math.Abs(history[2].Numeric["beta-5y"]-2) > 1e-12 || history[2].Sources["beta-5y"] != betaCalculationSource {
		t.Fatalf("beta history did not initialize maps: %+v", history[2])
	}
}

func TestCompositeMarketPrefersFirstProviderWithDecadeCoverage(t *testing.T) {
	short := &fakeMarketProvider{rows: []model.PricePoint{{Date: "2024-01-01", Close: 1}, {Date: "2026-01-01", Close: 2}}, source: "short"}
	wantBasis := &model.HistoricalPriceBasis{Adjustment: "split-adjusted", SplitCoverageComplete: true}
	long := &fakeMarketProvider{rows: []model.PricePoint{{Date: "2012-01-01", Close: 1}, {Date: "2026-01-01", Close: 2}}, source: "long", basis: wantBasis}
	unused := &fakeMarketProvider{err: fmt.Errorf("should not be called")}
	rows, source, basis, err := NewCompositeMarket(short, long, unused).HistoryWithPriceBasis(context.Background(), "AMZN", time.Time{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if source != "long" || len(rows) != 2 || basis != wantBasis || unused.called {
		t.Fatalf("source=%q rows=%v basis=%+v unused.called=%v", source, rows, basis, unused.called)
	}
}

func TestPipelineBuildsMarketOnlyInstrumentWithoutSEC(t *testing.T) {
	basis := &model.HistoricalPriceBasis{Provider: "yahoo-finance", Adjustment: "split-adjusted", SplitCoverageComplete: true}
	market := &fakeMarketProvider{
		rows:   []model.PricePoint{{Date: "2000-01-31", Close: 10}, {Date: "2026-01-31", Close: 50}},
		source: "fixture",
		basis:  basis,
	}
	result, err := (&Pipeline{Market: market}).Analyze(context.Background(), "005930.KS", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Company != "Samsung Electronics Co., Ltd." || result.InstrumentType != "International equity" {
		t.Fatalf("unexpected profile: %+v", result)
	}
	if result.Status != "ready" || len(result.Prices) != 2 || result.Current.Price == nil || result.HistoricalPriceBasis != basis {
		t.Fatalf("unexpected market result: %+v", result)
	}
}

func TestPipelineRequiresMarketDataForMarketOnlyInstrument(t *testing.T) {
	_, err := (&Pipeline{}).Analyze(context.Background(), "SPY", nil)
	if !errors.Is(err, ErrNoMarketProvider) {
		t.Fatalf("expected ErrNoMarketProvider, got %v", err)
	}
}

type fakeMarketProvider struct {
	rows     []model.PricePoint
	source   string
	err      error
	called   bool
	quote    model.LiveQuote
	quoteErr error
	basis    *model.HistoricalPriceBasis
}

func (f *fakeMarketProvider) History(context.Context, string, time.Time, time.Time) ([]model.PricePoint, string, error) {
	f.called = true
	return f.rows, f.source, f.err
}

func (f *fakeMarketProvider) HistoryWithPriceBasis(context.Context, string, time.Time, time.Time) ([]model.PricePoint, string, *model.HistoricalPriceBasis, error) {
	f.called = true
	return f.rows, f.source, f.basis, f.err
}

func (f *fakeMarketProvider) Quote(context.Context, string) (model.LiveQuote, error) {
	return f.quote, f.quoteErr
}

func assertLiveFloat(t *testing.T, label string, actual *float64, expected float64) {
	t.Helper()
	if actual == nil || math.Abs(*actual-expected) > 1e-9 {
		t.Fatalf("%s = %v, want %v", label, actual, expected)
	}
}
