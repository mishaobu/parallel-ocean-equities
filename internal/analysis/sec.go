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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

const defaultSECBase = "https://data.sec.gov"

var (
	revenueTags = []string{
		"RevenueFromContractWithCustomerExcludingAssessedTax",
		"SalesRevenueNet",
		"Revenues",
	}
	netIncomeTags = []string{"NetIncomeLoss", "ProfitLoss"}
	capexTags     = []string{
		"PaymentsToAcquirePropertyPlantAndEquipment",
		"PaymentsForAdditionsToPropertyPlantAndEquipment",
		"PaymentsToAcquireProductiveAssets",
	}
	epsTags = []string{"EarningsPerShareDiluted"}
)

type SECClient struct {
	baseURL        string
	polygonBaseURL string
	polygonAPIKey  string
	userAgent      string
	http           *http.Client

	loadMu          sync.Mutex
	mu              sync.RWMutex
	tickers         map[string]companyTicker
	tickerMapLoaded bool
	tickerMapErr    error
}

type companyTicker struct {
	CIK    int    `json:"cik_str"`
	Ticker string `json:"ticker"`
	Title  string `json:"title"`
}

type companyFacts struct {
	EntityName string                            `json:"entityName"`
	Facts      map[string]map[string]factConcept `json:"facts"`
}

type factConcept struct {
	Units map[string][]fact `json:"units"`
}

type fact struct {
	Start string  `json:"start"`
	End   string  `json:"end"`
	Val   float64 `json:"val"`
	FY    int     `json:"fy"`
	FP    string  `json:"fp"`
	Form  string  `json:"form"`
	Filed string  `json:"filed"`
	Frame string  `json:"frame"`
}

func NewSECClient(userAgent, polygonAPIKey string, client *http.Client) *SECClient {
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	return &SECClient{
		baseURL:        defaultSECBase,
		polygonBaseURL: "https://api.polygon.io",
		polygonAPIKey:  polygonAPIKey,
		userAgent:      userAgent,
		http:           client,
	}
}

func (c *SECClient) Analyze(ctx context.Context, ticker string, existing *model.Equity) (*model.Equity, error) {
	company, err := c.lookup(ctx, ticker)
	if err != nil {
		return nil, err
	}
	var facts companyFacts
	if err := c.getJSON(ctx, fmt.Sprintf("/api/xbrl/companyfacts/CIK%010d.json", company.CIK), &facts); err != nil {
		return nil, fmt.Errorf("SEC CompanyFacts: %w", err)
	}
	annuals, err := extractAnnuals(facts)
	if err != nil {
		return nil, err
	}
	annuals = mergeEstimates(annuals, existing.Annuals)
	result := &model.Equity{
		Ticker:   strings.ToUpper(ticker),
		Company:  facts.EntityName,
		CIK:      fmt.Sprintf("%010d", company.CIK),
		Status:   "ready",
		Sources:  []string{"SEC CompanyFacts"},
		Annuals:  annuals,
		Current:  existing.Current,
		Prices:   existing.Prices,
		Warnings: nil,
	}
	if result.Company == "" {
		result.Company = company.Title
	}
	if result.Current.TTMEPS == nil {
		for index := len(annuals) - 1; index >= 0; index-- {
			if !annuals[index].Estimate && annuals[index].DilutedEPS != nil {
				result.Current.TTMEPS = annuals[index].DilutedEPS
				break
			}
		}
	}
	return result, nil
}

func (c *SECClient) lookup(ctx context.Context, ticker string) (companyTicker, error) {
	ticker = strings.ToUpper(ticker)
	if company, ok := c.cachedTicker(ticker); ok {
		return company, nil
	}
	c.ensureTickerMap(ctx)
	if company, ok := c.cachedTicker(ticker); ok {
		return company, nil
	}
	company, polygonErr := c.lookupPolygon(ctx, ticker)
	if polygonErr == nil {
		c.mu.Lock()
		c.tickers[ticker] = company
		c.mu.Unlock()
		return company, nil
	}
	c.mu.RLock()
	mapErr := c.tickerMapErr
	c.mu.RUnlock()
	if mapErr != nil {
		return companyTicker{}, fmt.Errorf("SEC ticker map: %v; Polygon ticker details: %w", mapErr, polygonErr)
	}
	return companyTicker{}, fmt.Errorf("ticker %s is not in the SEC company map: %w", ticker, polygonErr)
}

func (c *SECClient) cachedTicker(ticker string) (companyTicker, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	company, ok := c.tickers[ticker]
	return company, ok
}

func (c *SECClient) ensureTickerMap(ctx context.Context) {
	c.loadMu.Lock()
	defer c.loadMu.Unlock()
	c.mu.RLock()
	loaded := c.tickerMapLoaded
	c.mu.RUnlock()
	if loaded {
		return
	}

	var rows map[string]companyTicker
	err := c.getJSONFrom(ctx, "https://www.sec.gov/files/company_tickers.json", &rows)
	index := make(map[string]companyTicker, len(rows))
	if err == nil {
		for _, row := range rows {
			index[strings.ToUpper(row.Ticker)] = row
		}
	}
	c.mu.Lock()
	c.tickers = index
	c.tickerMapLoaded = true
	c.tickerMapErr = err
	c.mu.Unlock()
}

