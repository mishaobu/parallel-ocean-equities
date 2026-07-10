import type { AnnualPoint, Equity, MetricKey } from "./types";

export const metricLabels: Record<MetricKey, string> = {
  revenueB: "Revenue",
  capexB: "Capital expenditure",
  netIncomeB: "Net income",
  dilutedEps: "Diluted EPS",
  peRatio: "P/E",
};

export function formatMetric(metric: MetricKey, value?: number): string {
  if (value === undefined || value === null || !Number.isFinite(value)) return "n/a";
  if (metric === "dilutedEps") return `$${value.toFixed(2)}`;
  if (metric === "peRatio") return `${value.toFixed(1)}x`;
  return `$${value.toFixed(value >= 100 ? 0 : 1)}B`;
}

export function latestActual(equity: Equity): AnnualPoint | undefined {
  return [...equity.annuals].reverse().find((row) => !row.estimate);
}

export function latestEstimate(equity: Equity): AnnualPoint | undefined {
  return [...equity.annuals].reverse().find((row) => row.estimate);
}

export function comparisonRows(equities: Equity[], metric: MetricKey) {
  const years = [...new Set(equities.flatMap((equity) => equity.annuals.map((row) => row.fiscalYear)))].sort();
  return years.map((year) => {
    const row: Record<string, number | boolean> = { year };
    for (const equity of equities) {
      const point = equity.annuals.find((candidate) => candidate.fiscalYear === year);
      const value = point?.[metric];
      if (typeof value === "number") row[equity.ticker] = value;
      if (point?.estimate) row.estimate = true;
    }
    return row;
  });
}

export function delta(current?: number, previous?: number): number | undefined {
  if (current === undefined || previous === undefined || previous === 0) return undefined;
  return current / previous - 1;
}

export function descendingTooltipItem(item: { value?: unknown }) {
  const value = Number(item.value);
  return Number.isFinite(value) ? -value : Number.POSITIVE_INFINITY;
}
