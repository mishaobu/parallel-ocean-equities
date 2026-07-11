import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { BarChart3, Calculator, Download, GitCompareArrows, Globe2, ImageDown, Landmark, Link2, LoaderCircle, Pin, Plus, RefreshCw, Save, Trash2, TrendingUp, X } from "lucide-react";
import { api } from "./api";
import { metricLabels } from "./chartData";
import { historyDomain, qualityHistoryDomain, returnValue, valuationHistoryDomain, type HistoryBasis, type HistoryRange } from "./historyData";
import { AnnualTable } from "./components/AnnualTable";
import { MacroCharts } from "./components/MacroCharts";
import { MetricChart } from "./components/MetricChart";
import { PerformanceChart } from "./components/PerformanceChart";
import { PriceChart } from "./components/PriceChart";
import { BalanceSheetChart, QuarterlyChart } from "./components/QuarterlyCharts";
import { QuarterlyTable } from "./components/QuarterlyTable";
import { QualityHistoryCharts } from "./components/QualityHistoryCharts";
import { QualityMatrix } from "./components/QualityMatrix";
import { TickerRail } from "./components/TickerRail";
import { ValuationMatrix } from "./components/ValuationMatrix";
import { ValuationHistoryCharts } from "./components/ValuationHistoryCharts";
import { ValuationWorkbench } from "./components/ValuationWorkbench";
import type { Equity, MacroSeries, MetricKey, StateResponse } from "./types";
import { formatValuation, valuationRows, type ValuationMetricKey } from "./valuationData";
import { qualityRows, type QualityMetricKey } from "./qualityData";
import { copyCurrentLink, exportEquitiesCSV, exportPrimaryChartPNG } from "./exports";

