package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

var defaultOptionTickers = []string{"SPY", "QQQ", "AMD", "NVDA", "MU", "SMCI", "DELL", "BABA"}

type ThetaOptionsClient struct {
	baseURL string
	http    *http.Client
	tickers []string
	now     func() time.Time
}

type thetaExpiration struct {
	Symbol     string `json:"symbol"`
	Expiration string `json:"expiration"`
}

type thetaIVRow struct {
	Symbol            string  `json:"symbol"`
	Expiration        string  `json:"expiration"`
	Strike            float64 `json:"strike"`
	Right             string  `json:"right"`
	Timestamp         string  `json:"timestamp"`
	Bid               float64 `json:"bid"`
	BidIV             float64 `json:"bid_implied_vol"`
	Midpoint          float64 `json:"midpoint"`
	ImpliedVolatility float64 `json:"implied_vol"`
	Ask               float64 `json:"ask"`
	AskIV             float64 `json:"ask_implied_vol"`
	IVError           float64 `json:"iv_error"`
	UnderlyingPrice   float64 `json:"underlying_price"`
	UnderlyingTime    string  `json:"underlying_timestamp"`
}

type thetaIVContract struct {
	Contract struct {
		Symbol     string  `json:"symbol"`
		Expiration string  `json:"expiration"`
		Strike     float64 `json:"strike"`
		Right      string  `json:"right"`
	} `json:"contract"`
	Data []thetaIVRow `json:"data"`
}

func NewThetaOptionsClient(baseURL string, tickers []string, client *http.Client) *ThetaOptionsClient {
	if strings.TrimSpace(baseURL) == "" {
		return nil
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	cleaned := make([]string, 0, len(tickers))
	seen := make(map[string]bool)
	for _, ticker := range tickers {
		ticker = strings.ToUpper(strings.TrimSpace(ticker))
		if ticker != "" && !seen[ticker] {
			seen[ticker] = true
			cleaned = append(cleaned, ticker)
		}
	}
	if len(cleaned) == 0 {
		cleaned = append(cleaned, defaultOptionTickers...)
	}
	return &ThetaOptionsClient{baseURL: strings.TrimRight(baseURL, "/"), http: client, tickers: cleaned, now: time.Now}
}

func (t *ThetaOptionsClient) Analyze(ctx context.Context, previous model.OptionsSeries) (model.OptionsSeries, error) {
	now := t.now().UTC()
	if isConservativeUSMarketWindow(now) {
		return model.OptionsSeries{}, errors.New("ThetaData options refresh skipped during US market hours")
	}
	asOf := previousWeekday(now.AddDate(0, 0, -1))
	seriesAsOf := asOf
	previousByTicker := make(map[string]model.OptionSnapshot, len(previous.Snapshots))
	for _, snapshot := range previous.Snapshots {
		if snapshot.AsOf == "" {
			snapshot.AsOf = previous.AsOf
		}
		previousByTicker[snapshot.Ticker] = snapshot
	}

	snapshots := make([]model.OptionSnapshot, 0, len(t.tickers))
	warnings := make([]string, 0)
	fresh := 0
	for _, ticker := range t.tickers {
		if err := ctx.Err(); err != nil {
			return model.OptionsSeries{}, err
		}
		snapshot, observationDate, err := t.fetchSnapshot(ctx, ticker, asOf)
		if err != nil {
			warnings = append(warnings, err.Error())
			if prior, ok := previousByTicker[ticker]; ok {
				snapshots = append(snapshots, prior)
				if priorDate, parseErr := time.Parse("2006-01-02", prior.AsOf); parseErr == nil && priorDate.Before(seriesAsOf) {
					seriesAsOf = priorDate
				}
			}
			continue
		}
		if observationDate.Before(seriesAsOf) {
			seriesAsOf = observationDate
		}
		snapshots = append(snapshots, snapshot)
		fresh++
	}
	if fresh == 0 {
		return model.OptionsSeries{}, errors.New("ThetaData options refresh produced no fresh snapshots")
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].Ticker < snapshots[j].Ticker })
	sort.Strings(warnings)
	return model.OptionsSeries{
		UpdatedAt: now,
		AsOf:      seriesAsOf.Format("2006-01-02"),
		Source:    "ThetaData v3 option IV history and stock EOD",
		Warnings:  warnings,
		Snapshots: snapshots,
	}, nil
}

