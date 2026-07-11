package analysis

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

const fredStartDate = "1960-01-01"

var fredSeriesIDs = []string{
	"CPIAUCSL",
	"CPILFESL",
	"PCEPILFE",
	"CUSR0000SAH1",
	"CES0500000003",
	"FEDFUNDS",
	"GS3M",
	"GS2",
	"GS5",
	"GS10",
	"GS30",
	"DFII5",
	"DFII10",
	"T5YIE",
	"T5YIFR",
	"THREEFFTP10",
	"M1SL",
	"M2SL",
	"WALCL",
	"WTREGEN",
	"TOTBKCR",
	"BUSLOANS",
	"BAMLC0A0CM",
	"USREC",
	"UNRATE",
	"ICSA",
	"PAYEMS",
	"SAHMREALTIME",
	"GDPC1",
	"INDPRO",
	"T10YIE",
	"MORTGAGE30US",
	"NFCI",
	"DTWEXBGS",
	"VIXCLS",
	"BAMLH0A0HYM2",
	"DRTSCILM",
	"DCOILWTICO",
	"PCOPPUSDM",
	"GFDEGDQ188S",
	"BOGMBASE",
	"TOTRESNS",
	"RRPONTSYD",
}

var requiredFREDSeries = map[string]bool{
	"CPIAUCSL": true,
	"FEDFUNDS": true,
	"GS2":      true,
	"GS10":     true,
	"M1SL":     true,
	"M2SL":     true,
	"WALCL":    true,
}

type MacroAnalyzer interface {
	Analyze(context.Context) (model.MacroSeries, error)
}

type IncrementalMacroAnalyzer interface {
	AnalyzeWithPrevious(context.Context, model.MacroSeries) (model.MacroSeries, error)
}

type FREDClient struct {
	baseURL   string
	http      *http.Client
	userAgent string
}

func NewFREDClient(userAgent string, client *http.Client) *FREDClient {
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" {
		userAgent = "parallel-ocean-equities/1.0 (https://parallel-ocean.xyz/equities)"
	}
	return &FREDClient{
		baseURL:   "https://fred.stlouisfed.org",
		http:      client,
		userAgent: userAgent,
	}
}

func (f *FREDClient) Analyze(ctx context.Context) (model.MacroSeries, error) {
	series := make(map[string]map[string]float64, len(fredSeriesIDs))
	type result struct {
		id     string
		values map[string]float64
		err    error
	}
	fetchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan result, len(fredSeriesIDs))
	slots := make(chan struct{}, 6)
	for _, id := range fredSeriesIDs {
		go func() {
			select {
			case <-fetchCtx.Done():
				results <- result{id: id, err: fetchCtx.Err()}
				return
			case slots <- struct{}{}:
			}
			defer func() { <-slots }()
			values, err := f.fetch(fetchCtx, id)
			results <- result{id: id, values: values, err: err}
		}()
	}
	warnings := make([]string, 0)
	for range fredSeriesIDs {
		fetched := <-results
		if fetched.err != nil {
			if requiredFREDSeries[fetched.id] {
				cancel()
				return model.MacroSeries{}, fetched.err
			}
			warnings = append(warnings, fetched.err.Error())
			continue
		}
		series[fetched.id] = fetched.values
	}
	sort.Strings(warnings)

	sources := make([]string, 0, len(series))
	for _, id := range fredSeriesIDs {
		if len(series[id]) > 0 {
			sources = append(sources, "FRED:"+id)
		}
	}
	points := buildMacroPoints(series)
	countries, countryWarnings := f.analyzeCountries(ctx, points)
	warnings = append(warnings, countryWarnings...)
	sort.Strings(warnings)
	return model.MacroSeries{
		UpdatedAt: time.Now().UTC(),
		Sources:   sources,
		Warnings:  warnings,
		Basis:     "Latest-revised FRED observations; historical values are not vintage snapshots.",
		Points:    points,
		Countries: countries,
	}, nil
}

