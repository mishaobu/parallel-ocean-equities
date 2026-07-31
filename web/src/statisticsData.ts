import type { AnnualPoint, Equity, LiveQuote, PricePoint, QuarterlyPoint, QualityPoint } from "./types";

export type StatisticResolution = "month" | "quarter" | "year";
export type StatisticRange = "1y" | "3y" | "5y" | "10y" | "max";
export type StatisticUnit = "billions" | "currency" | "currency-per-share" | "percent" | "multiple" | "ratio" | "shares-billions" | "volume" | "date" | "text";

export interface StatisticPoint {
  date: string;
  label: string;
  value: number;
  basisDate?: string;
  source: string;
}

export interface StatisticTextPoint {
  date: string;
  label: string;
  value: string;
  source: string;
}

export interface StatisticMetric {
  key: string;
  label: string;
  group: string;
  unit: StatisticUnit;
  description: string;
  formula?: string;
  source: string;
  currentSource?: string;
  marketSensitive: boolean;
  currency?: string;
  nativeFrequency: string;
  current?: number | string;
  currentAsOf?: string;
  currentBasis?: string;
  unavailableReason?: string;
  yahoo: boolean;
  points: Record<StatisticResolution, StatisticPoint[]>;
  textPoints: Record<StatisticResolution, StatisticTextPoint[]>;
}

export interface StatisticsCatalog {
  metrics: StatisticMetric[];
  yahooMetricCount: number;
  availableYahooMetricCount: number;
  groups: string[];
}

interface Snapshot {
  date: string;
  label: string;
  basisDate?: string;
  source: string;
  price?: number;
  previousPrice?: number;
  revenueB?: number;
  grossProfitB?: number;
  ebitB?: number;
  ebitdaB?: number;
  operatingCashB?: number;
  fcfB?: number;
  dividendsB?: number;
  netIncomeB?: number;
  dilutedEps?: number;
  sharesB?: number;
  shareBasisAsOf?: string;
  dilutedSharesB?: number;
  cashB?: number;
  debtB?: number;
  netDebtB?: number;
  currentAssetsB?: number;
  currentLiabilitiesB?: number;
  assetsB?: number;
  liabilitiesB?: number;
  equityB?: number;
  priorAssetsB?: number;
  priorEquityB?: number;
  quarterlyRevenueGrowth?: number;
  quarterlyEarningsGrowth?: number;
}

interface Definition {
  key: string;
  label: string;
  group: string;
  unit: StatisticUnit;
  description: string;
  formula?: string;
  source: string;
  nativeFrequency: string;
  yahoo?: boolean;
  resolve?: (snapshot: Snapshot) => number | undefined;
  current?: (context: CurrentContext) => number | string | undefined;
  points?: (context: BuildContext, resolution: StatisticResolution) => StatisticPoint[];
  unavailableReason?: string;
  marketSensitive?: boolean;
  currentAsOf?: (context: CurrentContext) => string | undefined;
}

interface BuildContext {
  equity: Equity;
  quote?: LiveQuote;
  benchmark?: Equity;
  snapshots: Record<StatisticResolution, Snapshot[]>;
  current: Snapshot;
}

interface CurrentContext extends BuildContext {}

const yahooGroups = [
  "Valuation measures",
  "Fiscal year",
  "Profitability",
  "Management effectiveness",
  "Income statement",
  "Balance sheet",
  "Cash flow",
  "Stock price history",
  "Share statistics",
  "Dividends & splits",
];

const quoteSource = "Current market snapshot";
const filingSource = "SEC filings / point-in-time calculation";
const vendorGap = "A licensed fundamentals or ownership feed is required for this vendor-defined statistic.";

export function buildStatisticsCatalog(equity: Equity, quote?: LiveQuote, benchmark?: Equity): StatisticsCatalog {
  const quarter = buildQuarterSnapshots(equity);
  const year = buildAnnualSnapshots(equity);
  const month = buildMonthlySnapshots(equity, quarter);
  const current = buildCurrentSnapshot(equity, quote, quarter);
  const context: BuildContext = { equity, quote, benchmark, snapshots: { month, quarter, year }, current };
  const metrics = definitions.map((definition) => buildMetric(definition, context));
  const yahoo = metrics.filter((metric) => metric.yahoo);
  return {
    metrics,
    yahooMetricCount: yahoo.length,
    availableYahooMetricCount: yahoo.filter((metric) => metric.current !== undefined).length,
    groups: [...yahooGroups, "Parallel Ocean diagnostics"],
  };
}

function buildMetric(definition: Definition, context: BuildContext): StatisticMetric {
  const current = definition.current ? definition.current(context) : definition.resolve?.(context.current);
  const points = Object.fromEntries((["month", "quarter", "year"] as StatisticResolution[]).map((resolution) => {
    const rows = definition.points
      ? definition.points(context, resolution)
      : context.snapshots[resolution].flatMap((snapshot) => {
        const value = definition.resolve?.(snapshot);
        const source = historicalShareBasisKeys.has(definition.key) && snapshot.shareBasisAsOf
          ? `${snapshot.source}; actual basic shares disclosed ${snapshot.shareBasisAsOf}`
          : snapshot.source;
        return finite(value) ? [{ date: snapshot.date, label: snapshot.label, value, basisDate: snapshot.basisDate, source }] : [];
      });
    const recorded = recordedNumericPoints(context, definition.key, resolution);
    return [resolution, mergeRecordedPoints(rows, recorded, resolution)];
  })) as Record<StatisticResolution, StatisticPoint[]>;
  const textPoints = Object.fromEntries((["month", "quarter", "year"] as StatisticResolution[]).map((resolution) => [resolution, recordedTextPoints(context, definition.key, resolution)])) as Record<StatisticResolution, StatisticTextPoint[]>;
  const quoteAsOf = context.quote?.asOf ?? context.equity.current.priceAsOf;
  const filedAsOf = context.current.basisDate ?? context.current.date;
  const liveValuation = definition.group === "Valuation measures" && definition.key !== "peg-ratio" && context.current.price !== undefined;
  const liveQuoteMetric = quoteMarketMetricKeys.has(definition.key) && quoteAsOf !== undefined;
  const marketSensitive = definition.marketSensitive ?? (definition.source.includes(quoteSource) || liveValuation || liveQuoteMetric);
  const currentAsOf = definition.currentAsOf?.(context) ?? (marketSensitive ? quoteAsOf : filedAsOf);
  return {
    ...definition,
    yahoo: definition.yahoo !== false,
    current,
    currentAsOf,
    currentSource: currentFieldSource(definition, context),
    marketSensitive,
    currency: context.quote?.currency,
    currentBasis: currentBasis(definition, context),
    unavailableReason: current === undefined ? definition.unavailableReason ?? "No defensible current observation is available from the configured sources." : undefined,
    points,
    textPoints,
  };
}

function quoteHistory(context: BuildContext) {
  return context.quote?.history ?? context.equity.quoteHistory ?? [];
}

