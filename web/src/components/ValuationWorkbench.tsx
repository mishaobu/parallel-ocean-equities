import { useEffect, useMemo, useState } from "react";
import type { Equity } from "../types";
import { formatBillions, percentValue } from "../valuationData";

type ModelKind = "dcf" | "multiple" | "earnings";

export function ValuationWorkbench({ equity }: { equity: Equity }) {
	const [model, setModel] = useState<ModelKind>(() => preferredModel(equity));
  const [growth, setGrowth] = useState(percentInput(equity.models?.fcfGrowth, 8));
  const [wacc, setWacc] = useState(percentInput(equity.models?.wacc, 9));
  const [terminalGrowth, setTerminalGrowth] = useState(percentInput(equity.models?.terminalGrowth, 3));
  const [multiple, setMultiple] = useState(equity.models?.targetEvToEbitda ?? 15);
  const [targetPE, setTargetPE] = useState(equity.models?.targetPe ?? 20);

	useEffect(() => {
		setModel(preferredModel(equity));
    setGrowth(percentInput(equity.models?.fcfGrowth, 8));
    setWacc(percentInput(equity.models?.wacc, 9));
    setTerminalGrowth(percentInput(equity.models?.terminalGrowth, 3));
    setMultiple(equity.models?.targetEvToEbitda ?? 15);
    setTargetPE(equity.models?.targetPe ?? 20);
  }, [equity.ticker, equity.forecast, equity.models]);

  const calculations = useMemo(() => {
    const base = modelValue(model, equity, { growth, wacc, terminalGrowth, multiple, targetPE });
    const scenarios = scenarioValues(model, equity, { growth, wacc, terminalGrowth, multiple, targetPE });
    return { base, scenarios };
  }, [equity, growth, model, multiple, targetPE, terminalGrowth, wacc]);

  return (
    <>
      <div className="basis-strip">
        <Basis label="Price" value={money(equity.current.price)} />
        <Basis label="Market cap" value={formatBillions(equity.valuation?.marketCapB)} />
        <Basis label="Enterprise value" value={formatBillions(equity.valuation?.enterpriseValueB)} />
        <Basis label="Forward FCF" value={formatBillions(equity.forecast?.forwardFcfB)} />
        <Basis label="Net debt" value={formatBillions(equity.valuation?.netDebtB)} />
      </div>
      <div className="model-workbench">
        <div className="model-tabs" aria-label="Valuation model">
			<button type="button" className={model === "dcf" ? "is-active" : ""} onClick={() => setModel("dcf")} disabled={!viableModel("dcf", equity)} title={!viableModel("dcf", equity) ? "DCF requires positive forward free cash flow" : undefined}>DCF</button>
			<button type="button" className={model === "multiple" ? "is-active" : ""} onClick={() => setModel("multiple")} disabled={!viableModel("multiple", equity)} title={!viableModel("multiple", equity) ? "EV / EBITDA requires positive forward EBITDA" : undefined}>EV / EBITDA</button>
			<button type="button" className={model === "earnings" ? "is-active" : ""} onClick={() => setModel("earnings")} disabled={!viableModel("earnings", equity)} title={!viableModel("earnings", equity) ? "P/E requires positive forward EPS" : undefined}>P/E</button>
        </div>
        <div className="model-grid">
          <div className="model-controls">
            {model === "dcf" && <>
              <NumberField label="5Y FCF growth" value={growth} onChange={setGrowth} suffix="%" min={-20} max={40} step={0.5} />
              <NumberField label="WACC" value={wacc} onChange={setWacc} suffix="%" min={4} max={20} step={0.25} />
              <NumberField label="Terminal growth" value={terminalGrowth} onChange={setTerminalGrowth} suffix="%" min={0} max={6} step={0.25} />
            </>}
            {model === "multiple" && <NumberField label="Target EV / EBITDA" value={multiple} onChange={setMultiple} suffix="x" min={1} max={60} step={0.5} />}
            {model === "earnings" && <NumberField label="Target P/E" value={targetPE} onChange={setTargetPE} suffix="x" min={1} max={80} step={0.5} />}
          </div>
          <div className="model-output">
            <span>Implied value / share</span>
            <strong>{money(calculations.base)}</strong>
            <small className={tone(upside(calculations.base, equity.current.price))}>{percentValue(upside(calculations.base, equity.current.price))} vs price</small>
          </div>
          <div className="scenario-table">
            <table>
              <thead><tr><th>Scenario</th><th>Value / share</th><th>Upside</th></tr></thead>
              <tbody>{calculations.scenarios.map((scenario) => {
                const change = upside(scenario.value, equity.current.price);
                return <tr key={scenario.label}><th>{scenario.label}</th><td>{money(scenario.value)}</td><td className={tone(change)}>{percentValue(change)}</td></tr>;
              })}</tbody>
            </table>
          </div>
        </div>
        <div className="model-source"><span>Price {equity.current.priceAsOf ?? "n/a"} / fundamentals {equity.valuation?.asOf ?? equity.quarterlies?.at(-1)?.periodEnd ?? "n/a"} / filed {equity.quarterlies?.at(-1)?.filedAt ?? "n/a"}</span><span>{equity.forecast?.horizon ?? "Forward"} / {equity.forecast?.method ?? "Model inputs unavailable"}</span></div>
      </div>
    </>
  );
}

