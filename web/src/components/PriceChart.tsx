import { Area, AreaChart, CartesianGrid, ReferenceArea, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { ChartHeadingMeta, useChartZoom, useFittedYDomain } from "../chartInteraction";
import type { Equity } from "../types";

export function PriceChart({ equity }: { equity: Equity }) {
  const data = (equity.prices ?? []).map((row) => ({ ...row, timestamp: Date.parse(row.date) })).filter((row) => Number.isFinite(row.timestamp));
  const timestamps = data.map((row) => row.timestamp);
  const domain: [number, number] = [Math.min(...timestamps, Date.now()), Math.max(...timestamps, Date.now())];
  const chart = useChartZoom(domain, 20*24*60*60*1000);
  const fitted = useFittedYDomain(data, chart.activeDomain, ["close"], "timestamp");
  if (!data.length) return null;
  return (
    <div className="chart chart-wide">
      <div className="chart-heading"><strong>Adjusted close</strong><ChartHeadingMeta unit="monthly" zoom={chart.zoom} onReset={chart.reset} clippedCount={fitted.clippedCount} includeOutliers={fitted.includeOutliers} onToggleOutliers={fitted.toggleOutliers} /></div>
      <div className="chart-canvas chart-gesture-surface" {...chart.touchHandlers}>
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart className="interactive-chart" data={data} margin={{ top: 12, right: 16, bottom: 2, left: 0 }} onMouseDown={chart.start} onMouseMove={chart.move} onMouseUp={chart.finish} onMouseLeave={chart.finish}>
            <CartesianGrid vertical={false} stroke="#e5e9e6" />
            <XAxis dataKey="timestamp" type="number" scale="time" domain={chart.activeDomain} allowDataOverflow minTickGap={48} tickFormatter={(value) => String(new Date(Number(value)).getUTCFullYear())} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} />
            <YAxis domain={fitted.domain} allowDataOverflow tickFormatter={(value) => `$${Number(value).toFixed(0)}`} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} width={56} />
            <Tooltip formatter={(value) => `$${Number(value).toFixed(2)}`} labelFormatter={(value) => new Date(Number(value)).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric", timeZone: "UTC" })} />
            {chart.selection && <ReferenceArea x1={chart.selection[0]} x2={chart.selection[1]} fill="#7a9b88" fillOpacity={0.16} strokeOpacity={0} />}
            <Area type="monotone" dataKey="close" name={equity.ticker} stroke="#176b4d" fill="#dcebe4" strokeWidth={2.2} />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
