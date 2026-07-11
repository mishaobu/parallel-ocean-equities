import { useMemo, useState } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { sortSnapshots, type CountryMetric, type MatrixSort, type Snapshot } from "../data";

const columns: Array<{ key: MatrixSort; label: string; metric?: CountryMetric }> = [
  { key: "name", label: "Economy" }, { key: "regime", label: "Regime" }, { key: "inflation", label: "Inflation", metric: "inflation" },
  { key: "policyRate", label: "Policy / short", metric: "policyRate" }, { key: "realRate", label: "Real", metric: "realRate" },
  { key: "industrialGrowth", label: "Industry", metric: "industrialGrowth" }, { key: "moneyGrowth", label: "Money", metric: "moneyGrowth" },
	{ key: "longRate", label: "Long", metric: "longRate" }, { key: "yieldCurve", label: "Curve", metric: "yieldCurve" }, { key: "asOf", label: "Common through" },
];

export function CountryMatrix({ rows, selected, onSelect, compact = false }: { rows: Snapshot[]; selected?: string; onSelect?: (code: string) => void; compact?: boolean }) {
  const [sortKey, setSortKey] = useState<MatrixSort>("realRate");
  const [direction, setDirection] = useState<"asc" | "desc">("desc");
  const sorted = useMemo(() => sortSnapshots(rows, sortKey, direction), [direction, rows, sortKey]);
  function changeSort(key: MatrixSort) {
    if (key === sortKey) setDirection((value) => value === "asc" ? "desc" : "asc");
    else { setSortKey(key); setDirection(key === "name" || key === "regime" ? "asc" : "desc"); }
  }
  return <article className={`panel matrix-panel${compact ? " matrix-compact" : ""}`}><header className="panel-head"><div><h2>Global regime matrix</h2><span>Sortable columns / independent metric dates</span></div><small>Stale observations are amber</small></header><div className="table-scroll"><table>
    <thead><tr>{columns.map((column) => <th key={column.key}><button type="button" onClick={() => changeSort(column.key)} className={sortKey === column.key ? "is-sorted" : ""}>{column.label}{sortKey !== column.key ? <ArrowUpDown size={11} /> : direction === "asc" ? <ArrowUp size={11} /> : <ArrowDown size={11} />}</button></th>)}</tr></thead>
    <tbody>{sorted.map((row) => <tr key={row.country.code} className={selected === row.country.code ? "is-active" : ""}><th><button type="button" onClick={() => onSelect?.(row.country.code)}><b>{row.country.code}</b><span>{row.country.name}</span></button></th><td className="regime">{row.regime}</td>
      {(["inflation", "policyRate", "realRate", "industrialGrowth", "moneyGrowth", "longRate", "yieldCurve"] as CountryMetric[]).map((metric) => <td key={metric} className={(row.values[metric]?.ageMonths ?? 0) > 14 ? "is-stale" : ""} title={row.values[metric] ? `Observed ${formatDate(row.values[metric]!.date)}` : "Unavailable"}>{percent(row.values[metric]?.value)}{(row.values[metric]?.ageMonths ?? 0) > 14 && <small>{formatDate(row.values[metric]?.date)}</small>}</td>)}
      <td>{formatDate(row.asOf)}</td></tr>)}</tbody>
  </table></div></article>;
}

export function CountryRanks({ rows }: { rows: Snapshot[] }) {
  const rankings: Array<[string, CountryMetric, "asc" | "desc"]> = [["Most restrictive", "realRate", "desc"], ["Strongest industry", "industrialGrowth", "desc"], ["Fastest money", "moneyGrowth", "desc"], ["Steepest curve", "yieldCurve", "desc"]];
  return <section className="rank-strip">{rankings.map(([label, metric, direction]) => {
    const row = sortSnapshots(rows.filter((candidate) => (candidate.values[metric]?.ageMonths ?? Infinity) <= 14), metric, direction)[0];
    return <div key={metric}><span>{label}</span><strong>{row?.country.code ?? "--"}</strong><small>{row ? `${percent(row.values[metric]?.value)} / ${formatDate(row.values[metric]?.date)}` : "Unavailable"}</small></div>;
  })}</section>;
}

function percent(value?: number) { return value === undefined ? "--" : `${value.toFixed(1)}%`; }
function formatDate(value?: string) { return value ? new Date(value).toLocaleDateString("en-US", { month: "short", year: "numeric", timeZone: "UTC" }) : "--"; }
