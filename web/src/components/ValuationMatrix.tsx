import { useMemo, useState } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import type { Equity } from "../types";
import { formatValuation, valuationRows, type ValuationMetricKey, type ValuationRow } from "../valuationData";

type ValuationBasis = "actual" | "forward";
type SortKey = "ticker" | ValuationMetricKey;
type SortDirection = "asc" | "desc";

export interface ValuationSort {
  key: SortKey;
  direction: SortDirection;
  basis: ValuationBasis;
}

export function ValuationMatrix({ equities }: { equities: Equity[] }) {
  const [sort, setSort] = useState<ValuationSort>({ key: "ticker", direction: "asc", basis: "actual" });
  const sorted = useMemo(() => sortValuationEquities(equities, sort), [equities, sort]);

  function changeSort(key: SortKey) {
    setSort((current) => current.key === key
      ? { ...current, direction: current.direction === "asc" ? "desc" : "asc" }
      : { ...current, key, direction: key === "ticker" ? "asc" : "desc" });
  }

  return <div className="valuation-matrix-shell">
    <div className="matrix-toolbar">
      <span>Sort basis</span>
      <div className="segmented compact-segmented" aria-label="Current valuation sort basis">
        <button type="button" className={sort.basis === "actual" ? "is-active" : ""} onClick={() => setSort((current) => ({ ...current, basis: "actual" }))}>LTM</button>
        <button type="button" className={sort.basis === "forward" ? "is-active" : ""} onClick={() => setSort((current) => ({ ...current, basis: "forward" }))}>Model</button>
      </div>
    </div>
    <div className="table-wrap valuation-matrix">
      <table>
        <thead><tr>
          <th className="ticker-column"><SortButton label="Ticker" sortKey="ticker" sort={sort} onSort={changeSort} /></th>
          {valuationRows.map((row) => <th key={row.key}><SortButton label={row.label} detail="LTM / model" sortKey={row.key} sort={sort} onSort={changeSort} /></th>)}
        </tr></thead>
        <tbody>
          {sorted.map((equity) => <tr key={equity.ticker}>
            <th className="ticker-column"><strong>{equity.ticker}</strong><span>{equity.company}</span></th>
            {valuationRows.map((row) => <td key={`${equity.ticker}-${row.key}`}><ValuationCell equity={equity} row={row} basis={sort.basis} /></td>)}
          </tr>)}
        </tbody>
      </table>
    </div>
  </div>;
}

function ValuationCell({ equity, row, basis }: { equity: Equity; row: ValuationRow; basis: ValuationBasis }) {
  const primaryKey = row[basis];
  const secondaryKey = row[basis === "actual" ? "forward" : "actual"];
  const secondaryLabel = basis === "actual" ? "Model" : "LTM";
  return <div className="valuation-cell-values">
    <strong>{formatValuation(equity.valuation?.[primaryKey], row.kind)}</strong>
    <small>{secondaryLabel} {formatValuation(equity.valuation?.[secondaryKey], row.kind)}</small>
  </div>;
}

function SortButton({ label, detail, sortKey, sort, onSort }: { label: string; detail?: string; sortKey: SortKey; sort: ValuationSort; onSort: (key: SortKey) => void }) {
  const active = sort.key === sortKey;
  const Icon = !active ? ArrowUpDown : sort.direction === "asc" ? ArrowUp : ArrowDown;
  const direction = active ? sort.direction : "none";
  const basis = sortKey === "ticker" ? "" : ` using ${sort.basis === "actual" ? "LTM" : "model"}`;
  return <button type="button" className={active ? "sort-button is-active" : "sort-button"} onClick={() => onSort(sortKey)} aria-label={`Sort by ${label}${basis}, ${direction}`}>
    <span><b>{label}</b>{detail && <small>{detail}</small>}</span><Icon size={13} />
  </button>;
}

export function sortValuationEquities(equities: Equity[], sort: ValuationSort) {
  return [...equities].sort((left, right) => {
    if (sort.key === "ticker") {
      const compared = left.ticker.localeCompare(right.ticker);
      return sort.direction === "asc" ? compared : -compared;
    }
    const metric = valuationRows.find((row) => row.key === sort.key);
    if (!metric) return left.ticker.localeCompare(right.ticker);
    const property = metric[sort.basis];
    const leftValue = left.valuation?.[property];
    const rightValue = right.valuation?.[property];
    const leftMissing = typeof leftValue !== "number" || !Number.isFinite(leftValue);
    const rightMissing = typeof rightValue !== "number" || !Number.isFinite(rightValue);
    if (leftMissing || rightMissing) {
      if (leftMissing && rightMissing) return left.ticker.localeCompare(right.ticker);
      return leftMissing ? 1 : -1;
    }
    if (leftValue === rightValue) return left.ticker.localeCompare(right.ticker);
    return sort.direction === "asc" ? leftValue - rightValue : rightValue - leftValue;
  });
}
