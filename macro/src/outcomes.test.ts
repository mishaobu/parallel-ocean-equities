import { describe, expect, it } from "vitest";
import { currentRegime, fitScenarioCalibration, regimeOutcomes, type CalibrationRow } from "./outcomes";
import type { AssetSeries, CountrySeries } from "./types";

describe("regime outcomes", () => {
  it("uses bounded quarterly starts and reports path risk", () => {
    const points = monthlyDates("2018-01-01", 84);
    const country: CountrySeries = {
      code: "US", name: "United States", currency: "USD", region: "Americas", policyLabel: "Fed", fxLabel: "USD",
      points: points.map((date, index) => ({ date, inflation: index < 42 ? 2 : 4, industrialGrowth: index % 18 < 12 ? 2 : -1 })),
    };
    const asset: AssetSeries = {
      symbol: "SPY", label: "US equities", group: "Equities",
      points: points.map((date, index) => ({ date, close: 100 * 1.008 ** index * (index === 35 ? 0.8 : 1) })),
    };
    const stats = regimeOutcomes([asset], country, [Date.parse(points[0]), Date.parse(points.at(-1)!)]);
    expect(stats.some((row) => row.horizon === 12 && row.count > 0)).toBe(true);
    expect(stats.some((row) => row.worstDrawdown < 0)).toBe(true);
    expect(stats.every((row) => row.startDate >= points[0] && row.endDate <= points.at(-1)!)).toBe(true);
    expect(currentRegime(country)).toBe("Inflationary growth");
  });

  it("fits directional factor sensitivities from observations", () => {
    const rows: CalibrationRow[] = Array.from({ length: 72 }, (_, index) => {
      const factors = {
        growth: Math.sin(index * .71), inflation: Math.cos(index * .43), realRate: Math.sin(index * .19 + 1),
        dollar: index % 11 - 5, liquidity: Math.cos(index * .31) * 2,
      };
      return { factors, outcome: 2 * factors.growth - 1.5 * factors.inflation - .7 * factors.realRate - .2 * factors.dollar + .9 * factors.liquidity };
    });
    const model = fitScenarioCalibration(rows)!;
    expect(model.rSquared).toBeGreaterThan(.98);
    expect(model.exposures.growth).toBeGreaterThan(0);
    expect(model.exposures.inflation).toBeLessThan(0);
    expect(model.exposures.realRate).toBeLessThan(0);
    expect(model.exposures.liquidity).toBeGreaterThan(0);
  });
});

function monthlyDates(start: string, count: number) {
  const first = new Date(`${start}T00:00:00Z`);
  return Array.from({ length: count }, (_, index) => {
    const date = new Date(first); date.setUTCMonth(date.getUTCMonth() + index); return date.toISOString().slice(0, 10);
  });
}
