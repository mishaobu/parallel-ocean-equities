import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { valuationHistoryRows, type HistoryBasis } from "../historyData";
import type { Equity } from "../types";
import { formatValuation, valuationRows, type ValuationMetricKey, type ValuationRow } from "../valuationData";

const colors = ["#176b4d", "#2962a3", "#b46016", "#7047a3", "#a3304d", "#087b84"];

interface Props {
  equities: Equity[];
  metric: ValuationMetricKey;
  basis: HistoryBasis;
  domain: [number, number];
}

export function ValuationHistoryCharts({ equities, metric, basis, domain }: Props) {
  const primary = valuationRows.find((row) => row.key === metric) ?? valuationRows[0];
  return <>
    <ValuationHistoryChart equities={equities} metric={primary} basis={basis} domain={domain} />
    <div className="small-multiples valuation-grid">
      {valuationRows.filter((row) => row.key !== primary.key).map((row) => (
        <ValuationHistoryChart key={row.key} equities={equities} metric={row} basis={basis} domain={domain} compact />
      ))}
    </div>
  </>;
}

function ValuationHistoryChart({ equities, metric, basis, domain, compact = false }: { equities: Equity[]; metric: ValuationRow; basis: HistoryBasis; domain: [number, number]; compact?: boolean }) {
  const data = valuationHistoryRows(equities, metric, basis, domain);
  return (
    <div className={`chart history-chart${compact ? " chart-compact" : " chart-primary"}`}>
      <div className="chart-heading">
        <strong>{metric.label}</strong>
        <span>{basis === "actual" ? "LTM" : "Forward"} / {metric.kind === "yield" ? "percent" : "multiple"}</span>
      </div>
      <div className="chart-canvas">
        {data.length === 0 ? <ChartEmpty /> : <ResponsiveContainer width="100%" height="100%">
          <LineChart data={data} margin={{ top: 14, right: 18, bottom: 2, left: compact ? -10 : 2 }}>
            <CartesianGrid vertical={false} stroke="#e5e9e6" />
            <XAxis dataKey="date" type="number" scale="time" domain={domain} tickFormatter={yearLabel} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} minTickGap={32} />
            <YAxis tickFormatter={(value) => formatValuation(Number(value), metric.kind)} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} width={58} />
            <Tooltip formatter={(value) => formatValuation(Number(value), metric.kind)} labelFormatter={(value) => dateLabel(Number(value))} />
            {!compact && <Legend iconType="line" wrapperStyle={{ fontSize: 12 }} />}
            {equities.map((equity, index) => <Line
              key={equity.ticker}
              type="monotone"
              dataKey={equity.ticker}
              name={equity.ticker}
              stroke={colors[index % colors.length]}
              strokeWidth={compact ? 1.8 : 2.2}
              dot={false}
              activeDot={{ r: 4 }}
              connectNulls
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