function recordedNumericPoints(context: BuildContext, key: string, resolution: StatisticResolution): StatisticPoint[] {
  const rows = quoteHistory(context).flatMap((snapshot) => {
    const value = snapshot.numeric?.[key];
    if (!finite(value) || !snapshot.asOf) return [];
    const date = snapshot.asOf.slice(0, 10);
    return [{ date, label: periodLabel(date, resolution), value, basisDate: snapshot.asOf, source: snapshot.sources?.[key] ?? snapshot.source ?? "Recorded market snapshot" }];
  });
  return resampleRecordedPoints(rows, resolution);
}

function recordedTextPoints(context: BuildContext, key: string, resolution: StatisticResolution): StatisticTextPoint[] {
  let rows: StatisticTextPoint[] = quoteHistory(context).flatMap((snapshot) => {
    const value = snapshot.text?.[key];
    if (!value || !snapshot.asOf) return [];
    const eventDate = key.endsWith("-date") && validDate(value)
      ? value.slice(0, 10)
      : key === "last-split-factor" && validDate(snapshot.text?.["last-split-date"])
        ? snapshot.text!["last-split-date"].slice(0, 10)
        : snapshot.asOf.slice(0, 10);
    return [{ date: eventDate, label: periodLabel(eventDate, resolution), value, source: snapshot.sources?.[key] ?? snapshot.source ?? "Recorded market snapshot" }];
  });
  if (key === "fiscal-year-ends") {
    rows = context.equity.annuals.filter((row) => !row.estimate && !!row.periodEnd).map((row) => ({ date: row.filedAt || row.periodEnd!, label: periodLabel(row.filedAt || row.periodEnd!, resolution), value: row.periodEnd!, source: filingSource }));
  } else if (key === "most-recent-quarter") {
    rows = (context.equity.quarterlies ?? []).map((row) => ({ date: row.filedAt || row.periodEnd, label: periodLabel(row.filedAt || row.periodEnd, resolution), value: row.periodEnd, source: filingSource }));
  }
  const changes: StatisticTextPoint[] = [];
  for (const row of rows.sort((left, right) => left.date.localeCompare(right.date))) {
    const previous = changes[changes.length - 1];
    if (previous?.value !== row.value || previous.date !== row.date) changes.push(row);
  }
  return resampleTextPoints(changes, resolution);
}

function resampleRecordedPoints(rows: StatisticPoint[], resolution: StatisticResolution): StatisticPoint[] {
  const buckets = new Map<string, StatisticPoint>();
  for (const row of rows) {
    const key = periodBucket(row.date, resolution);
    const current = buckets.get(key);
    if (!current || row.date >= current.date) buckets.set(key, { ...row, label: periodLabel(row.date, resolution) });
  }
  return [...buckets.values()].sort((left, right) => left.date.localeCompare(right.date));
}

function resampleTextPoints(rows: StatisticTextPoint[], resolution: StatisticResolution): StatisticTextPoint[] {
  const buckets = new Map<string, StatisticTextPoint>();
  for (const row of rows) {
    const key = periodBucket(row.date, resolution);
    const current = buckets.get(key);
    if (!current || row.date >= current.date) buckets.set(key, { ...row, label: periodLabel(row.date, resolution) });
  }
  return [...buckets.values()].sort((left, right) => left.date.localeCompare(right.date));
}

function mergeRecordedPoints(derived: StatisticPoint[], recorded: StatisticPoint[], resolution: StatisticResolution): StatisticPoint[] {
  if (!recorded.length) return dedupePoints(derived);
  const buckets = new Map<string, StatisticPoint>();
  for (const row of [...derived, ...recorded]) {
    const key = periodBucket(row.date, resolution);
    const current = buckets.get(key);
    if (!current || row.date >= current.date || recorded.includes(row)) buckets.set(key, row);
  }
  return [...buckets.values()].sort((left, right) => left.date.localeCompare(right.date));
}

const quoteFieldByMetric: Record<string, keyof LiveQuote> = {
  "market-cap": "marketCapB",
  "enterprise-value": "enterpriseValueB",
  "previous-close": "previousClose",
  change: "change",
  "change-percent": "changePercent",
  "beta-5y": "beta5YMonthly",
  "change-52-week": "change52Week",
  "high-52-week": "high52Week",
  "low-52-week": "low52Week",
  "moving-average-50d": "movingAverage50Day",
  "moving-average-200d": "movingAverage200Day",
  "average-volume-3m": "averageVolume3Month",
  "average-volume-10d": "averageVolume10Day",
  "shares-outstanding": "sharesOutstandingB",
  "forward-dividend-rate": "forwardAnnualDividendRate",
  "forward-dividend-yield": "forwardAnnualDividendYield",
  "trailing-dividend-rate": "trailingAnnualDividendRate",
  "trailing-dividend-yield": "trailingAnnualDividendYield",
  "average-dividend-yield-5y": "averageDividendYield5Year",
  "dividend-date": "lastDividendDate",
  "ex-dividend-date": "exDividendDate",
  "last-split-factor": "lastSplitFactor",
  "last-split-date": "lastSplitDate",
  price: "price",
};

const quoteMarketMetricKeys = new Set(Object.keys(quoteFieldByMetric).filter((key) => key !== "shares-outstanding"));
const historicalShareBasisKeys = new Set(["market-cap", "enterprise-value", "price-sales", "price-book", "ev-revenue", "ev-ebitda", "cash-per-share", "book-value-per-share", "shares-outstanding", "ev-ebit", "free-cash-flow-yield"]);

function currentFieldSource(definition: Definition, context: BuildContext): string {
  const quoteField = quoteFieldByMetric[definition.key];
  if (quoteField) {
    const exact = context.quote?.fieldSources?.[String(quoteField)];
    if (exact) return exact;
  }
  return definition.source;
}

function currentBasis(definition: Definition, context: BuildContext): string | undefined {
  const price = context.quote?.price ?? context.equity.current.price;
  const shares = exactShares(context);
  const filingDate = context.current.basisDate;
  if (definition.group === "Valuation measures" && price !== undefined) {
    if (definition.key === "market-cap" && shares !== undefined) {
      const shareDate = context.quote?.shareBasisAsOf ?? context.equity.current.sharesOutstandingAsOf;
      return `${formatCurrencyNumber(price, context.quote?.currency, true)} × ${shares.toFixed(3)}B disclosed shares${shareDate ? ` (${shareDate})` : ""}`;
    }
    if (definition.key === "enterprise-value") return `Live market cap + net debt${filingDate ? ` through ${filingDate}` : ""}`;
    if (definition.key === "trailing-pe") return `Live price ÷ TTM diluted EPS${filingDate ? ` through ${filingDate}` : ""}`;
    if (definition.key === "forward-pe") return "Live price ÷ modeled N12M diluted EPS";
    if (["price-sales", "price-book"].includes(definition.key)) return `Live market cap ÷ reported fundamentals${filingDate ? ` through ${filingDate}` : ""}`;
    if (["ev-revenue", "ev-ebitda"].includes(definition.key)) return `Live enterprise value ÷ reported fundamentals${filingDate ? ` through ${filingDate}` : ""}`;
  }
  if (definition.key === "shares-outstanding" && shares !== undefined) {
    const shareDate = context.quote?.shareBasisAsOf ?? context.equity.current.sharesOutstandingAsOf;
    return shareDate ? `Issuer-disclosed basic shares as of ${shareDate}` : "Latest issuer-disclosed basic shares";
  }
  if (definition.source === quoteSource) return context.quote?.marketState || "market snapshot";
  return context.current.basisDate ? `Fundamentals through ${context.current.basisDate}` : undefined;
}

