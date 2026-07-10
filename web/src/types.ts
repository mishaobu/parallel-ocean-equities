export type MetricKey = "revenueB" | "capexB" | "netIncomeB" | "dilutedEps" | "peRatio";

export interface AnnualPoint {
  fiscalYear: number;
  periodEnd?: string;
  revenueB?: number;
  capexB?: number;
  netIncomeB?: number;
  dilutedEps?: number;
  peRatio?: number;
  estimate?: boolean;
  confidence?: string;
}

export interface PricePoint {
  date: string;
  close: number;
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
  ebitB?: number;
  daB?: number;
  ebitdaB?: number;
  netIncomeB?: number;
  operatingCashB?: number;
  capexB?: number;
  fcfB?: number;
  dividendsB?: number;
  dilutedEps?: number;
  dilutedSharesB?: number;
  cashB?: number;
  investmentsB?: number;
  debtB?: number;
  netDebtB?: number;
  assetsB?: number;
  liabilitiesB?: number;
  equityB?: number;
}

export interface ValuationMetrics {
  asOf?: string;
  marketCapB?: number;
  enterpriseValueB?: number;
  ttmRevenueB?: number;
  ttmEbitdaB?: number;
  ttmEbitB?: number;
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
  fcfToMarketCap?: number;
  forwardFcfToMarketCap?: number;
  fcfToEv?: number;
  forwardFcfToEv?: number;
  netDebtToEbitda?: number;
  forwardNetDebtToEbitda?: number;
  dividendToFcf?: number;
  forwardDividendToFcf?: number;
}

export interface ForecastModel {
  horizon?: string;
  method?: string;
  revenueGrowth?: number;
  ebitMargin?: number;
  ebitdaMargin?: number;
  fcfMargin?: number;
  dividendGrowth?: number;
  forwardRevenueB?: number;
  forwardEbitB?: number;
  forwardEbitdaB?: number;
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
}

export interface Equity {
  ticker: string;
  company?: string;
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
  valuation?: ValuationMetrics;
  forecast?: ForecastModel;
  models?: ValuationModels;
}

export interface RuntimeStats {
  refreshTotal: number;
  refreshFailures: number;
  queueDepth: number;
  inFlight: number;
  lastRefresh?: string;
}

export interface StateResponse {
  state: {
    version: number;
    updatedAt: string;
    tickers: Record<string, Equity>;
  };
  runtime: RuntimeStats;
}
