export type MetricKey = "revenueB" | "capexB" | "netIncomeB" | "dilutedEps" | "peRatio";

export interface AnnualPoint {
  fiscalYear: number;
  periodEnd?: string;
  filedAt?: string;
  revenueB?: number;
  grossProfitB?: number;
  ebitB?: number;
  daB?: number;
  ebitdaB?: number;
  operatingCashB?: number;
  capexB?: number;
  fcfB?: number;
  dividendsB?: number;
  netIncomeB?: number;
  pretaxIncomeB?: number;
  incomeTaxB?: number;
  stockCompB?: number;
  dilutedEps?: number;
  dilutedSharesB?: number;
  sharesOutstandingB?: number;
  sharesOutstandingAsOf?: string;
  cashB?: number;
  investmentsB?: number;
  debtB?: number;
  netDebtB?: number;
  inventoryB?: number;
  receivablesB?: number;
  payablesB?: number;
  assetsB?: number;
  currentAssetsB?: number;
  liabilitiesB?: number;
  currentLiabilitiesB?: number;
  equityB?: number;
  peRatio?: number;
  estimate?: boolean;
  confidence?: string;
}

export interface PricePoint {
  date: string;
  close: number;
  totalReturnClose?: number;
}

export interface QuarterlyPoint {
  fiscalYear: number;
  fiscalQuarter: string;
  periodEnd: string;
  filedAt?: string;
  accession?: string;
  form?: string;
  filingUrl?: string;
  derived?: boolean;
  revenueB?: number;
  grossProfitB?: number;
  ebitB?: number;
  daB?: number;
  ebitdaB?: number;
  netIncomeB?: number;
  pretaxIncomeB?: number;
  incomeTaxB?: number;
  stockCompB?: number;
  operatingCashB?: number;
  capexB?: number;
  fcfB?: number;
  dividendsB?: number;
  dilutedEps?: number;
  dilutedSharesB?: number;
  sharesOutstandingB?: number;
  sharesOutstandingAsOf?: string;
  cashB?: number;
  investmentsB?: number;
  debtB?: number;
  netDebtB?: number;
  inventoryB?: number;
  receivablesB?: number;
  payablesB?: number;
  assetsB?: number;
  currentAssetsB?: number;
  liabilitiesB?: number;
  currentLiabilitiesB?: number;
  equityB?: number;
}

export interface ValuationMetrics {
  asOf?: string;
  marketCapB?: number;
  enterpriseValueB?: number;
  ttmRevenueB?: number;
  ttmEbitdaB?: number;
  ttmEbitB?: number;
  ttmOperatingCashB?: number;
  ttmFcfB?: number;
  ttmNetIncomeB?: number;
  ttmDividendsB?: number;
  netDebtB?: number;
  dilutedSharesB?: number;
  pe?: number;
  forwardPe?: number;
  evToEbitda?: number;
  forwardEvToEbitda?: number;
  evToEbit?: number;
  forwardEvToEbit?: number;
  operatingCashToMarketCap?: number;
  forwardOperatingCashToMarketCap?: number;
  fcfToMarketCap?: number;
  forwardFcfToMarketCap?: number;
  fcfToEv?: number;
  forwardFcfToEv?: number;
  netDebtToEbitda?: number;
  forwardNetDebtToEbitda?: number;
  dividendToFcf?: number;
  forwardDividendToFcf?: number;
}

export interface ValuationPoint {
  date: string;
  pe?: number;
  forwardPe?: number;
  evToEbitda?: number;
  forwardEvToEbitda?: number;
  evToEbit?: number;
  forwardEvToEbit?: number;
  operatingCashToMarketCap?: number;
  forwardOperatingCashToMarketCap?: number;
  fcfToMarketCap?: number;
  forwardFcfToMarketCap?: number;
  fcfToEv?: number;
  forwardFcfToEv?: number;
  netDebtToEbitda?: number;
  forwardNetDebtToEbitda?: number;
  dividendToFcf?: number;
  forwardDividendToFcf?: number;
}

export interface QualityMetrics {
  asOf?: string;
  cashConversion?: number;
  grossMargin?: number;
  operatingMargin?: number;
  operatingCashMargin?: number;
  fcfMargin?: number;
  inventoryDays?: number;
  receivableDays?: number;
  payableDays?: number;
  cashConversionCycleDays?: number;
  roic?: number;
  incrementalRoic?: number;
  stockCompToRevenue?: number;
  dilutedShareGrowth?: number;
}

export interface QualityPoint extends Omit<QualityMetrics, "asOf"> {
  date: string;
}

