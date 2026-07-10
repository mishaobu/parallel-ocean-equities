const palette = ["#176b4d", "#2962a3", "#b46016", "#7047a3", "#a3304d", "#087b84", "#8a6d1d", "#3f4f9a", "#9b4f16", "#2e7d32", "#6b5b3e", "#b23a75", "#4c7a8b", "#555b66"];
const tickerOrder = ["AMZN", "GOOGL", "META", "MSFT", "SPY", "QQQ", "AMD", "NVDA", "MU", "SMCI", "DELL", "005930.KS", "BABA", "JD"];

export function equityColor(ticker: string) {
  const known = tickerOrder.indexOf(ticker);
  if (known >= 0) return palette[known % palette.length];
  let hash = 0;
  for (const character of ticker) hash = (hash * 31 + character.charCodeAt(0)) >>> 0;
  return palette[hash % palette.length];
}
