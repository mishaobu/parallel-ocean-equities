import { RotateCcw } from "lucide-react";
import { scenarioImpacts, type ScenarioInputs } from "../data";
import type { AssetSeries } from "../types";
import { ImpactChart } from "./Charts";

const fields: Array<{ key: keyof ScenarioInputs; label: string; note: string; min: number; max: number; step: number; unit: string }> = [
  { key: "growth", label: "Growth impulse", note: "Change in growth momentum", min: -3, max: 3, step: .25, unit: "pp" },
  { key: "inflation", label: "Inflation shock", note: "Unexpected inflation change", min: -3, max: 3, step: .25, unit: "pp" },
  { key: "realRate", label: "Real-rate shock", note: "Discount-rate repricing", min: -3, max: 3, step: .25, unit: "pp" },
  { key: "dollar", label: "Dollar shock", note: "Broad USD move", min: -10, max: 10, step: 1, unit: "%" },
  { key: "liquidity", label: "Liquidity impulse", note: "Global balance-sheet impulse", min: -10, max: 10, step: 1, unit: "%" },
];
export const neutralScenario: ScenarioInputs = { growth: 0, inflation: 0, realRate: 0, dollar: 0, liquidity: 0 };

export function ScenarioLab({ assets, values, onChange }: { assets: AssetSeries[]; values: ScenarioInputs; onChange: (values: ScenarioInputs) => void }) {
  const impacts = scenarioImpacts(assets, values);
  return <><section className="scenario-layout"><article className="panel scenario-controls"><header className="panel-head"><div><h2>Shock assumptions</h2><span>Move one factor at a time or combine a regime</span></div><button type="button" className="icon-button" title="Reset scenario" onClick={() => onChange(neutralScenario)}><RotateCcw size={15} /></button></header><div className="slider-list">{fields.map((field) => <label key={field.key}><div><span>{field.label}<small>{field.note}</small></span><output>{signed(values[field.key])}{field.unit}</output></div><input type="range" min={field.min} max={field.max} step={field.step} value={values[field.key]} onChange={(event) => onChange({ ...values, [field.key]: Number(event.target.value) })} /></label>)}</div></article><ImpactChart rows={impacts} /></section>
    <article className="panel scenario-table"><header className="panel-head"><div><h2>Exposure decomposition</h2><span>Relative factor loadings used by the scenario engine</span></div></header><div className="table-scroll"><table><thead><tr><th>Asset</th><th>Impact</th><th>Interpretation</th></tr></thead><tbody>{impacts.map((row) => <tr key={row.symbol}><th>{row.symbol}<span>{row.label}</span></th><td className={row.impact >= 0 ? "positive" : "negative"}>{signed(row.impact)}</td><td>{row.impact > 1 ? "Material tailwind" : row.impact > .15 ? "Tailwind" : row.impact < -1 ? "Material headwind" : row.impact < -.15 ? "Headwind" : "Limited response"}</td></tr>)}</tbody></table></div></article>
    <div className="method-note"><strong>Model boundary</strong><span>Static directional sensitivities express relative macro exposure. They do not estimate probabilities, valuation starting points, convexity, correlations, or forecast returns.</span></div></>;
}
function signed(value: number) { return `${value > 0 ? "+" : ""}${value.toFixed(2)}`; }