export interface MacroPoint {
  date: string;
  inflation?: number;
  fedFunds?: number;
  treasury2Y?: number;
  treasury10Y?: number;
  realPolicyRate?: number;
  real10Y?: number;
  yieldCurve?: number;
  breakeven10Y?: number;
  mortgage30Y?: number;
  logM1?: number;
  logM2?: number;
  logFedAssets?: number;
  logMonetaryBase?: number;
  logBankReserves?: number;
  m1Growth?: number;
  m2Growth?: number;
  fedAssetsGrowth?: number;
  monetaryBaseGrowth?: number;
  reverseRepoB?: number;
  realGdpGrowth?: number;
  industrialGrowth?: number;
  unemployment?: number;
  financialConditions?: number;
  dollarIndex?: number;
  vix?: number;
  corporateSpread?: number;
  highYieldSpread?: number;
  recession?: number;
}

export interface MacroSeries {
  updatedAt?: string;
  sources?: string[];
  warnings?: string[];
  error?: string;
  points?: MacroPoint[];
}

export interface ForecastModel {
  horizon?: string;
  method?: string;
  revenueGrowth?: number;
  ebitMargin?: number;
  ebitdaMargin?: number;
  operatingCashMargin?: number;
  fcfMargin?: number;
  dividendGrowth?: number;
  forwardRevenueB?: number;
  forwardEbitB?: number;
  forwardEbitdaB?: number;
  forwardOperatingCashB?: number;
  forwardFcfB?: number;
  forwardNetIncomeB?: number;
  forwardDividendsB?: number;
  forwardEps?: number;
}

export interface ValuationModels {
  projectionYears?: number;
  fcfGrowth?: number;
  wacc?: number;
  terminalGrowth?: number;
  dcfValuePerShare?: number;
  targetEvToEbitda?: number;
  multipleValuePerShare?: number;
  targetPe?: number;
  earningsValuePerShare?: number;
}

export interface CurrentMetrics {
  price?: number;
  ttmEps?: number;
  forwardEps?: number;
  trailingPE?: number;
  forwardPE?: number;
  return1Y?: number;
  low52Week?: number;
  high52Week?: number;
  priceAsOf?: string;
  sharesOutstandingB?: number;
  sharesOutstandingAsOf?: string;
}

export interface LiveQuote {
  ticker: string;
  price?: number;
  previousClose?: number;
  change?: number;
  changePercent?: number;
  asOf?: string;
  marketState?: string;
  currency?: string;
  exchange?: string;
  source?: string;
  fieldSources?: Record<string, string>;
  change52Week?: number;
  high52Week?: number;
  low52Week?: number;
  movingAverage50Day?: number;
  movingAverage200Day?: number;
  averageVolume3Month?: number;
  averageVolume10Day?: number;
  trailingAnnualDividendRate?: number;
  trailingAnnualDividendYield?: number;
  forwardAnnualDividendRate?: number;
  forwardAnnualDividendYield?: number;
  averageDividendYield5Year?: number;
  beta5YMonthly?: number;
  betaBenchmark?: string;
  exDividendDate?: string;
  lastDividendDate?: string;
  lastSplitFactor?: string;
  lastSplitDate?: string;
  stockSplits?: StockSplitEvent[];
  stockSplitCoverageStart?: string;
  stockSplitCoverageComplete?: boolean;
  sharesOutstandingB?: number;
  shareBasisAsOf?: string;
  marketCapB?: number;
  enterpriseValueB?: number;
  history?: StatisticSnapshot[];
}

export interface StockSplitEvent {
  date: string;
  numerator: number;
  denominator: number;
  ratio: number;
}

export interface StatisticSnapshot {
  asOf: string;
  source?: string;
  asOfSource?: string;
  numeric?: Record<string, number>;
  text?: Record<string, string>;
  sources?: Record<string, string>;
}

export interface Equity {
  ticker: string;
  company?: string;
  instrumentType?: string;
  cik?: string;
  status: "queued" | "refreshing" | "ready" | "error";
  error?: string;
  warnings?: string[];
  updatedAt?: string;
  sources?: string[];
  annuals: AnnualPoint[];
  quarterlies?: QuarterlyPoint[];
  prices?: PricePoint[];
  current: CurrentMetrics;
  quoteHistory?: StatisticSnapshot[];
  valuation?: ValuationMetrics;
  forecast?: ForecastModel;
  models?: ValuationModels;
  valuations?: ValuationPoint[];
  quality?: QualityMetrics;
  qualities?: QualityPoint[];
}

export interface RuntimeStats {
  refreshTotal: number;
  refreshFailures: number;
  queueDepth: number;
  inFlight: number;
  lastRefresh?: string;
  macroRefreshing?: boolean;
  macroLastRefresh?: string;
  macroFailures?: number;
}

export interface StateResponse {
  state: {
    version: number;
    updatedAt: string;
    tickers: Record<string, Equity>;
    macro?: MacroSeries;
  };
  runtime: RuntimeStats;
}
