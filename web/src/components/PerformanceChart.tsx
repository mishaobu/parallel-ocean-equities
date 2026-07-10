import { CartesianGrid, Legend, Line, LineChart, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { equityColor } from "../colors";
import { descendingTooltipItem } from "../chartData";
import { indexedPerformanceRows } from "../historyData";
import type { Equity } from "../types";

export function PerformanceChart({ equities, domain }: { equities: Equity[]; domain: [number, number] }) {
  const data = indexedPerformanceRows(equities, domain);
  return <div className="chart chart-primary performance-chart">
    <div className="chart-heading"><strong>Indexed performance</strong><span>each starts at 1.0x / log scale</span></div>
    <div className="chart-canvas">
      {data.length === 0 ? <div className="chart-empty">Market history refresh pending</div> : <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data} margin={{ top: 14, right: 18, bottom: 2, left: 2 }}>
          <CartesianGrid vertical={false} stroke="#e5e9e6" />
          <XAxis dataKey="date" type="number" scale="time" domain={domain} tickFormatter={yearLabel} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} minTickGap={34} />
          <YAxis scale="log" domain={["auto", "auto"]} tickFormatter={multipleLabel} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} width={62} />
          <Tooltip itemSorter={descendingTooltipItem} formatter={(value) => multipleLabel(Number(value))} labelFormatter={(value) => dateLabel(Number(value))} />
          <Legend iconType="line" wrapperStyle={{ fontSize: 11 }} />
          <ReferenceLine y={1} stroke="#aeb8b1" />
          {equities.map((equity) => <Line key={equity.ticker} type="monotone" dataKey={equity.ticker} name={equity.ticker} stroke={equityColor(equity.ticker)} strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls isAnimationActive={false} />)}
        </LineChart>
      </ResponsiveContainer>}
    </div>
  </div>;
}

function multipleLabel(value: number) {
  if (value >= 1000) return `${Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 }).format(value)}x`;
  return `${value >= 10 ? value.toFixed(0) : value.toFixed(1)}x`;
}
function yearLabel(value: number) { return new Date(value).getUTCFullYear().toString(); }
function dateLabel(value: number) { return new Date(value).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric", timeZone: "UTC" }); }
