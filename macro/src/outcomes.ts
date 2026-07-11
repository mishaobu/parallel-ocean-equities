import type { ScenarioInputs } from "./data";
import { returnValue } from "./data";
import type { AssetSeries, CountryPoint, CountrySeries, MacroPoint, PricePoint, VintagePoint } from "./types";

export const regimes = ["Inflationary growth", "Stagflation", "Disinflationary slowdown", "Disinflationary growth"] as const;
export type Regime = typeof regimes[number];
export type ForwardHorizon = 3 | 6 | 12;

export interface OutcomeObservation {
  symbol: string;
  label: string;
  group: string;
  region?: string;
  regime: Regime;
  horizon: ForwardHorizon;
  startDate: string;
  endDate: string;
  returnPct: number;
  maxDrawdownPct: number;
}

export interface OutcomeStat {
  symbol: string;
  label: string;
  group: string;
  region?: string;
  regime: Regime;
  horizon: ForwardHorizon;
  count: number;
  average: number;
  median: number;
  positiveRate: number;
  ciLow: number;
  ciHigh: number;
	adjustedPValue: number;
  medianDrawdown: number;
  worstDrawdown: number;
  startDate: string;
  endDate: string;
}

export function regimeOutcomes(assets: AssetSeries[], country: CountrySeries | undefined, domain: [number, number], vintages: VintagePoint[] = []): OutcomeStat[] {
  if (!country) return [];
  const observations = assets.flatMap((asset) => assetObservations(asset, country.points ?? [], domain, vintages));
  const grouped = new Map<string, OutcomeObservation[]>();
  observations.forEach((observation) => {
    const key = `${observation.symbol}|${observation.regime}|${observation.horizon}`;
    const values = grouped.get(key) ?? [];
    values.push(observation);
    grouped.set(key, values);
  });
	const summarized = [...grouped.values()].map(summarizeOutcomes);
	const families = new Map<string, OutcomeStat[]>();
	summarized.forEach((row) => {
		const key = `${row.regime}|${row.horizon}`;
		families.set(key, [...(families.get(key) ?? []), row]);
	});
	families.forEach(applyHolmAdjustment);
	return summarized.sort((left, right) => left.symbol.localeCompare(right.symbol) || left.horizon - right.horizon || left.regime.localeCompare(right.regime));
}

export function currentRegime(country: CountrySeries | undefined): Regime | undefined {
  if (!country) return undefined;
  for (let index = (country.points?.length ?? 0) - 1; index >= 0; index -= 1) {
    const point = country.points![index];
    if (finite(point.inflation) && finite(point.industrialGrowth)) return classifyRegime(point.inflation, point.industrialGrowth);
  }
  return undefined;
}

export function classifyRegime(inflation: number, growth: number): Regime {
  if (inflation >= 3 && growth < 0) return "Stagflation";
  if (inflation >= 3) return "Inflationary growth";
  if (growth < 0) return "Disinflationary slowdown";
  return "Disinflationary growth";
}

function assetObservations(asset: AssetSeries, macro: CountryPoint[], domain: [number, number], vintages: VintagePoint[]): OutcomeObservation[] {
	const prices = [...(asset.points ?? [])].filter((point) => returnValue(point) > 0).sort((left, right) => left.date.localeCompare(right.date));
	const observations: OutcomeObservation[] = [];
	const lastStart: Record<ForwardHorizon, number> = { 3: -Infinity, 6: -Infinity, 12: -Infinity };
	for (let index = 0; index < prices.length; index += 1) {
		const start = prices[index];
		const startTime = Date.parse(start.date);
		if (startTime < domain[0] || startTime > domain[1]) continue;
		const macroPoint = vintages.length ? pointAtOrBefore(vintages, startTime) : pointAtOrBefore(macro, addMonths(startTime, -2));
		if (!macroPoint || !finite(macroPoint.inflation) || !finite(macroPoint.industrialGrowth)) continue;
		for (const horizon of [3, 6, 12] as ForwardHorizon[]) {
			if (monthDistance(lastStart[horizon], startTime) < horizon) continue;
			const endIndex = pointAtOrAfter(prices, addMonths(startTime, horizon), index + 1);
      if (endIndex < 0) continue;
      const end = prices[endIndex];
      const endTime = Date.parse(end.date);
      if (endTime > domain[1] || monthDistance(addMonths(startTime, horizon), endTime) > 1) continue;
      observations.push({
        symbol: asset.symbol, label: asset.label, group: asset.group, region: asset.region,
        regime: classifyRegime(macroPoint.inflation, macroPoint.industrialGrowth), horizon,
			startDate: start.date, endDate: end.date, returnPct: (returnValue(end) / returnValue(start) - 1) * 100,
				maxDrawdownPct: maxDrawdown(prices.slice(index, endIndex + 1)),
			});
			lastStart[horizon] = startTime;
		}
	}
  return observations;
}