const metrics: MetricKey[] = ["revenueB", "capexB", "netIncomeB", "dilutedEps", "peRatio"];
type ViewMode = "compare" | "ticker" | "models";
type UniverseKey = string;

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
  const [selected, setSelected] = useState(() => new URLSearchParams(window.location.search).get("ticker")?.toUpperCase() || "AMZN");
  const [mode, setMode] = useState<ViewMode>(() => initialParam("view", ["compare", "ticker", "models"], "compare"));
  const [metric, setMetric] = useState<MetricKey>(() => initialParam("metric", metrics, "capexB"));
  const [input, setInput] = useState("");
	const [submitting, setSubmitting] = useState(false);
	const [loadingDetail, setLoadingDetail] = useState("");
	const [tickerPreview, setTickerPreview] = useState<{ ticker: string; company: string; instrumentType: string; source: string }>();
	const [previewError, setPreviewError] = useState("");
	const [recentlyRemoved, setRecentlyRemoved] = useState<string>();
	const refreshCount = (payload?.runtime.inFlight ?? 0) + (payload?.runtime.macroRefreshing ? 1 : 0);

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
		const timer = window.setInterval(() => void load(), refreshCount ? 10_000 : 300_000);
		return () => window.clearInterval(timer);
	}, [load, refreshCount]);

  const equities = useMemo(() => Object.values(payload?.state.tickers ?? {}).sort((a, b) => a.ticker.localeCompare(b.ticker)), [payload]);
  const overviewEquity = payload?.state.tickers[selected] ?? equities[0];
  const selectedEquity = details[selected] ?? overviewEquity;

  useEffect(() => {
    if (mode === "compare" || !selected || loadingDetail === selected) return;
    const detail = details[selected];
    if (!detail || detail.updatedAt !== overviewEquity?.updatedAt) void loadDetail(selected);
  }, [details, loadDetail, loadingDetail, mode, overviewEquity?.updatedAt, selected]);

	useEffect(() => {
		const ticker = input.trim().toUpperCase();
		setTickerPreview(undefined); setPreviewError("");
		if (!/^[A-Z0-9][A-Z0-9.-]{0,9}$/.test(ticker)) return;
		const timer = window.setTimeout(() => { void api.previewTicker(ticker).then((preview) => setTickerPreview(preview)).catch((requestError) => setPreviewError(requestError instanceof Error ? requestError.message : "Ticker not found")); }, 350);
		return () => window.clearTimeout(timer);
	}, [input]);
	useEffect(() => { const url = new URL(window.location.href); url.searchParams.set("view", mode); url.searchParams.set("ticker", selected); url.searchParams.set("metric", metric); window.history.replaceState({}, "", url); }, [metric, mode, selected]);

	async function addTicker(event: FormEvent) {
		event.preventDefault();
		const ticker = input.trim().toUpperCase();
		if (!ticker) return;
		if (!/^[A-Z0-9][A-Z0-9.-]{0,14}$/.test(ticker)) {
			setError("Enter a ticker with letters, numbers, dots, or hyphens (for example NVDA or 005930.KS)");
			return;
		}
		if (!tickerPreview || tickerPreview.ticker !== ticker) { setError(previewError || "Wait for the ticker preview before adding this instrument"); return; }
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
		if (!window.confirm(`Remove ${ticker} from this workspace? The persisted analysis will be deleted.`)) return;
		await api.removeTicker(ticker);
		setRecentlyRemoved(ticker);
    setDetails((current) => {
      const next = { ...current };
      delete next[ticker];
      return next;
    });
    setMode("compare");
    await load();
  }

	async function undoRemove() { const ticker = recentlyRemoved; if (!ticker) return; setRecentlyRemoved(undefined); await api.addTicker(ticker); setSelected(ticker); setMode("ticker"); await load(); }

  return (
    <div className="app-shell">
      <header className="topbar">
		<div className="brand"><BarChart3 size={21} /><strong>Equities</strong><a href={`/monetary/?ticker=${encodeURIComponent(selected)}&view=equity`}><Landmark size={14} />Monetary</a><a href="/macro/"><Globe2 size={14} />Macro</a><span>parallel-ocean</span></div>
		<div className="ticker-add"><form className="ticker-form" onSubmit={addTicker}>
          <label htmlFor="ticker-input">Add ticker</label>
			<input id="ticker-input" value={input} onChange={(event) => setInput(event.target.value.toUpperCase())} placeholder="NVDA or 005930.KS" maxLength={15} autoComplete="off" aria-invalid={Boolean(input && (!/^[A-Z0-9][A-Z0-9.-]{0,9}$/.test(input.trim()) || previewError))} aria-describedby="ticker-preview" />
		  <button type="submit" disabled={submitting || !tickerPreview || tickerPreview.ticker !== input.trim()} aria-label="Add validated ticker"><Plus size={17} /></button>
		</form><div id="ticker-preview" className={previewError ? "ticker-preview is-error" : "ticker-preview"}>{tickerPreview ? <><strong>{tickerPreview.ticker}</strong><span>{tickerPreview.company}</span><small>{tickerPreview.instrumentType} / {tickerPreview.source}</small></> : input.trim() ? <span>{previewError || "Validating instrument..."}</span> : <span>Enter an exchange-qualified symbol</span>}</div></div>
        <div className="freshness"><span className={refreshCount ? "status-dot active" : "status-dot"} />{refreshCount ? `${refreshCount} refreshing` : `Updated ${timeAgo(payload?.state.updatedAt)}`}</div>
      </header>

		{error && <div className="error-banner" role="alert">{payload ? `Live refresh failed; showing data from ${timeAgo(payload.state.updatedAt)}. ${error}` : error}</div>}
		{recentlyRemoved && <div className="undo-banner" role="status"><span>{recentlyRemoved} removed</span><button type="button" onClick={() => void undoRemove()}>Undo</button><button type="button" aria-label="Dismiss removal notice" onClick={() => setRecentlyRemoved(undefined)}><X size={14} /></button></div>}

		{!payload ? <div className="loading"><LoaderCircle className="spin" size={22} /><span>Loading equity workspace</span></div> : <div className="workspace">
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
		</div>}
    </div>
  );
}

