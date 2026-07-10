import type { StateResponse } from "./types";

const base = import.meta.env.BASE_URL.replace(/\/$/, "");

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${base}/api${path}`, {
    ...init,
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
}

export const api = {
  state: () => request<StateResponse>("/state"),
  addTicker: (ticker: string) => request<{ ticker: string; status: string }>("/tickers", {
    method: "POST",
    body: JSON.stringify({ ticker }),
  }),
  removeTicker: (ticker: string) => request<void>(`/tickers/${encodeURIComponent(ticker)}`, { method: "DELETE" }),
  refreshTicker: (ticker: string) => request<{ ticker: string; queued: boolean }>(`/tickers/${encodeURIComponent(ticker)}/refresh`, { method: "POST" }),
};
