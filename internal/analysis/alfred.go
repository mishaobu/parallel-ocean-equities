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
	"sync"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

const alfredStartYear = 1994

type ALFREDClient struct {
	baseURL   string
	http      *http.Client
	userAgent string
	now       func() time.Time
}

func NewALFREDClient(userAgent string, client *http.Client) *ALFREDClient {
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	return &ALFREDClient{
		baseURL:   "https://alfred.stlouisfed.org",
		http:      client,
		userAgent: strings.TrimSpace(userAgent),
		now:       time.Now,
	}
}

func (a *ALFREDClient) Analyze(ctx context.Context, previous model.VintageSeries) (model.VintageSeries, error) {
	targets := vintageQuarterDates(a.now().UTC())
	byDate := make(map[string]model.VintagePoint, len(previous.Points))
	for _, point := range previous.Points {
		if point.Date != "" && point.VintageDate != "" && (point.Inflation != nil || point.IndustrialGrowth != nil) {
			byDate[point.Date] = point
		}
	}

	type result struct {
		point model.VintagePoint
		err   error
	}
	results := make(chan result, len(targets))
	slots := make(chan struct{}, 4)
	var workers sync.WaitGroup
	for _, target := range targets {
		key := target.Format("2006-01-02")
		if _, ok := byDate[key]; ok {
			continue
		}
		workers.Add(1)
		go func(target time.Time) {
			defer workers.Done()
			select {
			case <-ctx.Done():
				results <- result{err: ctx.Err()}
				return
			case slots <- struct{}{}:
			}
			defer func() { <-slots }()
			point, err := a.fetchPoint(ctx, target)
			results <- result{point: point, err: err}
		}(target)
	}
	go func() {
		workers.Wait()
		close(results)
	}()

	warnings := make([]string, 0)
	for fetched := range results {
		if fetched.err != nil {
			warnings = append(warnings, fetched.err.Error())
			continue
		}
		byDate[fetched.point.Date] = fetched.point
	}
	points := make([]model.VintagePoint, 0, len(byDate))
	for _, target := range targets {
		if point, ok := byDate[target.Format("2006-01-02")]; ok {
			points = append(points, point)
		}
	}
	if len(points) == 0 {
		if len(warnings) > 0 {
			return model.VintageSeries{}, errors.New(warnings[0])
		}
		return model.VintageSeries{}, errors.New("ALFRED returned no point-in-time observations")
	}
	sort.Strings(warnings)
	return model.VintageSeries{
		UpdatedAt: a.now().UTC(),
		Source:    "ALFRED:CPIAUCSL,INDPRO quarterly vintages",
		Warnings:  warnings,
		Points:    points,
	}, nil
}

func vintageQuarterDates(now time.Time) []time.Time {
	latest := time.Date(now.Year(), time.Month((int(now.Month())-1)/3*3+1), 1, 0, 0, 0, 0, time.UTC)
	start := time.Date(alfredStartYear, time.January, 1, 0, 0, 0, 0, time.UTC)
	dates := make([]time.Time, 0, (latest.Year()-alfredStartYear+1)*4)
	for date := start; !date.After(latest); date = date.AddDate(0, 3, 0) {
		dates = append(dates, date)
	}
	return dates
}

func (a *ALFREDClient) fetchPoint(ctx context.Context, target time.Time) (model.VintagePoint, error) {
	vintage := target.AddDate(0, 0, -1)
	cpi, err := a.fetch(ctx, "CPIAUCSL", vintage)
	if err != nil {
		return model.VintagePoint{}, err
	}
	industrial, err := a.fetch(ctx, "INDPRO", vintage)
	if err != nil {
		return model.VintagePoint{}, err
	}
	inflation, inflationDate := latestVintageGrowth(cpi)
	growth, growthDate := latestVintageGrowth(industrial)
	if inflation == nil || growth == nil {
		return model.VintagePoint{}, fmt.Errorf("ALFRED vintage %s lacks sufficient CPI or industrial history", vintage.Format("2006-01-02"))
	}
	return model.VintagePoint{
		Date: target.Format("2006-01-02"), VintageDate: vintage.Format("2006-01-02"),
		Inflation: inflation, InflationObservationDate: inflationDate,
		IndustrialGrowth: growth, IndustrialObservationDate: growthDate,
	}, nil
}

func (a *ALFREDClient) fetch(ctx context.Context, id string, vintage time.Time) (map[string]float64, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		values, err := a.fetchOnce(ctx, id, vintage)
		if err == nil {
			return values, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 200 * time.Millisecond):
		}
	}
	return nil, lastErr
}

func (a *ALFREDClient) fetchOnce(ctx context.Context, id string, vintage time.Time) (map[string]float64, error) {
	query := url.Values{
		"id":           {id},
		"cosd":         {vintage.AddDate(-1, -6, 0).Format("2006-01-02")},
		"coed":         {vintage.Format("2006-01-02")},
		"vintage_date": {vintage.Format("2006-01-02")},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/graph/alfredgraph.csv?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if a.userAgent != "" {
		req.Header.Set("User-Agent", a.userAgent)
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ALFRED %s %s: %w", id, vintage.Format("2006-01-02"), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("ALFRED %s %s HTTP %d: %s", id, vintage.Format("2006-01-02"), resp.StatusCode, strings.TrimSpace(string(body)))
	}
	values, err := decodeVintageCSV(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("ALFRED %s %s: %w", id, vintage.Format("2006-01-02"), err)
	}
	return values, nil
}

func decodeVintageCSV(reader io.Reader) (map[string]float64, error) {
	rows := csv.NewReader(reader)
	if _, err := rows.Read(); err != nil {
		return nil, err
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
		if len(row) < 2 || len(row[0]) < 7 || row[1] == "." || row[1] == "" {
			continue
		}
		value, err := strconv.ParseFloat(row[1], 64)
		if err == nil && !math.IsNaN(value) && !math.IsInf(value, 0) {
			values[row[0][:7]] = value
		}
	}
	return values, nil
}

func latestVintageGrowth(values map[string]float64) (*float64, string) {
	months := make([]string, 0, len(values))
	for month := range values {
		months = append(months, month)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(months)))
	for _, month := range months {
		date, err := time.Parse("2006-01", month)
		if err != nil {
			continue
		}
		previous, ok := values[date.AddDate(-1, 0, 0).Format("2006-01")]
		if ok && previous != 0 {
			value := (values[month]/previous - 1) * 100
			return &value, month + "-01"
		}
	}
	return nil, ""
}