function CompareView({ equities, metric, onMetric, macro }: { equities: Equity[]; metric: MetricKey; onMetric: (metric: MetricKey) => void; macro?: MacroSeries }) {
  const [basis, setBasis] = useState<HistoryBasis>(() => initialParam("basis", ["actual", "forward"], "actual"));
  const [range, setRange] = useState<HistoryRange>(() => initialParam("range", ["max", "25y", "15y", "10y"], "max"));
  const [valuationMetric, setValuationMetric] = useState<ValuationMetricKey>(() => initialParam("valuation", valuationRows.map((row) => row.key), "pe"));
	const [qualityMetric, setQualityMetric] = useState<QualityMetricKey>(() => initialParam("quality", qualityRows.map((row) => row.key), "cash-conversion"));
	const [universe, setUniverse] = useState<UniverseKey>(() => new URLSearchParams(window.location.search).get("universe") || "core");
	const [selectedDomain, setSelectedDomain] = useState<[number, number] | undefined>(() => initialDateDomain());
	const [hiddenTickers, setHiddenTickers] = useState<Set<string>>(() => new Set((new URLSearchParams(window.location.search).get("hidden") || "").split(",").filter(Boolean)));
	const [savedUniverses, setSavedUniverses] = useState<Array<{ key: string; label: string; tickers: string[] }>>(() => loadJSON("equity-universes", []));
	const [universeName, setUniverseName] = useState("");
	const [pinnedMetrics, setPinnedMetrics] = useState<MetricKey[]>(() => loadJSON("equity-pinned-metrics", []));
	const [actionMessage, setActionMessage] = useState("");
	const allUniverses = [...universes, ...savedUniverses];
  const activeUniverse = allUniverses.find((candidate) => candidate.key === universe) ?? universes[0];
  const selectedEquities = useMemo(() => {
    if (activeUniverse.key === "all") return equities;
    const members = new Set(activeUniverse.tickers);
    return equities.filter((equity) => members.has(equity.ticker));
  }, [activeUniverse, equities]);
  const fundamentalEquities = useMemo(() => selectedEquities.filter((equity) => equity.annuals.length > 0), [selectedEquities]);
  const domain = useMemo(() => historyDomain(selectedEquities, macro?.points ?? [], range), [macro?.points, range, selectedEquities]);
  const valuationDomain = useMemo(() => valuationHistoryDomain(fundamentalEquities, range), [fundamentalEquities, range]);
	const qualityDomain = useMemo(() => qualityHistoryDomain(fundamentalEquities, range), [fundamentalEquities, range]);
	const displayDomain = selectedDomain ?? domain;
	const updateDomain = (next?: [number, number]) => setSelectedDomain(next);
	useEffect(() => {
		const available = new Set(selectedEquities.map((equity) => equity.ticker));
		setHiddenTickers((current) => new Set([...current].filter((ticker) => available.has(ticker))));
	}, [selectedEquities]);
	useEffect(() => {
		const url = new URL(window.location.href);
		url.searchParams.set("range", range); url.searchParams.set("universe", universe); url.searchParams.set("basis", basis); url.searchParams.set("valuation", valuationMetric); url.searchParams.set("quality", qualityMetric); url.searchParams.set("metric", metric);
		if (hiddenTickers.size) url.searchParams.set("hidden", [...hiddenTickers].sort().join(",")); else url.searchParams.delete("hidden");
		if (selectedDomain) { url.searchParams.set("from", dateInput(selectedDomain[0])); url.searchParams.set("to", dateInput(selectedDomain[1])); } else { url.searchParams.delete("from"); url.searchParams.delete("to"); }
		window.history.replaceState({}, "", url);
	}, [basis, hiddenTickers, metric, qualityMetric, range, selectedDomain, universe, valuationMetric]);
	function saveUniverse(event: FormEvent) { event.preventDefault(); const label = universeName.trim(); if (!label) return; const key = `saved:${label.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`; const tickers = selectedEquities.filter((equity) => !hiddenTickers.has(equity.ticker)).map((equity) => equity.ticker); const next = [...savedUniverses.filter((item) => item.key !== key), { key, label, tickers }]; setSavedUniverses(next); localStorage.setItem("equity-universes", JSON.stringify(next)); setUniverse(key); setUniverseName(""); setActionMessage(`Saved ${label}`); }
	function removeSavedUniverse() { if (!universe.startsWith("saved:")) return; const next = savedUniverses.filter((item) => item.key !== universe); setSavedUniverses(next); localStorage.setItem("equity-universes", JSON.stringify(next)); setUniverse("core"); setSelectedDomain(undefined); }
	function pinMetric() { const next = pinnedMetrics.includes(metric) ? pinnedMetrics.filter((item) => item !== metric) : [...pinnedMetrics, metric]; setPinnedMetrics(next); localStorage.setItem("equity-pinned-metrics", JSON.stringify(next)); }
	async function action(run: () => void | Promise<void>, success: string) { try { await run(); setActionMessage(success); } catch (error) { setActionMessage(error instanceof Error ? error.message : "Action failed"); } }
  return (
    <section className="view">
		<div className="view-title compare-title"><div><h1>Market history</h1><span>{selectedEquities.length} instruments / {domainLabel(displayDomain)}</span></div>
        <div className="compare-controls">
          <div className="segmented universe-switch" aria-label="Comparison universe">
            {allUniverses.map((candidate) => <button type="button" key={candidate.key} className={universe === candidate.key ? "is-active" : ""} onClick={() => { setUniverse(candidate.key); setSelectedDomain(undefined); }}>{candidate.label}</button>)}
          </div>
			<div className="segmented compact-segmented" aria-label="History range">
            {(["max", "25y", "15y", "10y"] as HistoryRange[]).map((value) => <button type="button" key={value} className={range === value ? "is-active" : ""} onClick={() => { setRange(value); setSelectedDomain(undefined); }}>{value === "max" ? "Max" : value.toUpperCase()}</button>)}
			</div>
			<div className="date-range" aria-label="Custom comparison period"><label>From<input type="date" min={dateInput(domain[0])} max={dateInput(displayDomain[1])} value={dateInput(displayDomain[0])} onChange={(event) => setDateDomain(event.target.value, 0, displayDomain, setSelectedDomain)} /></label><label>To<input type="date" min={dateInput(displayDomain[0])} max={dateInput(domain[1])} value={dateInput(displayDomain[1])} onChange={(event) => setDateDomain(event.target.value, 1, displayDomain, setSelectedDomain)} /></label></div>
        </div>
      </div>
		<div className="workspace-actions"><form onSubmit={saveUniverse}><input aria-label="Saved universe name" value={universeName} onChange={(event) => setUniverseName(event.target.value)} placeholder="Universe name" /><button type="submit" title="Save visible tickers as a universe"><Save size={14} />Save</button>{universe.startsWith("saved:") && <button type="button" title="Delete selected saved universe" aria-label="Delete selected saved universe" onClick={removeSavedUniverse}><X size={14} /></button>}</form><div><button type="button" onClick={() => exportEquitiesCSV(selectedEquities.filter((equity) => !hiddenTickers.has(equity.ticker)))} title="Export visible comparison data as CSV"><Download size={14} />CSV</button><button type="button" onClick={() => void action(exportPrimaryChartPNG, "Chart exported")} title="Export primary chart as PNG"><ImageDown size={14} />PNG</button><button type="button" onClick={() => void action(copyCurrentLink, "Link copied")} title="Copy a deep link to this workspace"><Link2 size={14} />Link</button></div><span role="status">{actionMessage}</span></div>
		<PerformanceChart equities={selectedEquities} domain={domain} zoom={selectedDomain} onZoom={updateDomain} hiddenKeys={hiddenTickers} onHiddenKeys={setHiddenTickers} />
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
		<ValuationHistoryCharts equities={fundamentalEquities} metric={valuationMetric} basis={basis} domain={valuationDomain} zoom={selectedDomain} onZoom={updateDomain} hiddenKeys={hiddenTickers} onHiddenKeys={setHiddenTickers} />
      <div className="section-heading"><div><h2>Monetary context</h2><span>Monthly FRED series / <a href="/monetary/">open full analysis</a></span></div></div>
		<MacroCharts macro={macro} domain={domain} zoom={selectedDomain} onZoom={updateDomain} />
      <div className="section-heading"><div><h2>Current valuation</h2><span>Sortable LTM and internal model snapshot</span></div></div>
      <ValuationMatrix equities={fundamentalEquities} />
      <div className="section-heading"><div><h2>Operating quality</h2><span>Cash conversion, margins, working capital, returns and dilution</span></div></div>
      <div className="metric-tabs quality-tabs" aria-label="Operating quality metric">
        {qualityRows.map((row) => <button type="button" key={row.key} className={qualityMetric === row.key ? "is-active" : ""} onClick={() => setQualityMetric(row.key)}>{row.label}</button>)}
      </div>
		<QualityHistoryCharts equities={fundamentalEquities} metric={qualityMetric} domain={qualityDomain} zoom={selectedDomain} onZoom={updateDomain} hiddenKeys={hiddenTickers} onHiddenKeys={setHiddenTickers} />
      <div className="section-heading compact-heading"><div><h2>Current operating quality</h2><span>Sortable trailing snapshot</span></div></div>
      <QualityMatrix equities={fundamentalEquities} />
      <div className="section-heading"><div><h2>Operating trajectories</h2><span>Annual actuals and estimates</span></div></div>
      <div className="metric-tabs annual-tabs">{metrics.map((key) => <button type="button" key={key} className={metric === key ? "is-active" : ""} onClick={() => onMetric(key)}>{metricLabels[key]}</button>)}</div>
		<MetricChart equities={fundamentalEquities} metric={metric} zoom={selectedDomain} onZoom={updateDomain} hiddenKeys={hiddenTickers} onHiddenKeys={setHiddenTickers} />
		<div className="pin-toolbar"><button type="button" onClick={pinMetric}><Pin size={13} />{pinnedMetrics.includes(metric) ? "Unpin current chart" : "Pin current chart"}</button><span>{pinnedMetrics.length} pinned</span></div>
		{pinnedMetrics.filter((key) => key !== metric).length > 0 && <div className="small-multiples pinned-charts">{pinnedMetrics.filter((key) => key !== metric).map((key) => <MetricChart key={key} equities={fundamentalEquities} metric={key} zoom={selectedDomain} onZoom={updateDomain} hiddenKeys={hiddenTickers} onHiddenKeys={setHiddenTickers} compact />)}</div>}
    </section>
  );
}