func (f *FREDClient) fetch(ctx context.Context, id string) (map[string]float64, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		values, err := f.fetchOnce(ctx, id)
		if err == nil {
			return values, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 250 * time.Millisecond):
		}
	}
	return nil, lastErr
}

func (f *FREDClient) fetchOnce(ctx context.Context, id string) (map[string]float64, error) {
	query := url.Values{"id": {id}, "cosd": {fredStartDate}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL+"/graph/fredgraph.csv?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if f.userAgent != "" {
		req.Header.Set("User-Agent", f.userAgent)
	}
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FRED %s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("FRED %s HTTP %d: %s", id, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	values, err := decodeFREDCSV(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("FRED %s: %w", id, err)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("FRED %s returned no observations", id)
	}
	return values, nil
}

func decodeFREDCSV(reader io.Reader) (map[string]float64, error) {
	rows := csv.NewReader(reader)
	header, err := rows.Read()
	if err != nil {
		return nil, err
	}
	if len(header) < 2 {
		return nil, errors.New("invalid CSV header")
	}
	values := make(map[string]float64)
	for {
		row, err := rows.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(row) < 2 || len(row[0]) < 7 || row[1] == "" || row[1] == "." {
			continue
		}
		value, err := strconv.ParseFloat(row[1], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		values[row[0][:7]] = value
	}
	return values, nil
}

func buildMacroPoints(series map[string]map[string]float64) []model.MacroPoint {
	months := make(map[string]struct{})
	for _, values := range series {
		for month := range values {
			months[month] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(months))
	for month := range months {
		ordered = append(ordered, month)
	}
	sort.Strings(ordered)

	points := make([]model.MacroPoint, 0, len(ordered))
	for _, month := range ordered {
		point := model.MacroPoint{
			Date:                month + "-01",
			Inflation:           yearOverYear(series["CPIAUCSL"], month),
			CoreInflation:       yearOverYear(series["CPILFESL"], month),
			CorePCEInflation:    yearOverYear(series["PCEPILFE"], month),
			ShelterInflation:    yearOverYear(series["CUSR0000SAH1"], month),
			WageGrowth:          yearOverYear(series["CES0500000003"], month),
			FedFunds:            valueAt(series["FEDFUNDS"], month),
			Treasury3M:          valueAt(series["GS3M"], month),
			Treasury2Y:          valueAt(series["GS2"], month),
			Treasury5Y:          valueAt(series["GS5"], month),
			Treasury10Y:         valueAt(series["GS10"], month),
			Treasury30Y:         valueAt(series["GS30"], month),
			Real5Y:              valueAt(series["DFII5"], month),
			Real10Y:             valueAt(series["DFII10"], month),
			Breakeven5Y:         valueAt(series["T5YIE"], month),
			Breakeven10Y:        valueAt(series["T10YIE"], month),
			ForwardInflation5Y:  valueAt(series["T5YIFR"], month),
			TermPremium10Y:      valueAt(series["THREEFFTP10"], month),
			Mortgage30Y:         valueAt(series["MORTGAGE30US"], month),
			LogM1:               logValue(series["M1SL"], month, 1),
			LogM2:               logValue(series["M2SL"], month, 1),
			LogFedAssets:        logValue(series["WALCL"], month, 1000),
			LogMonetaryBase:     logValue(series["BOGMBASE"], month, 1000),
			LogBankReserves:     logValue(series["TOTRESNS"], month, 1000),
			M1Growth:            yearOverYear(series["M1SL"], month),
			M2Growth:            yearOverYear(series["M2SL"], month),
			FedAssetsGrowth:     yearOverYear(series["WALCL"], month),
			MonetaryBaseGrowth:  yearOverYear(series["BOGMBASE"], month),
			TgaB:                scaledValue(series["WTREGEN"], month, 1000),
			ReverseRepoB:        valueAt(series["RRPONTSYD"], month),
			NetLiquidityB:       netLiquidityAt(series, month),
			NetLiquidityGrowth:  netLiquidityGrowth(series, month),
			BankCreditGrowth:    yearOverYear(series["TOTBKCR"], month),
			BusinessLoanGrowth:  yearOverYear(series["BUSLOANS"], month),
			RealGDPGrowth:       yearOverYear(series["GDPC1"], month),
			IndustrialGrowth:    yearOverYear(series["INDPRO"], month),
			PayrollGrowth:       yearOverYear(series["PAYEMS"], month),
			InitialClaimsK:      scaledValue(series["ICSA"], month, 1000),
			Unemployment:        valueAt(series["UNRATE"], month),
			SahmRule:            valueAt(series["SAHMREALTIME"], month),
			FinancialConditions: valueAt(series["NFCI"], month),
			LendingStandards:    valueAt(series["DRTSCILM"], month),
			DollarIndex:         valueAt(series["DTWEXBGS"], month),
			VIX:                 valueAt(series["VIXCLS"], month),
			CorporateSpread:     valueAt(series["BAMLC0A0CM"], month),
			HighYieldSpread:     valueAt(series["BAMLH0A0HYM2"], month),
			OilPrice:            valueAt(series["DCOILWTICO"], month),
			CopperPrice:         valueAt(series["PCOPPUSDM"], month),
			FederalDebtToGDP:    valueAt(series["GFDEGDQ188S"], month),
			Recession:           valueAt(series["USREC"], month),
		}
		point.RealPolicyRate = difference(point.FedFunds, point.Inflation)
		if point.Real10Y == nil {
			point.Real10Y = difference(point.Treasury10Y, point.Breakeven10Y)
		}
		point.YieldCurve = difference(point.Treasury10Y, point.Treasury2Y)
		point.YieldCurve3M = difference(point.Treasury10Y, point.Treasury3M)
		points = append(points, point)
	}
	return points
}

func scaledValue(values map[string]float64, month string, divisor float64) *float64 {
	value, ok := values[month]
	if !ok || divisor == 0 {
		return nil
	}
	return floatPtr(value / divisor)
}

func netLiquidityAt(series map[string]map[string]float64, month string) *float64 {
	fedAssets := scaledValue(series["WALCL"], month, 1000)
	tga := scaledValue(series["WTREGEN"], month, 1000)
	reverseRepo := valueAt(series["RRPONTSYD"], month)
	if fedAssets == nil || tga == nil || reverseRepo == nil {
		return nil
	}
	return floatPtr(*fedAssets - *tga - *reverseRepo)
}

func netLiquidityGrowth(series map[string]map[string]float64, month string) *float64 {
	date, err := time.Parse("2006-01", month)
	if err != nil {
		return nil
	}
	current := netLiquidityAt(series, month)
	prior := netLiquidityAt(series, date.AddDate(-1, 0, 0).Format("2006-01"))
	if current == nil || prior == nil || *prior == 0 {
		return nil
	}
	return floatPtr((*current / *prior - 1) * 100)
}

func valueAt(values map[string]float64, month string) *float64 {
	value, ok := values[month]
	if !ok {
		return nil
	}
	return floatPtr(value)
}

func yearOverYear(values map[string]float64, month string) *float64 {
	current, ok := values[month]
	if !ok || current == 0 {
		return nil
	}
	date, err := time.Parse("2006-01", month)
	if err != nil {
		return nil
	}
	prior, ok := values[date.AddDate(-1, 0, 0).Format("2006-01")]
	if !ok || prior == 0 {
		return nil
	}
	return floatPtr((current/prior - 1) * 100)
}

func logValue(values map[string]float64, month string, divisor float64) *float64 {
	value, ok := values[month]
	if !ok || value <= 0 || divisor <= 0 {
		return nil
	}
	return floatPtr(math.Log10(value / divisor))
}

func difference(left, right *float64) *float64 {
	if left == nil || right == nil {
		return nil
	}
	return floatPtr(*left - *right)
}
