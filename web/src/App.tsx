import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { BarChart3, Calculator, GitCompareArrows, Landmark, LoaderCircle, Plus, RefreshCw, Trash2, TrendingUp } from "lucide-react";
import { api } from "./api";
import { metricLabels } from "./chartData";
import { historyDomain, valuationHistoryDomain, type HistoryBasis, type HistoryRange } from "./historyData";
import { AnnualTable } from "./components/AnnualTable";
import { MacroCharts } from "./components/MacroCharts";
import { MetricChart } from "./components/MetricChart";
import { PerformanceChart } from "./components/PerformanceChart";
import { PriceChart } from "./components/PriceChart";
import { BalanceSheetChart, QuarterlyChart } from "./components/QuarterlyCharts";
import { QuarterlyTable } from "./components/QuarterlyTable";
import { TickerRail } from "./components/TickerRail";
import { ValuationMatrix } from "./components/ValuationMatrix";
import { ValuationHistoryCharts } from "./components/ValuationHistoryCharts";
import { ValuationWorkbench } from "./components/ValuationWorkbench";
import type { Equity, MacroSeries, MetricKey, StateResponse } from "./types";
import { formatValuation, valuationRows, type ValuationMetricKey } from "./valuationData";

const metrics: MetricKey[] = ["revenueB", "capexB", "netIncomeB", "dilutedEps", "peRatio"];
type ViewMode = "compare" | "ticker" | "models";
type UniverseKey = "core" | "compute" | "asia" | "all";

const universes: { key: UniverseKey; label: string; tickers: string[] }[] = [
  { key: "core", label: "Core", tickers: ["AMZN", "GOOGL", "META", "MSFT", "SPY", "QQQ"] },
  { key: "compute", label: "Compute", tickers: ["AMD", "NVDA", "MU", "SMCI", "DELL", "QQQ"] },
  { key: "asia", label: "Asia / ADR", tickers: ["005930.KS", "BABA", "JD", "QQQ"] },
  { key: "all", label: "All", tickers: [] },
];

