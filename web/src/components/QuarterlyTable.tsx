import { ExternalLink } from "lucide-react";
import type { Equity } from "../types";
import { formatBillions, quarterLabel } from "../valuationData";

export function QuarterlyTable({ equity }: { equity: Equity }) {
  const rows = [...(equity.quarterlies ?? [])].reverse();
  if (rows.length === 0) return null;
  return (
    <div className="table-wrap quarterly-table">
      <table>
        <thead><tr><th>Quarter</th><th>Revenue</th><th>EBITDA</th><th>EBIT</th><th>FCF</th><th>Net income</th><th>Capex</th><th>Net debt</th><th>Assets</th><th>Equity</th><th>Filed</th></tr></thead>
        <tbody>{rows.map((row) => (
          <tr key={`${row.fiscalYear}-${row.fiscalQuarter}-${row.periodEnd}`}>
            <th>{quarterLabel(row)}</th>
            <td>{formatBillions(row.revenueB)}</td>
            <td>{formatBillions(row.ebitdaB)}</td>
            <td>{formatBillions(row.ebitB)}</td>
            <td>{formatBillions(row.fcfB)}</td>
            <td>{formatBillions(row.netIncomeB)}</td>
            <td>{formatBillions(row.capexB)}</td>
            <td>{formatBillions(row.netDebtB)}</td>
            <td>{formatBillions(row.assetsB)}</td>
            <td>{formatBillions(row.equityB)}</td>
            <td>{row.filingUrl ? <a href={row.filingUrl} target="_blank" rel="noreferrer" title={`Open ${row.form} filing`}><span>{row.form}</span><ExternalLink size={12} /></a> : row.form ?? "n/a"}</td>
          </tr>
        ))}</tbody>
      </table>
    </div>
  );
}
