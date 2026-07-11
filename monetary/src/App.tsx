import { useCallback, useEffect, useMemo, useState } from "react";
import { Activity, ArrowUpRight, BarChart3, Globe2, Landmark, LoaderCircle } from "lucide-react";
import { coherentRegime, pillarSnapshots } from "./analysis";
import { getState } from "./api";
import { DateInspector } from "./components/DateInspector";
import { CountryAnalysis } from "./components/CountryAnalysis";
import { EpisodeCompare } from "./components/EpisodeCompare";
import { EquityTransmission } from "./components/EquityTransmission";
import { FreshnessPanel } from "./components/FreshnessPanel";
import { MacroChart, type SeriesSpec } from "./components/MacroChart";
import { PillarStrip } from "./components/PillarStrip";
import { RegimeMap } from "./components/RegimeMap";
import { RegimeTimeline } from "./components/RegimeTimeline";
import { rangeDomain, type MacroRange } from "./macroData";
import type { MacroPoint, MacroSeries, StateResponse } from "./types";

const ranges: MacroRange[] = ["max", "50y", "25y", "10y", "5y"];
const views = ["overview", "countries", "inflation", "liquidity", "rates", "credit", "growth", "equity"] as const;
type AnalysisView = typeof views[number];

const labels: Record<AnalysisView, string> = {
  overview: "US overview", countries: "Countries", inflation: "Inflation", liquidity: "Liquidity", rates: "Rates", credit: "Credit", growth: "Growth", equity: "Equity transmission",
};
const colors = { ink: "#17201b", red: "#b8493e", blue: "#3975a7", green: "#347b57", gold: "#b2832e", violet: "#765997", cyan: "#31838a", rose: "#a34e73" };

