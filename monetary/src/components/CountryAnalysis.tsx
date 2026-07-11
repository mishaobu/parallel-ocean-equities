import { useMemo, useState } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown, CircleAlert, ExternalLink, RotateCcw } from "lucide-react";
import { CartesianGrid, Legend, Line, LineChart, ReferenceArea, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { useRangeSelection, type RangeInteraction } from "../chartRange";
import { countryChartRows, countrySnapshots, sortCountrySnapshots, type CountryMetric, type CountrySnapshot, type CountrySort } from "../countryData";
import type { CountryPoint, CountrySeries } from "../types";

const palette = { red: "#b8493e", ink: "#17201b", blue: "#3975a7", green: "#347b57", gold: "#b2832e", violet: "#765997", cyan: "#31838a" };

const columns: Array<{ key: CountrySort; label: string; metric?: CountryMetric }> = [
  { key: "name", label: "Economy" }, { key: "regime", label: "Regime" },
  { key: "inflation", label: "Inflation", metric: "inflation" }, { key: "policyRate", label: "Policy / short", metric: "policyRate" },
  { key: "realRate", label: "Real rate", metric: "realRate" }, { key: "industrialGrowth", label: "Industry", metric: "industrialGrowth" },
  { key: "moneyGrowth", label: "Money", metric: "moneyGrowth" }, { key: "longRate", label: "Long rate", metric: "longRate" },
	{ key: "yieldCurve", label: "Curve", metric: "yieldCurve" }, { key: "asOf", label: "Common through" },
];

export function CountryAnalysis({ countries, domain, rangeSelected, onSelectDomain, onResetDomain }: { countries: CountrySeries[]; domain: [number, number] } & RangeInteraction) {
  const snapshots = useMemo(() => countrySnapshots(countries), [countries]);
  const [sortKey, setSortKey] = useState<CountrySort>("realRate");
  const [direction, setDirection] = useState<"asc" | "desc">("desc");
  const [selected, setSelected] = useState("US");
  const sorted = useMemo(() => sortCountrySnapshots(snapshots, sortKey, direction), [direction, snapshots, sortKey]);
  const active = snapshots.find((row) => row.country.code === selected) ?? snapshots[0];

  function sort(key: CountrySort) {
    if (key === sortKey) setDirection((current) => current === "asc" ? "desc" : "asc");
    else { setSortKey(key); setDirection(key === "name" || key === "regime" ? "asc" : "desc"); }
  }

  if (!active) return <div className="loading">Country observations refresh pending</div>;
  return <>
    <div className="section-title"><b>01</b><div><h2>Global monetary comparison</h2><span>Every column is sortable; metric dates are evaluated independently</span></div></div>
    <section className="country-matrix">
      <header><div><h2>Policy and transmission matrix</h2><span>{snapshots.length} economies / latest revised observations</span></div><small>Missing values sort last</small></header>
      <div className="table-wrap"><table>
        <thead><tr>{columns.map((column) => <th key={column.key}><button type="button" onClick={() => sort(column.key)} className={sortKey === column.key ? "is-sorted" : ""}>{column.label}{sortKey !== column.key ? <ArrowUpDown size={11} /> : direction === "asc" ? <ArrowUp size={11} /> : <ArrowDown size={11} />}</button></th>)}</tr></thead>
        <tbody>{sorted.map((row) => <CountryRow key={row.country.code} row={row} active={row.country.code === active.country.code} onSelect={() => setSelected(row.country.code)} />)}</tbody>
      </table></div>
    </section>

    <div className="country-selector" aria-label="Country detail">
      {snapshots.map((row) => <button key={row.country.code} type="button" className={row.country.code === active.country.code ? "is-active" : ""} onClick={() => setSelected(row.country.code)}><b>{row.country.code}</b><span>{row.country.name}</span></button>)}
    </div>

    <CountryHeader snapshot={active} />
    <section className="chart-grid country-charts">
		<CountryChart title="Inflation and policy" note={`${active.country.policyLabel} against headline and core inflation`} points={active.country.points ?? []} domain={domain} rangeSelected={rangeSelected} onSelectDomain={onSelectDomain} onResetDomain={onResetDomain} series={[
        { key: "inflation", label: "Headline inflation", color: palette.red }, { key: "coreInflation", label: "Core inflation", color: palette.gold }, { key: "policyRate", label: active.country.policyLabel, color: palette.ink }, { key: "realRate", label: "Ex-post real rate", color: palette.blue },
      ]} primary />
		<CountryChart title="Growth and money" note="Industrial momentum, broad-money growth and unemployment" points={active.country.points ?? []} domain={domain} rangeSelected={rangeSelected} onSelectDomain={onSelectDomain} onResetDomain={onResetDomain} series={[
        { key: "industrialGrowth", label: "Industrial production", color: palette.green }, { key: "moneyGrowth", label: "Broad money", color: palette.violet }, { key: "unemployment", label: "Unemployment", color: palette.red },
      ]} />
		<CountryChart title="Rates and currency" note={`${active.country.fxLabel}; FX uses the right axis`} points={active.country.points ?? []} domain={domain} rangeSelected={rangeSelected} onSelectDomain={onSelectDomain} onResetDomain={onResetDomain} series={[
        { key: "longRate", label: "Long government rate", color: palette.blue }, { key: "yieldCurve", label: "Long less policy", color: palette.cyan }, { key: "fx", label: active.country.fxLabel, color: palette.gold, axis: "right" },
      ]} wide />
    </section>
	<section className="country-basis"><div><strong>Series coverage</strong><span>{active.country.sources?.length ?? 0} source series / policy definitions differ by economy</span></div><a href={`/macro/?country=${active.country.code}&view=countries`}><span>Open in Macro</span><ExternalLink size={13} /></a></section>
    {!!active.country.warnings?.length && <div className="country-warning"><CircleAlert size={14} />{active.country.warnings.join(" / ")}</div>}
  </>;
}

function CountryRow({ row, active, onSelect }: { row: CountrySnapshot; active: boolean; onSelect: () => void }) {
  return <tr className={active ? "is-active" : ""}>
    <th><button type="button" onClick={onSelect}><b>{row.country.code}</b><span>{row.country.name}</span></button></th>
    <td className="regime-cell">{row.regime}</td>
    {(["inflation", "policyRate", "realRate", "industrialGrowth", "moneyGrowth", "longRate", "yieldCurve"] as CountryMetric[]).map((metric) => <td key={metric} className={(row.values[metric]?.ageMonths ?? 0) > 14 ? "is-stale" : ""} title={row.values[metric] ? `Observed ${formatDate(row.values[metric]!.date)}` : "Unavailable"}>{formatPercent(row.values[metric]?.value)}{row.values[metric] && row.values[metric]!.ageMonths > 14 ? <small>{formatDate(row.values[metric]!.date)}</small> : null}</td>)}
    <td>{formatDate(row.asOf)}</td>
  </tr>;
}

function CountryHeader({ snapshot }: { snapshot: CountrySnapshot }) {
  const fields: Array<[string, CountryMetric]> = [["Inflation", "inflation"], [snapshot.country.policyLabel, "policyRate"], ["Real policy", "realRate"], ["Industrial growth", "industrialGrowth"], ["Broad money", "moneyGrowth"], ["Long rate", "longRate"]];
  return <section className="country-detail-head"><div className="country-title"><span>{snapshot.country.region} / {snapshot.country.currency}</span><h2>{snapshot.country.name}</h2><p>{snapshot.regime}</p></div><div className="country-readings">{fields.map(([label, metric]) => {
    const reading = snapshot.values[metric];
    return <div key={metric} className={(reading?.ageMonths ?? 0) > 14 ? "is-stale" : ""}><span>{label}</span><strong>{formatPercent(reading?.value)}</strong><small>{reading ? `${formatDate(reading.date)}${reading.ageMonths > 14 ? " / stale" : ""}` : "Unavailable"}</small></div>;
  })}</div></section>;
}

type CountryChartMetric = Extract<keyof CountryPoint, CountryMetric>;
interface CountryChartSeries { key: CountryChartMetric; label: string; color: string; axis?: "left" | "right" }
function CountryChart({ title, note, points, domain, series, primary, wide, rangeSelected, onSelectDomain, onResetDomain }: { title: string; note: string; points: CountryPoint[]; domain: [number, number]; series: CountryChartSeries[]; primary?: boolean; wide?: boolean } & RangeInteraction) {
	const rows = countryChartRows(points, domain);
	const hasData = rows.some((row) => series.some((item) => typeof row[item.key] === "number"));
	const range = useRangeSelection(domain, onSelectDomain);
	return <article className={`chart-frame${primary ? " chart-primary" : ""}${wide ? " country-chart-wide" : ""}`}><header><div><h2>{title}</h2><span>{note}</span></div>{rangeSelected && <button type="button" className="icon-button" title="Reset selected period" aria-label="Reset selected period" onClick={onResetDomain}><RotateCcw size={13} /></button>}</header><div className="chart-canvas">
		{!hasData ? <div className="chart-empty">No observations in selected range</div> : <ResponsiveContainer width="100%" height="100%"><LineChart data={rows} margin={{ top: 15, right: 6, bottom: 3, left: 0 }} onMouseDown={range.start} onMouseMove={range.move} onMouseUp={range.finish} onMouseLeave={range.finish}>
      <CartesianGrid vertical={false} stroke="#e4e8e5" /><XAxis dataKey="timestamp" type="number" scale="time" domain={domain} tickFormatter={(value) => String(new Date(value).getUTCFullYear())} tick={{ fill: "#69746d", fontSize: 10 }} minTickGap={38} axisLine={false} tickLine={false} />
      <YAxis yAxisId="left" tickFormatter={(value) => `${Number(value).toFixed(Math.abs(Number(value)) < 10 ? 1 : 0)}%`} tick={{ fill: "#69746d", fontSize: 10 }} axisLine={false} tickLine={false} width={50} />
      {series.some((item) => item.axis === "right") && <YAxis yAxisId="right" orientation="right" tickFormatter={(value) => Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 }).format(Number(value))} tick={{ fill: "#69746d", fontSize: 10 }} axisLine={false} tickLine={false} width={48} />}
      <Tooltip itemSorter={(item) => -(Number(item.value) || 0)} labelFormatter={(value) => formatDate(new Date(Number(value)).toISOString())} formatter={(value, name) => [Number(value).toFixed(2), name]} contentStyle={{ border: "1px solid #cdd5cf", borderRadius: 4, fontSize: 11 }} />
			<Legend iconType="line" wrapperStyle={{ fontSize: 10 }} /><ReferenceLine yAxisId="left" y={0} stroke="#aeb7b1" />
			{range.selection && <ReferenceArea yAxisId="left" x1={range.selection[0]} x2={range.selection[1]} fill="#7a9b88" fillOpacity={0.16} strokeOpacity={0} />}
      {series.map((item) => <Line key={item.key} yAxisId={item.axis ?? "left"} type="monotone" dataKey={item.key} name={item.label} stroke={item.color} strokeWidth={2} dot={false} connectNulls isAnimationActive={false} />)}
    </LineChart></ResponsiveContainer>}
  </div></article>;
}

function formatPercent(value?: number) { return value === undefined ? "--" : `${value.toFixed(1)}%`; }
function formatDate(value?: string) { if (!value) return "--"; return new Date(value).toLocaleDateString("en-US", { month: "short", year: "numeric", timeZone: "UTC" }); }
