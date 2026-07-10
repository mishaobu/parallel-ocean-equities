import { useCallback, useEffect, useMemo, useState } from "react";
import { Activity, ArrowUpRight, BarChart3, Landmark, LoaderCircle } from "lucide-react";
import { getState } from "./api";
import { MacroChart, type SeriesSpec } from "./components/MacroChart";
import { RegimeMap } from "./components/RegimeMap";
import { currentValue, meanDefined, rangeDomain, regimeLabel, type MacroRange } from "./macroData";
import type { MacroPoint, MacroSeries } from "./types";

const ranges: MacroRange[] = ["max", "50y", "25y", "10y", "5y"];
const colors = { ink: "#17201b", red: "#b8493e", blue: "#3975a7", green: "#347b57", gold: "#b2832e", violet: "#765997", cyan: "#31838a" };

function App() {
  const [macro, setMacro] = useState<MacroSeries>();
  const [refreshing, setRefreshing] = useState(false);
  const [range, setRange] = useState<MacroRange>("max");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const response = await getState();
      setMacro(response.state.macro);
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

  const points = macro?.points ?? [];
  const domain = useMemo(() => rangeDomain(points, range), [points, range]);
  const inflation = currentValue(points, "inflation");
  const industrialGrowth = currentValue(points, "industrialGrowth");
  const liquidityPulse = meanDefined([currentValue(points, "m2Growth"), currentValue(points, "fedAssetsGrowth"), currentValue(points, "monetaryBaseGrowth")]);

  return <div className="macro-app">
    <header className="topbar">
      <div className="brand"><Landmark size={21} /><strong>Monetary</strong><span>parallel-ocean</span></div>
      <nav><a href="/equities/"><BarChart3 size={15} />Equities<ArrowUpRight size={13} /></a></nav>
      <div className="status"><span className={refreshing ? "pulse" : ""} />{refreshing ? "Refreshing" : updatedLabel(macro?.updatedAt)}</div>
    </header>

    {error && <div className="error-banner" role="alert">{error}</div>}
    <main>
      <section className="page-heading">
        <div><span className="eyebrow"><Activity size={14} />US monetary regime</span><h1>{regimeLabel(inflation, industrialGrowth)}</h1><p>{domainLabel(domain)} / {points.length} monthly observations / {macro?.sources?.length ?? 0} source series</p></div>
        <div className="segmented" aria-label="Analysis range">
          {ranges.map((value) => <button type="button" key={value} className={range === value ? "is-active" : ""} onClick={() => setRange(value)}>{value === "max" ? "Max" : value.toUpperCase()}</button>)}
        </div>
      </section>

      {points.length === 0 ? <div className="loading"><LoaderCircle className="spin" size={22} /><span>Macro history refresh pending</span></div> : <>
        <section className="metric-strip">
          <Metric label="Inflation" value={percent(inflation)} note="CPI YoY" tone={tone(inflation, 2.5)} />
          <Metric label="Real policy" value={percent(currentValue(points, "realPolicyRate"))} note="Fed funds less CPI" />
          <Metric label="Yield curve" value={percent(currentValue(points, "yieldCurve"))} note="10Y less 2Y" />
          <Metric label="Liquidity pulse" value={percent(liquidityPulse)} note="M2 / Fed / base YoY" tone={tone(liquidityPulse, 0)} />
          <Metric label="Growth pulse" value={percent(currentValue(points, "realGdpGrowth"))} note="Real GDP YoY" tone={tone(currentValue(points, "realGdpGrowth"), 0)} />
          <Metric label="Financial conditions" value={number(currentValue(points, "financialConditions"))} note="NFCI / tighter above zero" />
        </section>

        <section className="chart-grid primary-grid">
          <RegimeMap points={points} domain={domain} />
          <MacroChart title="Policy stance" note="Inflation, nominal policy and real policy rate" points={points} domain={domain} series={policySeries} primary />
        </section>

        <SectionTitle index="01" title="Price and policy" note="The nominal anchor, market inflation pricing and the shape of the rates complex" />
        <section className="chart-grid">
          <MacroChart title="Rates complex" note="Treasury, mortgage and breakeven rates" points={points} domain={domain} series={ratesSeries} />
          <MacroChart title="Real rates and curve" note="Ex-ante real 10Y and 10Y-2Y slope" points={points} domain={domain} series={realRatesSeries} />
        </section>

        <SectionTitle index="02" title="System liquidity" note="Money, central-bank balance sheet and reserve-system impulse" />
        <section className="chart-grid">
          <MacroChart title="Liquidity growth" note="Year-over-year change" points={points} domain={domain} series={liquidityGrowthSeries} />
          <MacroChart title="Liquidity stocks" note="Log10 of nominal levels" points={points} domain={domain} series={liquidityLevelSeries} unit="log" />
          <MacroChart title="Reverse repo absorption" note="Overnight reverse repurchase agreements" points={points} domain={domain} series={[{ key: "reverseRepoB", label: "Reverse repo", color: colors.violet }]} unit="billions" />
        </section>

        <SectionTitle index="03" title="Real economy" note="Output, industrial momentum and labor-market slack" />
        <section className="chart-grid">
          <MacroChart title="Growth and labor" note="Real GDP YoY, industrial production YoY and unemployment" points={points} domain={domain} series={growthSeries} />
          <MacroChart title="Money growth" note="M1 and M2 year-over-year change" points={points} domain={domain} series={moneyGrowthSeries} />
        </section>

        <SectionTitle index="04" title="Financial transmission" note="How policy reaches credit, volatility and risk appetite" />
        <section className="chart-grid">
          <MacroChart title="Conditions and credit" note="NFCI with investment-grade and high-yield spreads" points={points} domain={domain} series={creditSeries} unit="index" />
          <MacroChart title="Dollar and volatility" note="Broad trade-weighted dollar and VIX" points={points} domain={domain} series={riskSeries} unit="index" />
        </section>
      </>}
      {!!macro?.warnings?.length && <section className="warnings">{macro.warnings.map((warning) => <span key={warning}>{warning}</span>)}</section>}
    </main>
  </div>;
}