function App() {
  const [payload, setPayload] = useState<StateResponse | null>(null);
  const [details, setDetails] = useState<Record<string, Equity>>({});
  const [error, setError] = useState("");
  const [selected, setSelected] = useState("AMZN");
  const [mode, setMode] = useState<ViewMode>("compare");
  const [metric, setMetric] = useState<MetricKey>("capexB");
  const [input, setInput] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [loadingDetail, setLoadingDetail] = useState("");

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

  const loadDetail = useCallback(async (ticker: string) => {
    if (!ticker) return;
    setLoadingDetail(ticker);
    try {
      const detail = await api.equity(ticker);
      setDetails((current) => ({ ...current, [ticker]: detail }));
      setError("");
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : `Unable to load ${ticker}`);
    } finally {
      setLoadingDetail((current) => current === ticker ? "" : current);
    }
  }, []);

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 30_000);
    return () => window.clearInterval(timer);
  }, [load]);

  const equities = useMemo(() => Object.values(payload?.state.tickers ?? {}).sort((a, b) => a.ticker.localeCompare(b.ticker)), [payload]);
  const overviewEquity = payload?.state.tickers[selected] ?? equities[0];
  const selectedEquity = details[selected] ?? overviewEquity;
  const refreshCount = (payload?.runtime.inFlight ?? 0) + (payload?.runtime.macroRefreshing ? 1 : 0);

  useEffect(() => {
    if (mode === "compare" || !selected || loadingDetail === selected) return;
    const detail = details[selected];
    if (!detail || detail.updatedAt !== overviewEquity?.updatedAt) void loadDetail(selected);
  }, [details, loadDetail, loadingDetail, mode, overviewEquity?.updatedAt, selected]);

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
    setDetails((current) => {
      const next = { ...current };
      delete next[ticker];
      return next;
    });
    await load();
  }

  async function removeTicker(ticker: string) {
    await api.removeTicker(ticker);
    setDetails((current) => {
      const next = { ...current };
      delete next[ticker];
      return next;
    });
    setMode("compare");
    await load();
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand"><BarChart3 size={21} /><strong>Equities</strong><a href="/monetary/"><Landmark size={14} />Monetary</a><span>parallel-ocean</span></div>
        <form className="ticker-form" onSubmit={addTicker}>
          <label htmlFor="ticker-input">Add ticker</label>
          <input id="ticker-input" value={input} onChange={(event) => setInput(event.target.value.toUpperCase())} placeholder="NVDA" maxLength={10} autoComplete="off" />
          <button type="submit" disabled={submitting || !input.trim()} aria-label="Add ticker"><Plus size={17} /></button>
        </form>
        <div className="freshness"><span className={refreshCount ? "status-dot active" : "status-dot"} />{refreshCount ? `${refreshCount} refreshing` : `Updated ${timeAgo(payload?.state.updatedAt)}`}</div>
      </header>

      {error && <div className="error-banner" role="alert">{error}</div>}

      <div className="workspace">
        <TickerRail equities={equities} selected={selectedEquity?.ticker ?? ""} onSelect={(ticker) => { setSelected(ticker); setMode("ticker"); }} />
        <main className="content">
          <div className="view-toolbar">
            <div className="segmented" aria-label="View">
              <button type="button" className={mode === "compare" ? "is-active" : ""} onClick={() => setMode("compare")}><GitCompareArrows size={15} />Compare</button>
              <button type="button" className={mode === "ticker" ? "is-active" : ""} onClick={() => setMode("ticker")} disabled={!selectedEquity}><TrendingUp size={15} />Details</button>
              <button type="button" className={mode === "models" ? "is-active" : ""} onClick={() => setMode("models")} disabled={!selectedEquity || selectedEquity.annuals.length === 0}><Calculator size={15} />Models</button>
            </div>
          </div>

          {mode === "compare" && <CompareView equities={equities} metric={metric} onMetric={setMetric} macro={payload?.state.macro} />}
          {mode === "ticker" && selectedEquity && <TickerView equity={selectedEquity} loading={loadingDetail === selectedEquity.ticker} onRefresh={refreshTicker} onRemove={removeTicker} />}
          {mode === "models" && selectedEquity && <ModelsView equity={selectedEquity} loading={loadingDetail === selectedEquity.ticker} />}
        </main>
      </div>
    </div>
  );
}

