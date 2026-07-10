import type { QuarterlyPoint, ValuationMetrics, ValuationPoint } from "./types";

export type ValuationMetricKey = "pe" | "ev-ebitda" | "ev-ebit" | "fcf-market-cap" | "fcf-ev" | "net-debt-ebitda" | "dividend-fcf";

export interface ValuationRow {
  key: ValuationMetricKey;
  label: string;
  actual: keyof ValuationMetrics & keyof ValuationPoint;
  forward: keyof ValuationMetrics & keyof ValuationPoint;
  kind: "multiple" | "yield" | "leverage";
}

export const valuationRows: ValuationRow[] = [
  { key: "pe", label: "P/E", actual: "pe", forward: "forwardPe", kind: "multiple" },
  { key: "ev-ebitda", label: "EV / EBITDA", actual: "evToEbitda", forward: "forwardEvToEbitda", kind: "multiple" },
  { key: "ev-ebit", label: "EV / EBIT", actual: "evToEbit", forward: "forwardEvToEbit", kind: "multiple" },
  { key: "fcf-market-cap", label: "FCF / market cap", actual: "fcfToMarketCap", forward: "forwardFcfToMarketCap", kind: "yield" },
  { key: "fcf-ev", label: "FCF / EV", actual: "fcfToEv", forward: "forwardFcfToEv", kind: "yield" },
  { key: "net-debt-ebitda", label: "Net debt / EBITDA", actual: "netDebtToEbitda", forward: "forwardNetDebtToEbitda", kind: "leverage" },
  { key: "dividend-fcf", label: "Dividend / FCF", actual: "dividendToFcf", forward: "forwardDividendToFcf", kind: "yield" },
];

export function formatValuation(value: unknown, kind: ValuationRow["kind"]): string {
  if (typeof value !== "number" || !Number.isFinite(value)) return "n/a";
  if (kind === "yield") return `${(value * 100).toFixed(1)}%`;
  if (kind === "multiple" && value < 0) return "n/m";
  return `${value.toFixed(1)}x`;
}

export function formatBillions(value?: number): string {
  if (value === undefined || !Number.isFinite(value)) return "n/a";
  const absolute = Math.abs(value);
  const precision = absolute >= 100 ? 0 : absolute >= 10 ? 1 : 2;
  const formatted = absolute.toLocaleString("en-US", { minimumFractionDigits: precision, maximumFractionDigits: precision });
  return `${value < 0 ? "-" : ""}$${formatted}B`;
}

export function quarterLabel(row: QuarterlyPoint): string {
  return `FY${String(row.fiscalYear).slice(-2)} ${row.fiscalQuarter}${row.derived ? "*" : ""}`;
}

export function percentValue(value?: number): string {
  if (value === undefined || !Number.isFinite(value)) return "n/a";
  return `${value >= 0 ? "+" : ""}${(value * 100).toFixed(1)}%`;
}