const definitions: Definition[] = [
  // Yahoo valuation measures (9)
  metric("market-cap", "Market cap", "Valuation measures", "billions", "Live equity value uses the current price and latest disclosed basic shares. Filing history uses accession-matched actual shares and the last market close available when that filing arrived.", "price × shares outstanding", (s) => multiply(s.price, s.sharesB), { current: exactMarketCap }),
  metric("enterprise-value", "Enterprise value", "Valuation measures", "billions", "Equity value adjusted for reported net debt. Current EV is a timestamped market/filing hybrid; filing history uses accession-matched shares and available-date prices.", "market cap + debt − cash and investments", (s) => add(multiply(s.price, s.sharesB), s.netDebtB), { current: exactEnterpriseValue }),
  metric("trailing-pe", "Trailing P/E", "Valuation measures", "multiple", "Price relative to trailing diluted earnings available at that date.", "price ÷ TTM diluted EPS", (s) => positiveRatio(s.price, s.dilutedEps)),
  metric("forward-pe", "Forward P/E", "Valuation measures", "multiple", "Live price relative to the configured forward earnings model; this is not analyst consensus. History stays empty until point-in-time forecasts are archived.", "price ÷ modeled N12M diluted EPS", undefined, { current: (c) => positiveRatio(c.quote?.price ?? c.equity.current.price, c.equity.forecast?.forwardEps ?? c.equity.current.forwardEps), points: emptyPoints, source: "Current market snapshot + Parallel Ocean forecast model", unavailableReason: "A forward model or archived point-in-time forecast is not available." }),
  metric("peg-ratio", "PEG ratio (5Y expected)", "Valuation measures", "multiple", "Forward P/E relative to expected five-year earnings growth.", "forward P/E ÷ expected 5Y EPS growth", undefined, { unavailableReason: "Archived five-year consensus growth estimates require a licensed estimates feed." }),
  metric("price-sales", "Price / sales", "Valuation measures", "multiple", "Equity value relative to trailing revenue.", "market cap ÷ TTM revenue", (s) => positiveRatio(multiply(s.price, s.sharesB), s.revenueB), { current: (c) => positiveRatio(exactMarketCap(c), c.current.revenueB) }),
  metric("price-book", "Price / book", "Valuation measures", "multiple", "Equity value relative to reported common equity.", "market cap ÷ book equity", (s) => positiveRatio(multiply(s.price, s.sharesB), s.equityB), { current: (c) => positiveRatio(exactMarketCap(c), c.current.equityB) }),
  metric("ev-revenue", "Enterprise value / revenue", "Valuation measures", "multiple", "Enterprise value relative to trailing revenue.", "enterprise value ÷ TTM revenue", (s) => positiveRatio(add(multiply(s.price, s.sharesB), s.netDebtB), s.revenueB), { current: (c) => positiveRatio(exactEnterpriseValue(c), c.current.revenueB) }),
  metric("ev-ebitda", "Enterprise value / EBITDA", "Valuation measures", "multiple", "Enterprise value relative to trailing EBITDA.", "enterprise value ÷ TTM EBITDA", (s) => positiveRatio(add(multiply(s.price, s.sharesB), s.netDebtB), s.ebitdaB), { current: (c) => positiveRatio(exactEnterpriseValue(c), c.current.ebitdaB) }),

  // Yahoo financial highlights (22)
  textMetric("fiscal-year-ends", "Fiscal year ends", "Fiscal year", "Most recent reported fiscal-year end.", (c) => lastActual(c.equity.annuals)?.periodEnd, filingSource),
  textMetric("most-recent-quarter", "Most recent quarter", "Fiscal year", "Most recent reported fiscal quarter end.", (c) => last(c.equity.quarterlies)?.periodEnd, filingSource),
  metric("profit-margin", "Profit margin", "Profitability", "percent", "Trailing net income as a share of revenue.", "TTM net income ÷ TTM revenue", (s) => ratio(s.netIncomeB, s.revenueB)),
  metric("operating-margin", "Operating margin", "Profitability", "percent", "Trailing operating income as a share of revenue.", "TTM operating income ÷ TTM revenue", (s) => ratio(s.ebitB, s.revenueB)),
  metric("return-on-assets", "Return on assets", "Management effectiveness", "percent", "Trailing earnings relative to average reported assets.", "TTM net income ÷ average assets", (s) => ratio(s.netIncomeB, average(s.assetsB, s.priorAssetsB))),
  metric("return-on-equity", "Return on equity", "Management effectiveness", "percent", "Trailing earnings relative to average reported equity.", "TTM net income ÷ average equity", (s) => ratio(s.netIncomeB, average(s.equityB, s.priorEquityB))),
  metric("revenue", "Revenue", "Income statement", "billions", "Revenue over the trailing twelve months.", "sum of latest four reported quarters", (s) => s.revenueB),
  metric("revenue-per-share", "Revenue per share", "Income statement", "currency-per-share", "Trailing revenue divided by the weighted diluted share basis for the same period.", "TTM revenue ÷ TTM weighted diluted shares", (s) => ratio(s.revenueB, s.dilutedSharesB)),
  metric("quarterly-revenue-growth", "Quarterly revenue growth (YoY)", "Income statement", "percent", "Latest reported quarter revenue growth from the comparable prior-year quarter.", "quarter revenue ÷ prior-year quarter revenue − 1", (s) => s.quarterlyRevenueGrowth),
  metric("gross-profit", "Gross profit", "Income statement", "billions", "Gross profit over the trailing twelve months.", "sum of latest four reported quarters", (s) => s.grossProfitB),
  metric("ebitda", "EBITDA", "Income statement", "billions", "Operating income plus reported depreciation and amortization.", "TTM EBIT + TTM D&A", (s) => s.ebitdaB),
  metric("net-income", "Net income available to common", "Income statement", "billions", "Trailing standardized SEC net income. It is a transparent proxy when the issuer does not report a separate common-stockholder concept.", "sum of latest four reported quarters", (s) => s.netIncomeB),
  metric("diluted-eps", "Diluted EPS", "Income statement", "currency-per-share", "Diluted earnings per share over the trailing twelve months.", "sum of latest four reported quarters", (s) => s.dilutedEps),
  metric("quarterly-earnings-growth", "Quarterly earnings growth (YoY)", "Income statement", "percent", "Latest reported quarter net-income growth from the comparable prior-year quarter.", "quarter net income ÷ prior-year quarter net income − 1", (s) => s.quarterlyEarningsGrowth),
  metric("total-cash", "Total cash", "Balance sheet", "billions", "Cash, cash equivalents, and current marketable investments reported for the latest quarter.", undefined, (s) => s.cashB),
  metric("cash-per-share", "Total cash per share", "Balance sheet", "currency-per-share", "Reported cash divided by the latest disclosed basic shares.", "cash ÷ shares outstanding", (s) => ratio(s.cashB, s.sharesB), { current: (c) => ratio(c.current.cashB, exactShares(c)) }),
  metric("total-debt", "Total debt", "Balance sheet", "billions", "Current and non-current debt reported for the latest quarter.", undefined, (s) => s.debtB),
  metric("debt-equity", "Total debt / equity", "Balance sheet", "percent", "Reported debt relative to book equity.", "total debt ÷ book equity", (s) => ratio(s.debtB, s.equityB)),
  metric("current-ratio", "Current ratio", "Balance sheet", "ratio", "Current assets relative to current liabilities.", "current assets ÷ current liabilities", (s) => positiveRatio(s.currentAssetsB, s.currentLiabilitiesB)),
  metric("book-value-per-share", "Book value per share", "Balance sheet", "currency-per-share", "Reported book equity per latest disclosed basic share.", "book equity ÷ shares outstanding", (s) => ratio(s.equityB, s.sharesB), { current: (c) => ratio(c.current.equityB, exactShares(c)) }),
  metric("operating-cash-flow", "Operating cash flow", "Cash flow", "billions", "Cash generated by operations over the trailing twelve months.", "sum of latest four reported quarters", (s) => s.operatingCashB),
  metric("levered-free-cash-flow", "Levered free cash flow", "Cash flow", "billions", "Vendor-defined cash flow after operating and financing obligations.", undefined, undefined, { unavailableReason: "Yahoo's vendor-defined levered FCF cannot be reproduced from standardized SEC facts without methodology assumptions." }),

  // Yahoo trading information (29)
  metric("beta-5y", "Beta (5Y monthly)", "Stock price history", "ratio", "Sensitivity of 60 completed monthly total returns to the configured S&P 500 proxy (SPY); this is not Yahoo's proprietary beta series.", "covariance(stock, SPY) ÷ SPY variance", undefined, { current: (c) => c.quote?.beta5YMonthly ?? monthlyBeta(c.equity.prices, c.benchmark?.prices), source: "Yahoo adjusted closes / SPY proxy / calculated", points: emptyPoints }),
  metric("change-52-week", "52-week change", "Stock price history", "percent", "Price change over the trailing 52 weeks.", "latest price ÷ price one year earlier − 1", undefined, { current: (c) => c.quote?.change52Week ?? c.equity.current.return1Y, source: quoteSource, points: rollingPricePoints((rows, index) => trailingReturn(rows, index, 12)) }),
  metric("sp500-change-52-week", "S&P 500 52-week change", "Stock price history", "percent", "S&P 500 proxy price change over the trailing 52 weeks.", "benchmark price ÷ price one year earlier − 1", undefined, { current: (c) => latestRolling(c.benchmark?.prices, 12), currentAsOf: (c) => last(sortedPrices(c.benchmark?.prices))?.date, marketSensitive: true, nativeFrequency: "Monthly market series", source: "Benchmark monthly closes / calculated", points: benchmarkRollingPoints((rows, index) => trailingReturn(rows, index, 12)) }),
  metric("high-52-week", "52-week high", "Stock price history", "currency", "Highest traded price in the trailing 52 weeks. History uses recorded daily-stat snapshots, never monthly-close proxies.", undefined, undefined, { current: (c) => c.quote?.high52Week ?? c.equity.current.high52Week, source: quoteSource, points: emptyPoints }),
  metric("low-52-week", "52-week low", "Stock price history", "currency", "Lowest traded price in the trailing 52 weeks. History uses recorded daily-stat snapshots, never monthly-close proxies.", undefined, undefined, { current: (c) => c.quote?.low52Week ?? c.equity.current.low52Week, source: quoteSource, points: emptyPoints }),
  metric("moving-average-50d", "50-day moving average", "Stock price history", "currency", "Average daily close over the latest 50 trading sessions.", undefined, undefined, { current: (c) => c.quote?.movingAverage50Day, source: quoteSource, points: emptyPoints }),
  metric("moving-average-200d", "200-day moving average", "Stock price history", "currency", "Average daily close over the latest 200 trading sessions.", undefined, undefined, { current: (c) => c.quote?.movingAverage200Day, source: quoteSource, points: emptyPoints }),
  metric("average-volume-3m", "Average volume (3 month)", "Share statistics", "volume", "Average daily traded volume over roughly three months.", undefined, undefined, { current: (c) => c.quote?.averageVolume3Month, source: quoteSource, points: emptyPoints }),
  metric("average-volume-10d", "Average volume (10 day)", "Share statistics", "volume", "Average daily traded volume over ten sessions.", undefined, undefined, { current: (c) => c.quote?.averageVolume10Day, source: quoteSource, points: emptyPoints }),
  metric("shares-outstanding", "Shares outstanding", "Share statistics", "shares-billions", "Actual basic common shares disclosed by the issuer; no weighted-diluted fallback is used.", undefined, (s) => s.sharesB, { current: exactShares }),
  metric("implied-shares-outstanding", "Implied shares outstanding", "Share statistics", "shares-billions", "Common share equivalent after convertible subsidiary equity.", undefined, undefined, { unavailableReason: vendorGap }),
  metric("float", "Float", "Share statistics", "shares-billions", "Shares estimated to be publicly tradable.", undefined, undefined, { unavailableReason: vendorGap }),
  metric("held-by-insiders", "% held by insiders", "Share statistics", "percent", "Reported beneficial ownership attributed to insiders.", undefined, undefined, { unavailableReason: "Normalized Forms 3/4/5 and proxy ownership data are not configured." }),
  metric("held-by-institutions", "% held by institutions", "Share statistics", "percent", "Reported institutional ownership; filings are delayed and incomplete.", undefined, undefined, { unavailableReason: "Normalized Form 13F and 13D/G ownership data are not configured." }),
  metric("shares-short", "Shares short", "Share statistics", "volume", "Exchange-reported short interest for the latest settlement date.", undefined, undefined, { unavailableReason: "A dated exchange short-interest feed is not configured." }),
  metric("short-ratio", "Short ratio", "Share statistics", "ratio", "Short interest divided by average daily volume.", undefined, undefined, { unavailableReason: "A dated exchange short-interest feed is not configured." }),
  metric("short-percent-float", "Short % of float", "Share statistics", "percent", "Short interest relative to estimated float.", undefined, undefined, { unavailableReason: vendorGap }),
  metric("short-percent-shares", "Short % of shares outstanding", "Share statistics", "percent", "Short interest relative to basic shares outstanding.", undefined, undefined, { unavailableReason: "A dated exchange short-interest feed is not configured." }),
  metric("shares-short-prior-month", "Shares short prior month", "Share statistics", "volume", "Short interest from the preceding published observation.", undefined, undefined, { unavailableReason: "A dated exchange short-interest feed is not configured." }),
  metric("forward-dividend-rate", "Forward annual dividend rate", "Dividends & splits", "currency-per-share", "Latest declared regular dividend annualized.", "latest regular dividend × payment frequency", undefined, { current: (c) => c.quote?.forwardAnnualDividendRate, source: quoteSource, points: emptyPoints }),
  metric("forward-dividend-yield", "Forward annual dividend yield", "Dividends & splits", "percent", "Annualized latest regular dividend relative to current price.", "forward annual dividend rate ÷ price", undefined, { current: (c) => c.quote?.forwardAnnualDividendYield, source: quoteSource, points: emptyPoints }),
  metric("trailing-dividend-rate", "Trailing annual dividend rate", "Dividends & splits", "currency-per-share", "Regular cash dividends paid per share over the trailing year.", undefined, undefined, { current: (c) => c.quote?.trailingAnnualDividendRate, source: quoteSource, points: emptyPoints }),
  metric("trailing-dividend-yield", "Trailing annual dividend yield", "Dividends & splits", "percent", "Trailing annual dividends relative to current price.", "trailing annual dividend rate ÷ price", undefined, { current: (c) => c.quote?.trailingAnnualDividendYield, source: quoteSource, points: emptyPoints }),
  metric("average-dividend-yield-5y", "5-year average dividend yield", "Dividends & splits", "percent", "Average observed annual dividend yield over five years.", undefined, undefined, { current: (c) => c.quote?.averageDividendYield5Year, source: quoteSource, points: emptyPoints }),
  metric("payout-ratio", "Payout ratio", "Dividends & splits", "percent", "Trailing dividends per share relative to diluted EPS; filing history uses aggregate cash distributions / net income as a labeled proxy.", "TTM dividend rate ÷ TTM diluted EPS", (s) => ratio(s.dividendsB, s.netIncomeB), { current: (c) => positiveRatio(c.quote?.trailingAnnualDividendRate, c.current.dilutedEps), source: "Current market snapshot + SEC filings / calculated" }),
  textMetric("dividend-date", "Dividend date", "Dividends & splits", "Most recent cash-dividend payment date.", (c) => c.quote?.lastDividendDate, quoteSource),
  textMetric("ex-dividend-date", "Ex-dividend date", "Dividends & splits", "Most recent ex-dividend date in the market feed.", (c) => c.quote?.exDividendDate, quoteSource),
  textMetric("last-split-factor", "Last split factor", "Dividends & splits", "Ratio of the most recent stock split.", (c) => c.quote?.lastSplitFactor, quoteSource),
  textMetric("last-split-date", "Last split date", "Dividends & splits", "Effective date of the most recent stock split.", (c) => c.quote?.lastSplitDate, quoteSource),

  // Parallel Ocean additions
  metric("price", "Live price", "Parallel Ocean diagnostics", "currency", "Latest regular-market price or persisted close.", undefined, (s) => s.price, { yahoo: false, current: (c) => c.quote?.price ?? c.equity.current.price, source: quoteSource, points: pricePoints }),
  metric("previous-close", "Previous close", "Parallel Ocean diagnostics", "currency", "Close from the latest completed market session.", undefined, undefined, { yahoo: false, current: (c) => c.quote?.previousClose, source: quoteSource, points: emptyPoints }),
  metric("change", "Session change", "Parallel Ocean diagnostics", "currency", "Current price less the previous completed-session close.", "current price − previous close", undefined, { yahoo: false, current: (c) => c.quote?.change, source: quoteSource, points: emptyPoints }),
  metric("change-percent", "Session change %", "Parallel Ocean diagnostics", "percent", "Session price change relative to the previous close.", "session change ÷ previous close", undefined, { yahoo: false, current: (c) => c.quote?.changePercent, source: quoteSource, points: emptyPoints }),
  metric("gross-margin", "Gross margin", "Parallel Ocean diagnostics", "percent", "Trailing gross profit as a share of revenue.", "TTM gross profit ÷ TTM revenue", (s) => ratio(s.grossProfitB, s.revenueB), { yahoo: false }),
  metric("operating-cash-margin", "Operating cash margin", "Parallel Ocean diagnostics", "percent", "Operating cash flow as a share of revenue.", "TTM operating cash flow ÷ TTM revenue", (s) => ratio(s.operatingCashB, s.revenueB), { yahoo: false }),
  metric("free-cash-flow", "Free cash flow", "Parallel Ocean diagnostics", "billions", "Operating cash flow after capital expenditure.", "operating cash flow − capex", (s) => s.fcfB, { yahoo: false }),
  metric("free-cash-flow-margin", "Free cash flow margin", "Parallel Ocean diagnostics", "percent", "Free cash flow as a share of revenue.", "TTM free cash flow ÷ TTM revenue", (s) => ratio(s.fcfB, s.revenueB), { yahoo: false }),
  metric("ev-ebit", "Enterprise value / EBIT", "Parallel Ocean diagnostics", "multiple", "Enterprise value relative to trailing operating income.", "enterprise value ÷ TTM EBIT", (s) => positiveRatio(add(multiply(s.price, s.sharesB), s.netDebtB), s.ebitB), { yahoo: false, current: (c) => positiveRatio(exactEnterpriseValue(c), c.current.ebitB) }),
  metric("free-cash-flow-yield", "Free cash flow yield", "Parallel Ocean diagnostics", "percent", "Trailing free cash flow relative to live equity value.", "TTM free cash flow ÷ market cap", (s) => ratio(s.fcfB, multiply(s.price, s.sharesB)), { yahoo: false, current: (c) => ratio(c.current.fcfB, exactMarketCap(c)) }),
  metric("net-debt-ebitda", "Net debt / EBITDA", "Parallel Ocean diagnostics", "multiple", "Reported net debt relative to trailing EBITDA.", "net debt ÷ TTM EBITDA", (s) => ratio(s.netDebtB, s.ebitdaB), { yahoo: false }),
  metric("net-debt", "Net debt", "Parallel Ocean diagnostics", "billions", "Debt less cash and short-term investments.", "debt − cash − investments", (s) => s.netDebtB, { yahoo: false }),
  metric("total-assets", "Total assets", "Parallel Ocean diagnostics", "billions", "Reported assets at period end.", undefined, (s) => s.assetsB, { yahoo: false }),
  metric("book-equity", "Book equity", "Parallel Ocean diagnostics", "billions", "Reported stockholders' equity at period end.", undefined, (s) => s.equityB, { yahoo: false }),
  metric("roic", "Return on invested capital", "Parallel Ocean diagnostics", "percent", "After-tax operating return on estimated invested capital.", undefined, undefined, { yahoo: false, current: (c) => c.equity.quality?.roic, points: qualityPoints("roic"), source: "SEC filings / Parallel Ocean calculation" }),
];

