import { useEffect, useMemo, useRef, useState } from "react";
import { BarChart3, Check, ChevronDown, CircleAlert, Info, Search, Star, Table2 } from "lucide-react";
import { CartesianGrid, Line, LineChart, ReferenceArea, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { ChartHeadingMeta, useChartZoom, useFittedYDomain } from "../chartInteraction";
import { buildStatisticsCatalog, filterStatisticPoints, filterStatisticTextPoints, formatStatisticValue, type StatisticMetric, type StatisticPoint, type StatisticRange, type StatisticResolution, type StatisticTextPoint } from "../statisticsData";
import type { Equity, LiveQuote } from "../types";

type Presentation = "split" | "chart" | "table";

export function StatisticsExplorer({ equity, quote, benchmark }: { equity: Equity; quote?: LiveQuote; benchmark?: Equity }) {
  const catalog = useMemo(() => buildStatisticsCatalog(equity, quote, benchmark), [benchmark, equity, quote]);
  const [selectedKey, setSelectedKey] = useState(() => initialParam("stat") || "market-cap");
  const [resolution, setResolution] = useState<StatisticResolution>(() => initialChoice("period", ["month", "quarter", "year"], "quarter"));
  const [range, setRange] = useState<StatisticRange>(() => initialChoice("statRange", ["1y", "3y", "5y", "10y", "max"], "5y"));
  const [presentation, setPresentation] = useState<Presentation>(() => initialChoice("display", ["split", "chart", "table"], window.innerWidth <= 650 ? "chart" : "split"));
  const [search, setSearch] = useState("");
  const [favorites, setFavorites] = useState<Set<string>>(() => {
    const saved = loadJSON<unknown>("equity-stat-favorites", []);
    return new Set(Array.isArray(saved) ? saved.filter((value): value is string => typeof value === "string") : []);
  });
  const selected = catalog.metrics.find((metric) => metric.key === selectedKey) ?? catalog.metrics[0];
  const [openGroups, setOpenGroups] = useState<Set<string>>(() => new Set([selected?.group ?? "Valuation measures"]));
	const inspectorRef = useRef<HTMLElement>(null);

  useEffect(() => {
    if (!catalog.metrics.some((metric) => metric.key === selectedKey)) setSelectedKey("market-cap");
  }, [catalog.metrics, selectedKey]);

  useEffect(() => {
    if (!selected) return;
    setOpenGroups((current) => current.has(selected.group) ? current : new Set([...current, selected.group]));
    const url = new URL(window.location.href);
    url.searchParams.set("stat", selected.key);
    url.searchParams.set("period", resolution);
    url.searchParams.set("statRange", range);
    url.searchParams.set("display", presentation);
    window.history.replaceState({}, "", url);
  }, [presentation, range, resolution, selected]);

  const query = search.trim().toLowerCase();
  const groups = catalog.groups.flatMap((group) => {
    const metrics = catalog.metrics.filter((metric) => metric.group === group && (!query || `${metric.label} ${metric.description} ${metric.formula ?? ""}`.toLowerCase().includes(query)));
    return metrics.length ? [{ group, metrics }] : [];
  });
	const resultCount = groups.reduce((count, group) => count + group.metrics.length, 0);
  const points = selected ? filterStatisticPoints(selected.points[resolution], range) : [];
  const textPoints = selected ? filterStatisticTextPoints(selected.textPoints[resolution], range) : [];
  const eventMetric = selected?.unit === "date" || selected?.unit === "text";

  function choose(metric: StatisticMetric) {
    setSelectedKey(metric.key);
    setOpenGroups((current) => new Set([...current, metric.group]));
		if (window.matchMedia?.("(max-width: 1150px)").matches) {
			window.requestAnimationFrame(() => {
				inspectorRef.current?.focus({ preventScroll: true });
				inspectorRef.current?.scrollIntoView({ behavior: window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "start" });
			});
		}
  }

  function toggleGroup(group: string) {
    setOpenGroups((current) => {
      const next = new Set(current);
      if (next.has(group)) next.delete(group); else next.add(group);
      return next;
    });
  }

  function toggleFavorite(key: string) {
    setFavorites((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key); else next.add(key);
      saveJSON("equity-stat-favorites", [...next]);
      return next;
    });
  }

  if (!selected) return null;
  return (
    <section className="statistics-section" aria-labelledby="statistics-heading">
      <div className="section-heading statistics-heading">
        <div><h2 id="statistics-heading">Statistics</h2><span>60 Yahoo definitions mapped · source gaps explicit</span></div>
        <div className="statistics-coverage" title="Coverage reflects values available from configured sources">
          <span><Check size={12} />{catalog.availableYahooMetricCount} current</span>
          <span>{catalog.yahooMetricCount} mapped</span>
        </div>
      </div>

      <div className="statistics-workspace">
		<aside className="statistics-catalog" aria-label="Statistics catalog">
			<div className="statistics-search" role="search">
				<Search size={15} />
				<input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search 60+ statistics" aria-label="Search statistics" aria-controls="statistics-group-list" />
				{search && <button type="button" onClick={() => setSearch("")} aria-label="Clear statistics search">Clear</button>}
			</div>
			<div className="statistics-search-status" role="status" aria-live="polite">{query ? `${resultCount} ${resultCount === 1 ? "result" : "results"} for “${search.trim()}”` : ""}</div>
			{favorites.size > 0 && !query && <MetricGroup group="Pinned" metrics={catalog.metrics.filter((metric) => favorites.has(metric.key))} selectedKey={selected.key} favorites={favorites} open collapsible={false} onChoose={choose} onToggle={() => undefined} onFavorite={toggleFavorite} />}
			{query && resultCount === 0 && <div className="statistics-no-results">No statistics match “{search.trim()}”. Try a name such as margin, volume, or moving average.</div>}
			<div className="statistics-group-list" id="statistics-group-list">
            {groups.map(({ group, metrics }) => <MetricGroup
              key={group}
              group={group}
              metrics={metrics}
              selectedKey={selected.key}
              favorites={favorites}
              open={query.length > 0 || openGroups.has(group)}
              onChoose={choose}
              onToggle={() => toggleGroup(group)}
              onFavorite={toggleFavorite}
            />)}
          </div>
        </aside>

		<article ref={inspectorRef} className="statistic-inspector" tabIndex={-1} aria-labelledby="statistic-inspector-heading">
			<p className="sr-only" role="status" aria-live="polite">{selected.label} selected</p>
			<header className="statistic-inspector-header">
            <div className="statistic-current">
              <span>{selected.group}</span>
				<h3 id="statistic-inspector-heading">{selected.label}</h3>
				<strong>{formatStatisticValue(selected.current, selected.unit, true, selected.currency)}</strong>
				<CurrentDelta metric={selected} points={points} resolution={resolution} />
			</div>
			<button type="button" className={favorites.has(selected.key) ? "favorite-button is-active" : "favorite-button"} onClick={() => toggleFavorite(selected.key)} aria-label={`${favorites.has(selected.key) ? "Unpin" : "Pin"} ${selected.label}`} aria-pressed={favorites.has(selected.key)} title={favorites.has(selected.key) ? "Unpin statistic" : "Pin statistic"}><Star size={16} fill={favorites.has(selected.key) ? "currentColor" : "none"} /></button>
          </header>

          <div className="statistic-provenance">
            <span className={selected.current === undefined ? "source-status is-missing" : "source-status"} />
            <strong>{selected.current === undefined ? "Source needed" : freshnessLabel(selected, quote)}</strong>
            <span>{selected.currentAsOf ? `as of ${formatDateTime(selected.currentAsOf)}` : selected.nativeFrequency}</span>
            {selected.currentBasis && <span>{selected.currentBasis}</span>}
          </div>

          <div className="statistic-toolbar">
			<div className="segmented compact-segmented" role="group" aria-label="Statistic period">
              {(["month", "quarter", "year"] as StatisticResolution[]).map((value) => <button type="button" key={value} className={resolution === value ? "is-active" : ""} onClick={() => setResolution(value)} aria-pressed={resolution === value}>{value === "month" ? "Monthly" : value === "quarter" ? "Quarterly" : "Annual"}</button>)}
            </div>
			<div className="segmented compact-segmented stat-range" role="group" aria-label="Statistic range">
              {(["1y", "3y", "5y", "10y", "max"] as StatisticRange[]).map((value) => <button type="button" key={value} className={range === value ? "is-active" : ""} onClick={() => setRange(value)} aria-pressed={range === value}>{value.toUpperCase()}</button>)}
            </div>
			<div className="segmented compact-segmented presentation-switch" role="group" aria-label="Statistic presentation">
              <button type="button" className={presentation === "chart" ? "is-active" : ""} onClick={() => setPresentation("chart")} aria-label="Chart only" aria-pressed={presentation === "chart"}><BarChart3 size={14} /></button>
              <button type="button" className={presentation === "split" ? "is-active" : ""} onClick={() => setPresentation("split")} aria-label="Chart and table" aria-pressed={presentation === "split"}><BarChart3 size={13} /><Table2 size={13} /></button>
              <button type="button" className={presentation === "table" ? "is-active" : ""} onClick={() => setPresentation("table")} aria-label="Table only" aria-pressed={presentation === "table"}><Table2 size={14} /></button>
            </div>
          </div>

          <div className="statistic-definition">
            <Info size={14} />
            <div><span>{selected.description}</span>{selected.formula && <small>{selected.formula}</small>}<small>{selected.currentSource || selected.source}</small></div>
          </div>

          {selected.current === undefined && points.length === 0 && textPoints.length === 0 ? <div className="statistic-empty">
            <CircleAlert size={20} />
            <div><strong>Metric mapped; feed not configured</strong><span>{selected.unavailableReason}</span></div>
          </div> : <>
            {(presentation === "chart" || presentation === "split") && (eventMetric ? <StatisticEventTimeline metric={selected} points={textPoints} /> : <StatisticChart metric={selected} points={points} />)}
            {(presentation === "table" || presentation === "split") && (eventMetric ? <StatisticEventTable metric={selected} points={textPoints} /> : <StatisticHistoryTable metric={selected} points={points} />)}
          </>}
        </article>
      </div>
    </section>
  );
}

