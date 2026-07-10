import type { MacroPoint } from "./types";

export type MacroRange = "max" | "50y" | "25y" | "10y" | "5y";
export type MacroMetric = Exclude<keyof MacroPoint, "date">;

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

export function descendingTooltipItem(item: { value?: unknown }) {
  const value = Number(item.value);
  return Number.isFinite(value) ? -value : Number.POSITIVE_INFINITY;
}
