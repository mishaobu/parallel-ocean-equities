import type { AssetSeries, CountryPoint, CountrySeries } from "./types";

export type Range = "max" | "20y" | "10y" | "5y" | "3y" | "1y";
export type CountryMetric = "inflation" | "policyRate" | "realRate" | "industrialGrowth" | "moneyGrowth" | "longRate" | "yieldCurve" | "fx" | "unemployment";
export type MatrixSort = "name" | "regime" | "asOf" | CountryMetric;
export interface Reading { value: number; date: string; ageMonths: number }
export interface Snapshot { country: CountrySeries; regime: string; asOf: string; values: Partial<Record<CountryMetric, Reading>> }

const dateFields: Partial<Record<CountryMetric, keyof CountryPoint>> = {
  inflation: "inflationDate", policyRate: "policyRateDate", industrialGrowth: "industrialDate", moneyGrowth: "moneyGrowthDate",
  longRate: "longRateDate", fx: "fxDate", unemployment: "unemploymentDate",
};

export function rangeDomain(assets: AssetSeries[], countries: CountrySeries[], range: Range): [number, number] {
  const dates = [...assets.flatMap((asset) => asset.points?.map((point) => Date.parse(point.date)) ?? []), ...countries.flatMap((country) => country.points?.map((point) => Date.parse(point.date)) ?? [])].filter(Number.isFinite);
  const end = Math.max(...dates, Date.now());
  if (range === "max") return [Math.min(...dates, end), end];
  const start = new Date(end);
  start.setUTCFullYear(start.getUTCFullYear() - Number.parseInt(range, 10));
  return [start.getTime(), end];
}

export function snapshots(countries: CountrySeries[]): Snapshot[] {
  const latest = countries.flatMap((country) => country.points ?? []).reduce((max, point) => point.date > max ? point.date : max, "");
  return countries.map((country) => {
    const values: Snapshot["values"] = {};
    for (const metric of ["inflation", "policyRate", "realRate", "industrialGrowth", "moneyGrowth", "longRate", "yieldCurve", "fx", "unemployment"] as CountryMetric[]) {
      const reading = latestReading(country.points ?? [], metric, latest);
      if (reading) values[metric] = reading;
    }
    const asOf = Object.values(values).reduce((max, reading) => reading && reading.date > max ? reading.date : max, "");
    return { country, values, asOf, regime: regime(values.inflation, values.industrialGrowth) };
  });
}

export function latestReading(points: CountryPoint[], metric: CountryMetric, end?: string): Reading | undefined {
  const latest = end || points.reduce((max, point) => point.date > max ? point.date : max, "");
  for (let index = points.length - 1; index >= 0; index -= 1) {
    const value = points[index][metric];
    if (typeof value !== "number" || !Number.isFinite(value)) continue;
    const dateField = dateFields[metric];
    const explicit = dateField ? points[index][dateField] : undefined;
    const date = typeof explicit === "string" && explicit ? explicit : points[index].date;
    return { value, date, ageMonths: monthDistance(date, latest) };
  }
  return undefined;
}

export function sortSnapshots(rows: Snapshot[], key: MatrixSort, direction: "asc" | "desc") {
  const sign = direction === "asc" ? 1 : -1;
  return [...rows].sort((left, right) => {
    if (key === "name") return sign * left.country.name.localeCompare(right.country.name);
    if (key === "regime") return sign * left.regime.localeCompare(right.regime);
    if (key === "asOf") return sign * left.asOf.localeCompare(right.asOf);
    const a = left.values[key]?.value;
    const b = right.values[key]?.value;
    if (a === undefined) return 1;
    if (b === undefined) return -1;
    return sign * (a - b);
  });
}

export function indexedAssetRows(assets: AssetSeries[], symbols: string[], domain: [number, number]) {
  const selected = assets.filter((asset) => symbols.includes(asset.symbol));
  const baselines = new Map<string, number>();
  const rows = new Map<string, Record<string, string | number>>();
  selected.forEach((asset) => {
    const points = (asset.points ?? []).filter((point) => { const date = Date.parse(point.date); return date >= domain[0] && date <= domain[1]; });
    const baseline = points.find((point) => point.close > 0)?.close;
    if (!baseline) return;
    baselines.set(asset.symbol, baseline);
    points.forEach((point) => {
      const row = rows.get(point.date) ?? { date: point.date, timestamp: Date.parse(point.date) };
      row[asset.symbol] = point.close / baseline * 100;
      rows.set(point.date, row);
    });
  });
  return [...rows.values()].sort((left, right) => Number(left.timestamp) - Number(right.timestamp));
}

