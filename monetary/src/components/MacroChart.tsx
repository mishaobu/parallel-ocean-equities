import { useState } from "react";
import { CartesianGrid, Legend, Line, LineChart, ReferenceArea, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { chartRows, descendingTooltipItem, recessionIntervals, transformRows, type ChartTransform, type MacroMetric } from "../macroData";
import type { MacroPoint } from "../types";

export interface SeriesSpec {
  key: MacroMetric;
  label: string;
  color: string;
  axis?: "left" | "right";
}

export function MacroChart({ title, note, points, domain, series, unit = "percent", primary = false, selectedDate, onInspect, onPin }: {
  title: string;
  note: string;
  points: MacroPoint[];
  domain: [number, number];
  series: SeriesSpec[];
  unit?: "percent" | "index" | "log" | "billions";
  primary?: boolean;
  selectedDate?: number;
  onInspect?: (date?: number) => void;
  onPin?: (date: number) => void;
}) {
  const [transform, setTransform] = useState<ChartTransform>("native");
  const [hidden, setHidden] = useState<Set<string>>(() => new Set());
  const nativeRows = chartRows(points, domain);
  const rows = transformRows(nativeRows, series.map((item) => item.key), transform);
  const recessions = recessionIntervals(points, domain);
  const hasRightAxis = series.some((item) => item.axis === "right");
  const displayUnit = transform === "zscore" ? "index" : transform === "percentile" ? "percent" : unit;
  function toggleSeries(event: unknown) {
    if (!event || typeof event !== "object" || !("dataKey" in event)) return;
    const key = String((event as { dataKey?: unknown }).dataKey ?? "");
    if (!key) return;
    setHidden((current) => { const next = new Set(current); if (next.has(key)) next.delete(key); else next.add(key); return next; });
  }
  return <article className={`chart-frame${primary ? " chart-primary" : ""}`}>
    <header><div><h2>{title}</h2><span>{note}{transform === "change3m" ? " / 3M change" : transform === "zscore" ? " / z-score" : transform === "percentile" ? " / percentile" : ""}</span></div>
      <div className="chart-transform" aria-label={`${title} transformation`}>
        {(["native", "change3m", "zscore", "percentile"] as ChartTransform[]).map((value) => <button type="button" key={value} className={transform === value ? "is-active" : ""} onClick={() => setTransform(value)} title={transformLabel(value)}>{value === "native" ? "N" : value === "change3m" ? "3M" : value === "zscore" ? "Z" : "%"}</button>)}
      </div>
    </header>
    <div className="chart-canvas">
      {rows.length === 0 ? <div className="chart-empty">Series unavailable</div> : <ResponsiveContainer width="100%" height="100%">
        <LineChart data={rows} margin={{ top: 15, right: hasRightAxis ? 5 : 20, bottom: 3, left: 0 }} onMouseMove={(event) => onInspect?.(eventDate(event))} onMouseLeave={() => onInspect?.()} onClick={(event) => { const date = eventDate(event); if (date !== undefined) onPin?.(date); }}>
          <CartesianGrid vertical={false} stroke="#e4e8e5" />
          <XAxis dataKey="timestamp" type="number" scale="time" domain={domain} tickFormatter={yearLabel} tick={{ fill: "#69746d", fontSize: 10 }} minTickGap={38} axisLine={false} tickLine={false} />
          <YAxis yAxisId="left" tickFormatter={(value) => axisLabel(Number(value), displayUnit)} tick={{ fill: "#69746d", fontSize: 10 }} axisLine={false} tickLine={false} width={52} />
          {hasRightAxis && <YAxis yAxisId="right" orientation="right" tickFormatter={(value) => axisLabel(Number(value), displayUnit)} tick={{ fill: "#69746d", fontSize: 10 }} axisLine={false} tickLine={false} width={48} />}
          <Tooltip itemSorter={descendingTooltipItem} labelFormatter={(value) => dateLabel(Number(value))} formatter={(value, name) => [tooltipValue(Number(value), displayUnit), name]} contentStyle={{ border: "1px solid #cdd5cf", borderRadius: 4, fontSize: 11 }} />
          <Legend iconType="line" wrapperStyle={{ fontSize: 10, cursor: "pointer" }} onClick={toggleSeries} />
          {recessions.map((interval) => <ReferenceArea key={interval.start} yAxisId="left" x1={interval.start} x2={interval.end} fill="#c8ccc9" fillOpacity={0.28} strokeOpacity={0} />)}
          <ReferenceLine yAxisId="left" y={0} stroke="#aeb7b1" />
          {selectedDate !== undefined && selectedDate >= domain[0] && selectedDate <= domain[1] && <ReferenceLine yAxisId="left" x={selectedDate} stroke="#17201b" strokeOpacity={0.45} />}
          {series.map((item) => <Line key={item.key} yAxisId={item.axis ?? "left"} type="monotone" dataKey={item.key} name={item.label} stroke={item.color} strokeWidth={2} dot={false} connectNulls hide={hidden.has(item.key)} isAnimationActive={false} />)}
        </LineChart>
      </ResponsiveContainer>}
    </div>
  </article>;
}

function eventDate(event: unknown) {
  if (!event || typeof event !== "object" || !("activeLabel" in event)) return undefined;
  const value = Number((event as { activeLabel?: unknown }).activeLabel);
  return Number.isFinite(value) ? value : undefined;
}

function transformLabel(value: ChartTransform) {
  if (value === "native") return "Native values";
  if (value === "change3m") return "Three-month change";
  if (value === "zscore") return "Z-score over selected range";
  return "Percentile over selected range";
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
