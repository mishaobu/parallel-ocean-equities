import { formatMetric } from "../chartData";
import type { Equity } from "../types";

export function AnnualTable({ equity }: { equity: Equity }) {
  return (
    <div className="table-wrap">
      <table>
        <thead><tr><th>Fiscal year</th><th>Revenue</th><th>Capex</th><th>Net income</th><th>Diluted EPS</th><th>P/E</th><th>Confidence</th></tr></thead>
        <tbody>
          {equity.annuals.map((row) => (
            <tr key={row.fiscalYear} className={row.estimate ? "estimate-row" : ""}>
              <th>{row.fiscalYear}{row.estimate ? "E" : ""}</th>
              <td>{formatMetric("revenueB", row.revenueB)}</td>
              <td>{formatMetric("capexB", row.capexB)}</td>
              <td>{formatMetric("netIncomeB", row.netIncomeB)}</td>
              <td>{formatMetric("dilutedEps", row.dilutedEps)}</td>
              <td>{formatMetric("peRatio", row.peRatio)}</td>
              <td><span className={`confidence confidence-${row.confidence || "unknown"}`}>{row.confidence || "n/a"}</span></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