function App() {
  const [payload, setPayload] = useState<StateResponse>();
  const [refreshing, setRefreshing] = useState(false);
  const [range, setRange] = useState<MacroRange>("25y");
  const [view, setView] = useState<AnalysisView>("overview");
  const [error, setError] = useState("");
  const [hoveredDate, setHoveredDate] = useState<number>();
  const [pinnedDate, setPinnedDate] = useState<number>();
  const [ticker, setTicker] = useState(() => new URLSearchParams(window.location.search).get("ticker")?.toUpperCase() || "SPY");

  const load = useCallback(async () => {
    try {
      const response = await getState();
      setPayload(response);
      setRefreshing(Boolean(response.runtime.macroRefreshing));
      setError(response.state.macro?.error ?? "");
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Macro data unavailable");
    }
  }, []);

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 60_000);
    return () => window.clearInterval(timer);
  }, [load]);

  const macro = payload?.state.macro;
  const points = macro?.points ?? [];
  const domain = useMemo(() => rangeDomain(points, range), [points, range]);
  const visiblePointCount = useMemo(() => points.filter((point) => { const date = Date.parse(point.date); return date >= domain[0] && date <= domain[1]; }).length, [domain, points]);
  const regime = useMemo(() => coherentRegime(points), [points]);
  const pillars = useMemo(() => pillarSnapshots(points, domain), [domain, points]);
  const selectedDate = pinnedDate ?? hoveredDate;
  const equities = payload?.state.tickers ?? {};

  useEffect(() => {
    if (equities[ticker] || Object.keys(equities).length === 0) return;
    setTicker(equities.SPY ? "SPY" : Object.keys(equities).sort()[0]);
  }, [equities, ticker]);

  function selectTicker(next: string) {
    setTicker(next);
    const url = new URL(window.location.href);
    url.searchParams.set("ticker", next);
    window.history.replaceState({}, "", url);
  }

  function pinDate(date: number) {
    setPinnedDate((current) => current === date ? undefined : date);
  }

  const interaction = { selectedDate, onInspect: pinnedDate === undefined ? setHoveredDate : undefined, onPin: pinDate };

  return <div className="macro-app">
    <header className="topbar">
      <div className="brand"><Landmark size={21} /><strong>Monetary</strong><span>parallel-ocean</span></div>
      <nav><a href="/equities/"><BarChart3 size={15} />Equities<ArrowUpRight size={13} /></a><a href="/macro/"><Globe2 size={15} />Macro<ArrowUpRight size={13} /></a></nav>
      <div className="status"><span className={refreshing ? "pulse" : ""} />{refreshing ? "Refreshing" : updatedLabel(macro?.updatedAt)}</div>
    </header>

    {error && <div className="error-banner" role="alert">{error}</div>}
    <main>
      <section className="page-heading">
        <div><span className="eyebrow"><Activity size={14} />{view === "countries" ? `${macro?.countries?.length ?? 0} monetary systems` : `US macro regime / ${regime.point?.date.slice(0, 7) ?? "pending"}`}</span><h1>{view === "countries" ? "Global monetary regimes" : regime.label}</h1><p>{domainLabel(domain)} / {view === "countries" ? "metric-level freshness" : `${visiblePointCount} monthly rows`} / {macro?.sources?.length ?? 0} US source series</p></div>
        <div className="segmented" aria-label="Analysis range">
          {ranges.map((value) => <button type="button" key={value} className={range === value ? "is-active" : ""} onClick={() => setRange(value)}>{value === "max" ? "Max" : value.toUpperCase()}</button>)}
        </div>
      </section>

      <nav className="analysis-nav" aria-label="Monetary analysis view">
        {views.map((candidate) => <button type="button" key={candidate} className={view === candidate ? "is-active" : ""} onClick={() => setView(candidate)}>{labels[candidate]}</button>)}
      </nav>

      {points.length === 0 ? <div className="loading"><LoaderCircle className="spin" size={22} /><span>Macro history refresh pending</span></div> : <>
        {view !== "countries" && <DateInspector points={points} date={selectedDate} pinned={pinnedDate !== undefined} onClear={() => setPinnedDate(undefined)} />}
        {view === "overview" && <Overview points={points} domain={domain} pillars={pillars} interaction={interaction} pinnedDate={pinnedDate} />}
        {view === "countries" && <CountryAnalysis countries={macro?.countries ?? []} domain={domain} />}
        {view === "inflation" && <InflationView points={points} domain={domain} interaction={interaction} />}
        {view === "liquidity" && <LiquidityView points={points} domain={domain} interaction={interaction} />}
        {view === "rates" && <RatesView points={points} domain={domain} interaction={interaction} />}
        {view === "credit" && <CreditView points={points} domain={domain} interaction={interaction} />}
        {view === "growth" && <GrowthView points={points} domain={domain} interaction={interaction} />}
        {view === "equity" && <EquityTransmission equities={equities} ticker={ticker} onTicker={selectTicker} points={points} domain={domain} {...interaction} />}
      </>}

      <DataBasis macro={macro} />
      {!!macro?.warnings?.length && <section className="warnings">{macro.warnings.map((warning) => <span key={warning}>{warning}</span>)}</section>}
    </main>
  </div>;
}

type Interaction = { selectedDate?: number; onInspect?: (date?: number) => void; onPin?: (date: number) => void };

function Overview({ points, domain, pillars, interaction, pinnedDate }: { points: MacroPoint[]; domain: [number, number]; pillars: ReturnType<typeof pillarSnapshots>; interaction: Interaction; pinnedDate?: number }) {
  return <>
    <PillarStrip pillars={pillars} />
    <RegimeTimeline points={points} domain={domain} selectedDate={interaction.selectedDate} onPin={interaction.onPin} />
    <section className="chart-grid primary-grid">
      <RegimeMap points={points} domain={domain} {...interaction} />
      <MacroChart title="Policy stance" note="Inflation, nominal policy and ex-post real policy" points={points} domain={domain} series={policySeries} primary {...interaction} />
    </section>
    <SectionTitle index="01" title="Current transmission" note="Liquidity creation, credit pressure and market-sensitive financial conditions" />
    <section className="chart-grid">
      <MacroChart title="Net liquidity impulse" note="Fed assets less TGA and reverse repos" points={points} domain={domain} series={netLiquiditySeries} unit="billions" {...interaction} />
      <MacroChart title="Credit pressure" note="NFCI, high-yield spreads and bank lending standards" points={points} domain={domain} series={creditPressureSeries} unit="index" {...interaction} />
    </section>
    <section className="overview-secondary"><EpisodeCompare points={points} pinnedDate={pinnedDate} /><FreshnessPanel points={points} /></section>
  </>;
}

