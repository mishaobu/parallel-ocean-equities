import { useCallback, useEffect, useMemo, useState } from "react";
import { Activity, ArrowUpRight, BarChart3, Globe2, Landmark, LoaderCircle, SlidersHorizontal } from "lucide-react";
import { assetReturns, countryMetricRows, indexedAssetRows, rangeDomain, snapshots, type Range, type ScenarioInputs } from "./data";
import type { AssetSeries, StateResponse } from "./types";
import { AssetTable } from "./components/AssetTable";
import { DivergenceMap, SeriesChart, type LineSpec } from "./components/Charts";
import { CountryMatrix, CountryRanks } from "./components/Matrix";
import { OutcomesLab } from "./components/Outcomes";
import { neutralScenario, ScenarioLab } from "./components/Scenario";
import { currentRegime, regimeOutcomes } from "./outcomes";

const views = ["overview", "countries", "assets", "outcomes", "relative", "scenarios"] as const;
type View = typeof views[number];
const labels: Record<View, string> = { overview: "Overview", countries: "Countries", assets: "Cross-asset", outcomes: "Outcomes", relative: "Relative policy", scenarios: "Scenarios" };
const ranges: Range[] = ["max", "20y", "10y", "5y", "3y", "1y"];
const colors = ["#3975a7", "#b8493e", "#347b57", "#b2832e", "#765997", "#31838a", "#a34e73", "#596b62", "#c05d32", "#4e7590", "#87924b", "#6f5347"];

