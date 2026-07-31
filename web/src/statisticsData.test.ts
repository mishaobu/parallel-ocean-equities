import { describe, expect, it } from "vitest";
import { buildStatisticsCatalog, equityCurrency, filterStatisticPoints, formatStatisticValue } from "./statisticsData";
import type { Equity, LiveQuote, QuarterlyPoint } from "./types";

const exactShareSource = "SEC CompanyFacts dei:EntityCommonStockSharesOutstanding instant fact";
const emptyHistoricalPriceBasis = {
  provider: "yahoo-finance",
  adjustment: "split-adjusted",
  source: "Yahoo Finance monthly close and adjusted close; Yahoo Finance chart split events",
  stockSplits: [],
  splitCoverageStart: "2020-01-01",
  splitCoverageEnd: "2026-03-02",
  splitCoverageComplete: true,
};

const quarterDates = [
  [2024, "Q1", "2024-03-31", "2024-05-01"],
  [2024, "Q2", "2024-06-30", "2024-08-01"],
  [2024, "Q3", "2024-09-30", "2024-11-01"],
  [2024, "Q4", "2024-12-31", "2025-02-01"],
  [2025, "Q1", "2025-03-31", "2025-05-01"],
  [2025, "Q2", "2025-06-30", "2025-08-01"],
  [2025, "Q3", "2025-09-30", "2025-11-01"],
  [2025, "Q4", "2025-12-31", "2026-02-01"],
] as const;

const quarterlies: QuarterlyPoint[] = quarterDates.map(([fiscalYear, fiscalQuarter, periodEnd, filedAt], index) => ({
  fiscalYear,
  fiscalQuarter,
  periodEnd,
  filedAt,
  revenueB: index < 4 ? 20 : 25,
  grossProfitB: index < 4 ? 10 : 13,
  ebitB: index < 4 ? 5 : 7,
  ebitdaB: index < 4 ? 6 : 8,
  netIncomeB: index < 4 ? 4 : 5,
  operatingCashB: index < 4 ? 5 : 6,
  capexB: 1,
  fcfB: index < 4 ? 4 : 5,
  dividendsB: 0.5,
  dilutedEps: index < 4 ? 0.4 : 0.5,
  dilutedSharesB: 10,
  sharesOutstandingB: 9.8,
  sharesOutstandingAsOf: filedAt,
  sharesOutstandingSource: exactShareSource,
  cashB: 15,
  investmentsB: 5,
  debtB: 50,
  netDebtB: 30,
  currentAssetsB: 40,
  currentLiabilitiesB: 20,
  assetsB: index < 4 ? 180 : 200,
  liabilitiesB: 120,
  equityB: index < 4 ? 60 : 80,
}));

const equity: Equity = {
  ticker: "TEST",
  company: "Test Company",
  status: "ready",
  sources: ["SEC CompanyFacts", "Yahoo Finance monthly close and adjusted close"],
  annuals: [
    { fiscalYear: 2024, periodEnd: "2024-12-31", filedAt: "2025-02-01", revenueB: 80, grossProfitB: 40, ebitB: 20, ebitdaB: 24, netIncomeB: 16, operatingCashB: 20, fcfB: 16, dividendsB: 2, dilutedEps: 1.6, dilutedSharesB: 10, sharesOutstandingB: 9.8, sharesOutstandingAsOf: "2025-01-20", sharesOutstandingSource: exactShareSource, cashB: 15, investmentsB: 5, debtB: 50, netDebtB: 30, currentAssetsB: 40, currentLiabilitiesB: 20, assetsB: 180, liabilitiesB: 120, equityB: 60 },
    { fiscalYear: 2025, periodEnd: "2025-12-31", filedAt: "2026-02-01", revenueB: 100, grossProfitB: 52, ebitB: 28, ebitdaB: 32, netIncomeB: 20, operatingCashB: 24, fcfB: 20, dividendsB: 2, dilutedEps: 2, dilutedSharesB: 10, sharesOutstandingB: 9.8, sharesOutstandingAsOf: "2026-01-20", sharesOutstandingSource: exactShareSource, cashB: 15, investmentsB: 5, debtB: 50, netDebtB: 30, currentAssetsB: 40, currentLiabilitiesB: 20, assetsB: 200, liabilitiesB: 120, equityB: 80 },
  ],
  quarterlies,
  prices: [
    { date: "2025-01-31", close: 70 },
    { date: "2025-02-28", close: 75 },
    { date: "2025-05-31", close: 80 },
    { date: "2025-08-31", close: 85 },
    { date: "2025-11-30", close: 90 },
    { date: "2026-02-28", close: 95 },
  ],
  historicalPriceBasis: emptyHistoricalPriceBasis,
  current: { price: 95, priceAsOf: "2026-02-28", sharesOutstandingB: 9.8, sharesOutstandingAsOf: "2026-01-20" },
  valuation: { forwardPe: 22 },
};

