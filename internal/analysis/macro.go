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

const fredStartDate = "2000-01-01"

var fredSeriesIDs = []string{
	"CPIAUCSL",
	"FEDFUNDS",
	"GS2",
	"GS10",
	"M1SL",
	"M2SL",
	"WALCL",
	"BAMLC0A0CM",
}

type MacroAnalyzer interface {
	Analyze(context.Context) (model.MacroSeries, error)
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
	for _, id := range fredSeriesIDs {
		go func() {
			values, err := f.fetch(fetchCtx, id)
			results <- result{id: id, values: values, err: err}
		}()
	}
	for range fredSeriesIDs {
		fetched := <-results
		if fetched.err != nil {
			cancel()
			return model.MacroSeries{}, fetched.err
		}
		series[fetched.id] = fetched.values
	}

	sources := make([]string, 0, len(fredSeriesIDs))
	for _, id := range fredSeriesIDs {
		sources = append(sources, "FRED:"+id)
	}
	return model.MacroSeries{
		UpdatedAt: time.Now().UTC(),
		Sources:   sources,
		Points:    buildMacroPoints(series),
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
			Date:            month + "-01",
			Inflation:       yearOverYear(series["CPIAUCSL"], month),
			FedFunds:        valueAt(series["FEDFUNDS"], month),
			Treasury2Y:      valueAt(series["GS2"], month),
			Treasury10Y:     valueAt(series["GS10"], month),
			LogM1:           logValue(series["M1SL"], month, 1),
			LogM2:           logValue(series["M2SL"], month, 1),
			LogFedAssets:    logValue(series["WALCL"], month, 1000),
			M1Growth:        yearOverYear(series["M1SL"], month),
			M2Growth:        yearOverYear(series["M2SL"], month),
			CorporateSpread: valueAt(series["BAMLC0A0CM"], month),
		}
		point.RealPolicyRate = difference(point.FedFunds, point.Inflation)
		point.YieldCurve = difference(point.Treasury10Y, point.Treasury2Y)
		points = append(points, point)
	}
	return points
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
