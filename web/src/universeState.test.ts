import { describe, expect, it } from "vitest";
import { mergeUniverses, parseUniverseTickers, resolveSharedUniverse, resolveUniverseKey } from "./universeState";

describe("portable comparison universes", () => {
  it("canonicalizes a missing local universe to core", () => {
    expect(resolveUniverseKey("saved:private", ["core", "compute"], [])).toBe("core");
  });

  it("restores a shared universe when its tickers travel with the URL", () => {
    const tickers = parseUniverseTickers("aapl,NVDA,AAPL,invalid ticker");
    expect(tickers).toEqual(["AAPL", "NVDA"]);
    expect(resolveUniverseKey("saved:quality", ["core", "compute"], tickers)).toBe("saved:quality");
  });

	it("lets portable URL tickers win a same-named local saved-universe collision", () => {
		const shared = resolveSharedUniverse("saved:quality", ["AAPL", "NVDA"], ["core", "compute"]);
		const merged = mergeUniverses(
			[{ key: "core", label: "Core", tickers: ["AMZN"] }],
			[{ key: "saved:quality", label: "Quality", tickers: ["META"] }],
			shared,
		);
		expect(merged.filter((candidate) => candidate.key === "saved:quality")).toEqual([shared]);
	});

	it("does not let URL ticker data redefine a built-in universe", () => {
		expect(resolveSharedUniverse("core", ["AAPL"], ["core", "compute"])).toBeUndefined();
	});
});
