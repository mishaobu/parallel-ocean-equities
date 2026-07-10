import { describe, expect, it } from "vitest";
import { comparisonRows, delta, formatMetric } from "./chartData";
import type { Equity } from "./types";

const equity: Equity = {
  ticker: "TEST",
  status: "ready",
  annuals: [
    { fiscalYear: 2024, capexB: 10 },
    { fiscalYear: 2025, capexB: 12 },
    { fiscalYear: 2026, capexB: 15, estimate: true },
  ],
  current: {},
};

describe("chart data", () => {
  it("aligns ticker values by fiscal year", () => {
    expect(comparisonRows([equity], "capexB")).toEqual([
      { year: 2024, TEST: 10 },
      { year: 2025, TEST: 12 },
      { year: 2026, TEST: 15, estimate: true },
    ]);
  });

  it("formats and compares metrics", () => {
    expect(formatMetric("capexB", 12.25)).toBe("$12.3B");
    expect(formatMetric("peRatio", 22.04)).toBe("22.0x");
    expect(delta(15, 12)).toBeCloseTo(0.25);
  });
});
