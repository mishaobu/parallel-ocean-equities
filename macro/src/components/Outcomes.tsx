import { useMemo, useState } from "react";
import { ArrowDown, ArrowUp } from "lucide-react";
import { Bar, BarChart, CartesianGrid, Cell, ErrorBar, Legend, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { regimes, type ForwardHorizon, type OutcomeStat, type Regime } from "../outcomes";
import { PanelHeader } from "./Charts";

type SortKey = "symbol" | "group" | "count" | "average" | "median" | "positiveRate" | "medianDrawdown" | "worstDrawdown";
const columns: Array<[SortKey, string]> = [["symbol", "Asset"], ["group", "Sleeve"], ["count", "Samples"], ["average", "Average"], ["median", "Median"], ["positiveRate", "Positive"], ["medianDrawdown", "Median drawdown"], ["worstDrawdown", "Worst drawdown"]];

export function OutcomesLab({ stats, current }: { stats: OutcomeStat[]; current?: Regime }) {
  const [horizon, setHorizon] = useState<ForwardHorizon>(12);
  const [regimeChoice, setRegimeChoice] = useState<Regime | "current">("current");
  const [asset, setAsset] = useState("SPY");
  const [sortKey, setSortKey] = useState<SortKey>("median");
  const [direction, setDirection] = useState<"asc" | "desc">("desc");
  const activeRegime = regimeChoice === "current" ? current ?? regimes[0] : regimeChoice;
  const rows = useMemo(() => stats.filter((row) => row.horizon === horizon && row.regime === activeRegime), [activeRegime, horizon, stats]);
  const sorted = useMemo(() => sortRows(rows, sortKey, direction), [direction, rows, sortKey]);
  const symbols = useMemo(() => [...new Set(stats.map((row) => row.symbol))].sort(), [stats]);
  const activeAsset = symbols.includes(asset) ? asset : symbols[0];
  const comparison = useMemo(() => regimes.flatMap((regime) => stats.find((row) => row.symbol === activeAsset && row.horizon === horizon && row.regime === regime) ?? []), [activeAsset, horizon, stats]);
  const best = [...rows].sort((a, b) => b.median - a.median)[0];
  const sampleCount = rows.reduce((sum, row) => sum + row.count, 0);
  const positive = rows.filter((row) => row.median > 0).length;

  function sort(next: SortKey) {
    if (next === sortKey) setDirection((value) => value === "asc" ? "desc" : "asc");
    else { setSortKey(next); setDirection(next === "symbol" || next === "group" ? "asc" : "desc"); }
  }

  return <>
    <section className="outcome-toolbar">
      <label>Regime<select aria-label="Outcome regime" value={regimeChoice} onChange={(event) => setRegimeChoice(event.target.value as Regime | "current")}><option value="current">Current / {current ?? "pending"}</option>{regimes.map((regime) => <option key={regime} value={regime}>{regime}</option>)}</select></label>
      <div><span>Forward window</span><div className="segmented-control">{([3, 6, 12] as ForwardHorizon[]).map((value) => <button type="button" key={value} className={horizon === value ? "is-active" : ""} onClick={() => setHorizon(value)}>{value}M</button>)}</div></div>
      <label>Regime detail<select aria-label="Outcome asset" value={activeAsset} onChange={(event) => setAsset(event.target.value)}>{symbols.map((symbol) => <option key={symbol}>{symbol}</option>)}</select></label>
    </section>
    <section className="signal-strip outcome-signals"><div><span>Selected regime</span><strong>{shortRegime(activeRegime)}</strong><small>US macro / 2m availability lag</small></div><div><span>Qualified samples</span><strong>{sampleCount}</strong><small>Quarterly-spaced starts</small></div><div><span>Positive median sleeves</span><strong>{positive}/{rows.length}</strong><small>{horizon}-month outcomes</small></div><div><span>Highest median</span><strong>{best?.symbol ?? "--"}</strong><small>{best ? percent(best.median) : "No observations"}</small></div></section>
    <section className="outcome-grid"><OutcomeChart rows={rows} horizon={horizon} /><RegimeChart rows={comparison} symbol={activeAsset} horizon={horizon} /></section>
    <article className="panel outcome-table"><PanelHeader title="Forward outcome matrix" note="Every column is sortable / confidence interval applies to the mean" /><div className="table-scroll"><table><thead><tr>{columns.map(([key, label]) => <th key={key}><button type="button" className={sortKey === key ? "is-sorted" : ""} onClick={() => sort(key)}>{label}{sortKey === key && (direction === "asc" ? <ArrowUp size={11} /> : <ArrowDown size={11} />)}</button></th>)}</tr></thead><tbody>{sorted.map((row) => <tr key={row.symbol}><th><b>{row.symbol}</b><span>{row.label}</span></th><td>{row.group}</td><td>{row.count}<small>{year(row.startDate)}-{year(row.endDate)}</small></td><td className={tone(row.average)}>{percent(row.average)}<small>{percent(row.ciLow)} to {percent(row.ciHigh)}</small></td><td className={tone(row.median)}>{percent(row.median)}</td><td>{unsignedPercent(row.positiveRate * 100)}</td><td className="negative">{percent(row.medianDrawdown)}</td><td className="negative">{percent(row.worstDrawdown)}</td></tr>)}</tbody></table></div></article>
    <div className="method-note"><strong>Evidence boundary</strong><span>Forward windows use quarterly-spaced starts and macro observations available two months before each start. Macro histories are latest-revised, not ALFRED vintages; confidence bands measure sampling uncertainty, not forecast uncertainty.</span></div>
  </>;
}

function OutcomeChart({ rows, horizon }: { rows: OutcomeStat[]; horizon: ForwardHorizon }) {
  const data = [...rows].sort((a, b) => b.average - a.average).map((row) => ({ ...row, ci: [Math.max(0, row.average - row.ciLow), Math.max(0, row.ciHigh - row.average)] }));
  return <article className="panel chart-panel outcome-chart"><PanelHeader title="Average forward return" note={`${horizon}-month return / whiskers show 95% mean confidence interval`} /><div className="chart-body">{!data.length ? <div className="chart-empty">No qualified observations in selected history</div> : <ResponsiveContainer width="100%" height="100%"><BarChart data={data} layout="vertical" margin={{ top: 8, right: 32, bottom: 3, left: 8 }}><CartesianGrid horizontal={false} stroke="#e2e7e3" /><XAxis type="number" unit="%" tick={{ fill: "#68746d", fontSize: 10 }} axisLine={false} tickLine={false} /><YAxis type="category" dataKey="symbol" width={42} tick={{ fill: "#34473c", fontSize: 10, fontWeight: 700 }} axisLine={false} tickLine={false} /><ReferenceLine x={0} stroke="#8f9c93" /><Tooltip formatter={(value, name) => [percent(Number(value)), name]} contentStyle={{ border: "1px solid #c9d2cb", borderRadius: 3, fontSize: 11 }} /><Bar dataKey="average" name="Average return" isAnimationActive={false}>{data.map((row) => <Cell key={row.symbol} fill={row.average >= 0 ? "#347b57" : "#b8493e"} />)}<ErrorBar dataKey="ci" direction="x" width={4} stroke="#263c30" /></Bar></BarChart></ResponsiveContainer>}</div></article>;
}

function RegimeChart({ rows, symbol, horizon }: { rows: OutcomeStat[]; symbol: string; horizon: ForwardHorizon }) {
  return <article className="panel chart-panel outcome-chart"><PanelHeader title={`${symbol} across regimes`} note={`${horizon}-month average and median forward returns`} /><div className="chart-body">{!rows.length ? <div className="chart-empty">No comparable regime observations</div> : <ResponsiveContainer width="100%" height="100%"><BarChart data={rows} margin={{ top: 14, right: 8, bottom: 24, left: 0 }}><CartesianGrid vertical={false} stroke="#e2e7e3" /><XAxis dataKey="regime" tickFormatter={shortRegime} interval={0} tick={{ fill: "#68746d", fontSize: 8 }} axisLine={false} tickLine={false} /><YAxis unit="%" tick={{ fill: "#68746d", fontSize: 10 }} axisLine={false} tickLine={false} width={48} /><ReferenceLine y={0} stroke="#8f9c93" /><Tooltip formatter={(value, name) => [percent(Number(value)), name]} labelFormatter={shortRegime} contentStyle={{ border: "1px solid #c9d2cb", borderRadius: 3, fontSize: 11 }} /><Legend iconType="square" wrapperStyle={{ fontSize: 9 }} /><Bar dataKey="average" name="Average" fill="#3975a7" isAnimationActive={false} /><Bar dataKey="median" name="Median" fill="#347b57" isAnimationActive={false} /></BarChart></ResponsiveContainer>}</div></article>;
}

function sortRows(rows: OutcomeStat[], key: SortKey, direction: "asc" | "desc") {
  const sign = direction === "asc" ? 1 : -1;
  return [...rows].sort((left, right) => typeof left[key] === "number" && typeof right[key] === "number" ? sign * (Number(left[key]) - Number(right[key])) : sign * String(left[key]).localeCompare(String(right[key])));
}
function shortRegime(value: string) { return value.replace("Disinflationary ", "Disinfl. ").replace("Inflationary ", "Infl. "); }
function percent(value: number) { return `${value > 0 ? "+" : ""}${value.toFixed(1)}%`; }
function unsignedPercent(value: number) { return `${value.toFixed(1)}%`; }
function tone(value: number) { return value >= 0 ? "positive" : "negative"; }
function year(value: string) { return value.slice(0, 4); }