function domainLabel(domain: [number, number]) {
  return `${new Date(domain[0]).getUTCFullYear()}-${new Date(domain[1]).getUTCFullYear()}`;
}
function dateInput(value: number) { return new Date(value).toISOString().slice(0, 10); }
function setDateDomain(value: string, index: 0 | 1, domain: [number, number], update: (domain: [number, number]) => void) { const parsed = Date.parse(`${value}T00:00:00Z`); if (!Number.isFinite(parsed)) return; update(index === 0 ? [parsed, domain[1]] : [domain[0], parsed]); }
function initialDateDomain(): [number, number] | undefined { const query = new URLSearchParams(window.location.search); const from = Date.parse(`${query.get("from")}T00:00:00Z`); const to = Date.parse(`${query.get("to")}T00:00:00Z`); return Number.isFinite(from) && Number.isFinite(to) && from < to ? [from, to] : undefined; }
function initialParam<T extends string>(key: string, values: T[], fallback: T) { const value = new URLSearchParams(window.location.search).get(key) as T | null; return value && values.includes(value) ? value : fallback; }
function loadJSON<T>(key: string, fallback: T): T { try { const value = localStorage.getItem(key); return value ? JSON.parse(value) as T : fallback; } catch { return fallback; } }

function TickerView({ equity, loading, onRefresh, onRemove }: { equity: Equity; loading: boolean; onRefresh: (ticker: string) => Promise<void>; onRemove: (ticker: string) => Promise<void> }) {
  const peRow = valuationRows[0];
  const ebitdaRow = valuationRows[1];
  const fcfRow = valuationRows.find((row) => row.key === "fcf-market-cap") ?? valuationRows[0];
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
          <div className="section-heading"><div><h2>Operating quality</h2><span>Trailing twelve months</span></div></div>
          <QualityMatrix equities={[equity]} />
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
	const totalReturn = first && latest && returnValue(first) > 0 ? returnValue(latest) / returnValue(first) - 1 : undefined;
  return <>
    <div className="metric-strip market-metric-strip">
      <Metric label="Price" value={money(equity.current.price)} context={equity.current.priceAsOf || "latest close"} />
      <Metric label="1 year" value={percent(equity.current.return1Y)} context="price return" valueTone={tone(equity.current.return1Y)} />
      <Metric label="52 week high" value={money(equity.current.high52Week)} context={equity.current.low52Week === undefined ? "range unavailable" : `Low ${money(equity.current.low52Week)}`} />
			<Metric label="Full-history total return" value={percent(totalReturn)} context={first ? `since ${first.date.slice(0, 4)}` : "history pending"} valueTone={tone(totalReturn)} />
    </div>
    <div className="section-heading"><div><h2>Market history</h2><span>{equity.prices?.length ?? 0} monthly observations</span></div></div>
    <PriceChart equity={equity} />
  </>;
}

function ModelsView({ equity, loading }: { equity: Equity; loading: boolean }) {
  return (
    <section className="view">
		<div className="view-title"><div><h1>{equity.ticker} <span>valuation models</span></h1><small>{equity.company} · Fundamentals {equity.valuation?.asOf ?? "pending"} · Price {equity.current.priceAsOf ?? "pending"}</small></div></div>
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