function App() {
  const [payload, setPayload] = useState<StateResponse>();
  const [error, setError] = useState("");
  const [range, setRange] = useState<Range>("10y");
  const [view, setView] = useState<View>("overview");
  const [selectedCountry, setSelectedCountry] = useState(() => new URLSearchParams(window.location.search).get("country")?.toUpperCase() || "US");
  const [selectedAssets, setSelectedAssets] = useState<string[]>(["SPY", "QQQ", "FEZ", "EWJ", "FXI", "TLT", "GLD"]);
  const [scenario, setScenario] = useState<ScenarioInputs>(neutralScenario);

  const load = useCallback(async () => {
    try {
      const response = await fetch("/macro/api/state", { headers: { Accept: "application/json" } });
      if (!response.ok) throw new Error(`Macro API returned ${response.status}`);
      const body = await response.json() as StateResponse;
      setPayload(body); setError(body.state.macro?.error ?? "");
    } catch (requestError) { setError(requestError instanceof Error ? requestError.message : "Macro data unavailable"); }
  }, []);
  useEffect(() => { void load(); const timer = window.setInterval(() => void load(), 60_000); return () => window.clearInterval(timer); }, [load]);

  const macro = payload?.state.macro;
  const countries = macro?.countries ?? [];
  const assets = macro?.assets ?? [];
  const countryRows = useMemo(() => snapshots(countries), [countries]);
  const domain = useMemo(() => rangeDomain(assets, countries, range), [assets, countries, range]);
  const returns = useMemo(() => assetReturns(assets), [assets]);
  const indexed = useMemo(() => indexedAssetRows(assets, selectedAssets, domain), [assets, domain, selectedAssets]);
  const lineSpecs = useMemo(() => assetSpecs(assets, selectedAssets), [assets, selectedAssets]);
  const activeCountry = countryRows.find((row) => row.country.code === selectedCountry) ?? countryRows[0];
  const signals = useMemo(() => globalSignals(countryRows, returns), [countryRows, returns]);
  const usCountry = countries.find((country) => country.code === "US");
  const outcomes = useMemo(() => regimeOutcomes(assets, usCountry, domain), [assets, domain, usCountry]);
  const activeRegime = useMemo(() => currentRegime(usCountry), [usCountry]);

  useEffect(() => {
    if (!activeCountry || activeCountry.country.code === selectedCountry) return;
    setSelectedCountry(activeCountry.country.code);
  }, [activeCountry, selectedCountry]);
  function selectCountry(code: string) { setSelectedCountry(code); const url = new URL(window.location.href); url.searchParams.set("country", code); window.history.replaceState({}, "", url); }

  return <div className="app-shell"><header className="topbar"><div className="brand"><Globe2 size={21} /><strong>Macro</strong><span>parallel-ocean</span></div><nav><a href="/equities/"><BarChart3 size={15} />Equities<ArrowUpRight size={12} /></a><a href="/monetary/"><Landmark size={15} />Monetary<ArrowUpRight size={12} /></a></nav><div className="status"><i className={payload?.runtime.macroRefreshing ? "pulse" : ""} />{payload?.runtime.macroRefreshing ? "Refreshing" : updated(macro?.updatedAt)}</div></header>
    {error && <div className="error-banner" role="alert">{error}</div>}
    <main><section className="page-head"><div><span className="eyebrow"><Activity size={14} />Global macro synthesis / {signals.asOf}</span><h1>{signals.label}</h1><p>{domainLabel(domain)} / {countries.length} monetary systems / {assets.length} cross-asset series</p></div><div className="range-control" aria-label="Analysis range">{ranges.map((value) => <button type="button" key={value} className={range === value ? "is-active" : ""} onClick={() => setRange(value)}>{value === "max" ? "Max" : value.toUpperCase()}</button>)}</div></section>
      <nav className="view-nav" aria-label="Macro analysis view">{views.map((candidate) => <button type="button" key={candidate} className={view === candidate ? "is-active" : ""} onClick={() => setView(candidate)}>{candidate === "scenarios" && <SlidersHorizontal size={13} />}{labels[candidate]}</button>)}</nav>
      {!macro || countries.length === 0 ? <div className="loading"><LoaderCircle size={22} className="spin" />Global data refresh pending</div> : <>
        {view === "overview" && <Overview rows={countryRows} signals={signals} indexed={indexed} domain={domain} series={lineSpecs} />}
        {view === "countries" && <CountriesView rows={countryRows} selected={activeCountry?.country.code} onSelect={selectCountry} domain={domain} />}
        {view === "assets" && <AssetsView assets={assets} selected={selectedAssets} onSelected={setSelectedAssets} indexed={indexed} domain={domain} series={lineSpecs} returns={returns} />}
        {view === "outcomes" && <OutcomesView stats={outcomes} current={activeRegime} />}
        {view === "relative" && <RelativeView rows={countryRows} domain={domain} />}
        {view === "scenarios" && <ScenariosView assets={assets} points={macro.points ?? []} domain={domain} values={scenario} onChange={setScenario} />}
      </>}
      <div className="data-basis"><strong>Data basis</strong><span>{macro?.basis ?? "Latest revised observations."} Country cells use their own observation dates; cross-country policy definitions and publication lags differ.</span></div>
      {!!macro?.warnings?.length && <details className="warnings"><summary>{macro.warnings.length} source warnings</summary>{macro.warnings.map((warning) => <span key={warning}>{warning}</span>)}</details>}
    </main>
  </div>;
}

function Overview({ rows, signals, indexed, domain, series }: { rows: ReturnType<typeof snapshots>; signals: ReturnType<typeof globalSignals>; indexed: Array<Record<string, string | number>>; domain: [number, number]; series: LineSpec[] }) {
  return <><section className="signal-strip">{signals.items.map((item) => <div key={item.label}><span>{item.label}</span><strong>{item.value}</strong><small>{item.note}</small></div>)}</section><section className="overview-grid"><SeriesChart title="Cross-asset regime tape" note="Indexed to 100 at each asset's first selected-range observation" rows={indexed} domain={domain} series={series} unit="index" primary /><DivergenceMap rows={rows} /></section><SectionTitle index="01" title="Monetary divergence" note="Latest available metrics, with stale series explicitly dated" /><CountryRanks rows={rows} /><CountryMatrix rows={rows} compact /></>;
}

