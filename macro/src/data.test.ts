import { describe, expect, it } from "vitest";
import { indexedAssetRows, scenarioImpacts, sortSnapshots, snapshots } from "./data";
import type { AssetSeries, CountrySeries } from "./types";

describe("macro data", () => {
  it("indexes every asset to its first in-range observation", () => {
    const assets: AssetSeries[] = [{ symbol: "SPY", label: "US", group: "Equities", points: [{ date: "2020-01-01", close: 10 }, { date: "2020-02-01", close: 12 }] }];
    expect(indexedAssetRows(assets, ["SPY"], [Date.parse("2020-01-01"), Date.parse("2020-12-01")])).toMatchObject([{ SPY: 100 }, { SPY: 120 }]);
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

  it("applies directional scenario exposures", () => {
    const assets: AssetSeries[] = [{ symbol: "TLT", label: "Duration", group: "Rates" }, { symbol: "QQQ", label: "Growth", group: "Equities" }];
    const impacts = scenarioImpacts(assets, { growth: 0, inflation: 0, realRate: 1, dollar: 0, liquidity: 0 });
    expect(impacts.find((row) => row.symbol === "TLT")!.impact).toBeLessThan(impacts.find((row) => row.symbol === "QQQ")!.impact);
  });
});
