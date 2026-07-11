import { CartesianGrid, Legend, Line, LineChart, ReferenceArea, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { ChartHeadingMeta, fittedYDomain, useChartZoom, useLegendFilter } from "../chartInteraction";
import { macroHistoryRows } from "../historyData";
import { descendingTooltipItem } from "../chartData";
import type { MacroPoint, MacroSeries } from "../types";

type MacroKey = Exclude<keyof MacroPoint, "date">;
type Unit = "percent" | "log";

const panels: { title: string; unit: Unit; zero?: boolean; lines: { key: MacroKey; label: string; color: string }[] }[] = [
  {
    title: "Inflation and rates",
    unit: "percent",
    zero: true,
    lines: [
      { key: "inflation", label: "CPI inflation", color: "#a3304d" },
      { key: "fedFunds", label: "Fed funds", color: "#176b4d" },
      { key: "treasury2Y", label: "2Y Treasury", color: "#b46016" },
      { key: "treasury10Y", label: "10Y Treasury", color: "#2962a3" },
    ],
  },
  {
    title: "Real rate and yield curve",
    unit: "percent",
    zero: true,
    lines: [
      { key: "realPolicyRate", label: "Real policy rate", color: "#7047a3" },
      { key: "yieldCurve", label: "10Y - 2Y", color: "#087b84" },
    ],
  },
  {
    title: "Money and central-bank assets",
    unit: "log",
    lines: [
      { key: "logM1", label: "log10 M1", color: "#176b4d" },
      { key: "logM2", label: "log10 M2", color: "#2962a3" },
      { key: "logFedAssets", label: "log10 Fed assets", color: "#b46016" },
    ],
  },
  {
    title: "Liquidity growth and credit",
    unit: "percent",
    zero: true,
    lines: [
      { key: "m1Growth", label: "M1 YoY", color: "#176b4d" },
      { key: "m2Growth", label: "M2 YoY", color: "#2962a3" },
      { key: "corporateSpread", label: "Corporate OAS", color: "#a3304d" },
    ],
  },
];

export function MacroCharts({ macro, domain }: { macro?: MacroSeries; domain: [number, number] }) {
  const data = macroHistoryRows(macro?.points ?? [], domain);
  if (data.length === 0) {
    return <div className="chart macro-empty"><div className="chart-empty">Macro history refresh pending{macro?.error ? `: ${macro.error}` : ""}</div></div>;
  }
  return <div className="macro-grid">
    {panels.map((panel) => <MacroChart key={panel.title} panel={panel} data={data} domain={domain} />)}
  </div>;
}

function MacroChart({ panel, data, domain }: { panel: typeof panels[number]; data: ReturnType<typeof macroHistoryRows>; domain: [number, number] }) {
  const keys = panel.lines.map((line) => line.key);
  const legend = useLegendFilter(keys);
  const chart = useChartZoom(domain, 20*24*60*60*1000);
  const fitted = fittedYDomain(data, chart.activeDomain, legend.visibleKeys, "date", { includeZero: panel.zero });
  return <div className="chart chart-compact macro-chart">
    <div className="chart-heading"><strong>{panel.title}</strong><ChartHeadingMeta unit={panel.unit === "log" ? "log10 / USD billions" : "percent"} zoom={chart.zoom} onReset={chart.reset} clipped={fitted.clipped} /></div>
    <div className="chart-canvas"><ResponsiveContainer width="100%" height="100%">
      <LineChart className="interactive-chart" data={data} margin={{ top: 12, right: 16, bottom: 2, left: -6 }} onMouseDown={chart.start} onMouseMove={chart.move} onMouseUp={chart.finish} onMouseLeave={chart.finish}>
        <CartesianGrid vertical={false} stroke="#e5e9e6" />
        <XAxis dataKey="date" type="number" scale="time" domain={chart.activeDomain} allowDataOverflow tickFormatter={yearLabel} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} minTickGap={30} />
        <YAxis domain={fitted.domain} allowDataOverflow tickFormatter={(value) => formatMacro(Number(value), panel.unit)} tick={{ fill: "#66736b", fontSize: 11 }} axisLine={false} tickLine={false} width={55} />
        <Tooltip itemSorter={descendingTooltipItem} formatter={(value) => formatMacro(Number(value), panel.unit)} labelFormatter={(value) => dateLabel(Number(value))} />
        <Legend iconType="line" wrapperStyle={{ fontSize: 11, cursor: "pointer" }} onClick={(event) => legend.toggle(String(event.dataKey ?? ""))} />
        {panel.zero && <ReferenceLine y={0} stroke="#aeb8b1" strokeDasharray="3 3" />}
        {chart.selection && <ReferenceArea x1={chart.selection[0]} x2={chart.selection[1]} fill="#7a9b88" fillOpacity={0.16} strokeOpacity={0} />}
        {panel.lines.map((line) => <Line key={line.key} type="monotone" dataKey={line.key} name={line.label} stroke={line.color} strokeWidth={1.8} dot={false} connectNulls hide={legend.hidden.has(line.key)} isAnimationActive={false} />)}
      </LineChart>
    </ResponsiveContainer></div>
  </div>;
}

function formatMacro(value: number, unit: Unit) {
  if (!Number.isFinite(value)) return "n/a";
  return unit === "percent" ? `${value.toFixed(1)}%` : value.toFixed(3);
}
function yearLabel(value: number) { return new Date(value).getUTCFullYear().toString(); }
function dateLabel(value: number) { return new Date(value).toLocaleDateString("en-US", { year: "numeric", month: "short", timeZone: "UTC" }); }
