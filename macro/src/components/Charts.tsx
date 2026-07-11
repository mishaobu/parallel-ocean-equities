import { useState } from "react";
import { Bar, BarChart, CartesianGrid, Cell, Legend, Line, LineChart, ReferenceLine, ResponsiveContainer, Scatter, ScatterChart, Tooltip, XAxis, YAxis, ZAxis } from "recharts";
import type { Snapshot } from "../data";

export interface LineSpec { key: string; label: string; color: string; axis?: "left" | "right" }
export function SeriesChart({ title, note, rows, domain, series, unit = "percent", primary = false }: { title: string; note: string; rows: Array<Record<string, string | number>>; domain: [number, number]; series: LineSpec[]; unit?: "percent" | "index"; primary?: boolean }) {
  const [hidden, setHidden] = useState<Set<string>>(() => new Set());
  const hasRight = series.some((item) => item.axis === "right");
  const hasData = rows.some((row) => series.some((item) => typeof row[item.key] === "number"));
  return <article className={`panel chart-panel${primary ? " chart-primary" : ""}`}><PanelHeader title={title} note={note} /><div className="chart-body">
    {!hasData ? <div className="chart-empty">Series unavailable for selected range</div> : <ResponsiveContainer width="100%" height="100%"><LineChart data={rows} margin={{ top: 16, right: 6, bottom: 3, left: 0 }}>
      <CartesianGrid vertical={false} stroke="#e2e7e3" />
      <XAxis dataKey="timestamp" type="number" scale="time" domain={domain} tickFormatter={(value) => String(new Date(value).getUTCFullYear())} tick={{ fill: "#68746d", fontSize: 10 }} axisLine={false} tickLine={false} minTickGap={42} />
      <YAxis yAxisId="left" tickFormatter={(value) => axisValue(Number(value), unit)} tick={{ fill: "#68746d", fontSize: 10 }} axisLine={false} tickLine={false} width={50} />
      {hasRight && <YAxis yAxisId="right" orientation="right" tickFormatter={(value) => axisValue(Number(value), unit)} tick={{ fill: "#68746d", fontSize: 10 }} axisLine={false} tickLine={false} width={48} />}
      <Tooltip itemSorter={(item) => -(Number(item.value) || 0)} labelFormatter={(value) => dateLabel(Number(value))} formatter={(value, name) => [tooltipValue(Number(value), unit), name]} contentStyle={{ border: "1px solid #c9d2cb", borderRadius: 3, fontSize: 11 }} />
      <Legend iconType="line" wrapperStyle={{ fontSize: 10, cursor: "pointer" }} onClick={(event) => { const key = String(event.dataKey ?? ""); if (!key) return; setHidden((current) => { const next = new Set(current); if (next.has(key)) next.delete(key); else next.add(key); return next; }); }} />
      {unit === "percent" && <ReferenceLine yAxisId="left" y={0} stroke="#aab5ad" />}
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
      <Scatter data={points} fill="#347b57" isAnimationActive={false}>{points.map((point) => <Cell key={point.code} fill={point.realRate >= 0 ? "#3975a7" : "#b8493e"} />)}</Scatter>
    </ScatterChart></ResponsiveContainer>}
  </div></article>;
}

export function ImpactChart({ rows }: { rows: Array<{ symbol: string; impact: number }> }) {
  return <article className="panel chart-panel impact-panel"><PanelHeader title="Estimated relative impact" note="Directional sensitivity score; percentage points are not forecast returns" /><div className="chart-body"><ResponsiveContainer width="100%" height="100%"><BarChart data={rows} layout="vertical" margin={{ top: 8, right: 24, bottom: 4, left: 7 }}>
    <CartesianGrid horizontal={false} stroke="#e2e7e3" /><XAxis type="number" tick={{ fill: "#68746d", fontSize: 10 }} axisLine={false} tickLine={false} /><YAxis type="category" dataKey="symbol" tick={{ fill: "#34473c", fontSize: 10, fontWeight: 700 }} axisLine={false} tickLine={false} width={40} />
    <ReferenceLine x={0} stroke="#8f9c93" /><Tooltip formatter={(value) => [Number(value).toFixed(2), "Sensitivity score"]} contentStyle={{ border: "1px solid #c9d2cb", borderRadius: 3, fontSize: 11 }} /><Bar dataKey="impact" radius={0} isAnimationActive={false}>{rows.map((row) => <Cell key={row.symbol} fill={row.impact >= 0 ? "#347b57" : "#b8493e"} />)}</Bar>
  </BarChart></ResponsiveContainer></div></article>;
}

export function PanelHeader({ title, note, aside }: { title: string; note: string; aside?: React.ReactNode }) { return <header className="panel-head"><div><h2>{title}</h2><span>{note}</span></div>{aside}</header>; }
function axisValue(value: number, unit: string) { return unit === "percent" ? `${value.toFixed(Math.abs(value) < 10 ? 1 : 0)}%` : Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 }).format(value); }
function tooltipValue(value: number, unit: string) { return unit === "percent" ? `${value.toFixed(2)}%` : value.toFixed(1); }
function dateLabel(value: number) { return new Date(value).toLocaleDateString("en-US", { month: "short", year: "numeric", timeZone: "UTC" }); }
