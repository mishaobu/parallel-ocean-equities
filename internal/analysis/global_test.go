package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaobu/parallel-ocean-equities/internal/model"
)

func TestBuildCountryTransformsSeriesAndKeepsObservationDates(t *testing.T) {
	spec := countrySpec{
		code: "XX", name: "Example", currency: "X", region: "Test", policyLabel: "Policy", fxLabel: "FX",
		policy: "POLICY", inflation: "CPI", industrial: "IND", money: "MONEY", industrialAbove100: true,
	}
	series := map[string]map[string]float64{
		"POLICY": {"2025-01": 4},
		"CPI":    {"2024-01": 100, "2025-01": 103},
		"IND":    {"2025-01": 101.5},
		"MONEY":  {"2024-01": 200, "2025-01": 220},
	}
	country := buildCountry(spec, series, nil)
	point := country.Points[len(country.Points)-1]
	assertClose(t, "inflation", point.Inflation, 3)
	assertClose(t, "industrial growth", point.IndustrialGrowth, 1.5)
	assertClose(t, "money growth", point.MoneyGrowth, 10)
	assertClose(t, "real rate", point.RealRate, 1)
	if point.InflationDate != "2025-01-01" || point.PolicyRateDate != "2025-01-01" {
		t.Fatalf("observation dates = inflation %q policy %q", point.InflationDate, point.PolicyRateDate)
	}
}

type staticMacroAnalyzer struct{ series model.MacroSeries }

func (f staticMacroAnalyzer) Analyze(context.Context) (model.MacroSeries, error) {
	return f.series, nil
}

type fakeAssetMarket struct{}

func (fakeAssetMarket) History(_ context.Context, ticker string, _, _ time.Time) ([]model.PricePoint, string, error) {
	if ticker == "HYG" {
		return nil, "", errors.New("test unavailable")
	}
	return []model.PricePoint{{Date: "2025-01-31", Close: 100}, {Date: "2025-02-28", Close: 101}}, "test market", nil
}

func TestMacroPipelineKeepsPartialCrossAssetResults(t *testing.T) {
	pipeline := NewMacroPipeline(staticMacroAnalyzer{series: model.MacroSeries{Basis: "test"}}, fakeAssetMarket{})
	series, err := pipeline.Analyze(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Assets) != len(macroAssetSpecs)-1 {
		t.Fatalf("assets = %d, want %d", len(series.Assets), len(macroAssetSpecs)-1)
	}
	if len(series.Warnings) != 1 || series.Assets[0].Points[0].Date != "2025-01-31" {
		t.Fatalf("unexpected macro output: warnings=%v assets=%+v", series.Warnings, series.Assets)
	}
}