function StatisticEventTimeline({ metric, points }: { metric: StatisticMetric; points: StatisticTextPoint[] }) {
  return <div className="chart statistic-event-chart">
    <div className="chart-heading"><strong>{metric.label} timeline</strong><span>{points.length} events</span></div>
    {!points.length ? <div className="chart-empty">No recorded events in this range</div> : <ol className="statistic-event-track">
      {points.map((point) => <li key={`${point.date}-${point.value}`}>
        <time dateTime={point.date}>{formatDate(point.date)}</time>
        <span aria-hidden="true" />
        <div><strong>{formatTextPointValue(point.value, metric.unit)}</strong><small>{point.source}</small></div>
      </li>)}
    </ol>}
  </div>;
}

function StatisticEventTable({ metric, points }: { metric: StatisticMetric; points: StatisticTextPoint[] }) {
  if (!points.length) return <div className="statistic-table-empty">No recorded event history at this resolution.</div>;
  return <div className="table-wrap statistic-history-table statistic-event-table">
    <table>
      <thead><tr><th>Observed</th><th>Value</th><th>Source</th></tr></thead>
      <tbody>{[...points].reverse().map((point) => <tr key={`${point.date}-${point.value}`}><th><strong>{point.label}</strong><small>{formatDate(point.date)}</small></th><td>{formatTextPointValue(point.value, metric.unit)}</td><td>{point.source}</td></tr>)}</tbody>
    </table>
  </div>;
}