function summarizeOutcomes(rows: OutcomeObservation[]): OutcomeStat {
  const returns = rows.map((row) => row.returnPct);
  const drawdowns = rows.map((row) => row.maxDrawdownPct);
  const average = mean(returns);
	const standardError = hacMeanStandardError(returns);
	const margin = returns.length > 1 ? critical95(returns.length) * standardError : 0;
  return {
    symbol: rows[0].symbol, label: rows[0].label, group: rows[0].group, region: rows[0].region,
    regime: rows[0].regime, horizon: rows[0].horizon, count: rows.length,
    average, median: median(returns), positiveRate: returns.filter((value) => value > 0).length / returns.length,
		adjustedPValue: twoSidedNormalP(standardError > 0 ? average / standardError : 0),
    ciLow: average - margin, ciHigh: average + margin, medianDrawdown: median(drawdowns), worstDrawdown: Math.min(...drawdowns),
    startDate: rows.reduce((value, row) => row.startDate < value ? row.startDate : value, rows[0].startDate),
    endDate: rows.reduce((value, row) => row.endDate > value ? row.endDate : value, rows[0].endDate),
  };
}

export interface CalibrationRow { factors: ScenarioInputs; outcome: number }
export interface CalibrationModel {
  symbol: string;
  label: string;
  group: string;
  sampleSize: number;
  rSquared: number;
	stability: number;
  exposures: ScenarioInputs;
	uncertainty: ScenarioInputs;
  scales: ScenarioInputs;
}

export function calibrateScenarioModels(assets: AssetSeries[], points: MacroPoint[], domain: [number, number]): CalibrationModel[] {
  return assets.flatMap((asset) => {
    const rows = calibrationRows(asset.points ?? [], points, domain);
    const fitted = fitScenarioCalibration(rows);
		return fitted && fitted.rSquared >= .1 && fitted.stability >= .6 ? [{ symbol: asset.symbol, label: asset.label, group: asset.group, ...fitted }] : [];
  });
}

export function calibratedScenarioImpacts(models: CalibrationModel[], inputs: ScenarioInputs) {
  const raw = models.map((model) => ({
    symbol: model.symbol, label: model.label, group: model.group, sampleSize: model.sampleSize, rSquared: model.rSquared,
    impact: factorKeys.reduce((sum, key) => sum + model.exposures[key] * inputs[key] / model.scales[key], 0),
  }));
  const center = raw.length ? mean(raw.map((row) => row.impact)) : 0;
  return raw.map((row) => ({ ...row, impact: row.impact - center })).sort((left, right) => right.impact - left.impact);
}

export function fitScenarioCalibration(rows: CalibrationRow[]): Omit<CalibrationModel, "symbol" | "label" | "group"> | undefined {
	if (rows.length < 60) return undefined;
	const split = Math.floor(rows.length * .7);
	const training = rows.slice(0, split);
	const validation = rows.slice(split);
	if (training.length < 36 || validation.length < 12) return undefined;
	const scales = factorObject((key) => sampleDeviation(training.map((row) => row.factors[key])));
	if (factorKeys.some((key) => !finite(scales[key]) || scales[key] < 1e-6)) return undefined;
	const centers = factorObject((key) => mean(training.map((row) => row.factors[key])));
	const matrix = training.map((row) => [1, ...factorKeys.map((key) => (row.factors[key] - centers[key]) / scales[key])]);
	const outcome = training.map((row) => row.outcome);
	const coefficients = ridgeRegression(matrix, outcome, 0.35);
	if (!coefficients) return undefined;
	const validationOutcome = validation.map((row) => row.outcome);
	const validationFitted = validation.map((row) => coefficients[0] + factorKeys.reduce((sum, key, index) => sum + ((row.factors[key] - centers[key]) / scales[key]) * coefficients[index + 1], 0));
	const outcomeMean = mean(validationOutcome);
	const total = validationOutcome.reduce((sum, value) => sum + (value - outcomeMean) ** 2, 0);
	const residual = validationOutcome.reduce((sum, value, index) => sum + (value - validationFitted[index]) ** 2, 0);
	const bootstrap = bootstrapCoefficients(training, centers, scales, 36);
	if (bootstrap.length < 12) return undefined;
	const uncertainty = factorObject((key) => sampleDeviation(bootstrap.map((sample) => sample[factorKeys.indexOf(key) + 1])));
	const stability = factorKeys.reduce((sum, _key, index) => {
		const coefficient = coefficients[index + 1];
		return sum + bootstrap.filter((sample) => Math.sign(sample[index + 1]) === Math.sign(coefficient)).length / bootstrap.length;
	}, 0) / factorKeys.length;
	return {
		sampleSize: rows.length,
		rSquared: total > 0 ? Math.max(-1, Math.min(1, 1 - residual / total)) : 0,
		stability,
    exposures: factorObject((key) => coefficients[factorKeys.indexOf(key) + 1]),
		uncertainty,
    scales,
  };
}

