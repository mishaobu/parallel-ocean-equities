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

export interface MacroSeries {
  updatedAt?: string; sources?: string[]; warnings?: string[]; error?: string; basis?: string;
  points?: Array<Record<string, number | string>>;
  countries?: CountrySeries[]; assets?: AssetSeries[];
}

export interface StateResponse {
  state: { version: number; updatedAt: string; macro?: MacroSeries };
  runtime: { macroRefreshing?: boolean };
}
