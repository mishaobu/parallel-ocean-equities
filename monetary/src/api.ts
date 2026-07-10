import type { StateResponse } from "./types";

export async function getState(): Promise<StateResponse> {
  const response = await fetch("/monetary/api/state", { headers: { Accept: "application/json" } });
  if (!response.ok) throw new Error(`Macro API returned ${response.status}`);
  return response.json() as Promise<StateResponse>;
}
