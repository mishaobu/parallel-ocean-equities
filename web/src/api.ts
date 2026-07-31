import type { Equity, LiveQuote, StateResponse } from "./types";

const base = import.meta.env.BASE_URL.replace(/\/$/, "");
const requestTimeoutMs = 15_000;
const quoteTimeoutMs = 20_000;

async function request<T>(path: string, init?: RequestInit, timeoutMs = requestTimeoutMs): Promise<T> {
	const controller = new AbortController();
	let timedOut = false;
	const timeout = globalThis.setTimeout(() => { timedOut = true; controller.abort(); }, timeoutMs);
	try {
		const response = await fetch(`${base}/api${path}`, {
			...init,
			signal: init?.signal ?? controller.signal,
			headers: {
				"Content-Type": "application/json",
				...init?.headers,
			},
		});
		if (!response.ok) {
			const payload = await response.json().catch(() => ({ error: response.statusText }));
			throw new Error(payload.error ?? `Request failed (${response.status})`);
		}
		if (response.status === 204) return undefined as T;
		return response.json() as Promise<T>;
	} catch (error) {
		if (timedOut) throw new Error("Request timed out. Try again.");
		throw error;
	} finally {
		globalThis.clearTimeout(timeout);
	}
}

export const api = {
	state: () => request<StateResponse>("/state"),
	equity: (ticker: string) => request<Equity>(`/tickers/${encodeURIComponent(ticker)}`),
	quote: (ticker: string, includeHistory = true) => request<LiveQuote>(`/tickers/${encodeURIComponent(ticker)}/quote${includeHistory ? "" : "?history=0"}`, undefined, quoteTimeoutMs),
};
