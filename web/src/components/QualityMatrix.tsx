import { useMemo, useState } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { formatQuality, qualityRows, type QualityMetricKey } from "../qualityData";
import type { Equity } from "../types";
import { historyPercentile, peerGroup, peerMedian } from "../peerData";

type SortKey = "ticker" | QualityMetricKey;
type SortDirection = "asc" | "desc";
export interface QualitySort { key: SortKey; direction: SortDirection }

export function QualityMatrix({ equities }: { equities: Equity[] }) {
  const [sort, setSort] = useState<QualitySort>({ key: "ticker", direction: "asc" });
  const sorted = useMemo(() => sortQualityEquities(equities, sort), [equities, sort]);
  function changeSort(key: SortKey) {
    setSort((current) => current.key === key
      ? { ...current, direction: current.direction === "asc" ? "desc" : "asc" }
      : { key, direction: key === "ticker" ? "asc" : "desc" });
  }
  return <div className="table-wrap quality-matrix">
    <table><thead><tr>
      <th className="ticker-column"><SortButton label="Ticker" sortKey="ticker" sort={sort} onSort={changeSort} /></th>
      {qualityRows.map((row) => <th key={row.key}><SortButton label={row.label} sortKey={row.key} sort={sort} onSort={changeSort} /></th>)}
    </tr></thead><tbody>{sorted.map((equity) => <tr key={equity.ticker}>
      <th className="ticker-column"><strong>{equity.ticker}</strong><span>{equity.company}</span><small>{peerGroup(equity)} / LTM {equity.quality?.asOf?.slice(0, 10) ?? "n/a"}</small></th>
		{qualityRows.map((row) => { const value = equity.quality?.[row.property]; const median = peerMedian(equities, equity, (candidate) => candidate.quality?.[row.property]); const percentile = historyPercentile(equity.qualities, row.property, value); return <td key={`${equity.ticker}-${row.key}`}><strong>{formatQuality(value, row.kind)}</strong><small>{median === undefined ? "Peer n/a" : `Peer ${formatQuality(median, row.kind)}`} / {percentile === undefined ? "Hist n/a" : `Hist P${percentile}`}</small></td>; })}
    </tr>)}</tbody></table>
  </div>;
}

function SortButton({ label, sortKey, sort, onSort }: { label: string; sortKey: SortKey; sort: QualitySort; onSort: (key: SortKey) => void }) {
  const active = sort.key === sortKey;
  const Icon = !active ? ArrowUpDown : sort.direction === "asc" ? ArrowUp : ArrowDown;
  return <button type="button" className={active ? "sort-button is-active" : "sort-button"} onClick={() => onSort(sortKey)} aria-label={`Sort by ${label}, ${active ? sort.direction : "none"}`}>
    <span><b>{label}</b></span><Icon size={13} />
  </button>;
}

export function sortQualityEquities(equities: Equity[], sort: QualitySort) {
  return [...equities].sort((left, right) => {
    if (sort.key === "ticker") {
      const compared = left.ticker.localeCompare(right.ticker);
      return sort.direction === "asc" ? compared : -compared;
    }
    const metric = qualityRows.find((row) => row.key === sort.key);
    if (!metric) return left.ticker.localeCompare(right.ticker);
    const leftValue = left.quality?.[metric.property];
    const rightValue = right.quality?.[metric.property];
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