function InflationView({ points, domain, interaction }: ViewProps) {
  return <AnalysisSection index="01" title="Inflation structure" note="Headline, underlying components, wages and market expectations">
    <MacroChart title="Inflation decomposition" note="Year-over-year change" points={points} domain={domain} series={inflationSeries} primary {...interaction} />
    <MacroChart title="Shelter and wages" note="Persistent domestic inflation pressure" points={points} domain={domain} series={wageSeries} {...interaction} />
    <MacroChart title="Inflation expectations" note="5Y, 10Y and 5Y5Y market pricing" points={points} domain={domain} series={expectationsSeries} {...interaction} />
    <MacroChart title="Policy against inflation" note="Fed funds, headline CPI and ex-post real policy" points={points} domain={domain} series={policySeries} {...interaction} />
  </AnalysisSection>;
}

function LiquidityView({ points, domain, interaction }: ViewProps) {
  return <AnalysisSection index="02" title="System liquidity" note="Central-bank balance sheet, Treasury cash, reverse repos, money and bank credit">
    <MacroChart title="Net liquidity stock" note="Fed assets less TGA and reverse repos" points={points} domain={domain} series={netLiquiditySeries} unit="billions" primary {...interaction} />
    <MacroChart title="Net liquidity impulse" note="Year-over-year change" points={points} domain={domain} series={liquidityImpulseSeries} {...interaction} />
    <MacroChart title="Treasury and reverse-repo drains" note="Liquidity absorbed outside private markets" points={points} domain={domain} series={drainSeries} unit="billions" {...interaction} />
    <MacroChart title="Money and bank credit" note="Year-over-year change" points={points} domain={domain} series={moneyCreditSeries} {...interaction} />
    <MacroChart title="Money aggregates" note="M1, M2 and monetary base year-over-year" points={points} domain={domain} series={moneyAggregateSeries} {...interaction} />
    <MacroChart title="Monetary stocks" note="Log10 of nominal levels" points={points} domain={domain} series={liquidityLevelSeries} unit="log" {...interaction} />
  </AnalysisSection>;
}

function RatesView({ points, domain, interaction }: ViewProps) {
  return <AnalysisSection index="03" title="Rates and discounting" note="Nominal curve, real yields, inflation pricing and term premium">
    <MacroChart title="Nominal yield curve" note="3M through 30Y Treasury yields" points={points} domain={domain} series={curveSeries} primary {...interaction} />
    <MacroChart title="Real yields and term premium" note="Direct TIPS yields and 10Y term premium" points={points} domain={domain} series={realYieldSeries} {...interaction} />
    <MacroChart title="Curve slopes" note="10Y less 2Y and 10Y less 3M" points={points} domain={domain} series={curveSlopeSeries} {...interaction} />
    <MacroChart title="Inflation and mortgage pricing" note="Breakevens, forward inflation and 30Y mortgage" points={points} domain={domain} series={ratesTransmissionSeries} {...interaction} />
  </AnalysisSection>;
}

function CreditView({ points, domain, interaction }: ViewProps) {
  return <AnalysisSection index="04" title="Credit transmission" note="Market spreads, lending standards, bank credit and volatility">
    <MacroChart title="Conditions and spreads" note="NFCI with investment-grade and high-yield OAS" points={points} domain={domain} series={creditSeries} unit="index" primary {...interaction} />
    <MacroChart title="Bank lending channel" note="SLOOS tightening and credit growth" points={points} domain={domain} series={bankChannelSeries} {...interaction} />
    <MacroChart title="Dollar and volatility" note="Broad dollar and VIX" points={points} domain={domain} series={riskSeries} unit="index" {...interaction} />
  </AnalysisSection>;
}

