package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

var ErrNoMarketProvider = errors.New("no market-data provider configured")

type MarketProvider interface {
	History(context.Context, string, time.Time, time.Time) ([]model.PricePoint, string, error)
}

type Pipeline struct {
	SEC    *SECClient
	Market MarketProvider
}

func (p *Pipeline) Analyze(ctx context.Context, ticker string, existing *model.Equity) (*model.Equity, error) {
	result, err := p.SEC.Analyze(ctx, ticker, existing)
	if err != nil {
		return nil, err
	}
	if p.Market == nil {
		result.Warnings = append(result.Warnings, ErrNoMarketProvider.Error())
		return result, nil
	}
	start := time.Now().UTC().AddDate(-9, 0, 0)
	end := time.Now().UTC()
	prices, source, err := p.Market.History(ctx, ticker, start, end)
	if err != nil {
		result.Warnings = append(result.Warnings, "market data: "+err.Error())
		return result, nil
	}
	enrichMarket(result, prices)
	result.Sources = append(result.Sources, source)
	return result, nil
}

type CompositeMarket struct {
	providers []MarketProvider
}

func NewCompositeMarket(providers ...MarketProvider) *CompositeMarket {
	filtered := make([]MarketProvider, 0, len(providers))
	for _, provider := range providers {
		if !isNilProvider(provider) {
			filtered = append(filtered, provider)
		}
	}
	return &CompositeMarket{providers: filtered}
}

func isNilProvider(provider MarketProvider) bool {
	if provider == nil {
		return true
	}
	value := reflect.ValueOf(provider)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

func (c *CompositeMarket) History(ctx context.Context, ticker string, start, end time.Time) ([]model.PricePoint, string, error) {
	if len(c.providers) == 0 {
		return nil, "", ErrNoMarketProvider
	}
	var failures []string
	for _, provider := range c.providers {
		rows, source, err := provider.History(ctx, ticker, start, end)
		if err == nil && len(rows) > 0 {
			return rows, source, nil
		}
		if err != nil {
			failures = append(failures, err.Error())
		}
	}
	return nil, "", errors.New(strings.Join(failures, "; "))
}

type ThetaMarket struct {
	baseURL string
	http    *http.Client
}

func NewThetaMarket(baseURL string, client *http.Client) *ThetaMarket {
	if strings.TrimSpace(baseURL) == "" {
		return nil
	}
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	return &ThetaMarket{baseURL: strings.TrimRight(baseURL, "/"), http: client}
}

type thetaEOD struct {
	Created   string  `json:"created"`
	LastTrade string  `json:"last_trade"`
	Close     float64 `json:"close"`
}

func (t *ThetaMarket) History(ctx context.Context, ticker string, start, end time.Time) ([]model.PricePoint, string, error) {
	query := url.Values{
		"symbol":     {ticker},
		"start_date": {start.Format("20060102")},
		"end_date":   {end.Format("20060102")},
		"format":     {"json"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL+"/v3/stock/history/eod?"+query.Encode(), nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("ThetaData: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, "", fmt.Errorf("ThetaData HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", err
	}
	rows, err := decodeThetaRows(body)
	if err != nil {
		return nil, "", err
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
	return prices, "ThetaData v3 EOD", nil
}

func decodeThetaRows(body []byte) ([]thetaEOD, error) {
	var rows []thetaEOD
	if err := json.Unmarshal(body, &rows); err == nil {
		return rows, nil
	}
	var envelope struct {
		Response []thetaEOD `json:"response"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode ThetaData response: %w", err)
	}
	return envelope.Response, nil
}

type PolygonMarket struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func NewPolygonMarket(apiKey string, client *http.Client) *PolygonMarket {
	if strings.TrimSpace(apiKey) == "" {
		return nil
	}
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &PolygonMarket{apiKey: apiKey, baseURL: "https://api.polygon.io", http: client}
}

func (p *PolygonMarket) History(ctx context.Context, ticker string, start, end time.Time) ([]model.PricePoint, string, error) {
	endpoint := fmt.Sprintf("%s/v2/aggs/ticker/%s/range/1/day/%s/%s", p.baseURL, url.PathEscape(ticker), start.Format("2006-01-02"), end.Format("2006-01-02"))
	query := url.Values{"adjusted": {"true"}, "sort": {"asc"}, "limit": {"50000"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("Polygon: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, "", fmt.Errorf("Polygon HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Results []struct {
			Timestamp int64   `json:"t"`
			Close     float64 `json:"c"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&payload); err != nil {
		return nil, "", err
	}
	prices := make([]model.PricePoint, 0, len(payload.Results))
	for _, row := range payload.Results {
		if row.Close <= 0 {
			continue
		}
		date := time.UnixMilli(row.Timestamp).UTC().Format("2006-01-02")
		prices = append(prices, model.PricePoint{Date: date, Close: row.Close})
	}
	return prices, "Polygon adjusted daily bars", nil
}

func enrichMarket(equity *model.Equity, prices []model.PricePoint) {
	if len(prices) == 0 {
		return
	}
	sort.Slice(prices, func(i, j int) bool { return prices[i].Date < prices[j].Date })
	for index := range equity.Annuals {
		row := &equity.Annuals[index]
		if row.Estimate || row.DilutedEPS == nil || *row.DilutedEPS <= 0 || row.PeriodEnd == "" {
			continue
		}
		if price, ok := priceOnOrBefore(prices, row.PeriodEnd); ok {
			row.PERatio = floatPtr(price / *row.DilutedEPS)
		}
	}
	latest := prices[len(prices)-1]
	equity.Current.Price = floatPtr(latest.Close)
	equity.Current.PriceAsOf = latest.Date
	if equity.Current.TTMEPS != nil && *equity.Current.TTMEPS > 0 {
		equity.Current.TrailingPE = floatPtr(latest.Close / *equity.Current.TTMEPS)
	}
	cutoff := time.Now().UTC().AddDate(-1, 0, 0).Format("2006-01-02")
	oneYear := make([]model.PricePoint, 0, 260)
	for _, row := range prices {
		if row.Date >= cutoff {
			oneYear = append(oneYear, row)
		}
	}
	if len(oneYear) > 0 {
		low, high := oneYear[0].Close, oneYear[0].Close
		for _, row := range oneYear {
			if row.Close < low {
				low = row.Close
			}
			if row.Close > high {
				high = row.Close
			}
		}
		equity.Current.Low52Week = floatPtr(low)
		equity.Current.High52Week = floatPtr(high)
		if oneYear[0].Close > 0 {
			equity.Current.Return1Y = floatPtr(latest.Close/oneYear[0].Close - 1)
		}
	}
	equity.Prices = downsampleMonthly(prices)
}

func priceOnOrBefore(prices []model.PricePoint, date string) (float64, bool) {
	index := sort.Search(len(prices), func(i int) bool { return prices[i].Date > date })
	if index == 0 {
		return 0, false
	}
	return prices[index-1].Close, true
}

func downsampleMonthly(prices []model.PricePoint) []model.PricePoint {
	byMonth := make(map[string]model.PricePoint)
	for _, row := range prices {
		if len(row.Date) < 7 {
			continue
		}
		byMonth[row.Date[:7]] = row
	}
	months := make([]string, 0, len(byMonth))
	for month := range byMonth {
		months = append(months, month)
	}
	sort.Strings(months)
	out := make([]model.PricePoint, 0, len(months))
	for _, month := range months {
		out = append(out, byMonth[month])
	}
	return out
}
