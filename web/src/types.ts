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
  prices?: PricePoint[];
  current: CurrentMetrics;
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
