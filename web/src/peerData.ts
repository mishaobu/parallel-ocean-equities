import type { Equity, QualityPoint, ValuationPoint } from "./types";

const groups: Record<string, string> = {
	AMZN: "Digital platforms", GOOGL: "Digital platforms", META: "Digital platforms", MSFT: "Digital platforms", BABA: "Digital platforms", JD: "Digital platforms",
	AMD: "Semiconductors", NVDA: "Semiconductors", MU: "Semiconductors", SMCI: "Compute hardware", DELL: "Compute hardware", "005930.KS": "Semiconductors",
	SPY: "Broad-market ETF", QQQ: "Broad-market ETF",
};

export function peerGroup(equity: Equity) {
	return groups[equity.ticker] ?? (equity.instrumentType === "ETF" ? "Other ETF" : "Unclassified");
}

export function peerMedian(equities: Equity[], equity: Equity, value: (candidate: Equity) => unknown) {
	const group = peerGroup(equity);
	const values = equities.filter((candidate) => peerGroup(candidate) === group).map(value).filter(finite).sort((left, right) => left - right);
	return values.length >= 2 ? median(values) : undefined;
}

export function historyPercentile<T extends ValuationPoint | QualityPoint>(rows: T[] | undefined, property: keyof T, current: unknown) {
	if (!finite(current)) return undefined;
	const values = (rows ?? []).flatMap((row) => { const value = row[property]; return finite(value) ? [value] : []; }).sort((left, right) => left - right);
	if (values.length < 4) return undefined;
	return Math.round(values.filter((value) => value <= current).length / values.length * 100);
}

function median(values: number[]) {
	const middle = Math.floor(values.length / 2);
	return values.length % 2 ? values[middle] : (values[middle - 1] + values[middle]) / 2;
}

function finite(value: unknown): value is number { return typeof value === "number" && Number.isFinite(value); }
