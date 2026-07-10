import { latestReading, type MacroMetric } from "../macroData";
import type { MacroPoint } from "../types";

const series: Array<{ key: MacroMetric; label: string }> = [
  { key: "inflation", label: "CPI" }, { key: "corePceInflation", label: "Core PCE" }, { key: "fedFunds", label: "Fed funds" }, { key: "real10Y", label: "Real 10Y" },
  { key: "netLiquidityB", label: "Net liquidity" }, { key: "m2Growth", label: "M2" }, { key: "industrialGrowth", label: "Industry" }, { key: "realGdpGrowth", label: "GDP" },
  { key: "unemployment", label: "Unemployment" }, { key: "financialConditions", label: "NFCI" }, { key: "lendingStandards", label: "SLOOS" }, { key: "highYieldSpread", label: "HY spread" },
];

export function FreshnessPanel({ points }: { points: MacroPoint[] }) {
  return <article className="freshness-panel"><header><div><h2>Observation freshness</h2><span>Latest populated month by analytical input</span></div></header><div className="freshness-grid">
    {series.map((item) => { const reading = latestReading(points, item.key); return <div key={item.key} className={reading && reading.ageMonths > 2 ? "is-stale" : ""}><span>{item.label}</span><strong>{reading?.date.slice(0, 7) ?? "n/a"}</strong><small>{reading ? reading.ageMonths === 0 ? "latest month" : `${reading.ageMonths}m lag` : "unavailable"}</small></div>; })}
  </div></article>;
}
