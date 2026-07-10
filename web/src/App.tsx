import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { BarChart3, GitCompareArrows, LoaderCircle, Plus, RefreshCw, Trash2, TrendingUp } from "lucide-react";
import { api } from "./api";
import { delta, formatMetric, latestActual, latestEstimate, metricLabels } from "./chartData";
import { AnnualTable } from "./components/AnnualTable";
import { MetricChart } from "./components/MetricChart";
import { PriceChart } from "./components/PriceChart";
import { TickerRail } from "./components/TickerRail";
import type { Equity, MetricKey, StateResponse } from "./types";

const metrics: MetricKey[] = ["revenueB", "capexB", "netIncomeB", "dilutedEps", "peRatio"];

function App() {
  const [payload, setPayload] = useState<StateResponse | null>(null);
  const [error, setError] = useState("");
  const [selected, setSelected] = useState("AMZN");
  const [mode, setMode] = useState<"compare" | "ticker">("compare");
  const [metric, setMetric] = useState<MetricKey>("capexB");
  const [input, setInput] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    try {
      const next = await api.state();
      setPayload(next);
      setError("");
      if (!next.state.tickers[selected]) setSelected(Object.keys(next.state.tickers).sort()[0] ?? "");
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to load equities");
    }
  }, [selected]);

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 12_000);
    return () => window.clearInterval(timer);
  }, [load]);

  const equities = useMemo(() => Object.values(payload?.state.tickers ?? {}).sort((a, b) => a.ticker.localeCompare(b.ticker)), [payload]);
  const selectedEquity = payload?.state.tickers[selected] ?? equities[0];

  async function addTicker(event: FormEvent) {
    event.preventDefault();
    const ticker = input.trim().toUpperCase();
    if (!ticker) return;
    setSubmitting(true);
    try {
      await api.addTicker(ticker);
      setInput("");
      setSelected(ticker);
      setMode("ticker");
      await load();
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to add ticker");
    } finally {
      setSubmitting(false);
    }
  }

  async function refreshTicker(ticker: string) {
    await api.refreshTicker(ticker);
    await load();
  }

  async function removeTicker(ticker: string) {
    await api.removeTicker(ticker);
    setMode("compare");
    await load();
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand"><BarChart3 size={21} /><strong>Equities</strong><span>parallel-ocean</span></div>
        <form className="ticker-form" onSubmit={addTicker}>
          <label htmlFor="ticker-input">Add ticker</label>
          <input id="ticker-input" value={input} onChange={(event) => setInput(event.target.value.toUpperCase())} placeholder="NVDA" maxLength={10} autoComplete="off" />
          <button type="submit" disabled={submitting || !input.trim()} aria-label="Add ticker"><Plus size={17} /></button>
        </form>
        <div className="freshness"><span className={payload?.runtime.inFlight ? "status-dot active" : "status-dot"} />{payload?.runtime.inFlight ? `${payload.runtime.inFlight} refreshing` : `Updated ${timeAgo(payload?.state.updatedAt)}`}</div>
      </header>

      {error && <div className="error-banner" role="alert">{error}</div>}

      <div className="workspace">
        <TickerRail equities={equities} selected={selectedEquity?.ticker ?? ""} onSelect={(ticker) => { setSelected(ticker); setMode("ticker"); }} />
        <main className="content">
          <div className="view-toolbar">
            <div className="segmented" aria-label="View">
              <button type="button" className={mode === "compare" ? "is-active" : ""} onClick={() => setMode("compare")}><GitCompareArrows size={15} />Compare</button>
              <button type="button" className={mode === "ticker" ? "is-active" : ""} onClick={() => setMode("ticker")} disabled={!selectedEquity}><TrendingUp size={15} />Ticker</button>
            </div>
            {mode === "compare" && <div className="metric-tabs">{metrics.map((key) => <button type="button" key={key} className={metric === key ? "is-active" : ""} onClick={() => setMetric(key)}>{metricLabels[key]}</button>)}</div>}
          </div>

          {mode === "compare" ? <CompareView equities={equities} metric={metric} /> : selectedEquity && <TickerView equity={selectedEquity} onRefresh={refreshTicker} onRemove={removeTicker} />}
        </main>
      </div>
    </div>
  );
}

