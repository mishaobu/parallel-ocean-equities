import { describe, expect, it } from "vitest";
import { historyPercentile, peerGroup, peerMedian } from "./peerData";
import type { Equity } from "./types";

const equity = (ticker: string, pe: number): Equity => ({ ticker, status: "ready", annuals: [], current: {}, valuation: { pe }, valuations: [{ date: "2020-01-01", pe: 10 }, { date: "2021-01-01", pe: 20 }, { date: "2022-01-01", pe: 30 }, { date: "2023-01-01", pe: 40 }] });

describe("peer analytics", () => {
	it("uses explicit groups and requires at least two members for a median", () => {
		const rows = [equity("AMD", 20), equity("NVDA", 40), equity("DELL", 12)];
		expect(peerGroup(rows[0])).toBe("Semiconductors");
		expect(peerMedian(rows, rows[0], (row) => row.valuation?.pe)).toBe(30);
		expect(peerMedian(rows, rows[2], (row) => row.valuation?.pe)).toBeUndefined();
	});

	it("reports the current value against its own history", () => {
		const row = equity("AMD", 30);
		expect(historyPercentile(row.valuations, "pe", 30)).toBe(75);
	});
});
