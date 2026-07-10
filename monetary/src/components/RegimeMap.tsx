import { CartesianGrid, ReferenceLine, ResponsiveContainer, Scatter, ScatterChart, Tooltip, XAxis, YAxis } from "recharts";
import { chartRows } from "../macroData";
import type { MacroPoint } from "../types";

export function RegimeMap({ points, domain }: { points: MacroPoint[]; domain: [number, number] }) {
  const observations = chartRows(points, domain)
    .filter((point, index) => index % 3 === 0 && point.inflation !== undefined && point.industrialGrowth !== undefined)
    .map((point) => ({ ...point, label: dateLabel(point.timestamp) }));
  const recessions = observations.filter((point) => (point.recession ?? 0) >= 0.5);
  const expansions = observations.filter((point) => (point.recession ?? 0) < 0.5);
  const current = observations.length ? [observations[observations.length - 1]] : [];
  return <article className="chart-frame chart-primary regime-map">
    <header><div><h2>Growth / inflation regime</h2><span>Industrial production YoY against CPI YoY</span></div><i>Current</i></header>
    <div className="chart-canvas">
      {observations.length === 0 ? <div className="chart-empty">Regime data unavailable</div> : <ResponsiveContainer width="100%" height="100%">
        <ScatterChart margin={{ top: 15, right: 17, bottom: 10, left: 0 }}>
          <CartesianGrid stroke="#e4e8e5" />
          <XAxis type="number" dataKey="inflation" name="Inflation" unit="%" tick={{ fill: "#69746d", fontSize: 10 }} axisLine={false} tickLine={false} />
          <YAxis type="number" dataKey="industrialGrowth" name="Industrial growth" unit="%" tick={{ fill: "#69746d", fontSize: 10 }} axisLine={false} tickLine={false} width={48} />
          <ReferenceLine x={2.5} stroke="#b68d32" strokeDasharray="4 4" />
          <ReferenceLine y={0} stroke="#8e9992" />
          <Tooltip cursor={{ strokeDasharray: "3 3" }} formatter={(value) => `${Number(value).toFixed(2)}%`} labelFormatter={(_, payload) => payload[0]?.payload.label ?? ""} contentStyle={{ border: "1px solid #cdd5cf", borderRadius: 4, fontSize: 11 }} />
          <Scatter name="Expansion" data={expansions} fill="#377c5b" fillOpacity={0.42} />
          <Scatter name="NBER recession" data={recessions} fill="#bd5a4b" fillOpacity={0.62} />
          <Scatter name="Current" data={current} fill="#17201b" shape="diamond" />
        </ScatterChart>
      </ResponsiveContainer>}
    </div>
  </article>;
}

function dateLabel(value: number) { return new Date(value).toLocaleDateString("en-US", { year: "numeric", month: "short", timeZone: "UTC" }); }