function CountriesView({ rows, selected, onSelect, domain }: { rows: ReturnType<typeof snapshots>; selected?: string; onSelect: (code: string) => void; domain: [number, number] }) {
  const active = rows.find((row) => row.country.code === selected) ?? rows[0];
  return <><CountryRanks rows={rows} /><CountryMatrix rows={rows} selected={selected} onSelect={onSelect} /><SectionTitle index="01" title={active.country.name} note={`${active.country.policyLabel} / ${active.country.fxLabel} / ${active.regime}`} /><section className="country-detail-grid"><SeriesChart title="Policy and inflation" note="Nominal policy, inflation and ex-post real policy" rows={countryRows(active.country, domain)} domain={domain} series={[{ key: "policyRate", label: active.country.policyLabel, color: colors[0] }, { key: "inflation", label: "Inflation", color: colors[1] }, { key: "realRate", label: "Real policy", color: colors[2] }]} /><SeriesChart title="Domestic transmission" note="Industrial growth, money growth and unemployment" rows={countryRows(active.country, domain)} domain={domain} series={[{ key: "industrialGrowth", label: "Industrial growth", color: colors[2] }, { key: "moneyGrowth", label: "Broad money", color: colors[4] }, { key: "unemployment", label: "Unemployment", color: colors[1] }]} /></section><div className="coverage-band"><div><span>Latest observation</span><strong>{active.asOf ? month(active.asOf) : "--"}</strong></div><div><span>FRED series</span><strong>{active.country.sources?.length ?? 0}</strong></div><div><span>Equity proxy</span><strong>{active.country.equityTicker ?? "--"}</strong></div><div><span>Currency</span><strong>{active.country.currency}</strong></div></div></>;
}

function AssetsView({ assets, selected, onSelected, indexed, domain, series, returns }: { assets: AssetSeries[]; selected: string[]; onSelected: (symbols: string[]) => void; indexed: Array<Record<string, string | number>>; domain: [number, number]; series: LineSpec[]; returns: ReturnType<typeof assetReturns> }) {
  const groups = [...new Set(assets.map((asset) => asset.group))];
  function toggle(symbol: string) { onSelected(selected.includes(symbol) ? selected.filter((value) => value !== symbol) : [...selected, symbol]); }
  return <><section className="asset-toolbar"><div>{groups.map((group) => <button type="button" key={group} onClick={() => onSelected(assets.filter((asset) => asset.group === group).map((asset) => asset.symbol))}>{group}</button>)}<button type="button" onClick={() => onSelected(assets.map((asset) => asset.symbol))}>All</button></div><span>{selected.length} visible</span></section><div className="asset-switches">{assets.map((asset) => <label key={asset.symbol} className={selected.includes(asset.symbol) ? "is-active" : ""}><input type="checkbox" checked={selected.includes(asset.symbol)} onChange={() => toggle(asset.symbol)} /><b>{asset.symbol}</b><span>{asset.label}</span></label>)}</div><SeriesChart title="Indexed performance" note="Click legend labels to isolate lines; tooltip values are sorted high to low" rows={indexed} domain={domain} series={series} unit="index" primary /><AssetTable rows={returns} /></>;
}

function RelativeView({ rows, domain }: { rows: ReturnType<typeof snapshots>; domain: [number, number] }) {
  const countries = rows.map((row) => row.country);
  const specs = countries.map((country, index) => ({ key: country.code, label: country.name, color: colors[index % colors.length] }));
  return <><section className="relative-grid"><SeriesChart title="Nominal policy divergence" note="Policy or short-rate proxy, as labeled by economy" rows={countryMetricRows(countries, "policyRate", domain)} domain={domain} series={specs} /><SeriesChart title="Real policy divergence" note="Nominal policy less headline inflation" rows={countryMetricRows(countries, "realRate", domain)} domain={domain} series={specs} /><SeriesChart title="Inflation divergence" note="Headline consumer-price inflation" rows={countryMetricRows(countries, "inflation", domain)} domain={domain} series={specs} /><SeriesChart title="Yield-curve divergence" note="Long government rate less policy or short-rate proxy" rows={countryMetricRows(countries, "yieldCurve", domain)} domain={domain} series={specs} /></section><SectionTitle index="01" title="Current relative-value map" note="Restrictiveness and growth are measured on each economy's latest available date" /><DivergenceMap rows={rows} /><CountryMatrix rows={rows} compact /></>;
}

