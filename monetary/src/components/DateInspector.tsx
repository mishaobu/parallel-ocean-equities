import { Pin, X } from "lucide-react";
import { pointAtOrBefore, readingAtOrBefore } from "../macroData";
import type { MacroPoint } from "../types";

const metrics: Array<{ key: keyof MacroPoint; label: string; unit: string }> = [
  { key: "inflation", label: "CPI", unit: "%" },
  { key: "coreInflation", label: "Core CPI", unit: "%" },
  { key: "industrialGrowth", label: "Industry", unit: "%" },
  { key: "netLiquidityGrowth", label: "Net liquidity", unit: "%" },
  { key: "real10Y", label: "Real 10Y", unit: "%" },
  { key: "highYieldSpread", label: "HY spread", unit: "%" },
  { key: "financialConditions", label: "NFCI", unit: "" },
];

export function DateInspector({ points, date, pinned, onClear }: { points: MacroPoint[]; date?: number; pinned: boolean; onClear: () => void }) {
  if (date === undefined) return <div className="date-inspector-placeholder"><span>Hover any time series to synchronize the dashboard. Click to pin a month.</span></div>;
  const point = pointAtOrBefore(points, date);
  if (!point) return null;
  return <aside className="date-inspector" aria-label="Selected macro month">
    <div className="inspector-date"><Pin size={14} /><div><strong>{monthLabel(point.date)}</strong><span>{pinned ? "Pinned snapshot" : "Synchronized hover"}</span></div></div>
    <div className="inspector-values">
      {metrics.map((metric) => { const reading = readingAtOrBefore(points, date, metric.key as Exclude<keyof MacroPoint, "date">); return <div key={metric.key}><span>{metric.label}</span><strong>{format(reading?.value, metric.unit)}</strong></div>; })}
    </div>
    {pinned && <button type="button" onClick={onClear} aria-label="Clear pinned month" title="Clear pinned month"><X size={15} /></button>}
  </aside>;
}

function format(value: unknown, unit: string) {
  return typeof value === "number" && Number.isFinite(value) ? `${value.toFixed(2)}${unit}` : "n/a";
}
function monthLabel(date: string) { return new Date(`${date.slice(0, 10)}T00:00:00Z`).toLocaleDateString("en-US", { month: "short", year: "numeric", timeZone: "UTC" }); }