function formatTextPointValue(value: string, unit: StatisticMetric["unit"]) {
  return unit === "date" ? formatDate(value) : value;
}

function MetricGroup({ group, metrics, selectedKey, favorites, open, collapsible = true, onChoose, onToggle, onFavorite }: { group: string; metrics: StatisticMetric[]; selectedKey: string; favorites: Set<string>; open: boolean; collapsible?: boolean; onChoose: (metric: StatisticMetric) => void; onToggle: () => void; onFavorite: (key: string) => void }) {
  if (!metrics.length) return null;
	const id = `statistic-group-${group.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`;
	const heading = <span>{group}<small>{metrics.filter((metric) => metric.current !== undefined).length}/{metrics.length}</small></span>;
	return <section className="statistic-group" aria-labelledby={`${id}-heading`}>
		{collapsible ? <button id={`${id}-heading`} type="button" className="statistic-group-heading" onClick={onToggle} aria-expanded={open} aria-controls={`${id}-rows`}>
			{heading}<ChevronDown size={14} />
		</button> : <div id={`${id}-heading`} className="statistic-group-heading statistic-group-heading-static">{heading}</div>}
		{open && <div className="statistic-group-rows" id={`${id}-rows`}>{metrics.map((metric) => <div key={metric.key} className={metric.key === selectedKey ? "statistic-row is-selected" : "statistic-row"}>
			<button type="button" className="statistic-row-main" onClick={() => onChoose(metric)} aria-current={metric.key === selectedKey ? "true" : undefined}>
				<span>{metric.label}<small>{metricCatalogFreshnessLabel(metric)}</small></span>
        <strong>{formatStatisticValue(metric.current, metric.unit, false, metric.currency)}</strong>
      </button>
      <button type="button" className={favorites.has(metric.key) ? "statistic-row-pin is-active" : "statistic-row-pin"} onClick={() => onFavorite(metric.key)} aria-label={`${favorites.has(metric.key) ? "Unpin" : "Pin"} ${metric.label}`} aria-pressed={favorites.has(metric.key)}><Star size={12} fill={favorites.has(metric.key) ? "currentColor" : "none"} /></button>
    </div>)}</div>}
  </section>;
}

