import { describe, expect, it } from "vitest";
import { currentValue, rangeDomain, recessionIntervals, regimeLabel } from "./macroData";
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
});