function CompareView({ equities, metric, onMetric, macro }: { equities: Equity[]; metric: MetricKey; onMetric: (metric: MetricKey) => void; macro?: MacroSeries }) {
  const [basis, setBasis] = useState<HistoryBasis>("actual");
  const [range, setRange] = useState<HistoryRange>("max");
  const [valuationMetric, setValuationMetric] = useState<ValuationMetricKey>("pe");
  const [universe, setUniverse] = useState<UniverseKey>("core");
  const activeUniverse = universes.find((candidate) => candidate.key === universe) ?? universes[0];
  const selectedEquities = useMemo(() => {
    if (activeUniverse.key === "all") return equities;
    const members = new Set(activeUniverse.tickers);
    return equities.filter((equity) => members.has(equity.ticker));
  }, [activeUniverse, equities]);
  const fundamentalEquities = useMemo(() => selectedEquities.filter((equity) => equity.annuals.length > 0), [selectedEquities]);
  const domain = useMemo(() => historyDomain(selectedEquities, macro?.points ?? [], range), [macro?.points, range, selectedEquities]);
  const valuationDomain = useMemo(() => valuationHistoryDomain(fundamentalEquities, range), [fundamentalEquities, range]);
  return (
    <section className="view">
      <div className="view-title compare-title"><div><h1>Market history</h1><span>{selectedEquities.length} instruments / {domainLabel(domain)}</span></div>
        <div className="compare-controls">
          <div className="segmented universe-switch" aria-label="Comparison universe">
            {universes.map((candidate) => <button type="button" key={candidate.key} className={universe === candidate.key ? "is-active" : ""} onClick={() => setUniverse(candidate.key)}>{candidate.label}</button>)}
          </div>
          <div className="segmented compact-segmented" aria-label="History range">
            {(["max", "25y", "15y", "10y"] as HistoryRange[]).map((value) => <button type="button" key={value} className={range === value ? "is-active" : ""} onClick={() => setRange(value)}>{value === "max" ? "Max" : value.toUpperCase()}</button>)}
          </div>
        </div>
      </div>
      <PerformanceChart equities={selectedEquities} domain={domain} />
      <div className="section-heading"><div><h2>Valuation history</h2><span>{fundamentalEquities.length} companies / filing-date coverage {domainLabel(valuationDomain)}</span></div></div>
      <div className="history-toolbar">
        <div className="metric-tabs valuation-tabs" aria-label="Valuation metric">
          {valuationRows.map((row) => <button type="button" key={row.key} className={valuationMetric === row.key ? "is-active" : ""} onClick={() => setValuationMetric(row.key)}>{row.label}</button>)}
        </div>
        <div className="history-switches">
          <div className="segmented compact-segmented" aria-label="Valuation basis">
            <button type="button" className={basis === "actual" ? "is-active" : ""} onClick={() => setBasis("actual")}>LTM</button>
            <button type="button" className={basis === "forward" ? "is-active" : ""} onClick={() => setBasis("forward")}>N12M realized</button>
          </div>
        </div>
      </div>
      <ValuationHistoryCharts equities={fundamentalEquities} metric={valuationMetric} basis={basis} domain={valuationDomain} />
      <div className="section-heading"><div><h2>Monetary context</h2><span>Monthly FRED series / <a href="/monetary/">open full analysis</a></span></div></div>
      <MacroCharts macro={macro} domain={domain} />
      <div className="section-heading"><div><h2>Current valuation</h2><span>Sortable LTM and internal model snapshot</span></div></div>
      <ValuationMatrix equities={fundamentalEquities} />
      <div className="section-heading"><div><h2>Operating trajectories</h2><span>Annual actuals and estimates</span></div></div>
      <div className="metric-tabs annual-tabs">{metrics.map((key) => <button type="button" key={key} className={metric === key ? "is-active" : ""} onClick={() => onMetric(key)}>{metricLabels[key]}</button>)}</div>
      <MetricChart equities={fundamentalEquities} metric={metric} />
      <div className="small-multiples">
        {metrics.filter((key) => key !== metric).map((key) => <MetricChart key={key} equities={fundamentalEquities} metric={key} compact />)}
      </div>
    </section>
  );
}

function domainLabel(domain: [number, number]) {
  return `${new Date(domain[0]).getUTCFullYear()}-${new Date(domain[1]).getUTCFullYear()}`;
}

function TickerView({ equity, loading, onRefresh, onRemove }: { equity: Equity; loading: boolean; onRefresh: (ticker: string) => Promise<void>; onRemove: (ticker: string) => Promise<void> }) {
  const peRow = valuationRows[0];
  const ebitdaRow = valuationRows[1];
  const fcfRow = valuationRows[3];
  return (
    <section className="view">
      <TickerTitle equity={equity} onRefresh={onRefresh} onRemove={onRemove} />
      {equity.status === "error" && <div className="inline-error">{equity.error}</div>}
      {equity.annuals.length === 0 && equity.prices?.length ? <MarketOnlyView equity={equity} /> : equity.annuals.length === 0 ? <Pending equity={equity} /> : <>
        <div className="metric-strip">
          <Metric label="Price" value={money(equity.current.price)} context={equity.current.return1Y === undefined ? "1Y n/a" : `${percent(equity.current.return1Y)} 1Y`} valueTone={tone(equity.current.return1Y)} />
          <Metric label="P/E" value={formatValuation(equity.valuation?.pe, peRow.kind)} context={`Model ${formatValuation(equity.valuation?.forwardPe, peRow.kind)}`} />
          <Metric label="EV / EBITDA" value={formatValuation(equity.valuation?.evToEbitda, ebitdaRow.kind)} context={`Model ${formatValuation(equity.valuation?.forwardEvToEbitda, ebitdaRow.kind)}`} />
          <Metric label="FCF / market cap" value={formatValuation(equity.valuation?.fcfToMarketCap, fcfRow.kind)} context={`Model ${formatValuation(equity.valuation?.forwardFcfToMarketCap, fcfRow.kind)}`} />
        </div>
        {loading && !(equity.quarterlies?.length) ? <PendingDetail ticker={equity.ticker} /> : <>
          <div className="section-heading"><div><h2>Quarterly trajectories</h2><span>{equity.quarterlies?.length ?? 0} persisted periods</span></div></div>
          <div className="detail-charts quarterly-charts">
            <QuarterlyChart equity={equity} metric="revenueB" />
            <QuarterlyChart equity={equity} metric="ebitdaB" />
            <QuarterlyChart equity={equity} metric="fcfB" />
            <QuarterlyChart equity={equity} metric="netDebtB" />
            <BalanceSheetChart equity={equity} />
          </div>
          <QuarterlyTable equity={equity} />
        </>}
        <div className="section-heading"><div><h2>Market and annual history</h2><span>{analysisDate(equity)}</span></div></div>
        <PriceChart equity={equity} />
        <AnnualTable equity={equity} />
      </>}
      {!!equity.warnings?.length && <div className="warnings">{equity.warnings.map((warning) => <span key={warning}>{warning}</span>)}</div>}
    </section>
  );
}