function metric(
  key: string,
  label: string,
  group: string,
  unit: StatisticUnit,
  description: string,
  formula: string | undefined,
  resolve: Definition["resolve"],
  options: Partial<Definition> = {},
): Definition {
  const source = options.source ?? filingSource;
  const nativeFrequency = options.nativeFrequency ?? (source.includes(quoteSource) ? "Market snapshot" : group === "Stock price history" ? "Market series" : "Filing-derived");
  return { key, label, group, unit, description, formula, resolve, source, nativeFrequency, yahoo: true, ...options };
}

function textMetric(key: string, label: string, group: string, description: string, current: (context: CurrentContext) => string | undefined, source: string): Definition {
  return { key, label, group, unit: key.includes("date") || key.includes("quarter") || key.includes("year") ? "date" : "text", description, source, nativeFrequency: "Event", yahoo: true, current, points: emptyPoints };
}

function buildQuarterSnapshots(equity: Equity): Snapshot[] {
  const byQuarter = new Map<number, QuarterlyPoint>();
  for (const row of equity.quarterlies ?? []) {
    const ordinal = fiscalQuarterOrdinal(row);
    if (ordinal === undefined) continue;
    const existing = byQuarter.get(ordinal);
    if (!existing || (row.filedAt || row.periodEnd) >= (existing.filedAt || existing.periodEnd)) byQuarter.set(ordinal, row);
  }
  const rows = [...byQuarter.values()].sort((left, right) => fiscalQuarterOrdinal(left)! - fiscalQuarterOrdinal(right)!);
  const prices = sortedPrices(equity.prices);
  const snapshots: Snapshot[] = [];
  for (let index = 3; index < rows.length; index++) {
    const window = rows.slice(index - 3, index + 1);
    if (!consecutiveQuarters(window)) continue;
    const latestRow = rows[index];
    const priorYear = byQuarter.get(fiscalQuarterOrdinal(latestRow)! - 4);
    const previousBalance = priorYear;
    const price = priceOnOrBefore(prices, latestRow.filedAt || latestRow.sharesOutstandingAsOf || latestRow.periodEnd);
    snapshots.push(snapshotFromQuarterWindow(window, latestRow, priorYear, previousBalance, price));
  }
  return snapshots;
}

