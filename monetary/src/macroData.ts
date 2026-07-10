import type { MacroPoint } from "./types";

export type MacroRange = "max" | "50y" | "25y" | "10y" | "5y";
export type MacroMetric = Exclude<keyof MacroPoint, "date">;
export type ChartTransform = "native" | "change3m" | "zscore" | "percentile";

export interface RecessionInterval { start: number; end: number }

export function rangeDomain(points: MacroPoint[], range: MacroRange): [number, number] {
  const dates = points.map((point) => Date.parse(point.date)).filter(Number.isFinite).sort((a, b) => a - b);
  const end = dates[dates.length - 1] ?? Date.now();
  if (range === "max") return [dates[0] ?? end, end];
  const years = Number.parseInt(range, 10);
  const start = new Date(end);
  start.setUTCFullYear(start.getUTCFullYear() - years);
  return [start.getTime(), end];
}

export function chartRows(points: MacroPoint[], domain: [number, number]) {
  return points
    .map((point) => ({ ...point, timestamp: Date.parse(point.date) }))
    .filter((point) => Number.isFinite(point.timestamp) && point.timestamp >= domain[0] && point.timestamp <= domain[1]);
}

export function currentValue(points: MacroPoint[], metric: MacroMetric) {
  for (let index = points.length - 1; index >= 0; index -= 1) {
    const value = points[index][metric];
    if (typeof value === "number" && Number.isFinite(value)) return value;
  }
  return undefined;
}

export interface MetricReading {
  value: number;
  date: string;
  ageMonths: number;
}

export function latestReading(points: MacroPoint[], metric: MacroMetric): MetricReading | undefined {
  const latestDate = points.reduce((latest, point) => point.date > latest ? point.date : latest, "");
  for (let index = points.length - 1; index >= 0; index -= 1) {
    const value = points[index][metric];
    if (typeof value === "number" && Number.isFinite(value)) {
      return { value, date: points[index].date, ageMonths: monthDistance(points[index].date, latestDate) };
    }
  }
  return undefined;
}

export function latestCommonPoint(points: MacroPoint[], metrics: MacroMetric[]) {
  for (let index = points.length - 1; index >= 0; index -= 1) {
    if (metrics.every((metric) => typeof points[index][metric] === "number" && Number.isFinite(points[index][metric]))) return points[index];
  }
  return undefined;
}

export function pointAtOrBefore(points: MacroPoint[], date: number) {
  for (let index = points.length - 1; index >= 0; index -= 1) {
    if (Date.parse(points[index].date) <= date) return points[index];
  }
  return undefined;
}

export function readingAtOrBefore(points: MacroPoint[], date: number, metric: MacroMetric) {
  for (let index = points.length - 1; index >= 0; index -= 1) {
    if (Date.parse(points[index].date) > date) continue;
    const value = numeric(points[index][metric]);
    if (value !== undefined) return { value, date: points[index].date };
  }
  return undefined;
}

export function transformRows(rows: Array<MacroPoint & { timestamp: number }>, metrics: MacroMetric[], transform: ChartTransform) {
  if (transform === "native") return rows;
  const transformed = rows.map((row) => ({ ...row }));
  for (const metric of metrics) {
    const values = rows.map((row) => numeric(row[metric]));
    const defined = values.filter((value): value is number => value !== undefined);
    const mean = defined.length ? defined.reduce((sum, value) => sum + value, 0) / defined.length : 0;
    const variance = defined.length ? defined.reduce((sum, value) => sum + (value - mean) ** 2, 0) / defined.length : 0;
    const deviation = Math.sqrt(variance);
    const sorted = [...defined].sort((left, right) => left - right);
    values.forEach((value, index) => {
      if (value === undefined) {
        delete transformed[index][metric];
        return;
      }
      if (transform === "change3m") {
        const prior = values[index - 3];
        if (prior === undefined) delete transformed[index][metric];
        else (transformed[index] as Record<string, unknown>)[metric] = value - prior;
      } else if (transform === "zscore") {
        (transformed[index] as Record<string, unknown>)[metric] = deviation > 0 ? (value - mean) / deviation : 0;
      } else {
        const rank = upperBound(sorted, value);
        (transformed[index] as Record<string, unknown>)[metric] = sorted.length > 1 ? (rank - 1) / (sorted.length - 1) * 100 : 50;
      }
    });
  }
  return transformed;
}

export function recessionIntervals(points: MacroPoint[], domain: [number, number]): RecessionInterval[] {
  const rows = chartRows(points, domain);
  const intervals: RecessionInterval[] = [];
  let start: number | undefined;
  rows.forEach((point, index) => {
    if ((point.recession ?? 0) >= 0.5 && start === undefined) start = point.timestamp;
    const ending = start !== undefined && ((point.recession ?? 0) < 0.5 || index === rows.length - 1);
    if (ending) {
      intervals.push({ start: start!, end: (point.recession ?? 0) >= 0.5 ? point.timestamp : rows[Math.max(0, index - 1)].timestamp });
      start = undefined;
    }
  });
  return intervals;
}

export function regimeLabel(inflation?: number, growth?: number) {
  if (inflation === undefined || growth === undefined) return "Incomplete signal";
  if (inflation >= 2.5 && growth >= 0) return "Inflationary expansion";
  if (inflation >= 2.5) return "Stagflation pressure";
  if (growth >= 0) return "Disinflationary expansion";
  return "Contraction / disinflation";
}

export function meanDefined(values: Array<number | undefined>) {
  const defined = values.filter((value): value is number => value !== undefined && Number.isFinite(value));
  return defined.length ? defined.reduce((sum, value) => sum + value, 0) / defined.length : undefined;
}

export function metricChange(points: MacroPoint[], metric: MacroMetric, months = 3) {
  const readings = points.flatMap((point) => {
    const value = numeric(point[metric]);
    return value === undefined ? [] : [{ value, date: point.date }];
  });
  if (readings.length < 2) return undefined;
  const latest = readings[readings.length - 1];
  const target = new Date(`${latest.date.slice(0, 7)}-01T00:00:00Z`);
  target.setUTCMonth(target.getUTCMonth() - months);
  const prior = [...readings].reverse().find((reading) => Date.parse(reading.date) <= target.getTime());
  return prior ? latest.value - prior.value : undefined;
}

export function percentileRank(points: MacroPoint[], metric: MacroMetric, value: number) {
  const values = points.map((point) => numeric(point[metric])).filter((candidate): candidate is number => candidate !== undefined).sort((left, right) => left - right);
  if (values.length < 2) return undefined;
  return (upperBound(values, value) - 1) / (values.length - 1) * 100;
}

export function descendingTooltipItem(item: { value?: unknown }) {
  const value = Number(item.value);
  return Number.isFinite(value) ? -value : Number.POSITIVE_INFINITY;
}

function numeric(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function upperBound(values: number[], target: number) {
  let low = 0;
  let high = values.length;
  while (low < high) {
    const middle = Math.floor((low + high) / 2);
    if (values[middle] <= target) low = middle + 1;
    else high = middle;
  }
  return low;
}

function monthDistance(left: string, right: string) {
  const leftDate = new Date(`${left.slice(0, 7)}-01T00:00:00Z`);
  const rightDate = new Date(`${right.slice(0, 7)}-01T00:00:00Z`);
  return Math.max(0, (rightDate.getUTCFullYear() - leftDate.getUTCFullYear()) * 12 + rightDate.getUTCMonth() - leftDate.getUTCMonth());
}