export interface AssetReturn { symbol: string; label: string; group: string; region?: string; oneYear?: number; threeYear?: number; fiveYear?: number; latestDate?: string }
export function assetReturns(assets: AssetSeries[]): AssetReturn[] {
  return assets.map((asset) => {
    const points = asset.points ?? [];
    const latest = points[points.length - 1];
    return { symbol: asset.symbol, label: asset.label, group: asset.group, region: asset.region, latestDate: latest?.date, oneYear: annualizedReturn(points, 12), threeYear: annualizedReturn(points, 36), fiveYear: annualizedReturn(points, 60) };
  });
}

export function countryMetricRows(countries: CountrySeries[], metric: CountryMetric, domain: [number, number]) {
  const rows = new Map<string, Record<string, string | number>>();
  countries.forEach((country) => (country.points ?? []).forEach((point) => {
    const date = Date.parse(point.date);
    const value = point[metric];
    if (date < domain[0] || date > domain[1] || typeof value !== "number") return;
    const row = rows.get(point.date) ?? { date: point.date, timestamp: date };
    row[country.code] = value;
    rows.set(point.date, row);
  }));
  return [...rows.values()].sort((a, b) => Number(a.timestamp) - Number(b.timestamp));
}

export interface ScenarioInputs { growth: number; inflation: number; realRate: number; dollar: number; liquidity: number }
const scenarioExposure: Record<string, ScenarioInputs> = {
  SPY: { growth: .8, inflation: -.25, realRate: -.65, dollar: -.2, liquidity: .45 }, QQQ: { growth: .9, inflation: -.35, realRate: -1.05, dollar: -.15, liquidity: .65 },
  FEZ: { growth: .75, inflation: -.2, realRate: -.55, dollar: .25, liquidity: .4 }, EWU: { growth: .65, inflation: -.18, realRate: -.45, dollar: .2, liquidity: .35 }, EWJ: { growth: .7, inflation: -.15, realRate: -.5, dollar: .35, liquidity: .4 }, FXI: { growth: 1.15, inflation: -.1, realRate: -.35, dollar: -.55, liquidity: .8 },
  EEM: { growth: 1, inflation: -.15, realRate: -.45, dollar: -.7, liquidity: .65 }, ACWI: { growth: .8, inflation: -.22, realRate: -.6, dollar: -.1, liquidity: .45 },
  TLT: { growth: -.35, inflation: -1.1, realRate: -1.25, dollar: .1, liquidity: .3 }, HYG: { growth: .75, inflation: -.2, realRate: -.5, dollar: -.15, liquidity: .7 },
  GLD: { growth: -.15, inflation: .65, realRate: -1, dollar: -.85, liquidity: .55 }, UUP: { growth: -.1, inflation: -.05, realRate: .65, dollar: 1, liquidity: -.2 },
};
export function scenarioImpacts(assets: AssetSeries[], inputs: ScenarioInputs) {
  return assets.flatMap((asset) => {
    const exposure = scenarioExposure[asset.symbol];
    if (!exposure) return [];
    const impact = (Object.keys(inputs) as Array<keyof ScenarioInputs>).reduce((sum, key) => sum + inputs[key] * exposure[key], 0);
    return [{ symbol: asset.symbol, label: asset.label, group: asset.group, impact }];
  }).sort((left, right) => right.impact - left.impact);
}

function annualizedReturn(points: Array<{ close: number }>, months: number) {
  if (points.length < 2) return undefined;
  const latest = points[points.length - 1].close;
  const prior = points[Math.max(0, points.length - 1 - months)]?.close;
  const elapsed = Math.min(months, points.length - 1) / 12;
  return prior > 0 && elapsed > 0 ? (Math.pow(latest / prior, 1 / elapsed) - 1) * 100 : undefined;
}
function regime(inflation?: Reading, growth?: Reading) {
  if (!inflation || !growth) return "Partial signal";
  if (inflation.ageMonths > 14 || growth.ageMonths > 14) return "Partial / stale signal";
  if (inflation.value >= 3 && growth.value < 0) return "Stagflation";
  if (inflation.value >= 3) return "Inflationary growth";
  if (growth.value < 0) return "Disinflationary slowdown";
  return "Disinflationary growth";
}
function monthDistance(left: string, right: string) {
  const a = new Date(`${left.slice(0, 7)}-01T00:00:00Z`); const b = new Date(`${right.slice(0, 7)}-01T00:00:00Z`);
  return Math.max(0, (b.getUTCFullYear() - a.getUTCFullYear()) * 12 + b.getUTCMonth() - a.getUTCMonth());
}
