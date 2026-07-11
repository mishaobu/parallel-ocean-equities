import { useMemo, useRef, useState } from "react";
import { RotateCcw } from "lucide-react";
import { Bar, BarChart, CartesianGrid, Cell, LabelList, Legend, Line, LineChart, ReferenceArea, ReferenceLine, ResponsiveContainer, Scatter, ScatterChart, Tooltip, XAxis, YAxis, ZAxis } from "recharts";
import type { Snapshot } from "../data";

export interface LineSpec { key: string; label: string; color: string; axis?: "left" | "right" }
export interface RangeInteraction { rangeSelected?: boolean; onSelectDomain?: (domain: [number, number]) => void; onResetDomain?: () => void }
export function SeriesChart({ title, note, rows, domain, series, unit = "percent", primary = false, rangeSelected, onSelectDomain, onResetDomain }: { title: string; note: string; rows: Array<Record<string, string | number>>; domain: [number, number]; series: LineSpec[]; unit?: "percent" | "index"; primary?: boolean } & RangeInteraction) {
	const [hidden, setHidden] = useState<Set<string>>(() => new Set());
	const [selection, setSelection] = useState<[number, number]>();
	const selectionRef = useRef<[number, number]>();
	const activeDomain = domain;
  const hasRight = series.some((item) => item.axis === "right");
  const hasData = rows.some((row) => series.some((item) => typeof row[item.key] === "number"));
  const visible = series.filter((item) => !hidden.has(item.key));
  const leftDomain = useMemo(() => fittedYDomain(rows, activeDomain, visible.filter((item) => item.axis !== "right").map((item) => item.key)), [activeDomain, rows, visible]);
  const rightDomain = useMemo(() => fittedYDomain(rows, activeDomain, visible.filter((item) => item.axis === "right").map((item) => item.key)), [activeDomain, rows, visible]);

  function startRegion(event: ChartPointer) { const value = eventTimestamp(event); if (value !== undefined) { selectionRef.current = [value, value]; setSelection(selectionRef.current); } }
  function moveRegion(event: ChartPointer) { const value = eventTimestamp(event); if (value !== undefined && selectionRef.current) { selectionRef.current = [selectionRef.current[0], value]; setSelection(selectionRef.current); } }
  function finishRegion() {
    const current = selectionRef.current;
    if (!current) return;
    const next: [number, number] = current[0] <= current[1] ? current : [current[1], current[0]];
		if (next[1] - next[0] >= 20 * 24 * 60 * 60 * 1000) onSelectDomain?.([Math.max(domain[0], next[0]), Math.min(domain[1], next[1])]);
    selectionRef.current = undefined;
    setSelection(undefined);
  }

	const aside = <div className="chart-region-control"><span>{rangeSelected ? `${shortDate(domain[0])} - ${shortDate(domain[1])}` : "Drag plot to fit period"}</span>{rangeSelected && <button type="button" className="icon-button" title="Reset selected period" onClick={onResetDomain}><RotateCcw size={13} /></button>}</div>;
  return <article className={`panel chart-panel series-chart-panel${primary ? " chart-primary" : ""}`}><PanelHeader title={title} note={note} aside={aside} /><div className="chart-body">
    {!hasData ? <div className="chart-empty">Series unavailable for selected range</div> : <ResponsiveContainer width="100%" height="100%"><LineChart data={rows} margin={{ top: 16, right: 6, bottom: 3, left: 0 }} onMouseDown={startRegion} onMouseMove={moveRegion} onMouseUp={finishRegion} onMouseLeave={finishRegion}>
      <CartesianGrid vertical={false} stroke="#e2e7e3" />
      <XAxis dataKey="timestamp" type="number" scale="time" domain={activeDomain} allowDataOverflow tickFormatter={(value) => String(new Date(value).getUTCFullYear())} tick={{ fill: "#68746d", fontSize: 10 }} axisLine={false} tickLine={false} minTickGap={42} />
      <YAxis yAxisId="left" domain={leftDomain} allowDataOverflow tickFormatter={(value) => axisValue(Number(value), unit)} tick={{ fill: "#68746d", fontSize: 10 }} axisLine={false} tickLine={false} width={50} />
      {hasRight && <YAxis yAxisId="right" orientation="right" domain={rightDomain} allowDataOverflow tickFormatter={(value) => axisValue(Number(value), unit)} tick={{ fill: "#68746d", fontSize: 10 }} axisLine={false} tickLine={false} width={48} />}
      <Tooltip itemSorter={(item) => -(Number(item.value) || 0)} labelFormatter={(value) => dateLabel(Number(value))} formatter={(value, name) => [tooltipValue(Number(value), unit), name]} contentStyle={{ border: "1px solid #c9d2cb", borderRadius: 3, fontSize: 11 }} />
      <Legend iconType="line" wrapperStyle={{ fontSize: 10, cursor: "pointer" }} onClick={(event) => { const key = String(event.dataKey ?? ""); if (!key) return; setHidden((current) => { const next = new Set(current); if (next.has(key)) next.delete(key); else next.add(key); return next; }); }} />
      {unit === "percent" && <ReferenceLine yAxisId="left" y={0} stroke="#aab5ad" />}
      {selection && <ReferenceArea yAxisId="left" x1={selection[0]} x2={selection[1]} fill="#7a9b88" fillOpacity={0.16} strokeOpacity={0} />}
      {series.map((item) => <Line key={item.key} yAxisId={item.axis ?? "left"} type="monotone" dataKey={item.key} name={item.label} stroke={item.color} strokeWidth={2} dot={false} connectNulls hide={hidden.has(item.key)} isAnimationActive={false} />)}
    </LineChart></ResponsiveContainer>}
  </div></article>;
}