function snapshotFromQuarterWindow(window: QuarterlyPoint[], latestRow: QuarterlyPoint, priorYear: QuarterlyPoint | undefined, previousBalance: QuarterlyPoint | undefined, price?: number): Snapshot {
  return {
    date: latestRow.periodEnd,
    label: `FY${String(latestRow.fiscalYear).slice(-2)} ${latestRow.fiscalQuarter}`,
    basisDate: latestRow.filedAt || latestRow.periodEnd,
    source: filingSource,
    price,
    revenueB: sumAll(window, "revenueB"),
    grossProfitB: sumAll(window, "grossProfitB"),
    ebitB: sumAll(window, "ebitB"),
    ebitdaB: sumAll(window, "ebitdaB"),
    operatingCashB: sumAll(window, "operatingCashB"),
    fcfB: sumAll(window, "fcfB"),
    dividendsB: sumAll(window, "dividendsB"),
    netIncomeB: sumAll(window, "netIncomeB"),
    dilutedEps: sumAll(window, "dilutedEps"),
    sharesB: latestRow.sharesOutstandingB,
    shareBasisAsOf: latestRow.sharesOutstandingAsOf,
    dilutedSharesB: averageAll(window, "dilutedSharesB"),
    cashB: add(latestRow.cashB, latestRow.investmentsB) ?? latestRow.cashB,
    debtB: latestRow.debtB,
    netDebtB: latestRow.netDebtB,
    currentAssetsB: latestRow.currentAssetsB,
    currentLiabilitiesB: latestRow.currentLiabilitiesB,
    assetsB: latestRow.assetsB,
    liabilitiesB: latestRow.liabilitiesB,
    equityB: latestRow.equityB,
    priorAssetsB: previousBalance?.assetsB,
    priorEquityB: previousBalance?.equityB,
    quarterlyRevenueGrowth: growth(latestRow.revenueB, priorYear?.revenueB),
    quarterlyEarningsGrowth: growth(latestRow.netIncomeB, priorYear?.netIncomeB),
  };
}

