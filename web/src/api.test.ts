import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";

describe("API request resilience", () => {
	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	it("turns a stalled state request into a retryable timeout", async () => {
		vi.useFakeTimers();
		vi.stubGlobal("fetch", vi.fn((_url: string | URL | Request, init?: RequestInit) => new Promise<Response>((_resolve, reject) => {
			init?.signal?.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")), { once: true });
		})));

		const pending = api.state();
		const rejection = expect(pending).rejects.toThrow("Request timed out. Try again.");
		await vi.advanceTimersByTimeAsync(15_000);
		await rejection;
	});
});
