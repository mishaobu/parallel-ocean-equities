export interface MacroPoint {
  date: string;
  inflation?: number;
  coreInflation?: number;
  corePceInflation?: number;
  shelterInflation?: number;
  wageGrowth?: number;
  fedFunds?: number;
  treasury3M?: number;
  treasury2Y?: number;
  treasury5Y?: number;
  treasury10Y?: number;
  treasury30Y?: number;
  realPolicyRate?: number;
  real5Y?: number;
  real10Y?: number;
  yieldCurve?: number;
  yieldCurve3M?: number;
  breakeven5Y?: number;
  breakeven10Y?: number;
  forwardInflation5Y?: number;
  termPremium10Y?: number;
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
  tgaB?: number;
  reverseRepoB?: number;
  netLiquidityB?: number;
  netLiquidityGrowth?: number;
  bankCreditGrowth?: number;
  businessLoanGrowth?: number;
  realGdpGrowth?: number;
  industrialGrowth?: number;
  payrollGrowth?: number;
  initialClaimsK?: number;
  unemployment?: number;
  sahmRule?: number;
  financialConditions?: number;
  lendingStandards?: number;
  dollarIndex?: number;
  vix?: number;
  corporateSpread?: number;
  highYieldSpread?: number;
  oilPrice?: number;
  copperPrice?: number;
  federalDebtToGdp?: number;
  recession?: number;
}

export interface MacroSeries {
  updatedAt?: string;
  sources?: string[];
  warnings?: string[];
  error?: string;
  basis?: string;
  points?: MacroPoint[];
}

export interface PricePoint { date: string; close: number }
export interface ValuationPoint {
  date: string;
  pe?: number;
  evToEbitda?: number;
  fcfToMarketCap?: number;
}
export interface EquitySummary {
  ticker: string;
  company?: string;
  prices?: PricePoint[];
  valuations?: ValuationPoint[];
}

export interface StateResponse {
  state: { updatedAt: string; macro?: MacroSeries; tickers: Record<string, EquitySummary> };
  runtime: { macroRefreshing?: boolean };
}
