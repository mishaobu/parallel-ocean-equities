package analysis

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

type OfficialCountryClient struct {
	eurostatURL string
	ecbURL      string
	onsURL      string
	http        *http.Client
	now         func() time.Time
}

func NewOfficialCountryClient(client *http.Client) *OfficialCountryClient {
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	return &OfficialCountryClient{
		eurostatURL: "https://ec.europa.eu/eurostat/api/dissemination/statistics/1.0/data",
		ecbURL:      "https://data-api.ecb.europa.eu/service/data",
		onsURL:      "https://www.ons.gov.uk/generator",
		http:        client,
		now:         time.Now,
	}
}

func (c *OfficialCountryClient) Enrich(ctx context.Context, countries []model.CountrySeries) ([]model.CountrySeries, []string) {
	result := append([]model.CountrySeries(nil), countries...)
	warnings := make([]string, 0)
	for index := range result {
		if result[index].Code != "EA" {
			continue
		}
		overlays := make(map[string]map[string]float64)
		queries := []struct {
			metric, dataset string
			params          url.Values
		}{
			{"inflation", "prc_hicp_manr", url.Values{"geo": {"EA"}, "coicop": {"CP00"}, "unit": {"RCH_A"}}},
			{"industrial", "sts_inpr_m", url.Values{"geo": {"EA21"}, "s_adj": {"SCA"}, "unit": {"I21"}, "nace_r2": {"B-D"}}},
			{"unemployment", "une_rt_m", url.Values{"geo": {"EA21"}, "sex": {"T"}, "age": {"TOTAL"}, "s_adj": {"SA"}, "unit": {"PC_ACT"}}},
		}
		for _, query := range queries {
			query.params.Set("lang", "en")
			query.params.Set("sinceTimePeriod", "2023-01")
			values, err := c.fetchEurostat(ctx, query.dataset, query.params)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("official EA %s: %v", query.metric, err))
				continue
			}
			overlays[query.metric] = values
		}
		money, err := c.fetchECBM3(ctx)
		if err != nil {
			warnings = append(warnings, "official EA money: "+err.Error())
		} else {
			overlays["money"] = growthSeries(money)
		}
		if industrial := overlays["industrial"]; len(industrial) > 0 {
			overlays["industrial"] = growthSeries(industrial)
		}
		result[index] = overlayEuroArea(result[index], overlays)
	}
	for index := range result {
		if result[index].Code != "GB" {
			continue
		}
		overlays := make(map[string]map[string]float64)
		queries := []struct {
			metric, path string
		}{
			{"inflation", "/economy/inflationandpriceindices/timeseries/d7g7/mm23"},
			{"unemployment", "/employmentandlabourmarket/peoplenotinwork/unemployment/timeseries/mgsx/lms"},
			{"industrial", "/economy/grossdomesticproductgdp/timeseries/l2kq/qna"},
		}
		for _, query := range queries {
			values, err := c.fetchONS(ctx, query.path)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("official GB %s: %v", query.metric, err))
				continue
			}
			if query.metric == "industrial" {
				values = growthSeries(values)
			}
			overlays[query.metric] = values
		}
		result[index] = overlayUnitedKingdom(result[index], overlays)
	}
	for index := range result {
		result[index].Warnings = append(result[index].Warnings, staleCountryWarnings(result[index], c.now().UTC())...)
		sort.Strings(result[index].Warnings)
	}
	sort.Strings(warnings)
	return result, warnings
}

func (c *OfficialCountryClient) fetchONS(ctx context.Context, seriesPath string) (map[string]float64, error) {
	query := url.Values{"format": {"csv"}, "uri": {seriesPath}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.onsURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ONS HTTP %d", resp.StatusCode)
	}
	rows := csv.NewReader(io.LimitReader(resp.Body, 8<<20))
	values := make(map[string]float64)
	for {
		row, err := rows.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(row) < 2 {
			continue
		}
		month := onsPeriodMonth(row[0])
		value, valueErr := strconv.ParseFloat(strings.TrimSpace(row[1]), 64)
		if month != "" && valueErr == nil {
			values[month] = value
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("ONS returned no monthly or quarterly observations")
	}
	return values, nil
}

func onsPeriodMonth(period string) string {
	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(period)))
	if len(parts) != 2 || len(parts[0]) != 4 {
		return ""
	}
	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return ""
	}
	months := map[string]time.Month{"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6, "JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12}
	month, ok := months[parts[1]]
	if !ok && len(parts[1]) == 2 && parts[1][0] == 'Q' {
		quarter, quarterErr := strconv.Atoi(parts[1][1:])
		if quarterErr == nil && quarter >= 1 && quarter <= 4 {
			month = time.Month(quarter * 3)
			ok = true
		}
	}
	if !ok {
		return ""
	}
	return fmt.Sprintf("%04d-%02d", year, month)
}

type eurostatResponse struct {
	Value     map[string]float64 `json:"value"`
	Dimension struct {
		Time struct {
			Category struct {
				Index map[string]int `json:"index"`
			} `json:"category"`
		} `json:"time"`
	} `json:"dimension"`
}

