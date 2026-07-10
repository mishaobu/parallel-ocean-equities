import { CartesianGrid, Legend, Line, LineChart, ReferenceArea, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { chartRows, descendingTooltipItem, recessionIntervals, type MacroMetric } from "../macroData";
import type { MacroPoint } from "../types";

export interface SeriesSpec {
  key: MacroMetric;
  label: string;
  color: string;
  axis?: "left" | "right";
}

export function MacroChart({ title, note, points, domain, series, unit = "percent", primary = false }: {
  title: string;
  note: string;
  points: MacroPoint[];
  domain: [number, number];
  series: SeriesSpec[];
  unit?: "percent" | "index" | "log" | "billions";
  primary?: boolean;
}) {
  const rows = chartRows(points, domain);
  const recessions = recessionIntervals(points, domain);
  const hasRightAxis = series.some((item) => item.axis === "right");
  return <article className={`chart-frame${primary ? " chart-primary" : ""}`}>
    <header><div><h2>{title}</h2><span>{note}</span></div></header>
    <div className="chart-canvas">
      {rows.length === 0 ? <div className="chart-empty">Series unavailable</div> : <ResponsiveContainer width="100%" height="100%">
        <LineChart data={rows} margin={{ top: 15, right: hasRightAxis ? 5 : 20, bottom: 3, left: 0 }}>
          <CartesianGrid vertical={false} stroke="#e4e8e5" />
          <XAxis dataKey="timestamp" type="number" scale="time" domain={domain} tickFormatter={yearLabel} tick={{ fill: "#69746d", fontSize: 10 }} minTickGap={38} axisLine={false} tickLine={false} />
          <YAxis yAxisId="left" tickFormatter={(value) => axisLabel(Number(value), unit)} tick={{ fill: "#69746d", fontSize: 10 }} axisLine={false} tickLine={false} width={52} />
          {hasRightAxis && <YAxis yAxisId="right" orientation="right" tickFormatter={(value) => compact(Number(value))} tick={{ fill: "#69746d", fontSize: 10 }} axisLine={false} tickLine={false} width={42} />}
          <Tooltip itemSorter={descendingTooltipItem} labelFormatter={(value) => dateLabel(Number(value))} formatter={(value, name) => [tooltipValue(Number(value), unit), name]} contentStyle={{ border: "1px solid #cdd5cf", borderRadius: 4, fontSize: 11 }} />
          <Legend iconType="line" wrapperStyle={{ fontSize: 10 }} />
          {recessions.map((interval) => <ReferenceArea key={interval.start} yAxisId="left" x1={interval.start} x2={interval.end} fill="#c8ccc9" fillOpacity={0.28} strokeOpacity={0} />)}
          <ReferenceLine yAxisId="left" y={0} stroke="#aeb7b1" />
          {series.map((item) => <Line key={item.key} yAxisId={item.axis ?? "left"} type="monotone" dataKey={item.key} name={item.label} stroke={item.color} strokeWidth={2} dot={false} connectNulls isAnimationActive={false} />)}
        </LineChart>
      </ResponsiveContainer>}
    </div>
  </article>;
}

function axisLabel(value: number, unit: string) {
  if (unit === "percent") return `${value.toFixed(Math.abs(value) < 10 ? 1 : 0)}%`;
  if (unit === "billions") return `$${compact(value)}`;
  return compact(value);
}
function tooltipValue(value: number, unit: string) {
  if (unit === "percent") return `${value.toFixed(2)}%`;
  if (unit === "billions") return `$${value.toFixed(1)}B`;
  return value.toFixed(2);
}
function compact(value: number) { return Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 }).format(value); }
function yearLabel(value: number) { return new Date(value).getUTCFullYear().toString(); }
function dateLabel(value: number) { return new Date(value).toLocaleDateString("en-US", { year: "numeric", month: "short", timeZone: "UTC" }); }