export function DivergenceMap({ rows }: { rows: Snapshot[] }) {
  const points = rows.flatMap((row) => row.values.realRate && row.values.industrialGrowth && row.values.realRate.ageMonths <= 14 && row.values.industrialGrowth.ageMonths <= 14 ? [{ code: row.country.code, name: row.country.name, realRate: row.values.realRate.value, growth: row.values.industrialGrowth.value, inflation: row.values.inflation?.value ?? 0 }] : []);
  return <article className="panel chart-panel"><PanelHeader title="Policy-growth map" note="Ex-post real rate vs industrial production growth" /><div className="chart-body">
    {points.length === 0 ? <div className="chart-empty">Comparable observations unavailable</div> : <ResponsiveContainer width="100%" height="100%"><ScatterChart margin={{ top: 17, right: 18, bottom: 10, left: 0 }}>
      <CartesianGrid stroke="#e2e7e3" /><XAxis type="number" dataKey="realRate" name="Real rate" unit="%" tick={{ fill: "#68746d", fontSize: 10 }} /><YAxis type="number" dataKey="growth" name="Industrial growth" unit="%" tick={{ fill: "#68746d", fontSize: 10 }} width={48} /><ZAxis type="number" dataKey="inflation" range={[90, 270]} />
      <ReferenceLine x={0} stroke="#aab5ad" /><ReferenceLine y={0} stroke="#aab5ad" /><Tooltip cursor={{ strokeDasharray: "3 3" }} formatter={(value, name) => [`${Number(value).toFixed(2)}%`, name]} labelFormatter={(_, payload) => payload?.[0]?.payload?.name ?? ""} contentStyle={{ border: "1px solid #c9d2cb", borderRadius: 3, fontSize: 11 }} />
      <Scatter data={points} fill="#347b57" isAnimationActive={false}>{points.map((point) => <Cell key={point.code} fill={point.realRate >= 0 ? "#3975a7" : "#b8493e"} />)}<LabelList dataKey="code" position="top" offset={7} fill="#263c30" fontSize={10} fontWeight={700} /></Scatter>
    </ScatterChart></ResponsiveContainer>}
  </div></article>;
}

export function ImpactChart({ rows, note = "Directional sensitivity score; percentage points are not forecast returns", valueLabel = "Sensitivity score" }: { rows: Array<{ symbol: string; impact: number }>; note?: string; valueLabel?: string }) {
  return <article className="panel chart-panel impact-panel"><PanelHeader title="Estimated relative impact" note={note} /><div className="chart-body">{!rows.length ? <div className="chart-empty">Insufficient observations for selected range</div> : <ResponsiveContainer width="100%" height="100%"><BarChart data={rows} layout="vertical" margin={{ top: 8, right: 24, bottom: 4, left: 7 }}>
    <CartesianGrid horizontal={false} stroke="#e2e7e3" /><XAxis type="number" tick={{ fill: "#68746d", fontSize: 10 }} axisLine={false} tickLine={false} /><YAxis type="category" dataKey="symbol" tick={{ fill: "#34473c", fontSize: 10, fontWeight: 700 }} axisLine={false} tickLine={false} width={40} />
    <ReferenceLine x={0} stroke="#8f9c93" /><Tooltip formatter={(value) => [Number(value).toFixed(2), valueLabel]} contentStyle={{ border: "1px solid #c9d2cb", borderRadius: 3, fontSize: 11 }} /><Bar dataKey="impact" radius={0} isAnimationActive={false}>{rows.map((row) => <Cell key={row.symbol} fill={row.impact >= 0 ? "#347b57" : "#b8493e"} />)}</Bar>
  </BarChart></ResponsiveContainer>}</div></article>;
}

export function PanelHeader({ title, note, aside }: { title: string; note: string; aside?: React.ReactNode }) { return <header className="panel-head"><div><h2>{title}</h2><span>{note}</span></div>{aside}</header>; }
export function fittedYDomain(rows: Array<Record<string, string | number>>, domain: [number, number], keys: string[]): [number, number] | ["auto", "auto"] {
  const values = rows.flatMap((row) => {
    const timestamp = Number(row.timestamp);
    if (!Number.isFinite(timestamp) || timestamp < domain[0] || timestamp > domain[1]) return [];
    return keys.flatMap((key) => typeof row[key] === "number" && Number.isFinite(row[key]) ? [Number(row[key])] : []);
  });
  if (!values.length) return ["auto", "auto"];
  const low = Math.min(...values); const high = Math.max(...values);
  const padding = high === low ? Math.max(Math.abs(high) * .05, .5) : (high - low) * .08;
  return [low - padding, high + padding];
}
interface ChartPointer { activeLabel?: string | number }
function eventTimestamp(event: ChartPointer) { const value = Number(event?.activeLabel); return Number.isFinite(value) ? value : undefined; }
function axisValue(value: number, unit: string) { return unit === "percent" ? `${value.toFixed(Math.abs(value) < 10 ? 1 : 0)}%` : Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 }).format(value); }
function tooltipValue(value: number, unit: string) { return unit === "percent" ? `${value.toFixed(2)}%` : value.toFixed(1); }
function dateLabel(value: number) { return new Date(value).toLocaleDateString("en-US", { month: "short", year: "numeric", timeZone: "UTC" }); }
function shortDate(value: number) { return new Date(value).toLocaleDateString("en-US", { month: "short", year: "2-digit", timeZone: "UTC" }); }
