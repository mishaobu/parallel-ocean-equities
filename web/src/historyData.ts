import type { Equity, MacroPoint } from "./types";
import type { ValuationRow } from "./valuationData";

export type HistoryBasis = "actual" | "forward";
export type HistoryRange = "max" | "25y" | "15y" | "10y";

export function historyDomain(equities: Equity[], macro: MacroPoint[], range: HistoryRange, now = new Date()): [number, number] {
  const end = now.getTime();
  if (range !== "max") {
    const years = range === "25y" ? 25 : range === "15y" ? 15 : 10;
    return [Date.UTC(now.getUTCFullYear() - years, now.getUTCMonth(), 1), end];
  }
  const marketDates = [
    ...equities.flatMap((equity) => equity.valuations?.map((point) => point.date) ?? []),
    ...equities.flatMap((equity) => equity.prices?.map((point) => point.date) ?? []),
  ].map(Date.parse).filter(Number.isFinite);
  const dates = marketDates.length ? marketDates : macro.map((point) => Date.parse(point.date)).filter(Number.isFinite);
  return [dates.length ? Math.min(...dates) : Date.UTC(now.getUTCFullYear() - 10, now.getUTCMonth(), 1), end];
}

export function valuationHistoryDomain(equities: Equity[], range: HistoryRange, now = new Date()): [number, number] {
  if (range !== "max") return historyDomain(equities, [], range, now);
  const dates = equities.flatMap((equity) => equity.valuations?.map((point) => Date.parse(point.date)) ?? []).filter(Number.isFinite);
  return [dates.length ? Math.min(...dates) : Date.UTC(now.getUTCFullYear() - 10, now.getUTCMonth(), 1), now.getTime()];
}

export function indexedPerformanceRows(equities: Equity[], domain: [number, number]) {
  const rows = new Map<number, Record<string, number>>();
  for (const equity of equities) {
    const prices = (equity.prices ?? []).map((point) => ({ ...point, timestamp: Date.parse(point.date) }))
      .filter((point) => Number.isFinite(point.timestamp) && point.timestamp >= domain[0] && point.timestamp <= domain[1] && point.close > 0)
      .sort((left, right) => left.timestamp - right.timestamp);
    const base = prices[0]?.close;
    if (!base) continue;
    for (const point of prices) {
      const row = rows.get(point.timestamp) ?? { date: point.timestamp };
      row[equity.ticker] = point.close / base;
      rows.set(point.timestamp, row);
    }
  }
  return [...rows.values()].sort((left, right) => left.date - right.date);
}

export function valuationHistoryRows(equities: Equity[], metric: ValuationRow, basis: HistoryBasis, domain: [number, number]) {
  const key = metric[basis];
  const rows = new Map<number, Record<string, number>>();
  for (const equity of equities) {
    for (const point of equity.valuations ?? []) {
      const date = Date.parse(point.date);
      const value = point[key];
      if (!Number.isFinite(date) || date < domain[0] || date > domain[1] || typeof value !== "number" || !Number.isFinite(value)) continue;
      const row = rows.get(date) ?? { date };
      row[equity.ticker] = value;
      rows.set(date, row);
    }
  }
  return [...rows.values()].sort((left, right) => left.date - right.date);
}

export function macroHistoryRows(points: MacroPoint[], domain: [number, number]) {
  return points.flatMap((point) => {
    const date = Date.parse(point.date);
    if (!Number.isFinite(date) || date < domain[0] || date > domain[1]) return [];
    return [{ ...point, date }];
  });
}
