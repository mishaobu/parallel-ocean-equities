export interface CountryPoint {
  date: string;
  policyRate?: number; policyRateDate?: string;
  inflation?: number; inflationDate?: string;
  coreInflation?: number; coreInflationDate?: string;
  industrialGrowth?: number; industrialDate?: string;
  unemployment?: number; unemploymentDate?: string;
  moneyGrowth?: number; moneyGrowthDate?: string;
  longRate?: number; longRateDate?: string;
  realRate?: number; yieldCurve?: number;
  fx?: number; fxDate?: string;
  leadingIndex?: number; leadingIndexDate?: string;
}

export interface CountrySeries {
  code: string; name: string; currency: string; region: string;
  policyLabel: string; fxLabel: string; equityTicker?: string;
  sources?: string[]; warnings?: string[]; points?: CountryPoint[];
}

export interface PricePoint { date: string; close: number }
export interface AssetSeries { symbol: string; label: string; group: string; region?: string; source?: string; points?: PricePoint[] }

export interface MacroPoint {
  date: string;
  inflation?: number;
  industrialGrowth?: number;
  realPolicyRate?: number;
  dollarIndex?: number;
  netLiquidityGrowth?: number;
}

export interface MacroSeries {
  updatedAt?: string; sources?: string[]; warnings?: string[]; error?: string; basis?: string;
  points?: MacroPoint[];
  countries?: CountrySeries[]; assets?: AssetSeries[];
  vintages?: VintageSeries;
  options?: OptionsSeries;
}

export interface VintagePoint {
  date: string; vintageDate: string;
  inflation?: number; inflationObservationDate?: string;
  industrialGrowth?: number; industrialObservationDate?: string;
}
export interface VintageSeries { updatedAt?: string; source?: string; warnings?: string[]; points?: VintagePoint[] }

export interface OptionTermPoint {
  expiration: string; daysToExpiration: number; spot?: number; atmIv?: number;
  putWingIv?: number; callWingIv?: number; skew?: number;
  expectedMove?: number; straddleMove?: number;
}
export interface OptionSnapshot {
  ticker: string; asOf?: string; spot?: number; realizedVolatility20D?: number; atmIv30D?: number;
  skew30D?: number; expectedMove30D?: number; impliedRealizedSpread?: number;
  terms?: OptionTermPoint[];
}
export interface OptionsSeries { updatedAt?: string; asOf?: string; source?: string; warnings?: string[]; snapshots?: OptionSnapshot[] }

export interface StateResponse {
  state: { version: number; updatedAt: string; macro?: MacroSeries };
  runtime: { macroRefreshing?: boolean };
}