func isConservativeUSMarketWindow(now time.Time) bool {
	weekday := now.Weekday()
	return weekday >= time.Monday && weekday <= time.Friday && now.Hour() >= 13 && now.Hour() < 22
}

func previousWeekday(date time.Time) time.Time {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	for date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		date = date.AddDate(0, 0, -1)
	}
	return date
}

func (t *ThetaOptionsClient) fetchSnapshot(ctx context.Context, ticker string, preferredDate time.Time) (model.OptionSnapshot, time.Time, error) {
	expirations, err := t.fetchExpirations(ctx, ticker)
	if err != nil {
		return model.OptionSnapshot{}, preferredDate, err
	}
	selected := selectOptionExpirations(expirations, preferredDate, []int{30, 60, 90})
	if len(selected) == 0 {
		return model.OptionSnapshot{}, preferredDate, fmt.Errorf("ThetaData %s: no expirations between 7 and 150 days", ticker)
	}

	var terms []model.OptionTermPoint
	observationDate := preferredDate
	candidate := preferredDate
	for attempt := 0; attempt < 3 && len(terms) == 0; attempt++ {
		terms = terms[:0]
		for _, expiration := range selected {
			rows, fetchErr := t.fetchIV(ctx, ticker, expiration, candidate)
			if fetchErr != nil {
				continue
			}
			if term := summarizeOptionTerm(rows, candidate, expiration); term != nil {
				terms = append(terms, *term)
			}
		}
		if len(terms) > 0 {
			observationDate = candidate
		}
		candidate = previousWeekday(candidate.AddDate(0, 0, -1))
	}
	if len(terms) == 0 {
		return model.OptionSnapshot{}, preferredDate, fmt.Errorf("ThetaData %s: no bounded IV history near %s", ticker, preferredDate.Format("2006-01-02"))
	}
	sort.Slice(terms, func(i, j int) bool { return terms[i].DaysToExpiration < terms[j].DaysToExpiration })
	prices, err := t.fetchStockEOD(ctx, ticker, observationDate.AddDate(0, -2, 0), observationDate)
	var realized *float64
	if err == nil {
		realized = realizedVolatility(prices, 20)
	}
	nearest := nearestOptionTerm(terms, 30)
	snapshot := model.OptionSnapshot{Ticker: ticker, AsOf: observationDate.Format("2006-01-02"), Terms: terms, RealizedVolatility20D: realized}
	if nearest != nil {
		snapshot.Spot = nearest.Spot
		snapshot.ATMIV30D = nearest.ATMIV
		snapshot.Skew30D = nearest.Skew
		snapshot.ExpectedMove30D = nearest.ExpectedMove
		if nearest.ATMIV != nil && realized != nil {
			spread := *nearest.ATMIV*100 - *realized
			snapshot.ImpliedRealizedSpread = &spread
		}
	}
	return snapshot, observationDate, nil
}

func (t *ThetaOptionsClient) fetchExpirations(ctx context.Context, ticker string) ([]time.Time, error) {
	body, err := t.get(ctx, "/v3/option/list/expirations", url.Values{"symbol": {ticker}, "format": {"json"}})
	if err != nil {
		return nil, fmt.Errorf("ThetaData %s expirations: %w", ticker, err)
	}
	rows := []thetaExpiration{}
	if err := decodeThetaEnvelope(body, &rows); err != nil {
		return nil, fmt.Errorf("ThetaData %s expirations: %w", ticker, err)
	}
	expirations := make([]time.Time, 0, len(rows))
	for _, row := range rows {
		if date, err := time.Parse("2006-01-02", row.Expiration); err == nil {
			expirations = append(expirations, date)
		}
	}
	return expirations, nil
}

