import { describe, expect, it } from "vitest";
import { currentValue, latestCommonPoint, latestReading, rangeDomain, readingAtOrBefore, recessionIntervals, regimeLabel, transformRows } from "./macroData";
import type { MacroPoint } from "./types";

const points: MacroPoint[] = [
  { date: "1970-01-01", inflation: 4 },
  { date: "2020-01-01", recession: 1 },
  { date: "2020-02-01", recession: 1, inflation: 2 },
  { date: "2020-03-01", recession: 0 },
  { date: "2025-01-01", inflation: 3, industrialGrowth: -1 },
];

describe("macro data transforms", () => {
  it("uses the complete available history for max", () => {
    expect(rangeDomain(points, "max").map((value) => new Date(value).getUTCFullYear())).toEqual([1970, 2025]);
  });

  it("finds the latest defined metric value", () => {
    expect(currentValue(points, "inflation")).toBe(3);
  });

  it("condenses NBER monthly flags into intervals", () => {
    const domain = rangeDomain(points, "max");
    expect(recessionIntervals(points, domain)).toEqual([{ start: Date.parse("2020-01-01"), end: Date.parse("2020-02-01") }]);
  });

  it("classifies the current inflation-growth quadrant", () => {
    expect(regimeLabel(3, -1)).toBe("Stagflation pressure");
    expect(regimeLabel(2, 1)).toBe("Disinflationary expansion");
  });

  it("reports metric freshness and uses a coherent common month", () => {
    const dated: MacroPoint[] = [
      { date: "2025-01-01", inflation: 2, industrialGrowth: 1 },
      { date: "2025-02-01", inflation: 3 },
      { date: "2025-03-01", fedFunds: 4 },
    ];
    expect(latestReading(dated, "inflation")).toEqual({ value: 3, date: "2025-02-01", ageMonths: 1 });
    expect(latestCommonPoint(dated, ["inflation", "industrialGrowth"])?.date).toBe("2025-01-01");
    expect(readingAtOrBefore(dated, Date.parse("2025-03-01"), "industrialGrowth")).toEqual({ value: 1, date: "2025-01-01" });
  });

  it("supports synchronized chart transformations", () => {
    const rows = [1, 2, 3, 5].map((inflation, index) => ({ date: `2025-0${index + 1}-01`, timestamp: index, inflation }));
    expect(transformRows(rows, ["inflation"], "change3m")[3].inflation).toBe(4);
    expect(transformRows(rows, ["inflation"], "percentile")[3].inflation).toBe(100);
  });
});
