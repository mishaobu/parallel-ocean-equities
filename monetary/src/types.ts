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

export interface StateResponse {
  state: { updatedAt: string; macro?: MacroSeries };
  runtime: { macroRefreshing?: boolean };
}
