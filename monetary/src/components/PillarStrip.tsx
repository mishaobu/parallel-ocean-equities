import { ArrowDown, ArrowRight, ArrowUp } from "lucide-react";
import type { PillarSnapshot } from "../analysis";

export function PillarStrip({ pillars }: { pillars: PillarSnapshot[] }) {
  return <section className="pillar-strip" aria-label="Current macro regime pillars">
    {pillars.map((pillar) => {
      const Direction = pillar.change === undefined || Math.abs(pillar.change) < 0.05 ? ArrowRight : pillar.change > 0 ? ArrowUp : ArrowDown;
      return <article key={pillar.key} className={`pillar pillar-${pillar.key}`}>
        <div className="pillar-heading"><span>{pillar.label}</span><b>{pillar.signal}</b></div>
		<div className="pillar-value"><strong title={pillar.valueLabel}>{formatValue(pillar.value, pillar.unit)}</strong><span><Direction size={13} />{formatChange(pillar.change)}</span></div>
        <div className="pillar-gauge"><i style={{ width: `${clamp(pillar.percentile ?? 50, 0, 100)}%` }} /></div>
        <div className="pillar-meta"><span>P{pillar.percentile === undefined ? "--" : Math.round(pillar.percentile)}</span><span>Composite through {pillar.date?.slice(0, 7) ?? "unavailable"}{pillar.ageMonths > 0 ? ` / ${pillar.ageMonths}m lag` : ""}</span></div>
		<small>{pillar.valueLabel}{pillar.valueDate ? ` (${pillar.valueDate.slice(0, 7)})` : ""} / {pillar.detail}</small>
      </article>;
    })}
  </section>;
}

function formatValue(value: number | undefined, unit: string) {
  if (value === undefined) return "n/a";
  return unit === "percent" ? `${value.toFixed(1)}%` : value.toFixed(2);
}
function formatChange(value: number | undefined) {
  if (value === undefined) return "n/a";
  return `${value >= 0 ? "+" : ""}${value.toFixed(1)} / 3m`;
}
function clamp(value: number, minimum: number, maximum: number) { return Math.max(minimum, Math.min(maximum, value)); }