func (c *SECClient) lookupPolygon(ctx context.Context, ticker string) (companyTicker, error) {
	if c.polygonAPIKey == "" {
		return companyTicker{}, errors.New("Polygon API key is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.polygonBaseURL+"/v3/reference/tickers/"+url.PathEscape(ticker), nil)
	if err != nil {
		return companyTicker{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.polygonAPIKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return companyTicker{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return companyTicker{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Results struct {
			Ticker string `json:"ticker"`
			Name   string `json:"name"`
			CIK    string `json:"cik"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&payload); err != nil {
		return companyTicker{}, err
	}
	cik, err := strconv.Atoi(strings.TrimLeft(payload.Results.CIK, "0"))
	if err != nil || cik == 0 {
		return companyTicker{}, errors.New("response has no valid CIK")
	}
	return companyTicker{CIK: cik, Ticker: strings.ToUpper(payload.Results.Ticker), Title: payload.Results.Name}, nil
}

func (c *SECClient) getJSON(ctx context.Context, path string, target any) error {
	return c.getJSONFrom(ctx, strings.TrimRight(c.baseURL, "/")+path, target)
}

func (c *SECClient) getJSONFrom(ctx context.Context, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(target)
}

func extractAnnuals(response companyFacts) ([]model.AnnualPoint, error) {
	gaap := response.Facts["us-gaap"]
	if gaap == nil {
		return nil, errors.New("SEC response has no us-gaap facts")
	}
	revenue := annualFacts(gaap, revenueTags, "USD")
	netIncome := annualFacts(gaap, netIncomeTags, "USD")
	capex := annualFacts(gaap, capexTags, "USD")
	eps := annualFacts(gaap, epsTags, "USD/shares")
	normalizeEPSForSplits(eps, stockSplitEvents(gaap))
	anchors := revenue
	if len(anchors) == 0 {
		anchors = netIncome
	}
	if len(anchors) == 0 {
		return nil, errors.New("no annual SEC revenue or net-income facts found")
	}

	rows := make([]model.AnnualPoint, 0, len(anchors))
	for period, anchor := range anchors {
		end, err := time.Parse("2006-01-02", anchor.End)
		if err != nil {
			continue
		}
		row := model.AnnualPoint{
			FiscalYear: end.Year(),
			PeriodEnd:  anchor.End,
			Confidence: "high",
		}
		if value, ok := revenue[period]; ok {
			row.RevenueB = floatPtr(value.Val / 1e9)
		}
		if value, ok := netIncome[period]; ok {
			row.NetIncomeB = floatPtr(value.Val / 1e9)
		}
		if value, ok := capex[period]; ok {
			row.CapexB = floatPtr(value.Val / 1e9)
		}
		if value, ok := eps[period]; ok {
			row.DilutedEPS = floatPtr(value.Val)
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].PeriodEnd < rows[j].PeriodEnd })
	if len(rows) > 8 {
		rows = rows[len(rows)-8:]
	}
	return rows, nil
}

func annualFacts(gaap map[string]factConcept, tags []string, unit string) map[string]fact {
	out := make(map[string]fact)
	for _, tag := range tags {
		concept, ok := gaap[tag]
		if !ok {
			continue
		}
		for _, candidate := range concept.Units[unit] {
			if candidate.Form != "10-K" || candidate.FP != "FY" {
				continue
			}
			start, startErr := time.Parse("2006-01-02", candidate.Start)
			end, endErr := time.Parse("2006-01-02", candidate.End)
			if startErr != nil || endErr != nil {
				continue
			}
			days := int(end.Sub(start).Hours() / 24)
			if days < 300 || days > 390 {
				continue
			}
			period := candidate.Start + "/" + candidate.End
			current, exists := out[period]
			if !exists || candidate.Filed > current.Filed {
				out[period] = candidate
			}
		}
	}
	return out
}

func mergeEstimates(actuals, previous []model.AnnualPoint) []model.AnnualPoint {
	years := make(map[int]struct{}, len(actuals))
	for _, row := range actuals {
		years[row.FiscalYear] = struct{}{}
	}
	for _, row := range previous {
		if !row.Estimate {
			continue
		}
		if _, exists := years[row.FiscalYear]; exists {
			continue
		}
		actuals = append(actuals, row)
	}
	sort.Slice(actuals, func(i, j int) bool { return actuals[i].FiscalYear < actuals[j].FiscalYear })
	return actuals
}

func floatPtr(value float64) *float64 { return &value }

type stockSplit struct {
	date  string
	ratio float64
}

func stockSplitEvents(gaap map[string]factConcept) []stockSplit {
	concept, ok := gaap["StockholdersEquityNoteStockSplitConversionRatio1"]
	if !ok {
		return nil
	}
	unique := make(map[string]stockSplit)
	for _, candidate := range concept.Units["pure"] {
		if candidate.Val <= 1 || candidate.End == "" {
			continue
		}
		key := fmt.Sprintf("%s/%g", candidate.End, candidate.Val)
		unique[key] = stockSplit{date: candidate.End, ratio: candidate.Val}
	}
	events := make([]stockSplit, 0, len(unique))
	for _, event := range unique {
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].date < events[j].date })

	merged := make([]stockSplit, 0, len(events))
	for _, event := range events {
		if len(merged) > 0 && merged[len(merged)-1].ratio == event.ratio && nearbyDates(merged[len(merged)-1].date, event.date, 120*24*time.Hour) {
			merged[len(merged)-1] = event
			continue
		}
		merged = append(merged, event)
	}
	return merged
}

func normalizeEPSForSplits(eps map[string]fact, splits []stockSplit) {
	for period, value := range eps {
		for _, split := range splits {
			if value.Filed != "" && value.Filed < split.date {
				value.Val /= split.ratio
			}
		}
		eps[period] = value
	}
}

func nearbyDates(left, right string, maximum time.Duration) bool {
	leftTime, leftErr := time.Parse("2006-01-02", left)
	rightTime, rightErr := time.Parse("2006-01-02", right)
	return leftErr == nil && rightErr == nil && rightTime.Sub(leftTime) <= maximum
}
