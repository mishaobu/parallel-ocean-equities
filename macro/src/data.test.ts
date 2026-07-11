import { describe, expect, it } from "vitest";
import { indexedAssetRows, scenarioImpacts, sortSnapshots, snapshots } from "./data";
import { fittedYDomain } from "./components/Charts";
import type { AssetSeries, CountrySeries } from "./types";

describe("macro data", () => {
	it("indexes assets to a common start using adjusted return closes", () => {
		const assets: AssetSeries[] = [
			{ symbol: "SPY", label: "US", group: "Equities", points: [{ date: "2019-01-01", close: 8, totalReturnClose: 5 }, { date: "2020-01-01", close: 10, totalReturnClose: 8 }, { date: "2020-02-01", close: 12, totalReturnClose: 12 }] },
			{ symbol: "QQQ", label: "Growth", group: "Equities", points: [{ date: "2020-01-01", close: 20, totalReturnClose: 16 }, { date: "2020-02-01", close: 22, totalReturnClose: 20 }] },
		];
		expect(indexedAssetRows(assets, ["SPY", "QQQ"], [Date.parse("2019-01-01"), Date.parse("2020-12-01")])).toEqual([
			{ date: "2020-01-01", timestamp: Date.parse("2020-01-01"), SPY: 100, QQQ: 100 },
			{ date: "2020-02-01", timestamp: Date.parse("2020-02-01"), SPY: 150, QQQ: 125 },
		]);
	});

  it("sorts missing country values last", () => {
    const countries: CountrySeries[] = [
      { code: "US", name: "United States", currency: "USD", region: "Americas", policyLabel: "Fed", fxLabel: "USD", points: [{ date: "2026-01-01", realRate: 1 }] },
      { code: "JP", name: "Japan", currency: "JPY", region: "Asia", policyLabel: "Rate", fxLabel: "JPY", points: [{ date: "2026-01-01" }] },
    ];
    expect(sortSnapshots(snapshots(countries), "realRate", "asc").map((row) => row.country.code)).toEqual(["US", "JP"]);
  });

  it("does not classify a regime from stale inputs", () => {
    const countries: CountrySeries[] = [{
      code: "EA", name: "Euro area", currency: "EUR", region: "Europe", policyLabel: "ECB", fxLabel: "EUR/USD",
      points: [{ date: "2024-01-01", inflation: 4, industrialGrowth: -1 }, { date: "2026-01-01", policyRate: 2 }],
    }];
    expect(snapshots(countries)[0].regime).toBe("Partial / stale signal");
  });

	it("reconstructs country readings at a common historical cutoff", () => {
		const countries: CountrySeries[] = [{ code: "US", name: "United States", currency: "USD", region: "Americas", policyLabel: "Fed", fxLabel: "USD", points: [
			{ date: "2020-01-01", inflation: 2, policyRate: 1 }, { date: "2021-01-01", inflation: 4, policyRate: 3 },
		] }];
		const row = snapshots(countries, "2020-06-01")[0];
		expect(row.values.inflation?.value).toBe(2);
		expect(row.values.policyRate?.value).toBe(1);
	});

  it("applies directional scenario exposures", () => {
    const assets: AssetSeries[] = [{ symbol: "TLT", label: "Duration", group: "Rates" }, { symbol: "QQQ", label: "Growth", group: "Equities" }];
    const impacts = scenarioImpacts(assets, { growth: 0, inflation: 0, realRate: 1, dollar: 0, liquidity: 0 });
    expect(impacts.find((row) => row.symbol === "TLT")!.impact).toBeLessThan(impacts.find((row) => row.symbol === "QQQ")!.impact);
  });

  it("fits the y axis to visible lines inside a selected region", () => {
    const rows = [
      { timestamp: Date.parse("2020-01-01"), SPY: 100, QQQ: 200 },
      { timestamp: Date.parse("2021-01-01"), SPY: 110, QQQ: 400 },
      { timestamp: Date.parse("2022-01-01"), SPY: 120, QQQ: 800 },
    ];
    const domain = fittedYDomain(rows, [Date.parse("2020-06-01"), Date.parse("2021-06-01")], ["SPY"]);
    expect(domain[0]).toBeLessThan(110);
    expect(domain[1]).toBeGreaterThan(110);
    expect(domain[1]).toBeLessThan(200);
  });
});
