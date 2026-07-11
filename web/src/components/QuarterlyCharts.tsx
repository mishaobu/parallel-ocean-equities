import { CartesianGrid, Legend, Line, LineChart, ReferenceArea, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { ChartHeadingMeta, useChartZoom, useFittedYDomain, useLegendFilter } from "../chartInteraction";
import type { Equity, QuarterlyPoint } from "../types";
import { formatBillions, quarterLabel } from "../valuationData";

type QuarterlyMetric = "revenueB" | "ebitdaB" | "fcfB" | "netDebtB";

const labels: Record<QuarterlyMetric, string> = {
  revenueB: "Revenue",
  ebitdaB: "EBITDA",
  fcfB: "Free cash flow",
  netDebtB: "Net debt",
};

const colors = {
  revenueB: "#176b4d",
  ebitdaB: "#2962a3",
  fcfB: "#b46016",
  netDebtB: "#7047a3",
};

export function QuarterlyChart({ equity, metric }: { equity: Equity; metric: QuarterlyMetric }) {
  const data = recentQuarters(equity).map((row) => ({ timestamp: Date.parse(row.periodEnd), period: quarterLabel(row), value: row[metric] }));
  const domain = timeDomain(data);
  const chart = useChartZoom(domain, 60*24*60*60*1000);
  const fitted = useFittedYDomain(data, chart.activeDomain, ["value"], "timestamp", { includeZero: true });
  return (
    <div className="chart chart-compact">
      <div className="chart-heading"><strong>{labels[metric]}</strong><ChartHeadingMeta unit="USD billions / quarter" zoom={chart.zoom} onReset={chart.reset} clippedCount={fitted.clippedCount} includeOutliers={fitted.includeOutliers} onToggleOutliers={fitted.toggleOutliers} /></div>
      <div className="chart-canvas">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart className="interactive-chart" data={data} margin={{ top: 12, right: 16, bottom: 2, left: -8 }} onMouseDown={chart.start} onMouseMove={chart.move} onMouseUp={chart.finish} onMouseLeave={chart.finish}>
            <CartesianGrid vertical={false} stroke="#e5e9e6" />
            <XAxis dataKey="timestamp" type="number" scale="time" domain={chart.activeDomain} allowDataOverflow interval="preserveStartEnd" tickFormatter={quarterDate} tick={{ fill: "#66736b", fontSize: 10 }} axisLine={false} tickLine={false} />
            <YAxis domain={fitted.domain} allowDataOverflow tickFormatter={(value) => formatBillions(Number(value))} tick={{ fill: "#66736b", fontSize: 10 }} axisLine={false} tickLine={false} width={64} />
            <Tooltip formatter={(value) => formatBillions(Number(value))} labelFormatter={(value) => quarterDate(Number(value))} />
            <ReferenceLine y={0} stroke="#aeb9b2" />
            <Line type="monotone" dataKey="value" name={labels[metric]} stroke={colors[metric]} strokeWidth={2.2} dot={{ r: 2.5, fill: "#fff" }} connectNulls />
            {chart.selection && <ReferenceArea x1={chart.selection[0]} x2={chart.selection[1]} fill="#7a9b88" fillOpacity={0.16} strokeOpacity={0} />}
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

export function BalanceSheetChart({ equity }: { equity: Equity }) {
  const data = recentQuarters(equity).map((row) => ({
    timestamp: Date.parse(row.periodEnd),
    period: quarterLabel(row),
    Assets: row.assetsB,
    Liquidity: add(row.cashB, row.investmentsB),
    Debt: row.debtB,
    Equity: row.equityB,
  }));
  const domain = timeDomain(data);
  const keys = ["Assets", "Liquidity", "Debt", "Equity"];
  const legend = useLegendFilter(keys);
  const chart = useChartZoom(domain, 60*24*60*60*1000);
  const fitted = useFittedYDomain(data, chart.activeDomain, legend.visibleKeys, "timestamp", { includeZero: true });
  return (
    <div className="chart chart-wide">
      <div className="chart-heading"><strong>Balance sheet trajectory</strong><ChartHeadingMeta unit="quarter-end / USD billions" zoom={chart.zoom} onReset={chart.reset} clippedCount={fitted.clippedCount} includeOutliers={fitted.includeOutliers} onToggleOutliers={fitted.toggleOutliers} /></div>
      <div className="chart-canvas">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart className="interactive-chart" data={data} margin={{ top: 12, right: 18, bottom: 2, left: 0 }} onMouseDown={chart.start} onMouseMove={chart.move} onMouseUp={chart.finish} onMouseLeave={chart.finish}>
            <CartesianGrid vertical={false} stroke="#e5e9e6" />
            <XAxis dataKey="timestamp" type="number" scale="time" domain={chart.activeDomain} allowDataOverflow interval="preserveStartEnd" tickFormatter={quarterDate} tick={{ fill: "#66736b", fontSize: 10 }} axisLine={false} tickLine={false} />
            <YAxis domain={fitted.domain} allowDataOverflow tickFormatter={(value) => formatBillions(Number(value))} tick={{ fill: "#66736b", fontSize: 10 }} axisLine={false} tickLine={false} width={68} />
            <Tooltip formatter={(value) => formatBillions(Number(value))} labelFormatter={(value) => quarterDate(Number(value))} />
            <Legend iconType="line" wrapperStyle={{ fontSize: 11, cursor: "pointer" }} onClick={(event) => legend.toggle(String(event.dataKey ?? ""))} />
            <Line type="monotone" dataKey="Assets" stroke="#176b4d" strokeWidth={2.2} dot={false} connectNulls hide={legend.hidden.has("Assets")} />
            <Line type="monotone" dataKey="Liquidity" stroke="#2962a3" strokeWidth={2.2} dot={false} connectNulls hide={legend.hidden.has("Liquidity")} />
            <Line type="monotone" dataKey="Debt" stroke="#b46016" strokeWidth={2.2} dot={false} connectNulls hide={legend.hidden.has("Debt")} />
            <Line type="monotone" dataKey="Equity" stroke="#7047a3" strokeWidth={2.2} dot={false} connectNulls hide={legend.hidden.has("Equity")} />
            {chart.selection && <ReferenceArea x1={chart.selection[0]} x2={chart.selection[1]} fill="#7a9b88" fillOpacity={0.16} strokeOpacity={0} />}
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

function recentQuarters(equity: Equity): QuarterlyPoint[] {
  return (equity.quarterlies ?? []).slice(-24);
}

function add(left?: number, right?: number): number | undefined {
  if (left === undefined && right === undefined) return undefined;
  return (left ?? 0) + (right ?? 0);
}

function timeDomain(rows: Array<{ timestamp: number }>): [number, number] {
  const values = rows.map((row) => row.timestamp).filter(Number.isFinite);
  return [Math.min(...values, Date.now()), Math.max(...values, Date.now())];
}

function quarterDate(value: number) {
  const date = new Date(value);
  return `${date.getUTCFullYear()} Q${Math.floor(date.getUTCMonth() / 3) + 1}`;
}
