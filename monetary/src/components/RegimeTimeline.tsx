import { pillarRows, type PillarKey } from "../analysis";
import type { MacroPoint } from "../types";

const labels: Record<PillarKey, string> = { inflation: "Inflation", growth: "Growth", liquidity: "Liquidity", credit: "Credit" };
const keys: PillarKey[] = ["inflation", "growth", "liquidity", "credit"];

export function RegimeTimeline({ points, domain, selectedDate, onPin }: { points: MacroPoint[]; domain: [number, number]; selectedDate?: number; onPin?: (date: number) => void }) {
  const rows = pillarRows(points, domain);
  const stride = Math.max(1, Math.ceil(rows.length / 144));
  const sampled = rows.filter((_, index) => index % stride === 0 || index === rows.length - 1);
  return <article className="regime-timeline">
    <header><div><h2>Macro regime timeline</h2><span>Standardized conditions across the selected history</span></div><i>Click a month to pin</i></header>
    <div className="timeline-body">
      {keys.map((key) => <div className="timeline-row" key={key}>
        <b>{labels[key]}</b>
        <div className="timeline-cells" style={{ gridTemplateColumns: `repeat(${sampled.length}, minmax(1px, 1fr))` }}>
          {sampled.map((row) => <button type="button" key={`${key}-${row.date}`} style={{ background: colorFor(row[key], key) }} className={selectedDate !== undefined && Math.abs(row.timestamp - selectedDate) < 45 * 86400_000 ? "is-selected" : ""} onClick={() => onPin?.(row.timestamp)} title={`${labels[key]} / ${row.date.slice(0, 7)} / ${row[key]?.toFixed(2) ?? "n/a"}`} aria-label={`Pin ${row.date.slice(0, 7)}`} />)}
        </div>
      </div>)}
      <div className="timeline-axis"><span>{year(domain[0])}</span><span>{year((domain[0] + domain[1]) / 2)}</span><span>{year(domain[1])}</span></div>
    </div>
  </article>;
}

function colorFor(value: number | undefined, key: PillarKey) {
  if (value === undefined) return "#e5e9e6";
  const intensity = Math.min(1, Math.abs(value) / 2);
  if (key === "inflation") return value > 0 ? mix("#f4e7e2", "#b4473c", intensity) : mix("#e7efeb", "#327651", intensity);
  return value > 0 ? mix("#e7efeb", "#327651", intensity) : mix("#f4e7e2", "#b4473c", intensity);
}
function mix(start: string, end: string, amount: number) {
  const left = start.match(/\w\w/g)?.map((part) => Number.parseInt(part, 16)) ?? [0, 0, 0];
  const right = end.match(/\w\w/g)?.map((part) => Number.parseInt(part, 16)) ?? [0, 0, 0];
  return `rgb(${left.map((value, index) => Math.round(value + (right[index] - value) * amount)).join(",")})`;
}
function year(value: number) { return new Date(value).getUTCFullYear(); }