function bootstrapCoefficients(rows: CalibrationRow[], centers: ScenarioInputs, scales: ScenarioInputs, samples: number) {
	const block = 4;
	return Array.from({ length: samples }, (_, sample) => {
		const sampled: CalibrationRow[] = [];
		for (let blockIndex = 0; sampled.length < rows.length; blockIndex += 1) {
			const start = (sample * 17 + blockIndex * 11 + sample * blockIndex * 3) % rows.length;
			for (let offset = 0; offset < block && sampled.length < rows.length; offset += 1) sampled.push(rows[(start + offset) % rows.length]);
		}
		const matrix = sampled.map((row) => [1, ...factorKeys.map((key) => (row.factors[key] - centers[key]) / scales[key])]);
		return ridgeRegression(matrix, sampled.map((row) => row.outcome), .35);
	}).filter((value): value is number[] => value !== undefined);
}

const factorKeys: Array<keyof ScenarioInputs> = ["growth", "inflation", "realRate", "dollar", "liquidity"];

function calibrationRows(pricesInput: PricePoint[], macroInput: MacroPoint[], domain: [number, number]): CalibrationRow[] {
	const prices = [...pricesInput].filter((point) => returnValue(point) > 0).sort((left, right) => left.date.localeCompare(right.date));
  const macro = [...macroInput].sort((left, right) => left.date.localeCompare(right.date));
  const rows: CalibrationRow[] = [];
  let lastStart = -Infinity;
  for (let index = 0; index < prices.length; index += 1) {
    const startTime = Date.parse(prices[index].date);
    if (startTime < domain[0] || startTime > domain[1] || monthDistance(lastStart, startTime) < 3) continue;
    const endIndex = pointAtOrAfter(prices, addMonths(startTime, 3), index + 1);
    if (endIndex < 0 || Date.parse(prices[endIndex].date) > domain[1]) continue;
    const macroIndex = indexAtOrBefore(macro, addMonths(startTime, -2));
    const factors = macroIndex < 0 ? undefined : factorChange(macro, macroIndex);
    if (!factors) continue;
		rows.push({ factors, outcome: (returnValue(prices[endIndex]) / returnValue(prices[index]) - 1) * 100 });
    lastStart = startTime;
  }
  return rows;
}

function factorChange(points: MacroPoint[], index: number): ScenarioInputs | undefined {
  const current = points[index];
  const priorIndex = indexAtOrBefore(points, addMonths(Date.parse(current.date), -3));
  if (priorIndex < 0) return undefined;
  const prior = points[priorIndex];
  if (![current.industrialGrowth, prior.industrialGrowth, current.inflation, prior.inflation, current.realPolicyRate, prior.realPolicyRate, current.dollarIndex, prior.dollarIndex, current.netLiquidityGrowth, prior.netLiquidityGrowth].every(finite)) return undefined;
  return {
    growth: current.industrialGrowth! - prior.industrialGrowth!,
    inflation: current.inflation! - prior.inflation!,
    realRate: current.realPolicyRate! - prior.realPolicyRate!,
    dollar: (current.dollarIndex! / prior.dollarIndex! - 1) * 100,
    liquidity: current.netLiquidityGrowth! - prior.netLiquidityGrowth!,
  };
}

function ridgeRegression(matrix: number[][], outcome: number[], lambda: number) {
  const width = matrix[0]?.length ?? 0;
  if (!width) return undefined;
  const normal = Array.from({ length: width }, (_, row) => Array.from({ length: width }, (_, column) => matrix.reduce((sum, values) => sum + values[row] * values[column], 0) + (row === column && row > 0 ? lambda : 0)));
  const target = Array.from({ length: width }, (_, column) => matrix.reduce((sum, values, row) => sum + values[column] * outcome[row], 0));
  return solve(normal, target);
}

