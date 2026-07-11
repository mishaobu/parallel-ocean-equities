import { CartesianGrid, Legend, Line, LineChart, ReferenceArea, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { ChartHeadingMeta, useChartZoom, useFittedYDomain, useLegendFilter, type SharedChartRange, type SharedLegendFilter } from "../chartInteraction";
import { valuationHistoryRows, type HistoryBasis } from "../historyData";
import { equityColor } from "../colors";
import { descendingTooltipItem } from "../chartData";
import type { Equity } from "../types";
import { formatValuation, valuationRows, type ValuationMetricKey, type ValuationRow } from "../valuationData";

interface Props extends SharedChartRange, SharedLegendFilter {
  equities: Equity[];
  metric: ValuationMetricKey;
  basis: HistoryBasis;
  domain: [number, number];
}

export function ValuationHistoryCharts({ equities, metric, basis, domain, zoom, onZoom, hiddenKeys, onHiddenKeys }: Props) {
  const primary = valuationRows.find((row) => row.key === metric) ?? valuationRows[0];
  return <ValuationHistoryChart equities={equities} metric={primary} basis={basis} domain={domain} zoom={zoom} onZoom={onZoom} hiddenKeys={hiddenKeys} onHiddenKeys={onHiddenKeys} />;
}

function ValuationHistoryChart({ equities, metric, basis, domain, compact = false, zoom, onZoom, hiddenKeys, onHiddenKeys }: { equities: Equity[]; metric: ValuationRow; basis: HistoryBasis; domain: [number, number]; compact?: boolean } & SharedChartRange & SharedLegendFilter) {
  const data = valuationHistoryRows(equities, metric, basis, domain);
  const keys = equities.map((equity) => equity.ticker);
  const legend = useLegendFilter(keys, hiddenKeys, onHiddenKeys);
  const chart = useChartZoom(domain, 20*24*60*60*1000, zoom, onZoom);
  const fitted = useFittedYDomain(data, chart.activeDomain, legend.visibleKeys, "date", { includeZero: metric.kind === "yield" || metric.kind === "leverage" });
  return (
    <div className={`chart history-chart${compact ? " chart-compact" : " chart-primary"}`}>
      <div className="chart-heading">
        <strong>{metric.label}</strong>
        <ChartHeadingMeta unit={`${basis === "actual" ? "LTM at filing" : "Next 12m realized"} / ${metric.kind === "yield" ? "percent" : "multiple"}`} zoom={chart.zoom} onReset={chart.reset} clippedCount={fitted.clippedCount} includeOutliers={fitted.includeOutliers} onToggleOutliers={fitted.toggleOutliers} />
      </div>
      <div className="chart-canvas">
        {data.length === 0 ? <ChartEmpty /> : <ResponsiveContainer width="100%" height="100%">
          <LineChart className="interactive-chart" data={data} margin={{ top: 14, right: 18, bottom: 2, left: compact ? -10 : 2 }} onMouseDown={chart.start} onMouseMove={chart.move} onMouseUp={chart.finish} onMouseLeave={chart.finish}>
            <CartesianGrid vertical={false} stroke="#e5e9e6" />
            <XAxis dataKey="date" type="number" scale="time" domain={chart.activeDomain} allowDataOverflow tickFormatter={yearLabel} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} minTickGap={32} />
            <YAxis domain={fitted.domain} allowDataOverflow tickFormatter={(value) => formatValuation(Number(value), metric.kind)} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} width={58} />
            <Tooltip itemSorter={descendingTooltipItem} formatter={(value) => formatValuation(Number(value), metric.kind)} labelFormatter={(value) => dateLabel(Number(value))} />
            {!compact && <Legend iconType="line" wrapperStyle={{ fontSize: 12, cursor: "pointer" }} onClick={(event) => legend.toggle(String(event.dataKey ?? ""))} />}
            {(metric.kind === "yield" || metric.kind === "leverage") && <ReferenceLine y={0} stroke="#aeb8b1" strokeDasharray="3 3" />}
            {chart.selection && <ReferenceArea x1={chart.selection[0]} x2={chart.selection[1]} fill="#7a9b88" fillOpacity={0.16} strokeOpacity={0} />}
            {equities.map((equity) => <Line
              key={equity.ticker}
              type="monotone"
              dataKey={equity.ticker}
              name={equity.ticker}
              stroke={equityColor(equity.ticker)}
              strokeWidth={compact ? 1.8 : 2.2}
              dot={false}
              activeDot={{ r: 4 }}
              connectNulls
              hide={legend.hidden.has(equity.ticker)}
              isAnimationActive={false}
            />)}
          </LineChart>
        </ResponsiveContainer>}
      </div>
    </div>
  );
}

function ChartEmpty() {
  return <div className="chart-empty">Historical valuation refresh pending</div>;
}

function yearLabel(value: number) { return new Date(value).getUTCFullYear().toString(); }
function dateLabel(value: number) { return new Date(value).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric", timeZone: "UTC" }); }