const quote: LiveQuote = {
  ticker: "TEST",
  price: 100,
	currency: "USD",
  asOf: "2026-03-02T15:30:00Z",
  marketState: "REGULAR",
  stockSplitCoverageStart: "2020-01-01",
  stockSplitCoverageComplete: true,
  stockSplits: [],
  sharesOutstandingB: 9.8,
  shareBasisAsOf: "2026-01-20",
  movingAverage50Day: 92,
  fieldSources: { marketCapB: "test: live price × issuer-disclosed shares" },
};

describe("statistics catalog", () => {
  it("maps every Yahoo row and keeps vendor gaps explicit", () => {
    const catalog = buildStatisticsCatalog(equity, quote);
    const yahoo = catalog.metrics.filter((metric) => metric.yahoo);
    expect(catalog.yahooMetricCount).toBe(60);
    expect(new Set(yahoo.map((metric) => metric.key)).size).toBe(60);
    expect(new Set(yahoo.map((metric) => metric.label)).size).toBe(60);
    expect(catalog.metrics.find((metric) => metric.key === "levered-free-cash-flow")?.current).toBeUndefined();
    expect(catalog.metrics.find((metric) => metric.key === "shares-short")?.unavailableReason).toContain("short-interest feed");
  });

  it("uses the live quote and disclosed basic shares for current market value", () => {
    const marketCap = buildStatisticsCatalog(equity, quote).metrics.find((metric) => metric.key === "market-cap")!;
    expect(marketCap.current).toBeCloseTo(980);
    expect(marketCap.currentBasis).toContain("$100.00 × 9.800B disclosed shares (2026-01-20)");
    expect(marketCap.currentAsOf).toBe("2026-03-02T15:30:00Z");
    expect(marketCap.currentSource).toContain("issuer-disclosed shares");
    expect(marketCap.marketSensitive).toBe(true);
  });

  it("does not substitute weighted diluted shares for exact current market value", () => {
    const withoutExactShares: Equity = { ...equity, current: { price: 95, priceAsOf: "2026-02-28" } };
    const withoutQuoteShares: LiveQuote = { ticker: "TEST", price: 100, asOf: "2026-03-02T15:30:00Z" };
    const marketCap = buildStatisticsCatalog(withoutExactShares, withoutQuoteShares).metrics.find((metric) => metric.key === "market-cap")!;
    expect(marketCap.current).toBeUndefined();
    expect(marketCap.unavailableReason).toContain("No defensible current observation");
  });

  it("uses only accession-matched actual shares for historical market value", () => {
    const catalog = buildStatisticsCatalog(equity, quote);
    const marketCap = catalog.metrics.find((metric) => metric.key === "market-cap")!;
    expect(marketCap.points.quarter.at(-1)?.value).toBeCloseTo(882);
    const withoutHistoricalShares = { ...equity, annuals: equity.annuals.map((row) => ({ ...row, sharesOutstandingB: undefined, sharesOutstandingAsOf: undefined })), quarterlies: equity.quarterlies?.map((row) => ({ ...row, sharesOutstandingB: undefined, sharesOutstandingAsOf: undefined })) };
    expect(buildStatisticsCatalog(withoutHistoricalShares, { ...quote, history: [] }).metrics.find((metric) => metric.key === "market-cap")?.points.quarter).toEqual([]);
  });

  it("aligns raw SEC shares to Yahoo's exact split-adjusted price basis", () => {
    const rows: QuarterlyPoint[] = [
      [2023, "Q2", "2023-06-30", "2023-08-15", 2.5, "2023-08-10"],
      [2023, "Q3", "2023-09-30", "2023-11-15", 2.5, "2023-11-10"],
      [2023, "Q4", "2023-12-31", "2024-02-15", 2.5, "2024-02-10"],
      [2024, "Q1", "2024-03-31", "2024-05-20", 2.46, "2024-05-15"],
      [2024, "Q2", "2024-06-30", "2024-08-20", 24.53, "2024-08-15"],
    ].map(([fiscalYear, fiscalQuarter, periodEnd, filedAt, sharesOutstandingB, sharesOutstandingAsOf]) => ({
      fiscalYear: fiscalYear as number,
      fiscalQuarter: fiscalQuarter as string,
      periodEnd: periodEnd as string,
      filedAt: filedAt as string,
      revenueB: 10,
      netIncomeB: 2,
      dilutedSharesB: 24.89,
      sharesOutstandingB: sharesOutstandingB as number,
      sharesOutstandingAsOf: sharesOutstandingAsOf as string,
      sharesOutstandingSource: exactShareSource,
    }));
    const nvda: Equity = {
      ticker: "NVDA",
      status: "ready",
      sources: ["SEC CompanyFacts", "Yahoo Finance monthly close and adjusted close"],
      annuals: [],
      quarterlies: rows,
      prices: [
        { date: "2024-05-20", close: 86.402 },
        { date: "2024-08-20", close: 117.02 },
      ],
      historicalPriceBasis: {
        provider: "yahoo-finance",
        adjustment: "split-adjusted",
        source: "Yahoo Finance monthly close and adjusted close; Yahoo Finance chart split events",
        stockSplits: [{ date: "2024-06-10", numerator: 10, denominator: 1, ratio: 10 }],
        splitCoverageStart: "2020-01-01",
        splitCoverageEnd: "2026-07-31",
        splitCoverageComplete: true,
      },
      current: {},
    };
    const nvdaQuote: LiveQuote = {
      ticker: "NVDA",
      asOf: "2026-07-31T17:00:00Z",
      stockSplitCoverageStart: "2020-01-01",
      stockSplitCoverageComplete: true,
      stockSplits: [{ date: "2024-06-10", numerator: 10, denominator: 1, ratio: 10 }],
    };
    const marketCap = buildStatisticsCatalog(nvda, nvdaQuote).metrics.find((metric) => metric.key === "market-cap")!;
    expect(marketCap.points.quarter.at(-2)?.value).toBeCloseTo(2125.4892);
    expect(marketCap.points.quarter.at(-1)?.value).toBeCloseTo(2870.5006);
    expect(marketCap.points.quarter.at(-2)?.source).toContain("shares aligned x10; applied 2024-06-10 10:1");
  });

  it("binds named split cases to the persisted price response and fails closed on an implausible aggregate", () => {
    const cases = [
      { ticker: "NVDA", raw: 0.62, diluted: 24.8, shareDate: "2021-02-19", events: [{ date: "2021-07-20", numerator: 4, denominator: 1, ratio: 4 }, { date: "2024-06-10", numerator: 10, denominator: 1, ratio: 10 }], expected: 2480 },
      { ticker: "AMZN", raw: 0.50884441, diluted: 10.1768882, shareDate: "2022-01-26", events: [{ date: "2022-06-06", numerator: 20, denominator: 1, ratio: 20 }], expected: 1017.68882 },
      { ticker: "SMCI", raw: 0.052905947, diluted: 0.52905947, shareDate: "2023-07-31", events: [{ date: "2024-10-01", numerator: 10, denominator: 1, ratio: 10 }], expected: 52.905947 },
      { ticker: "GOOGL", raw: 0.243872592, diluted: 13.55311943, shareDate: "2010-01-29", events: [{ date: "2014-04-03", numerator: 1998, denominator: 1000, ratio: 1.998 }, { date: "2022-07-18", numerator: 20, denominator: 1, ratio: 20 }], expected: undefined },
    ];
    for (const scenario of cases) {
      const rows: QuarterlyPoint[] = quarterDates.slice(0, 4).map(([fiscalYear, fiscalQuarter, periodEnd], index) => ({
        fiscalYear,
        fiscalQuarter,
        periodEnd,
        filedAt: `2025-0${index + 1}-20`,
        netIncomeB: 1,
        dilutedSharesB: scenario.diluted,
        sharesOutstandingB: scenario.raw,
        sharesOutstandingAsOf: scenario.shareDate,
        sharesOutstandingSource: exactShareSource,
      }));
      const candidate: Equity = {
        ticker: scenario.ticker,
        status: "ready",
        sources: ["SEC CompanyFacts", "Yahoo Finance monthly close and adjusted close"],
        annuals: [],
        quarterlies: rows,
        prices: [{ date: "2025-04-20", close: 100 }],
        historicalPriceBasis: {
          provider: "yahoo-finance",
          adjustment: "split-adjusted",
          source: "Yahoo Finance monthly close and adjusted close; Yahoo Finance chart split events",
          stockSplits: scenario.events,
          splitCoverageStart: "2004-08-01",
          splitCoverageEnd: "2026-07-31",
          splitCoverageComplete: true,
        },
        current: {},
      };
      const liveQuoteWithUnboundFutureSplit: LiveQuote = {
        ticker: scenario.ticker,
        asOf: "2027-01-01T00:00:00Z",
        stockSplitCoverageStart: "2020-01-01",
        stockSplitCoverageComplete: true,
        stockSplits: [{ date: "2026-12-01", numerator: 100, denominator: 1, ratio: 100 }],
      };
      const point = buildStatisticsCatalog(candidate, liveQuoteWithUnboundFutureSplit).metrics.find((metric) => metric.key === "market-cap")?.points.quarter.at(-1);
      if (scenario.expected === undefined) expect(point, scenario.ticker).toBeUndefined();
      else expect(point?.value, scenario.ticker).toBeCloseTo(scenario.expected);
    }
  });

  it("fails closed when split coverage, event math, share provenance, or the effective-date basis is ambiguous", () => {
    const variants: Equity[] = [
      { ...equity, historicalPriceBasis: { ...emptyHistoricalPriceBasis, splitCoverageComplete: false } },
      { ...equity, historicalPriceBasis: { ...emptyHistoricalPriceBasis, provider: "unbound-provider" } },
      { ...equity, historicalPriceBasis: { ...emptyHistoricalPriceBasis, stockSplits: [{ date: "2025-06-01", numerator: 2, denominator: 1, ratio: 3 }] } },
      { ...equity, quarterlies: quarterlies.map((row) => ({ ...row, sharesOutstandingSource: "unverified aggregate" })) },
      { ...equity, historicalPriceBasis: { ...emptyHistoricalPriceBasis, stockSplits: [{ date: "2026-02-01", numerator: 2, denominator: 1, ratio: 2 }] } },
    ];
    for (const candidate of variants) {
      expect(buildStatisticsCatalog(candidate, quote).metrics.find((metric) => metric.key === "market-cap")?.points.quarter).toEqual([]);
    }
  });

  it("does not expose monthly fundamentals before the filing was available", () => {
    const revenue = buildStatisticsCatalog(equity, quote).metrics.find((metric) => metric.key === "revenue")!;
    expect(revenue.points.month.map((point) => point.date)).not.toContain("2025-01-31");
    expect(revenue.points.month[0]).toMatchObject({ date: "2025-02-28", value: 80, basisDate: "2025-02-01" });
    expect(revenue.points.quarter.at(-1)?.value).toBe(100);
  });

  it("resamples recorded quote statistics and event changes by selected period", () => {
    const withHistory: LiveQuote = { ...quote, history: [
      { asOf: "2026-01-02T21:00:00Z", numeric: { "moving-average-50d": 90 }, text: { "ex-dividend-date": "2025-12-12" }, sources: { "moving-average-50d": "daily closes" } },
      { asOf: "2026-01-30T21:00:00Z", numeric: { "moving-average-50d": 92 }, text: { "ex-dividend-date": "2025-12-12" }, sources: { "moving-average-50d": "daily closes" } },
      { asOf: "2026-04-30T20:00:00Z", numeric: { "moving-average-50d": 101 }, text: { "ex-dividend-date": "2026-03-13" }, sources: { "moving-average-50d": "daily closes" } },
    ] };
    const catalog = buildStatisticsCatalog(equity, withHistory);
    const movingAverage = catalog.metrics.find((metric) => metric.key === "moving-average-50d")!;
    expect(movingAverage.points.month.map((point) => point.value)).toEqual([92, 101]);
    expect(movingAverage.points.quarter.map((point) => point.value)).toEqual([92, 101]);
    expect(movingAverage.points.year.map((point) => point.value)).toEqual([101]);
    const exDividend = catalog.metrics.find((metric) => metric.key === "ex-dividend-date")!;
    expect(exDividend.textPoints.month.map((point) => point.value)).toEqual(["2025-12-12", "2026-03-13"]);
  });

  it("derives filing metrics and preserves exact formatting data", () => {
    const catalog = buildStatisticsCatalog(equity, quote);
    expect(catalog.metrics.find((metric) => metric.key === "current-ratio")?.current).toBe(2);
    expect(catalog.metrics.find((metric) => metric.key === "quarterly-revenue-growth")?.current).toBe(0.25);
		expect(formatStatisticValue(980, "billions", true, "USD")).toBe("$980.0000B");
		expect(formatStatisticValue(3375.4302, "billions", true, "USD")).toBe("$3.3754T");
    expect(formatStatisticValue("2026-01-20", "date")).toBe("Jan 20, 2026");
		expect(formatStatisticValue(1000, "currency", false, "KRW")).toContain("₩");
	});

	it("uses persisted quote-history currency and never invents USD", () => {
		const korean: Equity = { ...equity, ticker: "005930.KS", current: { price: 207_000 }, quoteHistory: [{ asOf: "2026-07-30T06:30:00Z", text: { currency: "KRW" } }] };
		expect(equityCurrency(korean)).toBe("KRW");
		expect(formatStatisticValue(207_000, "currency", false, equityCurrency(korean))).toContain("₩");
		expect(formatStatisticValue(207_000, "currency")).toBe("207,000.00 · currency n/a");
		expect(equityCurrency(korean, { ticker: "AAPL", currency: "USD" })).toBe("KRW");
	});

	it("ignores a live quote that belongs to the previously selected ticker", () => {
		const staleQuote: LiveQuote = { ticker: "AAPL", price: 999, currency: "USD", sharesOutstandingB: 20 };
		const catalog = buildStatisticsCatalog({ ...equity, quoteHistory: [{ asOf: "2026-02-28T21:00:00Z", text: { currency: "EUR" } }] }, staleQuote);
		expect(catalog.metrics.find((metric) => metric.key === "price")?.current).toBe(95);
		expect(catalog.metrics.find((metric) => metric.key === "price")?.currency).toBe("EUR");
		expect(catalog.metrics.find((metric) => metric.key === "market-cap")?.current).toBeUndefined();
	});

  it("uses absolute prior earnings when growth crosses zero", () => {
    const changed = quarterlies.map((row, index) => ({ ...row, netIncomeB: index === 3 ? -2 : index === 7 ? 2 : row.netIncomeB }));
    const catalog = buildStatisticsCatalog({ ...equity, quarterlies: changed }, quote);
    expect(catalog.metrics.find((metric) => metric.key === "quarterly-earnings-growth")?.current).toBe(2);
  });

  it("does not fabricate TTM values across a missing quarter", () => {
    const withGap = { ...equity, quarterlies: quarterlies.filter((row) => row.periodEnd !== "2025-09-30") };
    const catalog = buildStatisticsCatalog(withGap, quote);
    expect(catalog.metrics.find((metric) => metric.key === "revenue")?.current).toBeUndefined();
    expect(catalog.metrics.find((metric) => metric.key === "current-ratio")?.current).toBe(2);
    expect(catalog.metrics.find((metric) => metric.key === "revenue")?.points.quarter.at(-1)?.date).toBe("2025-06-30");
  });

  it("filters ranges from the latest observation rather than wall-clock time", () => {
    const points = [
      { date: "2019-12-31", label: "2019", value: 1, source: "test" },
      { date: "2024-12-31", label: "2024", value: 2, source: "test" },
      { date: "2025-12-31", label: "2025", value: 3, source: "test" },
    ];
    expect(filterStatisticPoints(points, "1y").map((point) => point.value)).toEqual([2, 3]);
  });

	it("never labels a 12-row quarterly benchmark return as a 52-week change", () => {
		const benchmark = (dates: string[]): Equity => ({
			ticker: "SPY",
			status: "ready",
			annuals: [],
			prices: dates.map((date, index) => ({ date, close: 100 + index * 10 })),
			current: {},
		});
		const quarterly = benchmark(Array.from({ length: 13 }, (_, index) => {
			const date = new Date(Date.UTC(2023 + Math.floor(index * 3 / 12), (index * 3) % 12 + 1, 0));
			return date.toISOString().slice(0, 10);
		}));
		const monthly = benchmark(Array.from({ length: 13 }, (_, index) => new Date(Date.UTC(2025, index + 1, 0)).toISOString().slice(0, 10)));
		expect(buildStatisticsCatalog(equity, quote, quarterly).metrics.find((metric) => metric.key === "sp500-change-52-week")?.current).toBeUndefined();
		expect(buildStatisticsCatalog(equity, quote, monthly).metrics.find((metric) => metric.key === "sp500-change-52-week")?.current).toBeCloseTo(1.2);
	});
});