function solve(matrixInput: number[][], targetInput: number[]) {
  const matrix = matrixInput.map((row, index) => [...row, targetInput[index]]);
  for (let column = 0; column < matrix.length; column += 1) {
    let pivot = column;
    for (let row = column + 1; row < matrix.length; row += 1) if (Math.abs(matrix[row][column]) > Math.abs(matrix[pivot][column])) pivot = row;
    [matrix[column], matrix[pivot]] = [matrix[pivot], matrix[column]];
    if (Math.abs(matrix[column][column]) < 1e-10) return undefined;
    const divisor = matrix[column][column];
    for (let index = column; index <= matrix.length; index += 1) matrix[column][index] /= divisor;
    for (let row = 0; row < matrix.length; row += 1) {
      if (row === column) continue;
      const factor = matrix[row][column];
      for (let index = column; index <= matrix.length; index += 1) matrix[row][index] -= factor * matrix[column][index];
    }
  }
  return matrix.map((row) => row[matrix.length]);
}

function factorObject(factory: (key: keyof ScenarioInputs) => number): ScenarioInputs {
  return { growth: factory("growth"), inflation: factory("inflation"), realRate: factory("realRate"), dollar: factory("dollar"), liquidity: factory("liquidity") };
}

function pointAtOrBefore<T extends { date: string }>(points: T[], timestamp: number) {
  const index = indexAtOrBefore(points, timestamp);
  return index < 0 ? undefined : points[index];
}

function indexAtOrBefore(points: Array<{ date: string }>, timestamp: number) {
  for (let index = points.length - 1; index >= 0; index -= 1) if (Date.parse(points[index].date) <= timestamp) return index;
  return -1;
}

function pointAtOrAfter(points: Array<{ date: string }>, timestamp: number, start: number) {
  for (let index = start; index < points.length; index += 1) if (Date.parse(points[index].date) >= timestamp) return index;
  return -1;
}

function maxDrawdown(points: PricePoint[]) {
	let peak = points[0] ? returnValue(points[0]) : 0;
  let drawdown = 0;
	points.forEach((point) => { const value = returnValue(point); peak = Math.max(peak, value); if (peak > 0) drawdown = Math.min(drawdown, (value / peak - 1) * 100); });
  return drawdown;
}

function addMonths(timestamp: number, months: number) {
  const date = new Date(timestamp);
  date.setUTCMonth(date.getUTCMonth() + months);
  return date.getTime();
}

function monthDistance(left: number, right: number) {
  if (!Number.isFinite(left) || !Number.isFinite(right)) return Infinity;
  const a = new Date(left); const b = new Date(right);
  return (b.getUTCFullYear() - a.getUTCFullYear()) * 12 + b.getUTCMonth() - a.getUTCMonth();
}

function mean(values: number[]) { return values.reduce((sum, value) => sum + value, 0) / values.length; }
function median(values: number[]) { const sorted = [...values].sort((a, b) => a - b); const middle = Math.floor(sorted.length / 2); return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2; }
function sampleDeviation(values: number[], center = mean(values)) { return values.length > 1 ? Math.sqrt(values.reduce((sum, value) => sum + (value - center) ** 2, 0) / (values.length - 1)) : 0; }
function critical95(count: number) { return count <= 5 ? 2.776 : count <= 10 ? 2.262 : count <= 20 ? 2.093 : count <= 30 ? 2.045 : 1.96; }
function finite(value: unknown): value is number { return typeof value === "number" && Number.isFinite(value); }

function hacMeanStandardError(values: number[]) {
	if (values.length < 2) return 0;
	const center = mean(values);
	const residuals = values.map((value) => value - center);
	const lag = Math.max(1, Math.floor(4 * (values.length / 100) ** (2 / 9)));
	let longRunVariance = residuals.reduce((sum, value) => sum + value * value, 0) / values.length;
	for (let offset = 1; offset <= Math.min(lag, values.length - 1); offset += 1) {
		const covariance = residuals.slice(offset).reduce((sum, value, index) => sum + value * residuals[index], 0) / values.length;
		longRunVariance += 2 * (1 - offset / (lag + 1)) * covariance;
	}
	return Math.sqrt(Math.max(0, longRunVariance) / values.length);
}

function applyHolmAdjustment(rows: OutcomeStat[]) {
	const ordered = [...rows].sort((left, right) => left.adjustedPValue - right.adjustedPValue);
	let previous = 0;
	ordered.forEach((row, index) => {
		const adjusted = Math.min(1, row.adjustedPValue * (ordered.length - index));
		row.adjustedPValue = Math.max(previous, adjusted);
		previous = row.adjustedPValue;
	});
}

function twoSidedNormalP(value: number) {
	const z = Math.abs(value);
	const t = 1 / (1 + .2316419 * z);
	const density = .3989422804 * Math.exp(-z * z / 2);
	const tail = density * t * (.31938153 + t * (-.356563782 + t * (1.781477937 + t * (-1.821255978 + t * 1.330274429))));
	return Math.min(1, Math.max(0, 2 * tail));
}