function buildAnnualSnapshots(equity: Equity): Snapshot[] {
  const rows = equity.annuals.filter((row) => !row.estimate).sort((a, b) => (a.periodEnd ?? String(a.fiscalYear)).localeCompare(b.periodEnd ?? String(b.fiscalYear)));
  const prices = sortedPrices(equity.prices);
  return rows.map((row, index) => snapshotFromAnnual(row, rows[index - 1], priceOnOrBefore(prices, row.filedAt || row.sharesOutstandingAsOf || row.periodEnd || `${row.fiscalYear}-12-31`)));
}

function snapshotFromAnnual(row: AnnualPoint, previous: AnnualPoint | undefined, price?: number): Snapshot {
  const date = row.periodEnd ?? `${row.fiscalYear}-12-31`;
  return {
    date,
    label: `FY${row.fiscalYear}`,
    basisDate: row.filedAt || date,
    source: filingSource,
    price,
    revenueB: row.revenueB,
    grossProfitB: row.grossProfitB,
    ebitB: row.ebitB,
    ebitdaB: row.ebitdaB,
    operatingCashB: row.operatingCashB,
    fcfB: row.fcfB,
    dividendsB: row.dividendsB,
    netIncomeB: row.netIncomeB,
    dilutedEps: row.dilutedEps,
    sharesB: row.sharesOutstandingB,
    shareBasisAsOf: row.sharesOutstandingAsOf,
    dilutedSharesB: row.dilutedSharesB,
    cashB: add(row.cashB, row.investmentsB) ?? row.cashB,
    debtB: row.debtB,
    netDebtB: row.netDebtB,
    currentAssetsB: row.currentAssetsB,
    currentLiabilitiesB: row.currentLiabilitiesB,
    assetsB: row.assetsB,
    liabilitiesB: row.liabilitiesB,
    equityB: row.equityB,
    priorAssetsB: previous?.assetsB,
    priorEquityB: previous?.equityB,
  };
}

function buildMonthlySnapshots(equity: Equity, quarters: Snapshot[]): Snapshot[] {
  const prices = sortedPrices(equity.prices);
  if (!prices.length) return [];
  return prices.flatMap((price, index) => {
    const available = quarters.filter((snapshot) => (snapshot.basisDate ?? snapshot.date) <= price.date);
    const basis = last(available);
    if (!basis) return [];
    return [{ ...basis, date: price.date, label: monthLabel(price.date), price: price.close, previousPrice: prices[index - 1]?.close, source: `${basis.source}; month-end market close` }];
  });
}

function buildCurrentSnapshot(equity: Equity, quote: LiveQuote | undefined, quarters: Snapshot[]): Snapshot {
  const latestQuarter = last([...(equity.quarterlies ?? [])].sort((left, right) => left.periodEnd.localeCompare(right.periodEnd)));
  const latestTTM = last(quarters);
  const basis = latestQuarter && latestTTM?.date !== latestQuarter.periodEnd
    ? balanceSnapshot(latestQuarter, equity)
    : latestTTM ?? last(buildAnnualSnapshots(equity)) ?? { date: equity.current.priceAsOf ?? "", label: "Current", source: filingSource };
  const sharesB = quote?.sharesOutstandingB ?? equity.current.sharesOutstandingB ?? basis.sharesB;
  return {
    ...basis,
    date: quote?.asOf?.slice(0, 10) || equity.current.priceAsOf || basis.date,
    label: "Current",
    price: quote?.price ?? equity.current.price ?? basis.price,
    sharesB,
    shareBasisAsOf: quote?.shareBasisAsOf ?? equity.current.sharesOutstandingAsOf ?? basis.shareBasisAsOf,
  };
}

