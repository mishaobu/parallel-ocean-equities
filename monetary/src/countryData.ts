import type { CountryPoint, CountrySeries } from "./types";

export type CountryMetric = "inflation" | "coreInflation" | "policyRate" | "realRate" | "industrialGrowth" | "unemployment" | "moneyGrowth" | "longRate" | "yieldCurve" | "fx" | "leadingIndex";
export type CountrySort = "name" | "regime" | CountryMetric | "asOf";

export interface CountryReading { value: number; date: string; ageMonths: number }
export interface CountrySnapshot {
  country: CountrySeries;
  regime: string;
  asOf: string;
  values: Partial<Record<CountryMetric, CountryReading>>;
}

const dateFields: Partial<Record<CountryMetric, keyof CountryPoint>> = {
  inflation: "inflationDate", coreInflation: "coreInflationDate", policyRate: "policyRateDate",
  industrialGrowth: "industrialDate", unemployment: "unemploymentDate", moneyGrowth: "moneyGrowthDate",
  longRate: "longRateDate", fx: "fxDate", leadingIndex: "leadingIndexDate",
};

export function countrySnapshots(countries: CountrySeries[]): CountrySnapshot[] {
  const globalLatest = countries.flatMap((country) => country.points ?? []).reduce((latest, point) => point.date > latest ? point.date : latest, "");
  return countries.map((country) => {
    const values: CountrySnapshot["values"] = {};
    for (const metric of ["inflation", "coreInflation", "policyRate", "realRate", "industrialGrowth", "unemployment", "moneyGrowth", "longRate", "yieldCurve", "fx", "leadingIndex"] as CountryMetric[]) {
      const reading = latestCountryReading(country.points ?? [], metric, globalLatest);
      if (reading) values[metric] = reading;
    }
    const asOf = Object.values(values).reduce((latest, reading) => reading && reading.date > latest ? reading.date : latest, "");
    return { country, regime: countryRegime(values.inflation, values.industrialGrowth), asOf, values };
  });
}

export function latestCountryReading(points: CountryPoint[], metric: CountryMetric, latestDate?: string): CountryReading | undefined {
  const end = latestDate || points.reduce((latest, point) => point.date > latest ? point.date : latest, "");
  for (let index = points.length - 1; index >= 0; index -= 1) {
    const value = points[index][metric];
    if (typeof value !== "number" || !Number.isFinite(value)) continue;
    const dateField = dateFields[metric];
    const explicitDate = dateField ? points[index][dateField] : undefined;
    const date = typeof explicitDate === "string" && explicitDate ? explicitDate : points[index].date;
    return { value, date, ageMonths: monthDistance(date, end) };
  }
  return undefined;
}

export function sortCountrySnapshots(rows: CountrySnapshot[], key: CountrySort, direction: "asc" | "desc") {
  const multiplier = direction === "asc" ? 1 : -1;
  return [...rows].sort((left, right) => {
    if (key === "name") return multiplier * left.country.name.localeCompare(right.country.name);
    if (key === "regime") return multiplier * left.regime.localeCompare(right.regime);
    if (key === "asOf") return multiplier * left.asOf.localeCompare(right.asOf);
    const leftValue = left.values[key]?.value;
    const rightValue = right.values[key]?.value;
    if (leftValue === undefined) return 1;
    if (rightValue === undefined) return -1;
    return multiplier * (leftValue - rightValue);
  });
}

export function countryRegime(inflation?: CountryReading, growth?: CountryReading) {
  if (!inflation || !growth) return "Partial signal";
  if (inflation.ageMonths > 14 || growth.ageMonths > 14) return "Partial / stale signal";
  if (inflation.value >= 3 && growth.value < 0) return "Stagflation pressure";
  if (inflation.value >= 3) return "Inflationary expansion";
  if (growth.value < 0) return "Disinflationary slowdown";
  return "Disinflationary expansion";
}

export function countryChartRows(points: CountryPoint[], domain: [number, number]) {
  return points.map((point) => ({ ...point, timestamp: Date.parse(point.date) }))
    .filter((point) => Number.isFinite(point.timestamp) && point.timestamp >= domain[0] && point.timestamp <= domain[1]);
}

function monthDistance(left: string, right: string) {
  const start = new Date(`${left.slice(0, 7)}-01T00:00:00Z`);
  const end = new Date(`${right.slice(0, 7)}-01T00:00:00Z`);
  return Math.max(0, (end.getUTCFullYear() - start.getUTCFullYear()) * 12 + end.getUTCMonth() - start.getUTCMonth());
}
