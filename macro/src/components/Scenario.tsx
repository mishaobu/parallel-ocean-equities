import { useMemo, useState } from "react";
import { RotateCcw } from "lucide-react";
import { scenarioExposure, scenarioImpacts, type ScenarioInputs } from "../data";
import { calibratedScenarioImpacts, calibrateScenarioModels } from "../outcomes";
import type { AssetSeries, MacroPoint } from "../types";
import { ImpactChart } from "./Charts";

const fields: Array<{ key: keyof ScenarioInputs; label: string; note: string; min: number; max: number; step: number; unit: string }> = [
  { key: "growth", label: "Growth impulse", note: "Change in growth momentum", min: -3, max: 3, step: .25, unit: "pp" },
  { key: "inflation", label: "Inflation shock", note: "Unexpected inflation change", min: -3, max: 3, step: .25, unit: "pp" },
  { key: "realRate", label: "Real-rate shock", note: "Discount-rate repricing", min: -3, max: 3, step: .25, unit: "pp" },
  { key: "dollar", label: "Dollar shock", note: "Broad USD move", min: -10, max: 10, step: 1, unit: "%" },
  { key: "liquidity", label: "Liquidity impulse", note: "Global balance-sheet impulse", min: -10, max: 10, step: 1, unit: "%" },
];
export const neutralScenario: ScenarioInputs = { growth: 0, inflation: 0, realRate: 0, dollar: 0, liquidity: 0 };

export function ScenarioLab({ assets, points, domain, values, onChange }: { assets: AssetSeries[]; points: MacroPoint[]; domain: [number, number]; values: ScenarioInputs; onChange: (values: ScenarioInputs) => void }) {
  const [mode, setMode] = useState<"calibrated" | "structural">("calibrated");
  const models = useMemo(() => calibrateScenarioModels(assets, points, domain), [assets, domain, points]);
  const calibrated = useMemo(() => calibratedScenarioImpacts(models, values), [models, values]);
  const structural = useMemo(() => scenarioImpacts(assets, values), [assets, values]);
  const impacts = mode === "calibrated" ? calibrated : structural;
  const modelBySymbol = new Map(models.map((model) => [model.symbol, model]));
  return <><section className="scenario-mode"><div><strong>Model</strong><span>{mode === "calibrated" ? "Estimated from three-month outcomes in the selected history" : "Fixed directional assumptions"}</span></div><div className="segmented-control"><button type="button" className={mode === "calibrated" ? "is-active" : ""} onClick={() => setMode("calibrated")}>Calibrated</button><button type="button" className={mode === "structural" ? "is-active" : ""} onClick={() => setMode("structural")}>Structural</button></div></section>
    <section className="scenario-layout"><article className="panel scenario-controls"><header className="panel-head"><div><h2>Shock assumptions</h2><span>Move one factor at a time or combine a regime</span></div><button type="button" className="icon-button" title="Reset scenario" onClick={() => onChange(neutralScenario)}><RotateCcw size={15} /></button></header><div className="slider-list">{fields.map((field) => <label key={field.key}><div><span>{field.label}<small>{field.note}</small></span><output>{signed(values[field.key])}{field.unit}</output></div><input type="range" min={field.min} max={field.max} step={field.step} value={values[field.key]} onChange={(event) => onChange({ ...values, [field.key]: Number(event.target.value) })} /></label>)}</div></article><ImpactChart rows={impacts} note={mode === "calibrated" ? "Relative three-month return response estimated from selected history" : "Directional sensitivity score; percentage points are not forecast returns"} valueLabel={mode === "calibrated" ? "Relative response" : "Sensitivity score"} /></section>
    <article className="panel scenario-table"><header className="panel-head"><div><h2>Exposure decomposition</h2><span>{mode === "calibrated" ? "Return response per one-standard-deviation factor move / ridge fit diagnostics" : "Directional factor scores used by the structural model"}</span></div></header><div className="table-scroll"><table><thead><tr><th>Asset</th><th>Impact</th><th>Growth</th><th>Inflation</th><th>Real rate</th><th>USD</th><th>Liquidity</th><th>Fit</th><th>Samples</th><th>Interpretation</th></tr></thead><tbody>{impacts.map((row) => { const model = modelBySymbol.get(row.symbol); const exposure = mode === "calibrated" ? model?.exposures : scenarioExposure[row.symbol]; return <tr key={row.symbol}><th>{row.symbol}<span>{row.label}</span></th><td className={row.impact >= 0 ? "positive" : "negative"}>{signed(row.impact)}</td><td className={exposureTone(exposure?.growth)}>{factor(exposure?.growth)}</td><td className={exposureTone(exposure?.inflation)}>{factor(exposure?.inflation)}</td><td className={exposureTone(exposure?.realRate)}>{factor(exposure?.realRate)}</td><td className={exposureTone(exposure?.dollar)}>{factor(exposure?.dollar)}</td><td className={exposureTone(exposure?.liquidity)}>{factor(exposure?.liquidity)}</td><td>{mode === "calibrated" && model ? `${(model.rSquared * 100).toFixed(0)}%` : "--"}</td><td>{mode === "calibrated" && model ? model.sampleSize : "--"}</td><td>{row.impact > 1 ? "Material tailwind" : row.impact > .15 ? "Tailwind" : row.impact < -1 ? "Material headwind" : row.impact < -.15 ? "Headwind" : "Limited response"}</td></tr>; })}</tbody></table></div></article>
    <div className="method-note"><strong>Model boundary</strong><span>{mode === "calibrated" ? "Coefficients use quarterly-spaced observations, a two-month macro availability lag, standardized factors and ridge regularization. Responses are relative historical sensitivities, not expected returns." : "Static directional sensitivities express structural exposure. They do not estimate probabilities, valuation starting points, convexity, correlations, or forecast returns."}</span></div></>;
}
function signed(value: number) { return `${value > 0 ? "+" : ""}${value.toFixed(2)}`; }
function factor(value?: number) { return value === undefined ? "--" : signed(value); }
function exposureTone(value?: number) { return value === undefined ? "" : value >= 0 ? "positive" : "negative"; }