function balanceSnapshot(row: QuarterlyPoint, equity: Equity): Snapshot {
  const prices = sortedPrices(equity.prices);
  return {
    date: row.periodEnd,
    label: `${row.fiscalYear} ${row.fiscalQuarter}`,
    basisDate: row.filedAt || row.periodEnd,
    source: filingSource,
    price: priceOnOrBefore(prices, row.filedAt || row.periodEnd),
    sharesB: row.sharesOutstandingB,
    shareBasisAsOf: row.sharesOutstandingAsOf,
    dilutedSharesB: row.dilutedSharesB,
    cashB: add(row.cashB, row.investmentsB) ?? row.cashB,
    debtB: row.debtB,
    netDebtB: row.netDebtB,
    currentAssetsB: row.currentAssetsB,
    currentLiabilitiesB: row.currentLiabilitiesB,
    assetsB: row.assetsB,
    liabilitiesB: row.liabilitiesB,
    equityB: row.equityB,
  };
}

function qualityPoints(property: keyof QualityPoint) {
  return (context: BuildContext, resolution: StatisticResolution) => resampleDirect(context.equity.qualities, property, resolution, "SEC filings / Parallel Ocean calculation");
}

function resampleDirect<T extends { date: string }>(rows: T[] | undefined, property: keyof T, resolution: StatisticResolution, source: string): StatisticPoint[] {
  if (!rows?.length) return [];
  const raw = rows.flatMap((row) => finite(row[property]) ? [{ date: row.date, label: periodLabel(row.date, resolution), value: row[property] as number, source }] : []);
  return resamplePoints(raw, resolution);
}

function pricePoints(context: BuildContext, resolution: StatisticResolution): StatisticPoint[] {
  const rows = sortedPrices(context.equity.prices).map((point) => ({ date: point.date, label: periodLabel(point.date, resolution), value: point.close, source: "Market close" }));
  return resamplePoints(rows, resolution);
}

function rollingPricePoints(resolve: (rows: PricePoint[], index: number) => number | undefined) {
  return (context: BuildContext, resolution: StatisticResolution) => {
    const rows = sortedPrices(context.equity.prices);
    const points = rows.flatMap((row, index) => {
      const value = resolve(rows, index);
      return finite(value) ? [{ date: row.date, label: periodLabel(row.date, resolution), value, source: "Monthly closes / calculated" }] : [];
    });
    return resamplePoints(points, resolution);
  };
}

function benchmarkRollingPoints(resolve: (rows: PricePoint[], index: number) => number | undefined) {
  return (context: BuildContext, resolution: StatisticResolution) => {
    const rows = sortedPrices(context.benchmark?.prices);
    return resamplePoints(rows.flatMap((row, index) => {
      const value = resolve(rows, index);
      return finite(value) ? [{ date: row.date, label: periodLabel(row.date, resolution), value, source: "Benchmark monthly closes / calculated" }] : [];
    }), resolution);
  };
}

function emptyPoints(): StatisticPoint[] { return []; }

function resamplePoints(rows: StatisticPoint[], resolution: StatisticResolution): StatisticPoint[] {
  if (resolution === "month") return rows;
  const buckets = new Map<string, StatisticPoint>();
  for (const row of rows) {
    const date = new Date(`${row.date.slice(0, 10)}T00:00:00Z`);
    if (Number.isNaN(date.getTime())) continue;
    const key = resolution === "year" ? String(date.getUTCFullYear()) : `${date.getUTCFullYear()}-Q${Math.floor(date.getUTCMonth() / 3) + 1}`;
    const current = buckets.get(key);
    if (!current || row.date > current.date) buckets.set(key, { ...row, label: resolution === "year" ? key : key.replace("-", " ") });
  }
  return [...buckets.values()].sort((a, b) => a.date.localeCompare(b.date));
}

function dedupePoints(rows: StatisticPoint[]) {
  const byDate = new Map<string, StatisticPoint>();
  for (const row of rows) byDate.set(row.date, row);
  return [...byDate.values()].sort((a, b) => a.date.localeCompare(b.date));
}

export function filterStatisticPoints(points: StatisticPoint[], range: StatisticRange, now = new Date()): StatisticPoint[] {
  return filterDatedPoints(points, range, now);
}

export function filterStatisticTextPoints(points: StatisticTextPoint[], range: StatisticRange, now = new Date()): StatisticTextPoint[] {
  return filterDatedPoints(points, range, now);
}

function filterDatedPoints<T extends { date: string }>(points: T[], range: StatisticRange, now: Date): T[] {
  if (range === "max" || !points.length) return points;
  const years = range === "1y" ? 1 : range === "3y" ? 3 : range === "5y" ? 5 : 10;
  const latest = new Date(`${points[points.length - 1].date.slice(0, 10)}T00:00:00Z`);
  const anchor = Number.isNaN(latest.getTime()) ? now : latest;
  const cutoff = new Date(Date.UTC(anchor.getUTCFullYear() - years, anchor.getUTCMonth(), anchor.getUTCDate()));
  return points.filter((point) => Date.parse(point.date) >= cutoff.getTime());
}

export function formatStatisticValue(value: number | string | undefined, unit: StatisticUnit, precise = false, currency = "USD"): string {
  if (value === undefined || value === "") return "—";
  if (typeof value === "string") {
    if (unit !== "date" || !validDate(value)) return value;
    return new Date(`${value.slice(0, 10)}T00:00:00Z`).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric", timeZone: "UTC" });
  }
  if (!Number.isFinite(value)) return "—";
  switch (unit) {
    case "billions": return formatBillions(value, precise, currency);
    case "currency": return formatCurrencyNumber(value, currency, precise);
    case "currency-per-share": return formatCurrencyNumber(value, currency, precise);
    case "percent": return `${(value * 100).toLocaleString("en-US", { minimumFractionDigits: precise ? 2 : 1, maximumFractionDigits: precise ? 4 : 2 })}%`;
    case "multiple": return `${value.toLocaleString("en-US", { minimumFractionDigits: precise ? 2 : 1, maximumFractionDigits: precise ? 4 : 2 })}×`;
    case "ratio": return value.toLocaleString("en-US", { minimumFractionDigits: precise ? 2 : 1, maximumFractionDigits: precise ? 4 : 2 });
    case "shares-billions": return `${value.toLocaleString("en-US", { minimumFractionDigits: precise ? 3 : 2, maximumFractionDigits: precise ? 6 : 3 })}B`;
    case "volume": return compactNumber(value, precise);
    default: return value.toLocaleString("en-US");
  }
}

function formatBillions(value: number, precise: boolean, currency: string): string {
  const absolute = Math.abs(value);
  const symbol = currencySymbol(currency);
  if (absolute >= 1000) return `${value < 0 ? "−" : ""}${symbol}${(absolute / 1000).toFixed(precise ? 4 : 2)}T`;
  const digits = precise ? 4 : absolute >= 100 ? 1 : 2;
  return `${value < 0 ? "−" : ""}${symbol}${absolute.toLocaleString("en-US", { minimumFractionDigits: digits, maximumFractionDigits: digits })}B`;
}

function formatCurrencyNumber(value: number, currency = "USD", precise = false) {
  try {
    return value.toLocaleString("en-US", { style: "currency", currency, minimumFractionDigits: 2, maximumFractionDigits: precise ? 4 : 2 });
  } catch {
    return `${value.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: precise ? 4 : 2 })} ${currency}`;
  }
}

function currencySymbol(currency: string) {
  try {
    return new Intl.NumberFormat("en-US", { style: "currency", currency, currencyDisplay: "narrowSymbol", maximumFractionDigits: 0 }).formatToParts(0).find((part) => part.type === "currency")?.value ?? `${currency} `;
  } catch {
    return `${currency} `;
  }
}

