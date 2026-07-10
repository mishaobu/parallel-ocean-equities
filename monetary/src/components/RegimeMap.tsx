import { CartesianGrid, ReferenceArea, ReferenceLine, ResponsiveContainer, Scatter, ScatterChart, Tooltip, XAxis, YAxis, ZAxis } from "recharts";
import { chartRows } from "../macroData";
import type { MacroPoint } from "../types";

export function RegimeMap({ points, domain, selectedDate, onInspect, onPin }: { points: MacroPoint[]; domain: [number, number]; selectedDate?: number; onInspect?: (date?: number) => void; onPin?: (date: number) => void }) {
  const complete = chartRows(points, domain)
    .filter((point) => point.inflation !== undefined && point.industrialGrowth !== undefined)
    .map((point) => ({ ...point, label: dateLabel(point.timestamp) }));
  const observations = complete.filter((_, index) => index % 3 === 0);
  const recessions = observations.filter((point) => (point.recession ?? 0) >= 0.5);
  const expansions = observations.filter((point) => (point.recession ?? 0) < 0.5);
  const current = complete.length ? [{ ...complete[complete.length - 1], size: 90 }] : [];
  const trail = complete.slice(-24).map((point, index, rows) => ({ ...point, size: 20 + index / Math.max(1, rows.length - 1) * 34 }));
  const selected = selectedDate === undefined ? [] : complete.filter((point) => point.timestamp === selectedDate).map((point) => ({ ...point, size: 105 }));
  return <article className="chart-frame chart-primary regime-map">
    <header><div><h2>Growth / inflation regime</h2><span>Industrial production YoY against CPI YoY / 24M trail</span></div><i>{current[0]?.label ?? "Current unavailable"}</i></header>
    <div className="chart-canvas">
      {observations.length === 0 ? <div className="chart-empty">Regime data unavailable</div> : <ResponsiveContainer width="100%" height="100%">
        <ScatterChart margin={{ top: 15, right: 17, bottom: 10, left: 0 }}>
          <ReferenceArea x1={2.5} x2={20} y1={0} y2={20} fill="#f3ddd3" fillOpacity={0.28} />
          <ReferenceArea x1={-10} x2={2.5} y1={0} y2={20} fill="#d9eadf" fillOpacity={0.35} />
          <ReferenceArea x1={2.5} x2={20} y1={-20} y2={0} fill="#ead7d2" fillOpacity={0.5} />
          <ReferenceArea x1={-10} x2={2.5} y1={-20} y2={0} fill="#dde5e9" fillOpacity={0.35} />
          <CartesianGrid stroke="#e4e8e5" />
          <XAxis type="number" dataKey="inflation" name="Inflation" unit="%" tick={{ fill: "#69746d", fontSize: 10 }} axisLine={false} tickLine={false} />
          <YAxis type="number" dataKey="industrialGrowth" name="Industrial growth" unit="%" tick={{ fill: "#69746d", fontSize: 10 }} axisLine={false} tickLine={false} width={48} />
          <ZAxis type="number" dataKey="size" range={[20, 110]} />
          <ReferenceLine x={2.5} stroke="#b68d32" strokeDasharray="4 4" />
          <ReferenceLine y={0} stroke="#8e9992" />
          <Tooltip cursor={{ strokeDasharray: "3 3" }} formatter={(value) => `${Number(value).toFixed(2)}%`} labelFormatter={(_, payload) => payload[0]?.payload.label ?? ""} contentStyle={{ border: "1px solid #cdd5cf", borderRadius: 4, fontSize: 11 }} />
          <Scatter name="Expansion" data={expansions} fill="#377c5b" fillOpacity={0.42} />
          <Scatter name="NBER recession" data={recessions} fill="#bd5a4b" fillOpacity={0.62} />
          <Scatter name="24M path" data={trail} fill="#2d6f75" fillOpacity={0.55} line={{ stroke: "#2d6f75", strokeWidth: 1.5 }} lineType="joint" onMouseEnter={(point) => onInspect?.(point?.timestamp)} onClick={(point) => { if (typeof point?.timestamp === "number") onPin?.(point.timestamp); }} />
          <Scatter name="Current" data={current} fill="#17201b" shape="diamond" />
          <Scatter name="Selected" data={selected} fill="#bd4b3f" shape="diamond" />
        </ScatterChart>
      </ResponsiveContainer>}
    </div>
  </article>;
}

function dateLabel(value: number) { return new Date(value).toLocaleDateString("en-US", { year: "numeric", month: "short", timeZone: "UTC" }); }
