import { useMemo, useState } from "react";
import { CartesianGrid, Legend, Line, LineChart, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { equityMacroRows, forwardRegimeReturns } from "../analysis";
import { descendingTooltipItem, latestReading } from "../macroData";
import type { EquitySummary, MacroPoint } from "../types";

type Overlay = "real10Y" | "netLiquidityGrowth" | "highYieldSpread";
const overlays: Record<Overlay, { label: string; color: string }> = {
  real10Y: { label: "Real 10Y", color: "#3975a7" },
  netLiquidityGrowth: { label: "Net liquidity YoY", color: "#347b57" },
  highYieldSpread: { label: "HY spread", color: "#b8493e" },
};

export function EquityTransmission({ equities, ticker, onTicker, points, domain, selectedDate, onInspect, onPin }: {
  equities: Record<string, EquitySummary>;
  ticker: string;
  onTicker: (ticker: string) => void;
  points: MacroPoint[];
  domain: [number, number];
  selectedDate?: number;
  onInspect?: (date?: number) => void;
  onPin?: (date: number) => void;
}) {
  const [overlay, setOverlay] = useState<Overlay>("real10Y");
  const equity = equities[ticker];
  const rows = useMemo(() => equityMacroRows(equity, points, domain), [domain, equity, points]);
  const stats = useMemo(() => forwardRegimeReturns(equity, points), [equity, points]);
  const pe = [...(equity?.valuations ?? [])].reverse().find((point) => typeof point.pe === "number")?.pe;
  const earningsYield = pe && pe > 0 ? 100 / pe : undefined;
  const real10Y = latestReading(points, "real10Y")?.value;
  const yieldGap = earningsYield !== undefined && real10Y !== undefined ? earningsYield - real10Y : undefined;
  const priceHistory = equity?.prices ?? [];
  const oneYearReturn = priceHistory.length >= 5 ? (priceHistory.at(-1)!.close / priceHistory.at(-5)!.close - 1) * 100 : undefined;
  const fullReturn = priceHistory.length >= 2 ? (priceHistory.at(-1)!.close / priceHistory[0].close - 1) * 100 : undefined;
  const tickers = Object.keys(equities).filter((candidate) => (equities[candidate].prices?.length ?? 0) >= 8).sort();

  return <section className="equity-transmission">
    <div className="transmission-toolbar">
      <div><h2>Equity transmission</h2><span>Rates, liquidity and credit mapped to market outcomes</span></div>
      <label>Instrument<select value={ticker} onChange={(event) => onTicker(event.target.value)}>{tickers.map((candidate) => <option key={candidate} value={candidate}>{candidate}</option>)}</select></label>
    </div>
    <div className="transmission-strip">
      <Metric label={earningsYield === undefined ? "1Y price return" : "LTM earnings yield"} value={percent(earningsYield ?? oneYearReturn)} note={pe ? `${pe.toFixed(1)}x P/E` : "quarterly close history"} />
      <Metric label="Real 10Y" value={percent(real10Y)} note={latestReading(points, "real10Y")?.date.slice(0, 7) ?? "unavailable"} />
      <Metric label={yieldGap === undefined ? "Full-history return" : "Yield gap"} value={percent(yieldGap ?? fullReturn)} note={yieldGap === undefined ? `since ${priceHistory[0]?.date.slice(0, 4) ?? "n/a"}` : "earnings yield less real 10Y"} />
      <Metric label="Net liquidity" value={percent(latestReading(points, "netLiquidityGrowth")?.value)} note="year-over-year" />
    </div>
    <article className="chart-frame transmission-chart">
      <header><div><h2>{ticker} and macro pressure</h2><span>Indexed quarterly price against selected macro series</span></div>
        <div className="overlay-switch" aria-label="Equity macro overlay">{(Object.keys(overlays) as Overlay[]).map((key) => <button type="button" key={key} className={overlay === key ? "is-active" : ""} onClick={() => setOverlay(key)}>{overlays[key].label}</button>)}</div>
      </header>
      <div className="chart-canvas chart-wide"><ResponsiveContainer width="100%" height="100%">
        <LineChart data={rows} margin={{ top: 15, right: 5, bottom: 3, left: 0 }} onMouseMove={(event) => onInspect?.(eventDate(event))} onMouseLeave={() => onInspect?.()} onClick={(event) => { const date = eventDate(event); if (date !== undefined) onPin?.(date); }}>
          <CartesianGrid vertical={false} stroke="#e4e8e5" />
          <XAxis dataKey="timestamp" type="number" scale="time" domain={domain} tickFormatter={yearLabel} tick={{ fill: "#69746d", fontSize: 10 }} minTickGap={38} axisLine={false} tickLine={false} />
          <YAxis yAxisId="price" scale="log" domain={["auto", "auto"]} tickFormatter={(value) => Number(value).toFixed(0)} tick={{ fill: "#69746d", fontSize: 10 }} axisLine={false} tickLine={false} width={48} />
          <YAxis yAxisId="macro" orientation="right" tickFormatter={(value) => `${Number(value).toFixed(1)}%`} tick={{ fill: "#69746d", fontSize: 10 }} axisLine={false} tickLine={false} width={44} />
          <Tooltip itemSorter={descendingTooltipItem} labelFormatter={(value) => dateLabel(Number(value))} formatter={(value, name) => [Number(value).toFixed(2), name]} contentStyle={{ border: "1px solid #cdd5cf", borderRadius: 4, fontSize: 11 }} />
          <Legend iconType="line" wrapperStyle={{ fontSize: 10 }} />
          {selectedDate !== undefined && <ReferenceLine yAxisId="price" x={selectedDate} stroke="#17201b" strokeOpacity={0.45} />}
          <Line yAxisId="price" type="monotone" dataKey="priceIndex" name={`${ticker} index`} stroke="#17201b" strokeWidth={2.4} dot={false} connectNulls isAnimationActive={false} />
          <Line yAxisId="macro" type="monotone" dataKey={overlay} name={overlays[overlay].label} stroke={overlays[overlay].color} strokeWidth={2} dot={false} connectNulls isAnimationActive={false} />
        </LineChart>
      </ResponsiveContainer></div>
    </article>
    <div className="regime-return-table">
      <div className="table-heading"><h3>Forward 12-month returns by regime</h3><span>Quarterly observations / macro data lagged two months / latest-revised history</span></div>
      <div className="table-wrap"><table><thead><tr><th>Regime</th><th>Samples</th><th>Average</th><th>Median</th><th>Positive</th></tr></thead><tbody>
        {stats.map((row) => <tr key={row.regime}><th>{row.regime}</th><td>{row.count}</td><td>{signedPercent(row.average)}</td><td>{signedPercent(row.median)}</td><td>{percent(row.positiveRate * 100)}</td></tr>)}
      </tbody></table></div>
    </div>
  </section>;
}

function Metric({ label, value, note }: { label: string; value: string; note: string }) { return <div><span>{label}</span><strong>{value}</strong><small>{note}</small></div>; }
function percent(value?: number) { return value === undefined ? "n/a" : `${value.toFixed(1)}%`; }
function signedPercent(value: number) { return `${value >= 0 ? "+" : ""}${(value * 100).toFixed(1)}%`; }
function eventDate(event: unknown) { if (!event || typeof event !== "object" || !("activeLabel" in event)) return undefined; const value = Number((event as { activeLabel?: unknown }).activeLabel); return Number.isFinite(value) ? value : undefined; }
function yearLabel(value: number) { return new Date(value).getUTCFullYear().toString(); }
function dateLabel(value: number) { return new Date(value).toLocaleDateString("en-US", { year: "numeric", month: "short", timeZone: "UTC" }); }
