import { AlertCircle, LoaderCircle } from "lucide-react";
import type { Equity } from "../types";

interface Props {
  equities: Equity[];
  selected: string;
  onSelect: (ticker: string) => void;
}

export function TickerRail({ equities, selected, onSelect }: Props) {
  return (
    <aside className="ticker-rail" aria-label="Watchlist">
      <div className="rail-label">Watchlist <span>{equities.length}</span></div>
      <div className="ticker-list">
        {equities.map((equity) => (
          <button
            key={equity.ticker}
            type="button"
            className={`ticker-button ${selected === equity.ticker ? "is-selected" : ""}`}
            onClick={() => onSelect(equity.ticker)}
          >
            <span><b>{equity.ticker}</b><small>{equity.company || "Pending analysis"}</small></span>
            {equity.status === "refreshing" || equity.status === "queued" ? <LoaderCircle className="spin" size={16} aria-label="Refreshing" /> : equity.status === "error" ? <AlertCircle size={16} aria-label="Refresh failed" /> : <i aria-hidden="true" />}
          </button>
        ))}
      </div>
    </aside>
  );
}