function GrowthView({ points, domain, interaction }: ViewProps) {
  return <AnalysisSection index="05" title="Growth and labor" note="Output, employment, recession stress, commodities and fiscal capacity">
    <MacroChart title="Growth complex" note="Real GDP, industrial production and payrolls YoY" points={points} domain={domain} series={growthSeries} primary {...interaction} />
    <MacroChart title="Labor stress" note="Unemployment, initial claims and Sahm rule" points={points} domain={domain} series={laborStressSeries} {...interaction} />
    <MacroChart title="Commodities" note="WTI crude and copper" points={points} domain={domain} series={commoditySeries} unit="index" {...interaction} />
    <MacroChart title="Fiscal and dollar context" note="Federal debt to GDP and broad dollar" points={points} domain={domain} series={fiscalSeries} unit="index" {...interaction} />
  </AnalysisSection>;
}

interface ViewProps { points: MacroPoint[]; domain: [number, number]; interaction: Interaction }
function AnalysisSection({ index, title, note, children }: { index: string; title: string; note: string; children: React.ReactNode }) {
  return <><SectionTitle index={index} title={title} note={note} /><section className="chart-grid analysis-grid">{children}</section></>;
}
function SectionTitle({ index, title, note }: { index: string; title: string; note: string }) { return <div className="section-title"><b>{index}</b><div><h2>{title}</h2><span>{note}</span></div></div>; }
function DataBasis({ macro }: { macro?: MacroSeries }) { return <div className="data-basis"><strong>Data basis</strong><span>{macro?.basis || "Latest-revised observations."} Equity regime outcomes apply a conservative two-month publication lag but are not ALFRED vintage backtests.</span></div>; }