func selectOptionExpirations(expirations []time.Time, asOf time.Time, targets []int) []time.Time {
	eligible := make([]time.Time, 0, len(expirations))
	for _, expiration := range expirations {
		days := int(expiration.Sub(asOf).Hours() / 24)
		if days >= 7 && days <= 150 {
			eligible = append(eligible, expiration)
		}
	}
	selected := make([]time.Time, 0, len(targets))
	seen := make(map[string]bool)
	for _, target := range targets {
		sort.SliceStable(eligible, func(i, j int) bool {
			return absInt(int(eligible[i].Sub(asOf).Hours()/24)-target) < absInt(int(eligible[j].Sub(asOf).Hours()/24)-target)
		})
		for _, expiration := range eligible {
			key := expiration.Format("2006-01-02")
			if !seen[key] {
				seen[key] = true
				selected = append(selected, expiration)
				break
			}
		}
	}
	return selected
}

func (t *ThetaOptionsClient) fetchIV(ctx context.Context, ticker string, expiration, date time.Time) ([]thetaIVRow, error) {
	query := url.Values{
		"symbol": {ticker}, "expiration": {expiration.Format("20060102")}, "date": {date.Format("20060102")},
		"interval": {"1h"}, "strike_range": {"4"}, "right": {"both"}, "format": {"json"},
	}
	body, err := t.get(ctx, "/v3/option/history/greeks/implied_volatility", query)
	if err != nil {
		return nil, err
	}
	rows, err := decodeThetaIVRows(body)
	if err != nil {
		return nil, err
	}
	return latestOptionRows(rows), nil
}

