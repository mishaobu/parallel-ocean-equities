import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { BarChart3, Calculator, Download, GitCompareArrows, Globe2, ImageDown, Landmark, Link2, LoaderCircle, Pin, Save, TrendingUp, X } from "lucide-react";
import { api } from "./api";
import { metricLabels } from "./chartData";
import { historyDomain, qualityHistoryDomain, valuationHistoryDomain, type HistoryBasis, type HistoryRange } from "./historyData";
import { AnnualTable } from "./components/AnnualTable";
import { MacroCharts } from "./components/MacroCharts";
import { MetricChart } from "./components/MetricChart";
import { PerformanceChart } from "./components/PerformanceChart";
import { PriceChart } from "./components/PriceChart";
import { QuarterlyTable } from "./components/QuarterlyTable";
import { QualityHistoryCharts } from "./components/QualityHistoryCharts";
import { QualityMatrix } from "./components/QualityMatrix";
import { TickerRail } from "./components/TickerRail";
import { ValuationMatrix } from "./components/ValuationMatrix";
import { ValuationHistoryCharts } from "./components/ValuationHistoryCharts";
import { ValuationWorkbench } from "./components/ValuationWorkbench";
import { StatisticsExplorer } from "./components/StatisticsExplorer";
import { equityCurrency, formatCurrencyValue } from "./statisticsData";
import type { Equity, LiveQuote, MacroSeries, MetricKey, StateResponse } from "./types";
import { mergeUniverses, parseUniverseTickers, resolveSharedUniverse, resolveUniverseKey } from "./universeState";
import { valuationRows, type ValuationMetricKey } from "./valuationData";
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
	const [loadingDetail, setLoadingDetail] = useState("");
	const [stateLoading, setStateLoading] = useState(true);
	const [initialRetryCount, setInitialRetryCount] = useState(0);
	const refreshCount = (payload?.runtime.inFlight ?? 0) + (payload?.runtime.macroRefreshing ? 1 : 0);

  const load = useCallback(async () => {
		setStateLoading(true);
    try {
      const next = await api.state();
      setPayload(next);
      setError("");
			setInitialRetryCount(0);
      if (!next.state.tickers[selected]) setSelected(Object.keys(next.state.tickers).sort()[0] ?? "");
			return true;
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to load equities");
			return false;
		} finally {
			setStateLoading(false);
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

	useEffect(() => {
		if (payload || !error || initialRetryCount >= 3) return;
		const timer = window.setTimeout(() => {
			setInitialRetryCount((count) => count + 1);
			void load();
		}, 1_500 * (initialRetryCount + 1));
		return () => window.clearTimeout(timer);
	}, [error, initialRetryCount, load, payload]);

  const equities = useMemo(() => Object.values(payload?.state.tickers ?? {}).sort((a, b) => a.ticker.localeCompare(b.ticker)), [payload]);
  const overviewEquity = payload?.state.tickers[selected] ?? equities[0];
  const selectedEquity = details[selected] ?? overviewEquity;

  useEffect(() => {
    if (mode === "compare" || !selected || loadingDetail === selected) return;
    const detail = details[selected];
    if (!detail || detail.updatedAt !== overviewEquity?.updatedAt) void loadDetail(selected);
  }, [details, loadDetail, loadingDetail, mode, overviewEquity?.updatedAt, selected]);

	useEffect(() => { const url = new URL(window.location.href); url.searchParams.set("view", mode); url.searchParams.set("ticker", selected); url.searchParams.set("metric", metric); window.history.replaceState({}, "", url); }, [metric, mode, selected]);

  return (
    <div className="app-shell">
      <header className="topbar">
		<div className="brand"><BarChart3 size={21} /><strong>Equities</strong><a href={`/monetary/?ticker=${encodeURIComponent(selected)}&view=equity`}><Landmark size={14} />Monetary</a><a href="/macro/"><Globe2 size={14} />Macro</a><span>parallel-ocean</span></div>
		<div className="read-only-badge">Read-only analytics</div>
        <div className="freshness" role="status" aria-live="polite"><span className={refreshCount ? "status-dot active" : "status-dot"} />{refreshCount ? `${refreshCount} refreshing` : `Updated ${timeAgo(payload?.state.updatedAt)}`}</div>
      </header>

		{error && <div className="error-banner" role="alert"><span>{payload ? `Live refresh failed; showing data from ${timeAgo(payload.state.updatedAt)}. ${error}` : error}</span>{!payload && <button type="button" onClick={() => { setInitialRetryCount(3); void load(); }} disabled={stateLoading}>Retry now</button>}</div>}

		{!payload ? <div className="loading" role="status" aria-live="polite" aria-busy={stateLoading}><LoaderCircle className="spin" size={22} aria-hidden="true" /><span>{stateLoading ? "Loading equity workspace" : "Equity workspace is temporarily unavailable"}</span></div> : <div className="workspace">
			<TickerRail equities={equities} selected={selectedEquity?.ticker ?? ""} onSelect={(ticker) => { setSelected(ticker); setMode("ticker"); }} />
        <main className="content">
          <div className="view-toolbar">
			<div className="segmented" role="group" aria-label="View">
              <button type="button" className={mode === "compare" ? "is-active" : ""} aria-pressed={mode === "compare"} onClick={() => setMode("compare")}><GitCompareArrows size={15} />Compare</button>
              <button type="button" className={mode === "ticker" ? "is-active" : ""} aria-pressed={mode === "ticker"} onClick={() => setMode("ticker")} disabled={!selectedEquity}><TrendingUp size={15} />Details</button>
              <button type="button" className={mode === "models" ? "is-active" : ""} aria-pressed={mode === "models"} onClick={() => setMode("models")} disabled={!selectedEquity || selectedEquity.annuals.length === 0}><Calculator size={15} />Models</button>
			</div>
			</div>

          {mode === "compare" && <CompareView equities={equities} metric={metric} onMetric={setMetric} macro={payload?.state.macro} />}
          {mode === "ticker" && selectedEquity && <TickerView equity={selectedEquity} benchmark={details.SPY ?? payload.state.tickers.SPY} loading={loadingDetail === selectedEquity.ticker} />}
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
	const [savedUniverses, setSavedUniverses] = useState<Array<{ key: string; label: string; tickers: string[] }>>(() => loadJSON("equity-universes", []));
	const [sharedUniverse, setSharedUniverse] = useState<{ key: string; label: string; tickers: string[] } | undefined>(() => {
		const query = new URLSearchParams(window.location.search);
		const requested = query.get("universe");
		const tickers = parseUniverseTickers(query.get("universeTickers"));
		return resolveSharedUniverse(requested, tickers, universes.map((candidate) => candidate.key));
	});
	const [universe, setUniverse] = useState<UniverseKey>(() => {
		const query = new URLSearchParams(window.location.search);
		return resolveUniverseKey(query.get("universe"), [...universes, ...savedUniverses].map((candidate) => candidate.key), parseUniverseTickers(query.get("universeTickers")));
	});
	const [selectedDomain, setSelectedDomain] = useState<[number, number] | undefined>(() => initialDateDomain());
	const [hiddenTickers, setHiddenTickers] = useState<Set<string>>(() => new Set((new URLSearchParams(window.location.search).get("hidden") || "").split(",").filter(Boolean)));
	const [universeName, setUniverseName] = useState("");
	const [pinnedMetrics, setPinnedMetrics] = useState<MetricKey[]>(() => loadJSON("equity-pinned-metrics", []));
	const [actionMessage, setActionMessage] = useState("");
	const allUniverses = mergeUniverses(universes, savedUniverses, sharedUniverse);
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
		if (!universes.some((candidate) => candidate.key === activeUniverse.key)) url.searchParams.set("universeTickers", activeUniverse.tickers.join(",")); else url.searchParams.delete("universeTickers");
		if (hiddenTickers.size) url.searchParams.set("hidden", [...hiddenTickers].sort().join(",")); else url.searchParams.delete("hidden");
		if (selectedDomain) { url.searchParams.set("from", dateInput(selectedDomain[0])); url.searchParams.set("to", dateInput(selectedDomain[1])); } else { url.searchParams.delete("from"); url.searchParams.delete("to"); }
		window.history.replaceState({}, "", url);
	}, [activeUniverse.key, activeUniverse.tickers, basis, hiddenTickers, metric, qualityMetric, range, selectedDomain, universe, valuationMetric]);
	function saveUniverse(event: FormEvent) { event.preventDefault(); const label = universeName.trim(); if (!label) return; const key = `saved:${label.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`; const tickers = selectedEquities.filter((equity) => !hiddenTickers.has(equity.ticker)).map((equity) => equity.ticker); const next = [...savedUniverses.filter((item) => item.key !== key), { key, label, tickers }]; setSavedUniverses(next); setSharedUniverse((current) => current?.key === key ? undefined : current); localStorage.setItem("equity-universes", JSON.stringify(next)); setUniverse(key); setUniverseName(""); setActionMessage(`Saved ${label}`); }
	function removeSavedUniverse() { if (!universe.startsWith("saved:")) return; const next = savedUniverses.filter((item) => item.key !== universe); setSavedUniverses(next); localStorage.setItem("equity-universes", JSON.stringify(next)); setUniverse("core"); setSelectedDomain(undefined); }
	function pinMetric() { const next = pinnedMetrics.includes(metric) ? pinnedMetrics.filter((item) => item !== metric) : [...pinnedMetrics, metric]; setPinnedMetrics(next); localStorage.setItem("equity-pinned-metrics", JSON.stringify(next)); }
	async function action(run: () => void | Promise<void>, success: string) { try { await run(); setActionMessage(success); } catch (error) { setActionMessage(error instanceof Error ? error.message : "Action failed"); } }
  return (
    <section className="view">
		<div className="view-title compare-title"><div><h1>Market history</h1><span>{selectedEquities.length} instruments / {domainLabel(displayDomain)}</span></div>
        <div className="compare-controls">
			<div className="segmented universe-switch" role="group" aria-label="Comparison universe">
				{allUniverses.map((candidate) => <button type="button" key={candidate.key} className={universe === candidate.key ? "is-active" : ""} aria-pressed={universe === candidate.key} onClick={() => { setUniverse(candidate.key); setSelectedDomain(undefined); }}>{candidate.label}</button>)}
			</div>
			<div className="segmented compact-segmented" role="group" aria-label="History range">
				{(["max", "25y", "15y", "10y"] as HistoryRange[]).map((value) => <button type="button" key={value} className={range === value ? "is-active" : ""} aria-pressed={range === value} onClick={() => { setRange(value); setSelectedDomain(undefined); }}>{value === "max" ? "Max" : value.toUpperCase()}</button>)}
			</div>
			<div className="date-range" role="group" aria-label="Custom comparison period"><label>From<input type="date" min={dateInput(domain[0])} max={dateInput(displayDomain[1])} value={dateInput(displayDomain[0])} onChange={(event) => setDateDomain(event.target.value, 0, displayDomain, setSelectedDomain)} /></label><label>To<input type="date" min={dateInput(displayDomain[0])} max={dateInput(domain[1])} value={dateInput(displayDomain[1])} onChange={(event) => setDateDomain(event.target.value, 1, displayDomain, setSelectedDomain)} /></label></div>
        </div>
      </div>
		<div className="workspace-actions"><form onSubmit={saveUniverse}><input aria-label="Saved universe name" value={universeName} onChange={(event) => setUniverseName(event.target.value)} placeholder="Universe name" /><button type="submit" title="Save visible tickers as a universe"><Save size={14} />Save</button>{sharedUniverse?.key !== universe && savedUniverses.some((candidate) => candidate.key === universe) && <button type="button" title="Delete selected saved universe" aria-label="Delete selected saved universe" onClick={removeSavedUniverse}><X size={14} /></button>}</form><div><button type="button" onClick={() => exportEquitiesCSV(selectedEquities.filter((equity) => !hiddenTickers.has(equity.ticker)))} title="Export visible comparison data as CSV"><Download size={14} />CSV</button><button type="button" onClick={() => void action(exportPrimaryChartPNG, "Chart exported")} title="Export primary chart as PNG"><ImageDown size={14} />PNG</button><button type="button" onClick={() => void action(copyCurrentLink, "Link copied")} title="Copy a deep link to this workspace"><Link2 size={14} />Link</button></div><span role="status" aria-live="polite">{actionMessage}</span></div>
		<PerformanceChart equities={selectedEquities} domain={domain} zoom={selectedDomain} onZoom={updateDomain} hiddenKeys={hiddenTickers} onHiddenKeys={setHiddenTickers} />
      <div className="section-heading"><div><h2>Valuation history</h2><span>{fundamentalEquities.length} companies / filing-date coverage {domainLabel(valuationDomain)}</span></div></div>
      <div className="history-toolbar">
			<div className="metric-tabs valuation-tabs" role="group" aria-label="Valuation metric">
				{valuationRows.map((row) => <button type="button" key={row.key} className={valuationMetric === row.key ? "is-active" : ""} aria-pressed={valuationMetric === row.key} onClick={() => setValuationMetric(row.key)}>{row.label}</button>)}
        </div>
        <div className="history-switches">
				<div className="segmented compact-segmented" role="group" aria-label="Valuation basis">
					<button type="button" className={basis === "actual" ? "is-active" : ""} aria-pressed={basis === "actual"} onClick={() => setBasis("actual")}>LTM</button>
					<button type="button" className={basis === "forward" ? "is-active" : ""} aria-pressed={basis === "forward"} onClick={() => setBasis("forward")}>N12M realized</button>
          </div>
        </div>
      </div>
		<ValuationHistoryCharts equities={fundamentalEquities} metric={valuationMetric} basis={basis} domain={valuationDomain} zoom={selectedDomain} onZoom={updateDomain} hiddenKeys={hiddenTickers} onHiddenKeys={setHiddenTickers} />
      <div className="section-heading"><div><h2>Monetary context</h2><span>Monthly FRED series / <a href="/monetary/">open full analysis</a></span></div></div>
		<MacroCharts macro={macro} domain={domain} zoom={selectedDomain} onZoom={updateDomain} />
      <div className="section-heading"><div><h2>Current valuation</h2><span>Sortable LTM and internal model snapshot</span></div></div>
      <ValuationMatrix equities={fundamentalEquities} />
      <div className="section-heading"><div><h2>Operating quality</h2><span>Cash conversion, margins, working capital, returns and dilution</span></div></div>
		<div className="metric-tabs quality-tabs" role="group" aria-label="Operating quality metric">
			{qualityRows.map((row) => <button type="button" key={row.key} className={qualityMetric === row.key ? "is-active" : ""} aria-pressed={qualityMetric === row.key} onClick={() => setQualityMetric(row.key)}>{row.label}</button>)}
      </div>
		<QualityHistoryCharts equities={fundamentalEquities} metric={qualityMetric} domain={qualityDomain} zoom={selectedDomain} onZoom={updateDomain} hiddenKeys={hiddenTickers} onHiddenKeys={setHiddenTickers} />
      <div className="section-heading compact-heading"><div><h2>Current operating quality</h2><span>Sortable trailing snapshot</span></div></div>
      <QualityMatrix equities={fundamentalEquities} />
      <div className="section-heading"><div><h2>Operating trajectories</h2><span>Annual actuals and estimates</span></div></div>
		<div className="metric-tabs annual-tabs" role="group" aria-label="Operating trajectory metric">{metrics.map((key) => <button type="button" key={key} className={metric === key ? "is-active" : ""} aria-pressed={metric === key} onClick={() => onMetric(key)}>{metricLabels[key]}</button>)}</div>
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

function TickerView({ equity, benchmark, loading }: { equity: Equity; benchmark?: Equity; loading: boolean }) {
  const [quote, setQuote] = useState<LiveQuote>();
  const [quoteError, setQuoteError] = useState("");
	const [benchmarkDetail, setBenchmarkDetail] = useState<Equity>();
	const activeQuote = quote?.ticker === equity.ticker ? quote : undefined;
	const statisticsBenchmark = equity.ticker === "SPY" ? equity : benchmarkDetail?.ticker === "SPY" ? benchmarkDetail : undefined;
	useEffect(() => {
		if (equity.ticker === "SPY" || benchmark?.ticker !== "SPY") { setBenchmarkDetail(undefined); return; }
		let active = true;
		void api.equity("SPY").then((detail) => { if (active) setBenchmarkDetail(detail); }).catch(() => { if (active) setBenchmarkDetail(undefined); });
		return () => { active = false; };
	}, [benchmark?.ticker, benchmark?.updatedAt, equity.ticker]);
  useEffect(() => {
    let active = true;
    let historyLoaded = false;
    const update = () => {
      if (document.visibilityState === "hidden") return;
      const includeHistory = !historyLoaded;
      void api.quote(equity.ticker, includeHistory).then((next) => {
        if (!active) return;
				setQuote((current) => includeHistory ? next : { ...next, history: current?.ticker === next.ticker ? current.history : undefined });
        historyLoaded = true;
        setQuoteError("");
      }).catch((requestError) => { if (active) setQuoteError(requestError instanceof Error ? requestError.message : "Live quote unavailable"); });
    };
    setQuote(undefined); setQuoteError(""); update();
    const timer = window.setInterval(update, 30_000);
		const onVisibilityChange = () => { if (document.visibilityState === "visible") update(); };
		document.addEventListener("visibilitychange", onVisibilityChange);
		return () => { active = false; window.clearInterval(timer); document.removeEventListener("visibilitychange", onVisibilityChange); };
  }, [equity.ticker]);
  return (
    <section className="view">
			<TickerTitle equity={equity} quote={activeQuote} />
			{equity.status === "error" && <div className="inline-error" role="alert">{equity.error}</div>}
			{quoteError && <div className="quote-fallback" role="status">Live market update unavailable; current values fall back to the persisted close.</div>}
      {equity.annuals.length === 0 && !(equity.prices?.length) ? <Pending equity={equity} /> : <>
				<StatisticsExplorer equity={equity} quote={activeQuote} benchmark={statisticsBenchmark} />
        {loading && !(equity.quarterlies?.length) ? <PendingDetail ticker={equity.ticker} /> : equity.annuals.length > 0 && <details className="statement-archive">
          <summary><span><strong>Full statements & source records</strong><small>{equity.quarterlies?.length ?? 0} quarters · {equity.annuals.length} fiscal years</small></span><span>Expand</span></summary>
          <div className="statement-archive-content">
            <div className="section-heading"><div><h2>Quarterly statements</h2><span>Reported and derived periods with filing links</span></div></div>
            <QuarterlyTable equity={equity} />
            <div className="section-heading"><div><h2>Operating quality</h2><span>Trailing twelve months</span></div></div>
            <QualityMatrix equities={[equity]} />
            <div className="section-heading"><div><h2>Market and annual history</h2><span>{analysisDate(equity)}</span></div></div>
            <PriceChart equity={equity} />
            <AnnualTable equity={equity} />
          </div>
        </details>}
      </>}
      {!!equity.warnings?.length && <div className="warnings">{equity.warnings.map((warning) => <span key={warning}>{warning}</span>)}</div>}
    </section>
  );
}

function ModelsView({ equity, loading }: { equity: Equity; loading: boolean }) {
  return (
    <section className="view">
		<div className="view-title"><div><h1>{equity.ticker} <span>valuation models</span></h1><small>{equity.company} · Fundamentals {equity.valuation?.asOf ?? "pending"} · Price {equity.current.priceAsOf ?? "pending"}</small></div></div>
      {loading && !equity.forecast?.forwardFcfB ? <PendingDetail ticker={equity.ticker} /> : <ValuationWorkbench equity={equity} />}
    </section>
  );
}

function TickerTitle({ equity, quote }: { equity: Equity; quote?: LiveQuote }) {
	const currency = equityCurrency(equity, quote);
  return <div className="view-title ticker-title">
    <div><h1>{equity.ticker} <span>{equity.company}</span></h1><small>{equity.instrumentType ? `${equity.instrumentType} · ` : ""}{equity.sources?.join(" + ") || "Analysis pending"} · {analysisDate(equity)}</small></div>
		<div className="ticker-live-quote" role="status" aria-live="polite">
			<strong>{formatCurrencyValue(quote?.price ?? equity.current.price, currency)}</strong>
      {quote?.changePercent !== undefined && <span className={quote.changePercent > 0 ? "positive" : quote.changePercent < 0 ? "negative" : ""}>{quote.changePercent > 0 ? "+" : ""}{(quote.changePercent * 100).toFixed(2)}%</span>}
      <small>{quote?.marketState || (quote ? "Market snapshot" : "Persisted close")}</small>
    </div>
  </div>;
}

function Pending({ equity }: { equity: Equity }) {
	return <div className="analysis-pending" role="status" aria-live="polite">
    {equity.status !== "error" && <LoaderCircle className="spin" size={20} />}
    <div><strong>{equity.status === "error" ? "Analysis unavailable" : "Analysis in progress"}</strong><span>{equity.ticker}</span></div>
  </div>;
}

function PendingDetail({ ticker }: { ticker: string }) {
	return <div className="analysis-pending" role="status" aria-live="polite"><LoaderCircle className="spin" size={20} /><div><strong>Loading quarterly archive</strong><span>{ticker}</span></div></div>;
}

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