export function metricCatalogFreshnessLabel(metric: Pick<StatisticMetric, "current" | "marketSensitive" | "nativeFrequency">) {
	if (metric.current === undefined) return "feed needed";
	return metric.marketSensitive ? "Live snapshot" : metric.nativeFrequency;
}

function StatisticChart({ metric, points }: { metric: StatisticMetric; points: StatisticPoint[] }) {
  const data = points.map((point) => ({ ...point, timestamp: Date.parse(point.date) })).filter((point) => Number.isFinite(point.timestamp));
  const timestamps = data.map((point) => point.timestamp);
  const fallbackEnd = Date.now();
  const first = timestamps.length ? Math.min(...timestamps) : fallbackEnd - 24 * 60 * 60 * 1000;
  const lastValue = timestamps.length ? Math.max(...timestamps) : fallbackEnd;
  const domain: [number, number] = first === lastValue ? [first - 24 * 60 * 60 * 1000, lastValue + 24 * 60 * 60 * 1000] : [first, lastValue];
  const chart = useChartZoom(domain, 20 * 24 * 60 * 60 * 1000);
  const fitted = useFittedYDomain(data, chart.activeDomain, ["value"], "timestamp", { includeZero: false });
  const firstLabel = data[0] ? formatDate(data[0].date) : "no observations";
  const lastLabel = data[data.length - 1] ? formatDate(data[data.length - 1].date) : "no observations";
  return <div className="chart statistic-chart" role="group" aria-label={`${metric.label} history: ${data.length} observations from ${firstLabel} to ${lastLabel}`}>
    <div className="chart-heading"><strong>{metric.label} history</strong><ChartHeadingMeta unit={`${points.length} observations`} zoom={chart.zoom} onReset={chart.reset} clippedCount={fitted.clippedCount} includeOutliers={fitted.includeOutliers} onToggleOutliers={fitted.toggleOutliers} /></div>
    <div className="chart-canvas chart-gesture-surface" aria-hidden="true" {...chart.touchHandlers}>
      {data.length < 2 ? <div className="chart-empty">{data.length === 1 ? "One historical observation — use the table for detail" : "Historical series is not available for this resolution"}</div> : <ResponsiveContainer width="100%" height="100%">
        <LineChart className="interactive-chart" data={data} margin={{ top: 15, right: 22, bottom: 3, left: 4 }} onMouseDown={chart.start} onMouseMove={chart.move} onMouseUp={chart.finish} onMouseLeave={chart.finish}>
          <CartesianGrid vertical={false} stroke="#e5e9e6" />
          <XAxis dataKey="timestamp" type="number" scale="time" domain={chart.activeDomain} allowDataOverflow tickFormatter={axisDate} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} minTickGap={38} />
          <YAxis domain={fitted.domain} allowDataOverflow tickFormatter={(value) => formatStatisticValue(Number(value), metric.unit, false, metric.currency)} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} width={72} />
          <Tooltip formatter={(value) => formatStatisticValue(Number(value), metric.unit, true, metric.currency)} labelFormatter={(value) => formatDate(String(new Date(Number(value)).toISOString()))} contentStyle={{ borderColor: "#cfd7d2", borderRadius: 5, fontSize: 12 }} />
          {(metric.unit === "percent" || metric.key === "net-debt") && <ReferenceLine y={0} stroke="#aeb8b1" strokeDasharray="3 3" />}
          {chart.selection && <ReferenceArea x1={chart.selection[0]} x2={chart.selection[1]} fill="#7a9b88" fillOpacity={0.16} strokeOpacity={0} />}
          <Line type="monotone" dataKey="value" name={metric.label} stroke="#276347" strokeWidth={2.4} dot={data.length < 30 ? { r: 2.7, strokeWidth: 1.5, fill: "#fff" } : false} activeDot={{ r: 5 }} connectNulls isAnimationActive={false} />
        </LineChart>
      </ResponsiveContainer>}
    </div>
    <table className="sr-only"><caption>{metric.label} chart data</caption><thead><tr><th>Date</th><th>Value</th></tr></thead><tbody>{data.map((point) => <tr key={point.date}><td>{formatDate(point.date)}</td><td>{formatStatisticValue(point.value, metric.unit, true, metric.currency)}</td></tr>)}</tbody></table>
  </div>;
}

