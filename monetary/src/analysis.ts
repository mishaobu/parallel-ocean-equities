import { latestReading, metricChange, pointAtOrBefore, readingAtOrBefore, regimeLabel, type MacroMetric } from "./macroData";
import type { EquitySummary, MacroPoint } from "./types";

export type PillarKey = "inflation" | "growth" | "liquidity" | "credit";

export interface PillarSnapshot {
  key: PillarKey;
  label: string;
  value?: number;
  unit: "percent" | "index";
  date?: string;
  ageMonths: number;
  change?: number;
  percentile?: number;
  score?: number;
  signal: string;
  detail: string;
}

export interface PillarRow extends Record<PillarKey, number | undefined> {
  date: string;
  timestamp: number;
}

interface PillarDefinition {
  key: PillarKey;
  label: string;
  primary: MacroMetric;
  metrics: Array<{ key: MacroMetric; direction: 1 | -1 }>;
  unit: "percent" | "index";
  detail: string;
  high: string;
  low: string;
}

const definitions: PillarDefinition[] = [
  { key: "inflation", label: "Inflation", primary: "coreInflation", metrics: [{ key: "coreInflation", direction: 1 }, { key: "shelterInflation", direction: 1 }, { key: "wageGrowth", direction: 1 }], unit: "percent", detail: "Core CPI / shelter / wages", high: "Hot", low: "Disinflation" },
  { key: "growth", label: "Growth", primary: "industrialGrowth", metrics: [{ key: "industrialGrowth", direction: 1 }, { key: "payrollGrowth", direction: 1 }, { key: "realGdpGrowth", direction: 1 }, { key: "sahmRule", direction: -1 }], unit: "percent", detail: "Industry / payrolls / GDP / Sahm", high: "Expanding", low: "Contracting" },
  { key: "liquidity", label: "Liquidity", primary: "netLiquidityGrowth", metrics: [{ key: "netLiquidityGrowth", direction: 1 }, { key: "m2Growth", direction: 1 }, { key: "bankCreditGrowth", direction: 1 }], unit: "percent", detail: "Net liquidity / M2 / bank credit", high: "Expanding", low: "Draining" },
  { key: "credit", label: "Credit", primary: "highYieldSpread", metrics: [{ key: "financialConditions", direction: -1 }, { key: "highYieldSpread", direction: -1 }, { key: "lendingStandards", direction: -1 }], unit: "percent", detail: "Conditions / HY spreads / SLOOS", high: "Easy", low: "Tight" },
];

export function pillarRows(points: MacroPoint[], domain: [number, number]): PillarRow[] {
  const filtered = points.filter((point) => {
    const date = Date.parse(point.date);
    return Number.isFinite(date) && date >= domain[0] && date <= domain[1];
  });
  const stats = metricStats(filtered);
  return filtered.map((point) => ({
    date: point.date,
    timestamp: Date.parse(point.date),
    inflation: scorePoint(point, definitions[0], stats),
    growth: scorePoint(point, definitions[1], stats),
    liquidity: scorePoint(point, definitions[2], stats),
    credit: scorePoint(point, definitions[3], stats),
  }));
}

export function pillarSnapshots(points: MacroPoint[], domain: [number, number]): PillarSnapshot[] {
  const rows = pillarRows(points, domain);
  return definitions.map((definition) => {
    const reading = latestReading(points, definition.primary);
    const latestScore = [...rows].reverse().find((row) => row[definition.key] !== undefined)?.[definition.key];
    const scores = rows.map((row) => row[definition.key]).filter((value): value is number => value !== undefined).sort((left, right) => left - right);
    const percentile = latestScore === undefined || scores.length < 2 ? undefined : percentileOf(scores, latestScore);
    return {
      key: definition.key,
      label: definition.label,
      value: reading?.value,
      unit: definition.unit,
      date: reading?.date,
      ageMonths: reading?.ageMonths ?? 0,
      change: metricChange(points, definition.primary, 3),
      percentile,
      score: latestScore,
      signal: latestScore === undefined ? "Incomplete" : latestScore > 0.5 ? definition.high : latestScore < -0.5 ? definition.low : "Neutral",
      detail: definition.detail,
    };
  });
}