func (c *OfficialCountryClient) fetchEurostat(ctx context.Context, dataset string, query url.Values) (map[string]float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.eurostatURL+"/"+dataset+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Eurostat %s HTTP %d", dataset, resp.StatusCode)
	}
	var payload eurostatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	values := make(map[string]float64)
	for month, index := range payload.Dimension.Time.Category.Index {
		if value, ok := payload.Value[strconv.Itoa(index)]; ok {
			values[month] = value
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("Eurostat %s returned no observations", dataset)
	}
	return values, nil
}

func (c *OfficialCountryClient) fetchECBM3(ctx context.Context) (map[string]float64, error) {
	query := url.Values{"startPeriod": {"2023-01"}, "format": {"csvdata"}}
	endpoint := c.ecbURL + "/BSI/M.U2.Y.V.M30.X.1.U2.2300.Z01.E?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/csv")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ECB M3 HTTP %d", resp.StatusCode)
	}
	rows := csv.NewReader(io.LimitReader(resp.Body, 8<<20))
	header, err := rows.Read()
	if err != nil {
		return nil, err
	}
	timeIndex, valueIndex := -1, -1
	for index, field := range header {
		switch field {
		case "TIME_PERIOD":
			timeIndex = index
		case "OBS_VALUE":
			valueIndex = index
		}
	}
	if timeIndex < 0 || valueIndex < 0 {
		return nil, fmt.Errorf("ECB M3 CSV lacks time or value columns")
	}
	values := make(map[string]float64)
	for {
		row, err := rows.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if timeIndex >= len(row) || valueIndex >= len(row) {
			continue
		}
		value, err := strconv.ParseFloat(row[valueIndex], 64)
		if err == nil {
			values[row[timeIndex]] = value
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("ECB M3 returned no observations")
	}
	return values, nil
}

func growthSeries(levels map[string]float64) map[string]float64 {
	growth := make(map[string]float64)
	for month, current := range levels {
		date, err := time.Parse("2006-01", month)
		if err != nil {
			continue
		}
		if previous, ok := levels[date.AddDate(-1, 0, 0).Format("2006-01")]; ok && previous != 0 {
			growth[month] = (current/previous - 1) * 100
		}
	}
	return growth
}

func overlayEuroArea(country model.CountrySeries, overlays map[string]map[string]float64) model.CountrySeries {
	country = overlayCountryMetrics(country, overlays)
	country.Sources = append(country.Sources, "Eurostat:HICP,industrial production,unemployment", "ECB Data Portal:M3")
	return country
}

func overlayUnitedKingdom(country model.CountrySeries, overlays map[string]map[string]float64) model.CountrySeries {
	country = overlayCountryMetrics(country, overlays)
	country.Sources = append(country.Sources, "ONS:CPI,unemployment,total production")
	return country
}

func overlayCountryMetrics(country model.CountrySeries, overlays map[string]map[string]float64) model.CountrySeries {
	byMonth := make(map[string]int, len(country.Points))
	for index := range country.Points {
		byMonth[country.Points[index].Date[:7]] = index
	}
	for metric, values := range overlays {
		for month, value := range values {
			index, ok := byMonth[month]
			if !ok {
				country.Points = append(country.Points, model.CountryPoint{Date: month + "-01"})
				index = len(country.Points) - 1
				byMonth[month] = index
			}
			point := &country.Points[index]
			date := month + "-01"
			switch metric {
			case "inflation":
				point.Inflation, point.InflationDate = floatPtr(value), date
			case "industrial":
				point.IndustrialGrowth, point.IndustrialDate = floatPtr(value), date
			case "unemployment":
				point.Unemployment, point.UnemploymentDate = floatPtr(value), date
			case "money":
				point.MoneyGrowth, point.MoneyGrowthDate = floatPtr(value), date
			}
		}
	}
	for index := range country.Points {
		point := &country.Points[index]
		point.RealRate = difference(point.PolicyRate, point.Inflation)
		point.YieldCurve = difference(point.LongRate, point.PolicyRate)
	}
	sort.Slice(country.Points, func(i, j int) bool { return country.Points[i].Date < country.Points[j].Date })
	return country
}

func staleCountryWarnings(country model.CountrySeries, now time.Time) []string {
	type metricDate struct{ metric, date string }
	latest := map[string]string{}
	for _, point := range country.Points {
		for _, item := range []metricDate{
			{"policy", point.PolicyRateDate}, {"inflation", point.InflationDate}, {"industrial", point.IndustrialDate},
			{"unemployment", point.UnemploymentDate}, {"money", point.MoneyGrowthDate}, {"long rate", point.LongRateDate},
		} {
			if item.date > latest[item.metric] {
				latest[item.metric] = item.date
			}
		}
	}
	warnings := make([]string, 0)
	cutoff := now.AddDate(0, -9, 0)
	for _, metric := range []string{"policy", "inflation", "industrial", "unemployment", "money", "long rate"} {
		date := latest[metric]
		if date == "" {
			warnings = append(warnings, fmt.Sprintf("%s %s unavailable", country.Code, metric))
			continue
		}
		observed, err := time.Parse("2006-01-02", date)
		if err == nil && observed.Before(cutoff) {
			warnings = append(warnings, fmt.Sprintf("%s %s stale since %s", country.Code, metric, date[:7]))
		}
	}
	return warnings
}
