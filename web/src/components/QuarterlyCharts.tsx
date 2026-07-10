import { CartesianGrid, Legend, Line, LineChart, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
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
  const data = recentQuarters(equity).map((row) => ({ period: quarterLabel(row), value: row[metric] }));
  return (
    <div className="chart chart-compact">
      <div className="chart-heading"><strong>{labels[metric]}</strong><span>USD billions / quarter</span></div>
      <div className="chart-canvas">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={data} margin={{ top: 12, right: 16, bottom: 2, left: -8 }}>
            <CartesianGrid vertical={false} stroke="#e5e9e6" />
            <XAxis dataKey="period" interval="preserveStartEnd" tick={{ fill: "#66736b", fontSize: 10 }} axisLine={false} tickLine={false} />
            <YAxis tickFormatter={(value) => formatBillions(Number(value))} tick={{ fill: "#66736b", fontSize: 10 }} axisLine={false} tickLine={false} width={64} />
            <Tooltip formatter={(value) => formatBillions(Number(value))} />
            <ReferenceLine y={0} stroke="#aeb9b2" />
            <Line type="monotone" dataKey="value" name={labels[metric]} stroke={colors[metric]} strokeWidth={2.2} dot={{ r: 2.5, fill: "#fff" }} connectNulls />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

export function BalanceSheetChart({ equity }: { equity: Equity }) {
  const data = recentQuarters(equity).map((row) => ({
    period: quarterLabel(row),
    Assets: row.assetsB,
    Liquidity: add(row.cashB, row.investmentsB),
    Debt: row.debtB,
    Equity: row.equityB,
  }));
  return (
    <div className="chart chart-wide">
      <div className="chart-heading"><strong>Balance sheet trajectory</strong><span>quarter-end / USD billions</span></div>
      <div className="chart-canvas">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={data} margin={{ top: 12, right: 18, bottom: 2, left: 0 }}>
            <CartesianGrid vertical={false} stroke="#e5e9e6" />
            <XAxis dataKey="period" interval="preserveStartEnd" tick={{ fill: "#66736b", fontSize: 10 }} axisLine={false} tickLine={false} />
            <YAxis tickFormatter={(value) => formatBillions(Number(value))} tick={{ fill: "#66736b", fontSize: 10 }} axisLine={false} tickLine={false} width={68} />
            <Tooltip formatter={(value) => formatBillions(Number(value))} />
            <Legend iconType="line" wrapperStyle={{ fontSize: 11 }} />
            <Line type="monotone" dataKey="Assets" stroke="#176b4d" strokeWidth={2.2} dot={false} connectNulls />
            <Line type="monotone" dataKey="Liquidity" stroke="#2962a3" strokeWidth={2.2} dot={false} connectNulls />
            <Line type="monotone" dataKey="Debt" stroke="#b46016" strokeWidth={2.2} dot={false} connectNulls />
            <Line type="monotone" dataKey="Equity" stroke="#7047a3" strokeWidth={2.2} dot={false} connectNulls />
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