function exactShares(context: BuildContext) {
  return context.quote?.sharesOutstandingB ?? context.equity.current.sharesOutstandingB;
}

function exactMarketCap(context: BuildContext) {
  if (context.quote?.marketCapB !== undefined) return context.quote.marketCapB;
  const price = context.quote?.price ?? context.equity.current.price;
  return multiply(price, exactShares(context));
}

function exactEnterpriseValue(context: BuildContext) {
  if (context.quote?.enterpriseValueB !== undefined) return context.quote.enterpriseValueB;
  return add(exactMarketCap(context), context.current.netDebtB);
}

function compactNumber(value: number, precise: boolean): string {
  if (precise) return Math.round(value).toLocaleString("en-US");
  const absolute = Math.abs(value);
  if (absolute >= 1e9) return `${(value / 1e9).toFixed(2)}B`;
  if (absolute >= 1e6) return `${(value / 1e6).toFixed(2)}M`;
  if (absolute >= 1e3) return `${(value / 1e3).toFixed(1)}K`;
  return value.toLocaleString("en-US");
}

function monthlyBeta(stock?: PricePoint[], benchmark?: PricePoint[]): number | undefined {
  const stockMap = new Map(monthlyReturns(stock).map((row) => [row.date.slice(0, 7), row.value]));
  const benchmarkMap = new Map(monthlyReturns(benchmark).map((row) => [row.date.slice(0, 7), row.value]));
  const months = [...stockMap.keys()].filter((month) => benchmarkMap.has(month)).sort();
  if (months.length < 60) return undefined;
  const selected = months.slice(-60);
  if (selected.some((month, index) => index > 0 && monthOrdinal(month) - monthOrdinal(selected[index - 1]) !== 1)) return undefined;
  const pairs = selected.map((month) => [stockMap.get(month)!, benchmarkMap.get(month)!] as const);
  const benchmarkMean = pairs.reduce((sum, pair) => sum + pair[1], 0) / pairs.length;
  const stockMean = pairs.reduce((sum, pair) => sum + pair[0], 0) / pairs.length;
  const covariance = pairs.reduce((sum, pair) => sum + (pair[0] - stockMean) * (pair[1] - benchmarkMean), 0);
  const variance = pairs.reduce((sum, pair) => sum + (pair[1] - benchmarkMean) ** 2, 0);
  return variance > 0 ? covariance / variance : undefined;
}

function monthlyReturns(points?: PricePoint[]) {
  const rows = sortedPrices(points);
  return rows.slice(1).flatMap((row, index) => {
    const previousRow = rows[index];
    if (monthOrdinal(row.date) - monthOrdinal(previousRow.date) !== 1) return [];
    const previous = totalReturnValue(previousRow);
    const current = totalReturnValue(row);
    return previous > 0 && current > 0 ? [{ date: row.date, value: current / previous - 1 }] : [];
  });
}

function latestRolling(points: PricePoint[] | undefined, periods: number) {
  const rows = sortedPrices(points);
  return rows.length ? trailingReturn(rows, rows.length - 1, periods) : undefined;
}

function trailingReturn(rows: PricePoint[], index: number, periods: number) {
  const previous = rows[index - periods];
  if (!previous) return undefined;
  const start = previous.close;
  const end = rows[index].close;
  return start > 0 && end > 0 ? end / start - 1 : undefined;
}

function sortedPrices(points?: PricePoint[]) { return [...(points ?? [])].filter((point) => point.close > 0).sort((a, b) => a.date.localeCompare(b.date)); }
function totalReturnValue(point: PricePoint) { return point.totalReturnClose && point.totalReturnClose > 0 ? point.totalReturnClose : point.close; }
function priceOnOrBefore(points: PricePoint[], date: string) { return last(points.filter((point) => point.date <= date))?.close; }
function monthLabel(date: string) { return new Date(`${date.slice(0, 10)}T00:00:00Z`).toLocaleDateString("en-US", { month: "short", year: "numeric", timeZone: "UTC" }); }
function periodLabel(date: string, resolution: StatisticResolution) { const parsed = new Date(`${date.slice(0, 10)}T00:00:00Z`); return resolution === "year" ? String(parsed.getUTCFullYear()) : resolution === "quarter" ? `${parsed.getUTCFullYear()} Q${Math.floor(parsed.getUTCMonth() / 3) + 1}` : monthLabel(date); }
function periodBucket(date: string, resolution: StatisticResolution) { const parsed = new Date(`${date.slice(0, 10)}T00:00:00Z`); return resolution === "year" ? String(parsed.getUTCFullYear()) : resolution === "quarter" ? `${parsed.getUTCFullYear()}-Q${Math.floor(parsed.getUTCMonth() / 3) + 1}` : `${parsed.getUTCFullYear()}-${String(parsed.getUTCMonth() + 1).padStart(2, "0")}`; }
function monthOrdinal(date: string) { const [year, month] = date.slice(0, 7).split("-").map(Number); return year * 12 + month; }
function fiscalQuarterOrdinal(row: Pick<QuarterlyPoint, "fiscalYear" | "fiscalQuarter">) { const match = /^Q([1-4])$/.exec(row.fiscalQuarter); return match ? row.fiscalYear * 4 + Number(match[1]) - 1 : undefined; }
function consecutiveQuarters(rows: QuarterlyPoint[]) { return rows.length === 4 && rows.every((row, index) => index === 0 || fiscalQuarterOrdinal(row)! - fiscalQuarterOrdinal(rows[index - 1])! === 1); }
function validDate(value: unknown): value is string { return typeof value === "string" && /^\d{4}-\d{2}-\d{2}/.test(value) && !Number.isNaN(Date.parse(value)); }
function last<T>(rows: T[] | undefined): T | undefined { return rows?.[rows.length - 1]; }
function lastActual(rows: AnnualPoint[]) { return last(rows.filter((row) => !row.estimate)); }
function finite(value: unknown): value is number { return typeof value === "number" && Number.isFinite(value); }
function ratio(numerator?: number, denominator?: number) { return finite(numerator) && finite(denominator) && denominator !== 0 ? numerator / denominator : undefined; }
function positiveRatio(numerator?: number, denominator?: number) { return finite(denominator) && denominator > 0 ? ratio(numerator, denominator) : undefined; }
function growth(current?: number, previous?: number) { return finite(current) && finite(previous) && previous !== 0 ? (current - previous) / Math.abs(previous) : undefined; }
function multiply(left?: number, right?: number) { return finite(left) && finite(right) ? left * right : undefined; }
function add(left?: number, right?: number) { return finite(left) && finite(right) ? left + right : undefined; }
function average(left?: number, right?: number) { if (!finite(left)) return right; if (!finite(right)) return left; return (left + right) / 2; }
function sumAll<T>(rows: T[], property: keyof T) { const values = rows.map((row) => row[property]); return values.every(finite) ? (values as number[]).reduce((sum, value) => sum + value, 0) : undefined; }
function averageAll<T>(rows: T[], property: keyof T) { const values = rows.map((row) => row[property]); return values.every(finite) && values.length ? (values as number[]).reduce((sum, value) => sum + value, 0) / values.length : undefined; }