const policySeries: SeriesSpec[] = [{ key: "inflation", label: "Headline CPI", color: colors.red }, { key: "fedFunds", label: "Fed funds", color: colors.ink }, { key: "realPolicyRate", label: "Ex-post real policy", color: colors.blue }];
const inflationSeries: SeriesSpec[] = [{ key: "inflation", label: "Headline CPI", color: colors.red }, { key: "coreInflation", label: "Core CPI", color: colors.ink }, { key: "corePceInflation", label: "Core PCE", color: colors.blue }, { key: "shelterInflation", label: "Shelter", color: colors.gold }];
const wageSeries: SeriesSpec[] = [{ key: "shelterInflation", label: "Shelter", color: colors.gold }, { key: "wageGrowth", label: "Wages", color: colors.violet }, { key: "corePceInflation", label: "Core PCE", color: colors.blue }];
const expectationsSeries: SeriesSpec[] = [{ key: "breakeven5Y", label: "5Y breakeven", color: colors.green }, { key: "breakeven10Y", label: "10Y breakeven", color: colors.blue }, { key: "forwardInflation5Y", label: "5Y5Y forward", color: colors.violet }];
const netLiquiditySeries: SeriesSpec[] = [{ key: "netLiquidityB", label: "Net liquidity", color: colors.green }];
const liquidityImpulseSeries: SeriesSpec[] = [{ key: "netLiquidityGrowth", label: "Net liquidity", color: colors.green }, { key: "fedAssetsGrowth", label: "Fed assets", color: colors.gold }, { key: "m2Growth", label: "M2", color: colors.blue }];
const drainSeries: SeriesSpec[] = [{ key: "tgaB", label: "Treasury General Account", color: colors.gold }, { key: "reverseRepoB", label: "Reverse repo", color: colors.violet }];
const moneyCreditSeries: SeriesSpec[] = [{ key: "m2Growth", label: "M2", color: colors.green }, { key: "bankCreditGrowth", label: "Bank credit", color: colors.blue }, { key: "businessLoanGrowth", label: "Business loans", color: colors.red }];
const moneyAggregateSeries: SeriesSpec[] = [{ key: "m1Growth", label: "M1", color: colors.cyan }, { key: "m2Growth", label: "M2", color: colors.green }, { key: "monetaryBaseGrowth", label: "Monetary base", color: colors.violet }];
const liquidityLevelSeries: SeriesSpec[] = [{ key: "logM1", label: "M1", color: colors.cyan }, { key: "logM2", label: "M2", color: colors.green }, { key: "logFedAssets", label: "Fed assets", color: colors.gold }, { key: "logMonetaryBase", label: "Monetary base", color: colors.rose }, { key: "logBankReserves", label: "Bank reserves", color: colors.violet }];
const curveSeries: SeriesSpec[] = [{ key: "treasury3M", label: "3M", color: colors.cyan }, { key: "treasury2Y", label: "2Y", color: colors.blue }, { key: "treasury5Y", label: "5Y", color: colors.green }, { key: "treasury10Y", label: "10Y", color: colors.ink }, { key: "treasury30Y", label: "30Y", color: colors.red }];
const realYieldSeries: SeriesSpec[] = [{ key: "real5Y", label: "Real 5Y", color: colors.green }, { key: "real10Y", label: "Real 10Y", color: colors.blue }, { key: "termPremium10Y", label: "10Y term premium", color: colors.violet }];
const curveSlopeSeries: SeriesSpec[] = [{ key: "yieldCurve", label: "10Y-2Y", color: colors.violet }, { key: "yieldCurve3M", label: "10Y-3M", color: colors.blue }];
const ratesTransmissionSeries: SeriesSpec[] = [{ key: "breakeven5Y", label: "5Y breakeven", color: colors.green }, { key: "breakeven10Y", label: "10Y breakeven", color: colors.gold }, { key: "forwardInflation5Y", label: "5Y5Y", color: colors.violet }, { key: "mortgage30Y", label: "30Y mortgage", color: colors.red }];
const creditSeries: SeriesSpec[] = [{ key: "financialConditions", label: "NFCI", color: colors.ink }, { key: "corporateSpread", label: "IG OAS", color: colors.gold, axis: "right" }, { key: "highYieldSpread", label: "HY OAS", color: colors.red, axis: "right" }];
const creditPressureSeries: SeriesSpec[] = [{ key: "financialConditions", label: "NFCI", color: colors.ink }, { key: "highYieldSpread", label: "HY OAS", color: colors.red, axis: "right" }, { key: "lendingStandards", label: "SLOOS", color: colors.gold, axis: "right" }];
const bankChannelSeries: SeriesSpec[] = [{ key: "lendingStandards", label: "SLOOS tightening", color: colors.red }, { key: "bankCreditGrowth", label: "Bank credit", color: colors.blue }, { key: "businessLoanGrowth", label: "Business loans", color: colors.green }];
const riskSeries: SeriesSpec[] = [{ key: "dollarIndex", label: "Broad dollar", color: colors.blue }, { key: "vix", label: "VIX", color: colors.red, axis: "right" }];
const growthSeries: SeriesSpec[] = [{ key: "realGdpGrowth", label: "Real GDP", color: colors.green }, { key: "industrialGrowth", label: "Industrial production", color: colors.blue }, { key: "payrollGrowth", label: "Payrolls", color: colors.violet }];
const laborStressSeries: SeriesSpec[] = [{ key: "unemployment", label: "Unemployment", color: colors.red }, { key: "sahmRule", label: "Sahm rule", color: colors.gold }, { key: "initialClaimsK", label: "Initial claims (000s)", color: colors.blue, axis: "right" }];
const commoditySeries: SeriesSpec[] = [{ key: "oilPrice", label: "WTI crude", color: colors.red }, { key: "copperPrice", label: "Copper", color: colors.gold, axis: "right" }];
const fiscalSeries: SeriesSpec[] = [{ key: "federalDebtToGdp", label: "Federal debt / GDP", color: colors.red }, { key: "dollarIndex", label: "Broad dollar", color: colors.blue, axis: "right" }];

function domainLabel(domain: [number, number]) { return `${new Date(domain[0]).getUTCFullYear()}-${new Date(domain[1]).getUTCFullYear()}`; }
function updatedLabel(value?: string) { if (!value) return "Waiting for data"; return `Updated ${new Date(value).toLocaleDateString("en-US", { month: "short", day: "numeric", timeZone: "UTC" })}`; }

export default App;