export function coherentRegime(points: MacroPoint[]) {
  for (let index = points.length - 1; index >= 0; index -= 1) {
    const point = points[index];
    if (finite(point.inflation) && finite(point.industrialGrowth)) {
      return { point, label: regimeLabel(point.inflation, point.industrialGrowth) };
    }
  }
  return { point: undefined, label: "Incomplete signal" };
}

export interface RegimeReturnStat {
  regime: string;
  count: number;
  average: number;
  median: number;
  positiveRate: number;
}

export function forwardRegimeReturns(equity: EquitySummary | undefined, points: MacroPoint[]): RegimeReturnStat[] {
  const prices = [...(equity?.prices ?? [])].sort((left, right) => left.date.localeCompare(right.date));
  const grouped = new Map<string, number[]>();
  for (let index = 0; index + 4 < prices.length; index += 1) {
    const start = prices[index];
    const end = prices[index + 4];
    if (start.close <= 0) continue;
    const conservativeDate = new Date(`${start.date}T00:00:00Z`);
    conservativeDate.setUTCMonth(conservativeDate.getUTCMonth() - 2);
    const macro = pointAtOrBefore(points, conservativeDate.getTime());
    if (!macro || !finite(macro.inflation) || !finite(macro.industrialGrowth)) continue;
    const regime = regimeLabel(macro.inflation, macro.industrialGrowth);
    const values = grouped.get(regime) ?? [];
    values.push(end.close / start.close - 1);
    grouped.set(regime, values);
  }
  return [...grouped.entries()].map(([regime, values]) => {
    const sorted = [...values].sort((left, right) => left - right);
    return {
      regime,
      count: values.length,
      average: values.reduce((sum, value) => sum + value, 0) / values.length,
      median: sorted.length % 2 ? sorted[Math.floor(sorted.length / 2)] : (sorted[sorted.length / 2 - 1] + sorted[sorted.length / 2]) / 2,
      positiveRate: values.filter((value) => value > 0).length / values.length,
    };
  }).sort((left, right) => right.average - left.average);
}

export function equityMacroRows(equity: EquitySummary | undefined, points: MacroPoint[], domain: [number, number]) {
  const prices = (equity?.prices ?? []).filter((price) => {
    const date = Date.parse(price.date);
    return date >= domain[0] && date <= domain[1] && price.close > 0;
  });
  const base = prices[0]?.close;
  if (!base) return [];
  return prices.map((price) => {
    const timestamp = Date.parse(price.date);
    return {
      timestamp,
      priceIndex: price.close / base * 100,
      real10Y: readingAtOrBefore(points, timestamp, "real10Y")?.value,
      netLiquidityGrowth: readingAtOrBefore(points, timestamp, "netLiquidityGrowth")?.value,
      highYieldSpread: readingAtOrBefore(points, timestamp, "highYieldSpread")?.value,
    };
  });
}

function metricStats(points: MacroPoint[]) {
  const keys = new Set(definitions.flatMap((definition) => definition.metrics.map((metric) => metric.key)));
  const stats = new Map<MacroMetric, { mean: number; deviation: number }>();
  for (const key of keys) {
    const values = points.map((point) => point[key]).filter((value): value is number => finite(value));
    const mean = values.length ? values.reduce((sum, value) => sum + value, 0) / values.length : 0;
    const variance = values.length ? values.reduce((sum, value) => sum + (value - mean) ** 2, 0) / values.length : 0;
    stats.set(key, { mean, deviation: Math.sqrt(variance) });
  }
  return stats;
}

function scorePoint(point: MacroPoint, definition: PillarDefinition, stats: Map<MacroMetric, { mean: number; deviation: number }>) {
  const scores = definition.metrics.flatMap(({ key, direction }) => {
    const value = point[key];
    const stat = stats.get(key);
    if (!finite(value) || !stat || stat.deviation === 0) return [];
    return [Math.max(-3, Math.min(3, (value - stat.mean) / stat.deviation)) * direction];
  });
  return scores.length ? scores.reduce((sum, value) => sum + value, 0) / scores.length : undefined;
}

function percentileOf(values: number[], target: number) {
  let count = 0;
  for (const value of values) if (value <= target) count += 1;
  return (count - 1) / (values.length - 1) * 100;
}

function finite(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}
