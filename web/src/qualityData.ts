import type { QualityMetrics, QualityPoint } from "./types";

export type QualityMetricKey = "cash-conversion" | "gross-margin" | "operating-margin" | "ocf-margin" | "fcf-margin" | "inventory-days" | "receivable-days" | "payable-days" | "cash-cycle" | "roic" | "incremental-roic" | "stock-comp-revenue" | "share-growth";
export type QualityKind = "percent" | "days" | "multiple";

export interface QualityRow {
  key: QualityMetricKey;
  label: string;
  property: keyof QualityMetrics & keyof QualityPoint;
  kind: QualityKind;
}

export const qualityRows: QualityRow[] = [
  { key: "cash-conversion", label: "OCF / net income", property: "cashConversion", kind: "multiple" },
  { key: "gross-margin", label: "Gross margin", property: "grossMargin", kind: "percent" },
  { key: "operating-margin", label: "Operating margin", property: "operatingMargin", kind: "percent" },
  { key: "ocf-margin", label: "OCF margin", property: "operatingCashMargin", kind: "percent" },
  { key: "fcf-margin", label: "FCF margin", property: "fcfMargin", kind: "percent" },
  { key: "inventory-days", label: "Inventory days", property: "inventoryDays", kind: "days" },
  { key: "receivable-days", label: "Receivable days", property: "receivableDays", kind: "days" },
  { key: "payable-days", label: "Payable days", property: "payableDays", kind: "days" },
  { key: "cash-cycle", label: "Cash conversion cycle", property: "cashConversionCycleDays", kind: "days" },
  { key: "roic", label: "ROIC", property: "roic", kind: "percent" },
  { key: "incremental-roic", label: "Incremental ROIC", property: "incrementalRoic", kind: "percent" },
  { key: "stock-comp-revenue", label: "Stock comp / revenue", property: "stockCompToRevenue", kind: "percent" },
  { key: "share-growth", label: "Diluted share growth", property: "dilutedShareGrowth", kind: "percent" },
];

export function formatQuality(value: unknown, kind: QualityKind): string {
  if (typeof value !== "number" || !Number.isFinite(value)) return "n/a";
  if (kind === "percent") return `${(value * 100).toFixed(1)}%`;
  if (kind === "days") return `${value.toFixed(Math.abs(value) >= 100 ? 0 : 1)}d`;
  return `${value.toFixed(2)}x`;
}