function Basis({ label, value }: { label: string; value: string }) {
  return <div><span>{label}</span><strong>{value}</strong></div>;
}

function NumberField({ label, value, onChange, suffix, min, max, step }: { label: string; value: number; onChange: (value: number) => void; suffix: string; min: number; max: number; step: number }) {
  return <label className="model-field"><span>{label}</span><span><input type="number" value={value} min={min} max={max} step={step} onChange={(event) => onChange(Number(event.target.value))} /><b>{suffix}</b></span></label>;
}

interface Inputs {
  growth: number;
  wacc: number;
  terminalGrowth: number;
  multiple: number;
  targetPE: number;
}

function modelValue(model: ModelKind, equity: Equity, inputs: Inputs): number | undefined {
  if (model === "dcf") return dcf(
    equity.forecast?.forwardFcfB,
    equity.valuation?.netDebtB,
    equity.valuation?.dilutedSharesB,
    equity.models?.projectionYears ?? 5,
    inputs.growth / 100,
    inputs.wacc / 100,
    inputs.terminalGrowth / 100,
  );
  if (model === "multiple") return impliedMultiple(equity.forecast?.forwardEbitdaB, equity.valuation?.netDebtB, equity.valuation?.dilutedSharesB, inputs.multiple);
  const eps = equity.forecast?.forwardEps;
  return eps === undefined || eps <= 0 ? undefined : eps * inputs.targetPE;
}

function viableModel(model: ModelKind, equity: Equity) {
	if (model === "dcf") return (equity.forecast?.forwardFcfB ?? 0) > 0;
	if (model === "multiple") return (equity.forecast?.forwardEbitdaB ?? 0) > 0;
	return (equity.forecast?.forwardEps ?? 0) > 0;
}

function preferredModel(equity: Equity): ModelKind {
	return viableModel("dcf", equity) ? "dcf" : viableModel("multiple", equity) ? "multiple" : "earnings";
}

function scenarioValues(model: ModelKind, equity: Equity, inputs: Inputs) {
  const variants: Array<{ label: string; inputs: Inputs }> = model === "dcf" ? [
    { label: "Low", inputs: { ...inputs, growth: inputs.growth - 3, wacc: inputs.wacc + 1, terminalGrowth: Math.max(0, inputs.terminalGrowth - 0.5) } },
    { label: "Base", inputs },
    { label: "High", inputs: { ...inputs, growth: inputs.growth + 3, wacc: Math.max(inputs.terminalGrowth + 1, inputs.wacc - 1), terminalGrowth: inputs.terminalGrowth + 0.5 } },
  ] : model === "multiple" ? [
    { label: "Low", inputs: { ...inputs, multiple: Math.max(1, inputs.multiple - 2) } },
    { label: "Base", inputs },
    { label: "High", inputs: { ...inputs, multiple: inputs.multiple + 2 } },
  ] : [
    { label: "Low", inputs: { ...inputs, targetPE: Math.max(1, inputs.targetPE - 3) } },
    { label: "Base", inputs },
    { label: "High", inputs: { ...inputs, targetPE: inputs.targetPE + 3 } },
  ];
  return variants.map((variant) => ({ label: variant.label, value: modelValue(model, equity, variant.inputs) }));
}

function dcf(fcf?: number, netDebt = 0, shares?: number, years = 5, growth = 0.08, wacc = 0.09, terminalGrowth = 0.03): number | undefined {
  if (fcf === undefined || fcf <= 0 || shares === undefined || shares <= 0 || wacc <= terminalGrowth) return undefined;
  let projected = fcf;
  let presentValue = 0;
  for (let year = 1; year <= years; year += 1) {
    projected *= 1 + growth;
    presentValue += projected / ((1 + wacc) ** year);
  }
  presentValue += (projected * (1 + terminalGrowth) / (wacc - terminalGrowth)) / ((1 + wacc) ** years);
  return (presentValue - netDebt) / shares;
}

function impliedMultiple(ebitda?: number, netDebt = 0, shares?: number, multiple = 15): number | undefined {
  if (ebitda === undefined || ebitda <= 0 || shares === undefined || shares <= 0) return undefined;
  return ((ebitda * multiple) - netDebt) / shares;
}

function percentInput(value: number | undefined, fallback: number): number {
  return value === undefined ? fallback : Number((value * 100).toFixed(2));
}

function money(value?: number): string {
  return value === undefined || !Number.isFinite(value) ? "n/a" : `$${value.toFixed(2)}`;
}

function upside(value?: number, price?: number): number | undefined {
  return value === undefined || price === undefined || price === 0 ? undefined : value / price - 1;
}

function tone(value?: number): string {
  return value === undefined ? "" : value > 0 ? "positive" : value < 0 ? "negative" : "";
}
