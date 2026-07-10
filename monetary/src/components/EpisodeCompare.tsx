import { useState } from "react";
import { pointAtOrBefore, readingAtOrBefore, type MacroMetric } from "../macroData";
import type { MacroPoint } from "../types";

const episodes = [
  { key: "current", label: "Current", date: Number.POSITIVE_INFINITY },
  { key: "2021-11", label: "Nov 2021", date: Date.parse("2021-11-01") },
  { key: "2020-02", label: "Feb 2020", date: Date.parse("2020-02-01") },
  { key: "2007-10", label: "Oct 2007", date: Date.parse("2007-10-01") },
  { key: "2000-03", label: "Mar 2000", date: Date.parse("2000-03-01") },
  { key: "1981-09", label: "Sep 1981", date: Date.parse("1981-09-01") },
];
const metrics: Array<{ key: MacroMetric; label: string; unit: string }> = [
  { key: "inflation", label: "Headline CPI", unit: "%" },
  { key: "coreInflation", label: "Core CPI", unit: "%" },
  { key: "realPolicyRate", label: "Ex-post real policy", unit: "%" },
  { key: "industrialGrowth", label: "Industrial growth", unit: "%" },
  { key: "netLiquidityGrowth", label: "Net liquidity YoY", unit: "%" },
  { key: "real10Y", label: "Real 10Y", unit: "%" },
  { key: "highYieldSpread", label: "HY spread", unit: "%" },
  { key: "financialConditions", label: "NFCI", unit: "" },
];

export function EpisodeCompare({ points, pinnedDate }: { points: MacroPoint[]; pinnedDate?: number }) {
  const [left, setLeft] = useState("2007-10");
  const [right, setRight] = useState("current");
  const options = pinnedDate === undefined ? episodes : [{ key: "pinned", label: `Pinned ${monthLabel(pinnedDate)}`, date: pinnedDate }, ...episodes];
  const leftEpisode = options.find((episode) => episode.key === left) ?? options[0];
  const rightEpisode = options.find((episode) => episode.key === right) ?? episodes[0];
  const leftPoint = pointAtOrBefore(points, leftEpisode.date);
  const rightPoint = pointAtOrBefore(points, rightEpisode.date);
  return <article className="episode-compare">
    <header><div><h2>Episode comparison</h2><span>Side-by-side macro state using the latest observation at each date</span></div>
      <div className="episode-selects"><select aria-label="First comparison date" value={left} onChange={(event) => setLeft(event.target.value)}>{options.map((episode) => <option key={episode.key} value={episode.key}>{episode.label}</option>)}</select><span>vs</span><select aria-label="Second comparison date" value={right} onChange={(event) => setRight(event.target.value)}>{options.map((episode) => <option key={episode.key} value={episode.key}>{episode.label}</option>)}</select></div>
    </header>
    <div className="table-wrap"><table><thead><tr><th>Metric</th><th>{leftPoint?.date.slice(0, 7) ?? leftEpisode.label}</th><th>{rightPoint?.date.slice(0, 7) ?? rightEpisode.label}</th><th>Change</th></tr></thead><tbody>
      {metrics.map((metric) => { const a = readingAtOrBefore(points, leftEpisode.date, metric.key)?.value; const b = readingAtOrBefore(points, rightEpisode.date, metric.key)?.value; return <tr key={metric.key}><th>{metric.label}</th><td>{format(a, metric.unit)}</td><td>{format(b, metric.unit)}</td><td>{a === undefined || b === undefined ? "n/a" : format(b - a, metric.unit, true)}</td></tr>; })}
    </tbody></table></div>
  </article>;
}

function format(value: number | undefined, unit: string, signed = false) { return value === undefined ? "n/a" : `${signed && value >= 0 ? "+" : ""}${value.toFixed(2)}${unit}`; }
function monthLabel(value: number) { return new Date(value).toLocaleDateString("en-US", { month: "short", year: "numeric", timeZone: "UTC" }); }