function StatisticHistoryTable({ metric, points }: { metric: StatisticMetric; points: StatisticPoint[] }) {
  const rows = [...points].reverse();
  if (!rows.length) return <div className="statistic-table-empty">No recorded history at this resolution.</div>;
  return <div className="table-wrap statistic-history-table">
    <table>
      <thead><tr><th>Period</th><th>Value</th><th>Change</th><th>Data available / basis</th><th>Source</th></tr></thead>
      <tbody>{rows.map((point, reverseIndex) => {
        const chronologicalIndex = points.length - reverseIndex - 1;
        const previous = points[chronologicalIndex - 1];
        return <tr key={`${point.date}-${point.label}`}><th><strong>{point.label}</strong><small>{formatDate(point.date)}</small></th><td>{formatStatisticValue(point.value, metric.unit, true, metric.currency)}</td><td>{formatPointChange(point.value, previous?.value, metric.unit, metric.currency)}</td><td>{point.basisDate ? formatDate(point.basisDate) : "Market close"}</td><td>{point.source}</td></tr>;
      })}</tbody>
    </table>
  </div>;
}

function CurrentDelta({ metric, points, resolution }: { metric: StatisticMetric; points: StatisticPoint[]; resolution: StatisticResolution }) {
  const current = typeof metric.current === "number" ? metric.current : undefined;
	const prior = previousCompletedStatisticPoint(metric, points, resolution);
  if (current === undefined || !prior || Math.abs(current - prior.value) < 1e-12) return <small>{metric.marketSensitive ? "Live snapshot" : metric.nativeFrequency}</small>;
  return <small>{formatPointChange(current, prior.value, metric.unit, metric.currency)} vs {prior.label}</small>;
}

export function previousCompletedStatisticPoint(metric: Pick<StatisticMetric, "marketSensitive" | "currentAsOf">, points: StatisticPoint[], resolution: StatisticResolution): StatisticPoint | undefined {
	if (!metric.marketSensitive || !metric.currentAsOf) return points.at(-1);
	const currentBucket = statisticPeriodBucket(metric.currentAsOf, resolution);
	for (let index = points.length - 1; index >= 0; index -= 1) {
		if (statisticPeriodBucket(points[index].date, resolution) !== currentBucket) return points[index];
	}
	return undefined;
}

function statisticPeriodBucket(value: string, resolution: StatisticResolution): string {
	const date = new Date(`${value.slice(0, 10)}T00:00:00Z`);
	if (Number.isNaN(date.getTime())) return value;
	const year = date.getUTCFullYear();
	if (resolution === "year") return String(year);
	const month = date.getUTCMonth();
	return resolution === "quarter" ? `${year}-Q${Math.floor(month / 3) + 1}` : `${year}-${String(month + 1).padStart(2, "0")}`;
}

function formatPointChange(current: number, previous: number | undefined, unit: StatisticMetric["unit"], currency?: string) {
  if (previous === undefined || !Number.isFinite(previous)) return "—";
  const difference = current - previous;
  if (unit === "percent") return `${difference >= 0 ? "+" : ""}${(difference * 100).toFixed(2)} pp`;
  if (previous === 0) return formatStatisticValue(difference, unit, true, currency);
  const relative = difference / Math.abs(previous);
  return `${relative >= 0 ? "+" : ""}${(relative * 100).toFixed(1)}%`;
}

function freshnessLabel(metric: StatisticMetric, quote?: LiveQuote) {
  if (metric.marketSensitive) {
    const state = quote?.marketState?.toLowerCase();
    return state?.includes("regular") ? "Current market" : state ? quote?.marketState : "Latest market snapshot";
  }
  return metric.nativeFrequency;
}

function formatDate(value: string) {
  const date = new Date(`${value.slice(0, 10)}T00:00:00Z`);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric", timeZone: "UTC" });
}

function formatDateTime(value: string) {
  if (/^\d{4}-\d{2}-\d{2}$/.test(value)) return formatDate(value);
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? formatDate(value) : date.toLocaleString("en-US", { year: "numeric", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit", timeZoneName: "short" });
}

function axisDate(value: number) { return new Date(value).toLocaleDateString("en-US", { month: "short", year: "2-digit", timeZone: "UTC" }); }
function initialParam(key: string) { return new URLSearchParams(window.location.search).get(key) ?? ""; }
function initialChoice<T extends string>(key: string, choices: T[], fallback: T) { const value = initialParam(key) as T; return choices.includes(value) ? value : fallback; }
function loadJSON<T>(key: string, fallback: T): T { try { const value = localStorage.getItem(key); return value ? JSON.parse(value) as T : fallback; } catch { return fallback; } }
function saveJSON(key: string, value: unknown) { try { localStorage.setItem(key, JSON.stringify(value)); } catch { /* Persistence is optional. */ } }
