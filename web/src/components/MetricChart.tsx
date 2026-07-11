import { CartesianGrid, Legend, Line, LineChart, ReferenceArea, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { ChartHeadingMeta, useChartZoom, useFittedYDomain, useLegendFilter, type SharedChartRange, type SharedLegendFilter } from "../chartInteraction";
import { comparisonRows, descendingTooltipItem, formatMetric, metricLabels } from "../chartData";
import { equityColor } from "../colors";
import type { Equity, MetricKey } from "../types";

interface Props extends SharedChartRange, SharedLegendFilter {
  equities: Equity[];
  metric: MetricKey;
  compact?: boolean;
}

export function MetricChart({ equities, metric, compact = false, zoom, onZoom, hiddenKeys, onHiddenKeys }: Props) {
  const data = comparisonRows(equities, metric);
  const estimateYear = data.find((row) => row.estimate)?.year as number | undefined;
  const years = data.map((row) => Number(row.year)).filter(Number.isFinite);
  const domain: [number, number] = [Math.min(...years, new Date().getUTCFullYear()-10), Math.max(...years, new Date().getUTCFullYear())];
  const keys = equities.map((equity) => equity.ticker);
  const legend = useLegendFilter(keys, hiddenKeys, onHiddenKeys);
  const yearZoom: [number, number] | undefined = zoom ? [new Date(zoom[0]).getUTCFullYear(), new Date(zoom[1]).getUTCFullYear()] : undefined;
  const updateYearZoom = onZoom ? (next?: [number, number]) => onZoom(next ? [Date.UTC(Math.round(next[0]), 0, 1), Date.UTC(Math.round(next[1]), 11, 31)] : undefined) : undefined;
  const chart = useChartZoom(domain, 1, yearZoom, updateYearZoom);
  const fitted = useFittedYDomain(data, chart.activeDomain, legend.visibleKeys, "year", { includeZero: metric !== "peRatio" });
  return (
    <div className={compact ? "chart chart-compact" : "chart"}>
      <div className="chart-heading">
        <strong>{metricLabels[metric]}</strong>
        <ChartHeadingMeta unit={metric === "dilutedEps" ? "USD / share" : metric === "peRatio" ? "multiple" : "USD billions"} zoom={chart.zoom} onReset={chart.reset} clippedCount={fitted.clippedCount} includeOutliers={fitted.includeOutliers} onToggleOutliers={fitted.toggleOutliers} mode="year" />
      </div>
      <div className="chart-canvas">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart className="interactive-chart" data={data} margin={{ top: 14, right: 16, bottom: 2, left: compact ? -12 : 0 }} onMouseDown={chart.start} onMouseMove={chart.move} onMouseUp={chart.finish} onMouseLeave={chart.finish}>
            <CartesianGrid vertical={false} stroke="#e5e9e6" />
            <XAxis dataKey="year" type="number" domain={chart.activeDomain} allowDataOverflow tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} allowDecimals={false} />
            <YAxis domain={fitted.domain} allowDataOverflow tickFormatter={(value) => formatMetric(metric, Number(value))} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} width={58} />
            <Tooltip itemSorter={descendingTooltipItem} formatter={(value) => formatMetric(metric, Number(value))} labelFormatter={(label) => `FY${label}`} />
            {!compact && <Legend iconType="line" wrapperStyle={{ fontSize: 12, cursor: "pointer" }} onClick={(event) => legend.toggle(String(event.dataKey ?? ""))} />}
            {estimateYear && <ReferenceArea x1={estimateYear - 0.45} x2={estimateYear + 0.45} fill="#f3eee5" fillOpacity={0.72} />}
            {chart.selection && <ReferenceArea x1={chart.selection[0]} x2={chart.selection[1]} fill="#7a9b88" fillOpacity={0.16} strokeOpacity={0} />}
            {equities.map((equity) => (
              <Line
                key={equity.ticker}
                type="monotone"
                dataKey={equity.ticker}
                name={equity.ticker}
                stroke={equityColor(equity.ticker)}
                strokeWidth={2.2}
                dot={{ r: 3, strokeWidth: 2, fill: "#fff" }}
                activeDot={{ r: 5 }}
                connectNulls
                hide={legend.hidden.has(equity.ticker)}
              />
            ))}
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
