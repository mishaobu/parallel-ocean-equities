import { describe, expect, it } from "vitest";
import { countrySnapshots, sortCountrySnapshots } from "./countryData";
import type { CountrySeries } from "./types";

const countries: CountrySeries[] = [
  { code: "US", name: "United States", currency: "USD", region: "Americas", policyLabel: "Fed funds", fxLabel: "USD", points: [{ date: "2026-01-01", inflation: 3, policyRate: 4, industrialGrowth: 1 }] },
  { code: "JP", name: "Japan", currency: "JPY", region: "Asia", policyLabel: "Short rate", fxLabel: "JPY/USD", points: [{ date: "2025-01-01", inflation: 2 }, { date: "2026-01-01", policyRate: 0.5, industrialGrowth: -1 }] },
];

describe("countrySnapshots", () => {
  it("keeps each metric's own observation date and age", () => {
    const rows = countrySnapshots(countries);
    expect(rows[1].values.inflation).toEqual({ value: 2, date: "2025-01-01", ageMonths: 12 });
		expect(rows[1].asOf).toBe("2025-01-01");
  });

  it("sorts missing values last in either direction", () => {
    const rows = countrySnapshots(countries);
    expect(sortCountrySnapshots(rows, "realRate", "desc").map((row) => row.country.code)).toEqual(["US", "JP"]);
    expect(sortCountrySnapshots(rows, "policyRate", "asc").map((row) => row.country.code)).toEqual(["JP", "US"]);
  });

  it("does not classify a regime from stale inputs", () => {
    const rows = countrySnapshots([{
      code: "EA", name: "Euro area", currency: "EUR", region: "Europe", policyLabel: "ECB", fxLabel: "EUR/USD",
      points: [{ date: "2024-01-01", inflation: 4, industrialGrowth: -1 }, { date: "2026-01-01", policyRate: 2 }],
    }]);
    expect(rows[0].regime).toBe("Partial / stale signal");
  });
});
