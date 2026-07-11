package analysis

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

type countrySpec struct {
	code, name, currency, region, policyLabel, fxLabel, equityTicker                string
	policy, inflation, core, industrial, unemployment, money, longRate, fx, leading string
	inflationDirect, industrialAbove100                                             bool
}

var countrySpecs = []countrySpec{
	{
		code: "EA", name: "Euro area", currency: "EUR", region: "Europe", policyLabel: "ECB deposit facility", fxLabel: "USD per EUR", equityTicker: "FEZ",
		policy: "ECBDFR", inflation: "CP0000EZ19M086NEST", industrial: "EA19PRINTO01GYSAM", unemployment: "LRHUTTTTEZM156S", money: "MABMM301EZM189S", longRate: "IRLTLT01EZM156N", fx: "DEXUSEU",
	},
	{
		code: "GB", name: "United Kingdom", currency: "GBP", region: "Europe", policyLabel: "Short-term interest rate", fxLabel: "USD per GBP", equityTicker: "EWU",
		policy: "IRSTCI01GBM156N", inflation: "GBRCPIALLMINMEI", core: "GBRCPICORMINMEI", industrial: "GBRPRINTO01GYSAM", unemployment: "LRHUTTTTGBM156S", money: "MABMM301GBM189S", longRate: "IRLTLT01GBM156N", fx: "DEXUSUK",
	},
	{
		code: "JP", name: "Japan", currency: "JPY", region: "Asia", policyLabel: "Short-term interest rate", fxLabel: "JPY per USD", equityTicker: "EWJ",
		policy: "IRSTCI01JPM156N", inflation: "FPCPITOTLZGJPN", core: "JPNCPICORMINMEI", industrial: "JPNPRINTO01GYSAM", unemployment: "LRHUTTTTJPM156S", money: "MABMM301JPM189S", longRate: "IRLTLT01JPM156N", fx: "DEXJPUS", inflationDirect: true,
	},
	{
		code: "CN", name: "China", currency: "CNY", region: "Asia", policyLabel: "3M interbank rate", fxLabel: "CNY per USD", equityTicker: "FXI",
		policy: "IR3TIB01CNM156N", inflation: "CPALTT01CNM659N", industrial: "CHNPRINTO01IXPYM", money: "MABMM201CNM189S", fx: "DEXCHUS", leading: "CHNLOLITOAASTSAM", inflationDirect: true, industrialAbove100: true,
	},
}

func (f *FREDClient) analyzeCountries(ctx context.Context, usPoints []model.MacroPoint) ([]model.CountrySeries, []string) {
	ids := make(map[string]struct{})
	for _, spec := range countrySpecs {
		for _, id := range []string{spec.policy, spec.inflation, spec.core, spec.industrial, spec.unemployment, spec.money, spec.longRate, spec.fx, spec.leading} {
			if id != "" {
				ids[id] = struct{}{}
			}
		}
	}
	series := make(map[string]map[string]float64, len(ids))
	errorsByID := make(map[string]string)
	var mutex sync.Mutex
	var workers sync.WaitGroup
	slots := make(chan struct{}, 6)
	for id := range ids {
		workers.Add(1)
		go func(id string) {
			defer workers.Done()
			select {
			case <-ctx.Done():
				return
			case slots <- struct{}{}:
			}
			defer func() { <-slots }()
			values, err := f.fetch(ctx, id)
			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				errorsByID[id] = err.Error()
				return
			}
			series[id] = values
		}(id)
	}
	workers.Wait()

	countries := []model.CountrySeries{buildUSCountry(usPoints)}
	for _, spec := range countrySpecs {
		countries = append(countries, buildCountry(spec, series, errorsByID))
	}
	warnings := make([]string, 0, len(errorsByID))
	for _, warning := range errorsByID {
		warnings = append(warnings, warning)
	}
	sort.Strings(warnings)
	return countries, warnings
}

func buildUSCountry(points []model.MacroPoint) model.CountrySeries {
	rows := make([]model.CountryPoint, 0, len(points))
	for _, point := range points {
		row := model.CountryPoint{
			Date: point.Date, PolicyRate: point.FedFunds, Inflation: point.Inflation, CoreInflation: point.CoreInflation,
			IndustrialGrowth: point.IndustrialGrowth, Unemployment: point.Unemployment, MoneyGrowth: point.M2Growth,
			LongRate: point.Treasury10Y, RealRate: point.RealPolicyRate, YieldCurve: point.YieldCurve, FX: point.DollarIndex,
		}
		row.PolicyRateDate = dateIfValue(point.Date, row.PolicyRate)
		row.InflationDate = dateIfValue(point.Date, row.Inflation)
		row.CoreInflationDate = dateIfValue(point.Date, row.CoreInflation)
		row.IndustrialDate = dateIfValue(point.Date, row.IndustrialGrowth)
		row.UnemploymentDate = dateIfValue(point.Date, row.Unemployment)
		row.MoneyGrowthDate = dateIfValue(point.Date, row.MoneyGrowth)
		row.LongRateDate = dateIfValue(point.Date, row.LongRate)
		row.FXDate = dateIfValue(point.Date, row.FX)
		rows = append(rows, row)
	}
	return model.CountrySeries{
		Code: "US", Name: "United States", Currency: "USD", Region: "Americas", PolicyLabel: "Federal funds rate",
		FXLabel: "Broad USD index", EquityTicker: "SPY", Sources: []string{"FRED:US macro series"}, Points: rows,
	}
}