func decodeThetaIVRows(body []byte) ([]thetaIVRow, error) {
	flat := []thetaIVRow{}
	if err := decodeThetaEnvelope(body, &flat); err == nil {
		for _, row := range flat {
			if row.Strike > 0 {
				return flat, nil
			}
		}
	}
	contracts := []thetaIVContract{}
	if err := decodeThetaEnvelope(body, &contracts); err != nil {
		return nil, err
	}
	rows := make([]thetaIVRow, 0)
	for _, contract := range contracts {
		for _, row := range contract.Data {
			row.Symbol = contract.Contract.Symbol
			row.Expiration = contract.Contract.Expiration
			row.Strike = contract.Contract.Strike
			row.Right = contract.Contract.Right
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return nil, errors.New("ThetaData IV response has no contract observations")
	}
	return rows, nil
}

func latestOptionRows(rows []thetaIVRow) []thetaIVRow {
	latest := make(map[string]thetaIVRow)
	for _, row := range rows {
		key := fmt.Sprintf("%.4f|%s", row.Strike, strings.ToLower(row.Right))
		if previous, ok := latest[key]; !ok || row.Timestamp > previous.Timestamp {
			latest[key] = row
		}
	}
	result := make([]thetaIVRow, 0, len(latest))
	for _, row := range latest {
		result = append(result, row)
	}
	return result
}

func summarizeOptionTerm(rows []thetaIVRow, asOf, expiration time.Time) *model.OptionTermPoint {
	spots := make([]float64, 0, len(rows))
	for _, row := range rows {
		if row.UnderlyingPrice > 0 {
			spots = append(spots, row.UnderlyingPrice)
		}
	}
	if len(spots) == 0 {
		return nil
	}
	spot := medianFloat(spots)
	callATM := nearestIVRow(rows, spot, "call")
	putATM := nearestIVRow(rows, spot, "put")
	putWing := nearestIVRow(rows, spot*0.95, "put")
	callWing := nearestIVRow(rows, spot*1.05, "call")
	atmIV := averageIV(callATM, putATM)
	if atmIV == nil {
		return nil
	}
	days := int(expiration.Sub(asOf).Hours() / 24)
	expected := *atmIV * math.Sqrt(float64(days)/365) * 100
	term := model.OptionTermPoint{
		Expiration: expiration.Format("2006-01-02"), DaysToExpiration: days, Spot: floatPtr(spot), ATMIV: atmIV,
		PutWingIV: rowIV(putWing), CallWingIV: rowIV(callWing), ExpectedMove: &expected,
	}
	if term.PutWingIV != nil && term.CallWingIV != nil {
		skew := (*term.PutWingIV - *term.CallWingIV) * 100
		term.Skew = &skew
	}
	if callATM != nil && putATM != nil && spot > 0 && callATM.Midpoint > 0 && putATM.Midpoint > 0 {
		move := (callATM.Midpoint + putATM.Midpoint) / spot * 100
		term.StraddleMove = &move
	}
	return &term
}

func nearestIVRow(rows []thetaIVRow, strike float64, right string) *thetaIVRow {
	var best *thetaIVRow
	for index := range rows {
		row := &rows[index]
		if strings.ToLower(row.Right) != right || normalizedIV(*row) <= 0 || row.IVError > 0.25 {
			continue
		}
		if best == nil || math.Abs(row.Strike-strike) < math.Abs(best.Strike-strike) {
			best = row
		}
	}
	return best
}

func normalizedIV(row thetaIVRow) float64 {
	value := row.ImpliedVolatility
	if row.BidIV > 0 && row.AskIV > 0 {
		value = (row.BidIV + row.AskIV) / 2
	} else if value <= 0 && row.AskIV > 0 {
		value = row.AskIV
	} else if value <= 0 && row.BidIV > 0 {
		value = row.BidIV
	}
	if value > 3 {
		value /= 100
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func rowIV(row *thetaIVRow) *float64 {
	if row == nil {
		return nil
	}
	value := normalizedIV(*row)
	if value <= 0 {
		return nil
	}
	return &value
}

func averageIV(rows ...*thetaIVRow) *float64 {
	values := make([]float64, 0, len(rows))
	for _, row := range rows {
		if value := rowIV(row); value != nil {
			values = append(values, *value)
		}
	}
	if len(values) == 0 {
		return nil
	}
	value := 0.0
	for _, item := range values {
		value += item
	}
	value /= float64(len(values))
	return &value
}

func nearestOptionTerm(terms []model.OptionTermPoint, target int) *model.OptionTermPoint {
	if len(terms) == 0 {
		return nil
	}
	best := &terms[0]
	for index := 1; index < len(terms); index++ {
		if absInt(terms[index].DaysToExpiration-target) < absInt(best.DaysToExpiration-target) {
			best = &terms[index]
		}
	}
	return best
}

func (t *ThetaOptionsClient) fetchStockEOD(ctx context.Context, ticker string, start, end time.Time) ([]model.PricePoint, error) {
	query := url.Values{"symbol": {ticker}, "start_date": {start.Format("20060102")}, "end_date": {end.Format("20060102")}, "format": {"json"}}
	body, err := t.get(ctx, "/v3/stock/history/eod", query)
	if err != nil {
		return nil, err
	}
	rows, err := decodeThetaRows(body)
	if err != nil {
		return nil, err
	}
	prices := make([]model.PricePoint, 0, len(rows))
	for _, row := range rows {
		date := row.Created
		if len(date) < 10 {
			date = row.LastTrade
		}
		if len(date) >= 10 && row.Close > 0 {
			prices = append(prices, model.PricePoint{Date: date[:10], Close: row.Close})
		}
	}
	return prices, nil
}

func realizedVolatility(prices []model.PricePoint, window int) *float64 {
	sort.Slice(prices, func(i, j int) bool { return prices[i].Date < prices[j].Date })
	if len(prices) < window+1 {
		return nil
	}
	prices = prices[len(prices)-window-1:]
	returns := make([]float64, 0, window)
	for index := 1; index < len(prices); index++ {
		if prices[index-1].Close > 0 && prices[index].Close > 0 {
			returns = append(returns, math.Log(prices[index].Close/prices[index-1].Close))
		}
	}
	if len(returns) < 2 {
		return nil
	}
	mean := 0.0
	for _, value := range returns {
		mean += value
	}
	mean /= float64(len(returns))
	variance := 0.0
	for _, value := range returns {
		variance += math.Pow(value-mean, 2)
	}
	volatility := math.Sqrt(variance/float64(len(returns)-1)) * math.Sqrt(252) * 100
	return &volatility
}

func (t *ThetaOptionsClient) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
}

func decodeThetaEnvelope(body []byte, target any) error {
	if err := json.Unmarshal(body, target); err == nil {
		return nil
	}
	var envelope struct {
		Response json.RawMessage `json:"response"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&envelope); err != nil {
		return err
	}
	if len(envelope.Response) == 0 {
		return errors.New("ThetaData response has no data")
	}
	return json.Unmarshal(envelope.Response, target)
}

func medianFloat(values []float64) float64 {
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 0 {
		return (values[middle-1] + values[middle]) / 2
	}
	return values[middle]
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
