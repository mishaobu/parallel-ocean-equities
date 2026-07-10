import { describe, expect, it } from "vitest";
import { historyDomain, indexedPerformanceRows, macroHistoryRows, valuationHistoryRows } from "./historyData";
import type { Equity } from "./types";
import { valuationRows } from "./valuationData";

const equities = [{
  ticker: "AMZN",
  status: "ready",
  annuals: [],
  current: {},
  valuations: [
    { date: "2010-01-01", pe: 20, forwardPe: 18 },
    { date: "2020-01-01", pe: 30, forwardPe: 25 },
  ],
  prices: [{ date: "2005-01-01", close: 10 }, { date: "2020-01-01", close: 30 }],
}] satisfies Equity[];

describe("historical chart data", () => {
  it("uses the earliest valuation or macro date for max range", () => {
    const domain = historyDomain(equities, [{ date: "2000-01-01", fedFunds: 5 }], "max", new Date("2026-07-10T00:00:00Z"));
    expect(domain[0]).toBe(Date.parse("2005-01-01"));
    expect(domain[1]).toBe(Date.parse("2026-07-10T00:00:00Z"));
  });

  it("indexes each price history to a 1x wealth multiple in range", () => {
    const rows = indexedPerformanceRows(equities, [Date.parse("2000-01-01"), Date.parse("2026-01-01")]);
    expect(rows).toEqual([
      { date: Date.parse("2005-01-01"), AMZN: 1 },
      { date: Date.parse("2020-01-01"), AMZN: 3 },
    ]);
  });

  it("selects forward values and respects the shared date domain", () => {
    const rows = valuationHistoryRows(equities, valuationRows[0], "forward", [Date.parse("2015-01-01"), Date.parse("2026-01-01")]);
    expect(rows).toEqual([{ date: Date.parse("2020-01-01"), AMZN: 25 }]);
  });

  it("filters macro observations to the same domain", () => {
    const rows = macroHistoryRows([{ date: "2010-01-01", inflation: 2 }, { date: "2020-01-01", inflation: 3 }], [Date.parse("2015-01-01"), Date.parse("2025-01-01")]);
    expect(rows).toHaveLength(1);
    expect(rows[0].inflation).toBe(3);
  });
});
