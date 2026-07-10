import { describe, expect, it } from "vitest";
import { coherentRegime, forwardRegimeReturns, pillarSnapshots } from "./analysis";
import type { EquitySummary, MacroPoint } from "./types";

describe("macro analysis", () => {
  it("uses the latest month with both regime inputs", () => {
    const points: MacroPoint[] = [
      { date: "2025-01-01", inflation: 2, industrialGrowth: 1 },
      { date: "2025-02-01", inflation: 4 },
    ];
    expect(coherentRegime(points)).toMatchObject({ label: "Disinflationary expansion", point: { date: "2025-01-01" } });
  });

  it("builds dated pillar snapshots with changes and percentiles", () => {
    const points: MacroPoint[] = Array.from({ length: 18 }, (_, index) => ({
      date: `${2024 + Math.floor(index / 12)}-${String(index % 12 + 1).padStart(2, "0")}-01`,
      coreInflation: 2 + index / 10,
      shelterInflation: 3 + index / 10,
      wageGrowth: 3 + index / 20,
      industrialGrowth: index / 5,
      payrollGrowth: index / 10,
      realGdpGrowth: index / 8,
      sahmRule: 0,
      netLiquidityGrowth: index / 3,
      m2Growth: index / 4,
      bankCreditGrowth: index / 5,
      financialConditions: -index / 20,
      highYieldSpread: 5 - index / 20,
      lendingStandards: 20 - index,
    }));
    const snapshots = pillarSnapshots(points, [Date.parse(points[0].date), Date.parse(points.at(-1)!.date)]);
    expect(snapshots).toHaveLength(4);
    expect(snapshots[0]).toMatchObject({ key: "inflation", date: points.at(-1)!.date, ageMonths: 0 });
    expect(snapshots.every((snapshot) => snapshot.percentile !== undefined)).toBe(true);
  });

  it("groups release-lagged forward equity returns by regime", () => {
    const macro: MacroPoint[] = Array.from({ length: 36 }, (_, index) => ({
      date: `${2020 + Math.floor(index / 12)}-${String(index % 12 + 1).padStart(2, "0")}-01`,
      inflation: index < 18 ? 2 : 4,
      industrialGrowth: 1,
    }));
    const equity: EquitySummary = {
      ticker: "SPY",
      prices: Array.from({ length: 10 }, (_, index) => ({ date: `${2020 + Math.floor(index / 4)}-${String(index % 4 * 3 + 3).padStart(2, "0")}-28`, close: 100 + index * 10 })),
    };
    const stats = forwardRegimeReturns(equity, macro);
    expect(stats.reduce((sum, row) => sum + row.count, 0)).toBeGreaterThan(0);
    expect(stats.every((row) => row.average > 0)).toBe(true);
  });
});
