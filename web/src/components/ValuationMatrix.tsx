import type { Equity } from "../types";
import { formatValuation, valuationRows } from "../valuationData";

export function ValuationMatrix({ equities }: { equities: Equity[] }) {
  return (
    <div className="table-wrap valuation-matrix">
      <table>
        <thead>
          <tr>
            <th rowSpan={2}>Metric</th>
            {equities.map((equity) => <th key={equity.ticker} colSpan={2}>{equity.ticker}</th>)}
          </tr>
          <tr>
            {equities.flatMap((equity) => [
              <th key={`${equity.ticker}-ltm`}>LTM</th>,
              <th key={`${equity.ticker}-forward`} className="forward-cell">Forward</th>,
            ])}
          </tr>
        </thead>
        <tbody>
          {valuationRows.map((row) => (
            <tr key={row.key}>
              <th>{row.label}</th>
              {equities.flatMap((equity) => [
                <td key={`${equity.ticker}-${row.key}-actual`}>{formatValuation(equity.valuation?.[row.actual], row.kind)}</td>,
                <td key={`${equity.ticker}-${row.key}-forward`} className="forward-cell">{formatValuation(equity.valuation?.[row.forward], row.kind)}</td>,
              ])}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
