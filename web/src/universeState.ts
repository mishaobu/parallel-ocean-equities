const tickerPattern = /^[A-Z0-9][A-Z0-9.-]{0,14}$/;

export interface UniverseDefinition {
	key: string;
	label: string;
	tickers: string[];
}

export function parseUniverseTickers(value: string | null | undefined): string[] {
  const unique = new Set<string>();
  for (const candidate of (value ?? "").split(",")) {
    const ticker = candidate.trim().toUpperCase();
    if (tickerPattern.test(ticker)) unique.add(ticker);
  }
  return [...unique].slice(0, 100);
}

export function resolveUniverseKey(requested: string | null, knownKeys: string[], sharedTickers: string[]): string {
  if (requested && knownKeys.includes(requested)) return requested;
  if (requested && sharedTickers.length > 0) return requested;
  return "core";
}

export function resolveSharedUniverse(requested: string | null, sharedTickers: string[], builtInKeys: string[]): UniverseDefinition | undefined {
	if (!requested || builtInKeys.includes(requested) || sharedTickers.length === 0) return undefined;
	return { key: requested, label: "Shared", tickers: sharedTickers };
}

export function mergeUniverses<T extends UniverseDefinition>(builtIns: T[], saved: T[], shared?: T): T[] {
	return [...builtIns, ...(shared ? [shared] : []), ...saved.filter((candidate) => candidate.key !== shared?.key)];
}