function OutcomesView({ stats, current }: { stats: ReturnType<typeof regimeOutcomes>; current?: ReturnType<typeof currentRegime> }) { return <><SectionTitle index="01" title="What happened next" note="Cross-asset forward outcomes conditioned on the US growth-inflation regime available at each start" /><OutcomesLab stats={stats} current={current} /></>; }

function ScenariosView({ assets, points, domain, values, onChange }: { assets: AssetSeries[]; points: NonNullable<StateResponse["state"]["macro"]>["points"]; domain: [number, number]; values: ScenarioInputs; onChange: (values: ScenarioInputs) => void }) { return <><SectionTitle index="01" title="Macro shock workbench" note="Translate a coherent growth, inflation, rates, dollar and liquidity view into relative asset sensitivities" /><ScenarioLab assets={assets} points={points ?? []} domain={domain} values={values} onChange={onChange} /></>; }

function SectionTitle({ index, title, note }: { index: string; title: string; note: string }) { return <div className="section-title"><b>{index}</b><div><h2>{title}</h2><span>{note}</span></div></div>; }
function countryRows(country: ReturnType<typeof snapshots>[number]["country"], domain: [number, number]) { return (country.points ?? []).map((point) => ({ ...point, timestamp: Date.parse(point.date) })).filter((point) => point.timestamp >= domain[0] && point.timestamp <= domain[1]); }
function assetSpecs(assets: AssetSeries[], selected: string[]) { return assets.filter((asset) => selected.includes(asset.symbol)).map((asset, index) => ({ key: asset.symbol, label: `${asset.symbol} / ${asset.label}`, color: colors[index % colors.length] })); }
function globalSignals(rows: ReturnType<typeof snapshots>, returns: ReturnType<typeof assetReturns>) {
  const realRates = rows.flatMap((row) => row.values.realRate && row.values.realRate.ageMonths <= 14 ? [row.values.realRate.value] : []);
  const inflation = rows.flatMap((row) => row.values.inflation && row.values.inflation.ageMonths <= 14 ? [row.values.inflation.value] : []).sort((a, b) => a - b);
  const positive = returns.filter((row) => (row.oneYear ?? -Infinity) > 0).length;
  const spread = realRates.length ? Math.max(...realRates) - Math.min(...realRates) : 0;
  const medianInflation = inflation.length ? inflation[Math.floor(inflation.length / 2)] : undefined;
  const asOf = rows.reduce((latest, row) => row.asOf > latest ? row.asOf : latest, "");
  return { label: spread >= 4 ? "High policy divergence" : spread >= 2 ? "Moderate policy divergence" : "Convergent policy regime", asOf: asOf ? month(asOf) : "pending", items: [
    { label: "Real-rate dispersion", value: realRates.length > 1 ? `${spread.toFixed(1)}pp` : "--", note: `${realRates.length} fresh economies` }, { label: "Median inflation", value: medianInflation === undefined ? "--" : `${medianInflation.toFixed(1)}%`, note: `${inflation.length} fresh economies` },
    { label: "1Y asset breadth", value: returns.length ? `${positive}/${returns.length}` : "--", note: "Positive sleeves" }, { label: "Country coverage", value: String(rows.length), note: "Metric-dated" },
  ] };
}
function month(value: string) { return new Date(value).toLocaleDateString("en-US", { month: "short", year: "numeric", timeZone: "UTC" }); }
function updated(value?: string) { return value ? `Updated ${new Date(value).toLocaleDateString("en-US", { month: "short", day: "numeric", timeZone: "UTC" })}` : "Waiting for data"; }
function domainLabel(domain: [number, number]) { return `${new Date(domain[0]).getUTCFullYear()}-${new Date(domain[1]).getUTCFullYear()}`; }
export default App;
