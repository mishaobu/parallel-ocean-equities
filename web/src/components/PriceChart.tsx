import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { Equity } from "../types";

export function PriceChart({ equity }: { equity: Equity }) {
  if (!equity.prices?.length) return null;
  return (
    <div className="chart chart-wide">
      <div className="chart-heading"><strong>Adjusted close</strong><span>monthly</span></div>
      <div className="chart-canvas">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={equity.prices} margin={{ top: 12, right: 16, bottom: 2, left: 0 }}>
            <CartesianGrid vertical={false} stroke="#e5e9e6" />
            <XAxis dataKey="date" minTickGap={48} tickFormatter={(value) => String(value).slice(0, 4)} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} />
            <YAxis domain={["auto", "auto"]} tickFormatter={(value) => `$${Number(value).toFixed(0)}`} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} width={56} />
            <Tooltip formatter={(value) => `$${Number(value).toFixed(2)}`} labelFormatter={(value) => String(value)} />
            <Area type="monotone" dataKey="close" name={equity.ticker} stroke="#176b4d" fill="#dcebe4" strokeWidth={2.2} />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