function MarketOnlyView({ equity }: { equity: Equity }) {
  const latest = equity.prices?.[equity.prices.length - 1];
  const first = equity.prices?.[0];
  const totalReturn = first && latest && first.close > 0 ? latest.close / first.close - 1 : undefined;
  return <>
    <div className="metric-strip market-metric-strip">
      <Metric label="Price" value={money(equity.current.price)} context={equity.current.priceAsOf || "latest close"} />
      <Metric label="1 year" value={percent(equity.current.return1Y)} context="price return" valueTone={tone(equity.current.return1Y)} />
      <Metric label="52 week high" value={money(equity.current.high52Week)} context={equity.current.low52Week === undefined ? "range unavailable" : `Low ${money(equity.current.low52Week)}`} />
      <Metric label="Full history" value={percent(totalReturn)} context={first ? `since ${first.date.slice(0, 4)}` : "history pending"} valueTone={tone(totalReturn)} />
    </div>
    <div className="section-heading"><div><h2>Market history</h2><span>{equity.prices?.length ?? 0} monthly observations</span></div></div>
    <PriceChart equity={equity} />
  </>;
}

function ModelsView({ equity, loading }: { equity: Equity; loading: boolean }) {
  return (
    <section className="view">
      <div className="view-title"><div><h1>{equity.ticker} <span>valuation models</span></h1><small>{equity.company} · {equity.valuation?.asOf ?? analysisDate(equity)}</small></div></div>
      {loading && !equity.forecast?.forwardFcfB ? <PendingDetail ticker={equity.ticker} /> : <ValuationWorkbench equity={equity} />}
    </section>
  );
}

function TickerTitle({ equity, onRefresh, onRemove }: { equity: Equity; onRefresh: (ticker: string) => Promise<void>; onRemove: (ticker: string) => Promise<void> }) {
  return <div className="view-title ticker-title">
    <div><h1>{equity.ticker} <span>{equity.company}</span></h1><small>{equity.instrumentType ? `${equity.instrumentType} · ` : ""}{equity.sources?.join(" + ") || "Analysis pending"} · {analysisDate(equity)}</small></div>
    <div className="icon-actions">
      <button type="button" onClick={() => void onRefresh(equity.ticker)} disabled={equity.status === "refreshing"} aria-label={`Refresh ${equity.ticker}`} title="Refresh analysis"><RefreshCw size={17} className={equity.status === "refreshing" ? "spin" : ""} /></button>
      <button type="button" onClick={() => void onRemove(equity.ticker)} aria-label={`Remove ${equity.ticker}`} title="Remove ticker"><Trash2 size={17} /></button>
    </div>
  </div>;
}

function Pending({ equity }: { equity: Equity }) {
  return <div className="analysis-pending">
    {equity.status !== "error" && <LoaderCircle className="spin" size={20} />}
    <div><strong>{equity.status === "error" ? "Analysis unavailable" : "Analysis in progress"}</strong><span>{equity.ticker}</span></div>
  </div>;
}

function PendingDetail({ ticker }: { ticker: string }) {
  return <div className="analysis-pending"><LoaderCircle className="spin" size={20} /><div><strong>Loading quarterly archive</strong><span>{ticker}</span></div></div>;
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
