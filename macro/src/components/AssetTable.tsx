import { useMemo, useState } from "react";
import { ArrowDown, ArrowUp } from "lucide-react";
import type { AssetReturn } from "../data";

type SortKey = "symbol" | "group" | "oneYear" | "threeYear" | "fiveYear";
const columns: Array<[SortKey, string]> = [["symbol", "Asset"], ["group", "Sleeve"], ["oneYear", "1Y ann."], ["threeYear", "3Y ann."], ["fiveYear", "5Y ann."]];
export function AssetTable({ rows }: { rows: AssetReturn[] }) {
  const [key, setKey] = useState<SortKey>("oneYear");
  const [direction, setDirection] = useState<"asc" | "desc">("desc");
  const sorted = useMemo(() => [...rows].sort((a, b) => {
    const left = a[key]; const right = b[key]; const sign = direction === "asc" ? 1 : -1;
    if (typeof left === "number" && typeof right === "number") return sign * (left - right);
    if (left === undefined) return 1; if (right === undefined) return -1;
    return sign * String(left).localeCompare(String(right));
  }), [direction, key, rows]);
  function sort(next: SortKey) { if (next === key) setDirection((value) => value === "asc" ? "desc" : "asc"); else { setKey(next); setDirection(next === "symbol" || next === "group" ? "asc" : "desc"); } }
  return <article className="panel asset-table"><header className="panel-head"><div><h2>Cross-asset return board</h2><span>Annualized total price change / source-adjusted closes</span></div></header><div className="table-scroll"><table><thead><tr>{columns.map(([column, label]) => <th key={column}><button type="button" onClick={() => sort(column)}>{label}{key === column && (direction === "asc" ? <ArrowUp size={11} /> : <ArrowDown size={11} />)}</button></th>)}</tr></thead><tbody>{sorted.map((row) => <tr key={row.symbol}><th><b>{row.symbol}</b><span>{row.label}</span></th><td>{row.group}</td><td className={tone(row.oneYear)}>{format(row.oneYear)}</td><td className={tone(row.threeYear)}>{format(row.threeYear)}</td><td className={tone(row.fiveYear)}>{format(row.fiveYear)}</td></tr>)}</tbody></table></div></article>;
}
function format(value?: number) { return value === undefined ? "--" : `${value.toFixed(1)}%`; }
function tone(value?: number) { return value === undefined ? "" : value >= 0 ? "positive" : "negative"; }