function CompareView({ equities, metric }: { equities: Equity[]; metric: MetricKey }) {
  return (
    <section className="view">
      <div className="view-title"><div><h1>Cross-company trajectories</h1><span>{equities.length} tickers / actuals and estimates</span></div></div>
      <MetricChart equities={equities} metric={metric} />
      <div className="small-multiples">
        {metrics.filter((key) => key !== metric).map((key) => <MetricChart key={key} equities={equities} metric={key} compact />)}
      </div>
      <div className="table-wrap comparison-table">
        <table><thead><tr><th>Ticker</th><th>Price</th><th>1Y</th><th>Latest capex</th><th>2026E capex</th><th>Latest net income</th><th>Trailing P/E</th><th>Forward P/E</th><th>Updated</th></tr></thead>
          <tbody>{equities.map((equity) => {
            const actual = latestActual(equity);
            const estimate = latestEstimate(equity);
            return <tr key={equity.ticker}><th>{equity.ticker}</th><td>{money(equity.current.price)}</td><td className={tone(equity.current.return1Y)}>{percent(equity.current.return1Y)}</td><td>{formatMetric("capexB", actual?.capexB)}</td><td>{formatMetric("capexB", estimate?.capexB)}</td><td>{formatMetric("netIncomeB", actual?.netIncomeB)}</td><td>{formatMetric("peRatio", equity.current.trailingPE)}</td><td>{formatMetric("peRatio", equity.current.forwardPE)}</td><td>{timeAgo(equity.updatedAt)}</td></tr>;
          })}</tbody>
        </table>
      </div>
    </section>
  );
}

function TickerView({ equity, onRefresh, onRemove }: { equity: Equity; onRefresh: (ticker: string) => Promise<void>; onRemove: (ticker: string) => Promise<void> }) {
  const actual = latestActual(equity);
  const estimate = latestEstimate(equity);
  const capexDelta = delta(estimate?.capexB, actual?.capexB);
  return (
    <section className="view">
      <div className="view-title ticker-title">
        <div><h1>{equity.ticker} <span>{equity.company}</span></h1><small>{equity.sources?.join(" + ") || "Analysis pending"} · {analysisDate(equity)}</small></div>
        <div className="icon-actions">
          <button type="button" onClick={() => void onRefresh(equity.ticker)} disabled={equity.status === "refreshing"} aria-label={`Refresh ${equity.ticker}`} title="Refresh analysis"><RefreshCw size={17} className={equity.status === "refreshing" ? "spin" : ""} /></button>
          <button type="button" onClick={() => void onRemove(equity.ticker)} aria-label={`Remove ${equity.ticker}`} title="Remove ticker"><Trash2 size={17} /></button>
        </div>
      </div>
      {equity.status === "error" && <div className="inline-error">{equity.error}</div>}
      {equity.annuals.length === 0 ? (
        <div className="analysis-pending">
          {equity.status !== "error" && <LoaderCircle className="spin" size={20} />}
          <div><strong>{equity.status === "error" ? "Analysis unavailable" : "Analysis in progress"}</strong><span>{equity.ticker}</span></div>
        </div>
      ) : <>
        <div className="metric-strip">
          <Metric label="Price" value={money(equity.current.price)} context={equity.current.return1Y === undefined ? "1Y n/a" : `${percent(equity.current.return1Y)} 1Y`} valueTone={tone(equity.current.return1Y)} />
          <Metric label="2026E capex" value={formatMetric("capexB", estimate?.capexB)} context={capexDelta === undefined ? "estimate n/a" : `${percent(capexDelta)} vs actual`} valueTone={tone(capexDelta)} />
          <Metric label="Net income" value={formatMetric("netIncomeB", actual?.netIncomeB)} context={`FY${actual?.fiscalYear ?? "-"}`} />
          <Metric label="Forward P/E" value={formatMetric("peRatio", equity.current.forwardPE)} context={`Trailing ${formatMetric("peRatio", equity.current.trailingPE)}`} />
        </div>
        <div className="detail-charts">
          <MetricChart equities={[equity]} metric="revenueB" compact />
          <MetricChart equities={[equity]} metric="capexB" compact />
          <MetricChart equities={[equity]} metric="netIncomeB" compact />
          <MetricChart equities={[equity]} metric="dilutedEps" compact />
          <MetricChart equities={[equity]} metric="peRatio" compact />
          <PriceChart equity={equity} />
        </div>
        <AnnualTable equity={equity} />
      </>}
      {!!equity.warnings?.length && <div className="warnings">{equity.warnings.map((warning) => <span key={warning}>{warning}</span>)}</div>}
    </section>
  );
}

function Metric({ label, value, context, valueTone = "" }: { label: string; value: string; context: string; valueTone?: string }) {
  return <div className="metric-block"><span>{label}</span><strong className={valueTone}>{value}</strong><small>{context}</small></div>;
}

function money(value?: number) { return value === undefined ? "n/a" : `$${value.toFixed(2)}`; }
function percent(value?: number) { return value === undefined ? "n/a" : `${value >= 0 ? "+" : ""}${(value * 100).toFixed(1)}%`; }
function tone(value?: number) { return value === undefined ? "" : value > 0 ? "positive" : value < 0 ? "negative" : ""; }
function analysisDate(equity: Equity) {
  const value = equity.current.priceAsOf || equity.updatedAt?.slice(0, 10);
  return !value || value.startsWith("0001-") ? "queued" : value;
}
function timeAgo(value?: string) {
  if (!value) return "pending";
  const seconds = Math.max(0, (Date.now() - new Date(value).getTime()) / 1000);
  if (seconds < 60) return "just now";
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
}

export default App;