const policySeries: SeriesSpec[] = [
  { key: "inflation", label: "Inflation", color: colors.red },
  { key: "fedFunds", label: "Fed funds", color: colors.ink },
  { key: "realPolicyRate", label: "Real policy", color: colors.blue },
];
const ratesSeries: SeriesSpec[] = [
  { key: "treasury2Y", label: "2Y Treasury", color: colors.blue },
  { key: "treasury10Y", label: "10Y Treasury", color: colors.ink },
  { key: "mortgage30Y", label: "30Y mortgage", color: colors.red },
  { key: "breakeven10Y", label: "10Y breakeven", color: colors.gold },
];
const realRatesSeries: SeriesSpec[] = [
  { key: "real10Y", label: "Real 10Y", color: colors.green },
  { key: "yieldCurve", label: "10Y-2Y curve", color: colors.violet },
  { key: "realPolicyRate", label: "Real policy", color: colors.blue },
];
const liquidityGrowthSeries: SeriesSpec[] = [
  { key: "m2Growth", label: "M2", color: colors.green },
  { key: "fedAssetsGrowth", label: "Fed assets", color: colors.gold },
  { key: "monetaryBaseGrowth", label: "Monetary base", color: colors.blue },
];
const liquidityLevelSeries: SeriesSpec[] = [
  { key: "logM1", label: "M1", color: colors.cyan },
  { key: "logM2", label: "M2", color: colors.green },
  { key: "logFedAssets", label: "Fed assets", color: colors.gold },
  { key: "logMonetaryBase", label: "Monetary base", color: colors.violet },
];
const growthSeries: SeriesSpec[] = [
  { key: "realGdpGrowth", label: "Real GDP", color: colors.green },
  { key: "industrialGrowth", label: "Industrial production", color: colors.blue },
  { key: "unemployment", label: "Unemployment", color: colors.red },
];
const moneyGrowthSeries: SeriesSpec[] = [
  { key: "m1Growth", label: "M1", color: colors.cyan },
  { key: "m2Growth", label: "M2", color: colors.green },
];
const creditSeries: SeriesSpec[] = [
  { key: "financialConditions", label: "NFCI", color: colors.ink },
  { key: "corporateSpread", label: "IG OAS", color: colors.gold, axis: "right" },
  { key: "highYieldSpread", label: "HY OAS", color: colors.red, axis: "right" },
];
const riskSeries: SeriesSpec[] = [
  { key: "dollarIndex", label: "Broad dollar", color: colors.blue },
  { key: "vix", label: "VIX", color: colors.red, axis: "right" },
];

function Metric({ label, value, note, tone = "" }: { label: string; value: string; note: string; tone?: string }) {
  return <div><span>{label}</span><strong className={tone}>{value}</strong><small>{note}</small></div>;
}
function SectionTitle({ index, title, note }: { index: string; title: string; note: string }) {
  return <div className="section-title"><b>{index}</b><div><h2>{title}</h2><span>{note}</span></div></div>;
}
function percent(value?: number) { return value === undefined ? "n/a" : `${value.toFixed(1)}%`; }
function number(value?: number) { return value === undefined ? "n/a" : value.toFixed(2); }
function tone(value: number | undefined, threshold: number) { return value === undefined ? "" : value >= threshold ? "hot" : "cool"; }
function domainLabel(domain: [number, number]) { return `${new Date(domain[0]).getUTCFullYear()}-${new Date(domain[1]).getUTCFullYear()}`; }
function updatedLabel(value?: string) {
  if (!value) return "Waiting for data";
  return `Updated ${new Date(value).toLocaleDateString("en-US", { month: "short", day: "numeric", timeZone: "UTC" })}`;
}

export default App;