func buildCountry(spec countrySpec, series map[string]map[string]float64, errorsByID map[string]string) model.CountrySeries {
	months := make(map[string]struct{})
	ids := []string{spec.policy, spec.inflation, spec.core, spec.industrial, spec.unemployment, spec.money, spec.longRate, spec.fx, spec.leading}
	for _, id := range ids {
		for month := range series[id] {
			months[month] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(months))
	for month := range months {
		ordered = append(ordered, month)
	}
	sort.Strings(ordered)
	points := make([]model.CountryPoint, 0, len(ordered))
	for _, month := range ordered {
		row := model.CountryPoint{Date: month + "-01"}
		row.PolicyRate, row.PolicyRateDate = datedValue(series[spec.policy], month)
		if spec.inflationDirect {
			row.Inflation, row.InflationDate = datedValue(series[spec.inflation], month)
		} else {
			row.Inflation = yearOverYear(series[spec.inflation], month)
			row.InflationDate = dateIfValue(row.Date, row.Inflation)
		}
		row.CoreInflation = yearOverYear(series[spec.core], month)
		row.CoreInflationDate = dateIfValue(row.Date, row.CoreInflation)
		if spec.industrialAbove100 {
			if value := valueAt(series[spec.industrial], month); value != nil {
				row.IndustrialGrowth = floatPtr(*value - 100)
				row.IndustrialDate = row.Date
			}
		} else {
			row.IndustrialGrowth, row.IndustrialDate = datedValue(series[spec.industrial], month)
		}
		row.Unemployment, row.UnemploymentDate = datedValue(series[spec.unemployment], month)
		row.MoneyGrowth = yearOverYear(series[spec.money], month)
		row.MoneyGrowthDate = dateIfValue(row.Date, row.MoneyGrowth)
		row.LongRate, row.LongRateDate = datedValue(series[spec.longRate], month)
		row.FX, row.FXDate = datedValue(series[spec.fx], month)
		row.LeadingIndex, row.LeadingIndexDate = datedValue(series[spec.leading], month)
		row.RealRate = difference(row.PolicyRate, row.Inflation)
		row.YieldCurve = difference(row.LongRate, row.PolicyRate)
		points = append(points, row)
	}
	sources := make([]string, 0, len(ids))
	warnings := make([]string, 0)
	for _, id := range ids {
		if id == "" {
			continue
		}
		if warning, ok := errorsByID[id]; ok {
			warnings = append(warnings, warning)
		} else if len(series[id]) > 0 {
			sources = append(sources, "FRED:"+id)
		}
	}
	if len(points) == 0 {
		warnings = append(warnings, fmt.Sprintf("%s has no country observations", spec.name))
	}
	return model.CountrySeries{
		Code: spec.code, Name: spec.name, Currency: spec.currency, Region: spec.region, PolicyLabel: spec.policyLabel,
		FXLabel: spec.fxLabel, EquityTicker: spec.equityTicker, Sources: sources, Warnings: warnings, Points: points,
	}
}

func datedValue(values map[string]float64, month string) (*float64, string) {
	value := valueAt(values, month)
	return value, dateIfValue(month+"-01", value)
}

func dateIfValue(date string, value *float64) string {
	if value == nil {
		return ""
	}
	return date
}

type MacroPipeline struct {
	Macro  MacroAnalyzer
	Market MarketProvider
}

var macroAssetSpecs = []struct{ symbol, label, group, region string }{
	{"SPY", "US large cap", "Equities", "United States"},
	{"QQQ", "US growth", "Equities", "United States"},
	{"FEZ", "Euro area", "Equities", "Europe"},
	{"EWU", "United Kingdom", "Equities", "Europe"},
	{"EWJ", "Japan", "Equities", "Asia"},
	{"FXI", "China large cap", "Equities", "Asia"},
	{"EEM", "Emerging markets", "Equities", "Global"},
	{"ACWI", "Global equities", "Equities", "Global"},
	{"TLT", "US long duration", "Rates", "United States"},
	{"HYG", "US high yield", "Credit", "United States"},
	{"GLD", "Gold", "Commodities", "Global"},
	{"UUP", "US dollar", "FX", "United States"},
}

func NewMacroPipeline(macro MacroAnalyzer, market MarketProvider) *MacroPipeline {
	return &MacroPipeline{Macro: macro, Market: market}
}

func (p *MacroPipeline) Analyze(ctx context.Context) (model.MacroSeries, error) {
	series, err := p.Macro.Analyze(ctx)
	if err != nil || p.Market == nil {
		return series, err
	}
	type result struct {
		asset model.AssetSeries
		err   error
	}
	results := make(chan result, len(macroAssetSpecs))
	slots := make(chan struct{}, 4)
	start := time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Now().UTC()
	for _, spec := range macroAssetSpecs {
		go func(spec struct{ symbol, label, group, region string }) {
			select {
			case <-ctx.Done():
				results <- result{err: ctx.Err()}
				return
			case slots <- struct{}{}:
			}
			defer func() { <-slots }()
			points, source, fetchErr := p.Market.History(ctx, spec.symbol, start, end)
			if fetchErr != nil {
				results <- result{err: fmt.Errorf("cross-asset %s: %w", spec.symbol, fetchErr)}
				return
			}
			results <- result{asset: model.AssetSeries{Symbol: spec.symbol, Label: spec.label, Group: spec.group, Region: spec.region, Source: source, Points: downsampleMonthly(points)}}
		}(spec)
	}
	assets := make([]model.AssetSeries, 0, len(macroAssetSpecs))
	for range macroAssetSpecs {
		fetched := <-results
		if fetched.err != nil {
			series.Warnings = append(series.Warnings, fetched.err.Error())
			continue
		}
		assets = append(assets, fetched.asset)
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].Symbol < assets[j].Symbol })
	sort.Strings(series.Warnings)
	series.Assets = assets
	return series, nil
}
