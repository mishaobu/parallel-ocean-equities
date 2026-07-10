import { CartesianGrid, Legend, Line, LineChart, ReferenceArea, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { comparisonRows, descendingTooltipItem, formatMetric, metricLabels } from "../chartData";
import { equityColor } from "../colors";
import type { Equity, MetricKey } from "../types";

interface Props {
  equities: Equity[];
  metric: MetricKey;
  compact?: boolean;
}

export function MetricChart({ equities, metric, compact = false }: Props) {
  const data = comparisonRows(equities, metric);
  const estimateYear = data.find((row) => row.estimate)?.year as number | undefined;
  return (
    <div className={compact ? "chart chart-compact" : "chart"}>
      <div className="chart-heading">
        <strong>{metricLabels[metric]}</strong>
        <span>{metric === "dilutedEps" ? "USD / share" : metric === "peRatio" ? "multiple" : "USD billions"}</span>
      </div>
      <div className="chart-canvas">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={data} margin={{ top: 14, right: 16, bottom: 2, left: compact ? -12 : 0 }}>
            <CartesianGrid vertical={false} stroke="#e5e9e6" />
            <XAxis dataKey="year" tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} />
            <YAxis tickFormatter={(value) => formatMetric(metric, Number(value))} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} width={58} />
            <Tooltip itemSorter={descendingTooltipItem} formatter={(value) => formatMetric(metric, Number(value))} labelFormatter={(label) => `FY${label}`} />
            {!compact && <Legend iconType="line" wrapperStyle={{ fontSize: 12 }} />}
            {estimateYear && <ReferenceArea x1={estimateYear - 0.45} x2={estimateYear + 0.45} fill="#f3eee5" fillOpacity={0.72} />}
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
              />
            ))}
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
