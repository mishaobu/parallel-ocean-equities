import { describe, expect, it } from "vitest";
import { metricCatalogFreshnessLabel, previousCompletedStatisticPoint } from "./StatisticsExplorer";
import type { StatisticPoint } from "../statisticsData";

const points: StatisticPoint[] = [
  { date: "2026-03-31", label: "2026 Q1", value: 80, source: "test" },
  { date: "2026-06-30", label: "2026 Q2", value: 100, source: "test" },
  { date: "2026-07-30", label: "2026 Q3", value: 110, source: "test" },
];

describe("current statistic comparison", () => {
  it("uses the previous completed selected-period bucket for live values", () => {
    const metric = { marketSensitive: true, currentAsOf: "2026-07-31T18:00:00Z" };
    expect(previousCompletedStatisticPoint(metric, points, "month")?.value).toBe(100);
    expect(previousCompletedStatisticPoint(metric, points, "quarter")?.value).toBe(100);
    expect(previousCompletedStatisticPoint(metric, points, "year")).toBeUndefined();
  });

  it("keeps the latest observation for filing-based values", () => {
    expect(previousCompletedStatisticPoint({ marketSensitive: false }, points, "quarter")?.value).toBe(110);
  });
});

describe("catalog freshness labels", () => {
  it("labels live valuation metrics from their current value, not their filing history", () => {
    expect(metricCatalogFreshnessLabel({ current: 2_900, marketSensitive: true, nativeFrequency: "Filing-derived" })).toBe("Live snapshot");
  });

  it("preserves filing and feed-needed labels", () => {
    expect(metricCatalogFreshnessLabel({ current: 10, marketSensitive: false, nativeFrequency: "Filing-derived" })).toBe("Filing-derived");
    expect(metricCatalogFreshnessLabel({ current: undefined, marketSensitive: true, nativeFrequency: "Market snapshot" })).toBe("feed needed");
  });
});
