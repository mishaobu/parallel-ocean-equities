import { useEffect, useMemo, useRef, useState } from "react";
import { CircleAlert, RotateCcw } from "lucide-react";

export interface ChartPointer { activeLabel?: string | number }
export interface SharedChartRange { zoom?: [number, number]; onZoom?: (zoom?: [number, number]) => void }
export interface SharedLegendFilter { hiddenKeys?: Set<string>; onHiddenKeys?: (hidden: Set<string>) => void }

export function useChartZoom(domain: [number, number], minimumSpan: number, controlledZoom?: [number, number], onZoom?: (zoom?: [number, number]) => void) {
  const [localZoom, setLocalZoom] = useState<[number, number]>();
  const [selection, setSelection] = useState<[number, number]>();
  const selectionRef = useRef<[number, number]>();
  const controlled = onZoom !== undefined;
  const zoom = controlled ? controlledZoom : localZoom;
  useEffect(() => {
    setLocalZoom(undefined);
    setSelection(undefined);
    selectionRef.current = undefined;
  }, [domain[0], domain[1]]);

  function start(event: ChartPointer) {
    const value = eventCoordinate(event);
    if (value === undefined) return;
    selectionRef.current = [value, value];
    setSelection(selectionRef.current);
  }
  function move(event: ChartPointer) {
    const value = eventCoordinate(event);
    if (value === undefined || !selectionRef.current) return;
    selectionRef.current = [selectionRef.current[0], value];
    setSelection(selectionRef.current);
  }
  function finish() {
    const current = selectionRef.current;
    if (!current) return;
    const ordered: [number, number] = current[0] <= current[1] ? current : [current[1], current[0]];
    if (ordered[1] - ordered[0] >= minimumSpan) {
      const next: [number, number] = [Math.max(domain[0], ordered[0]), Math.min(domain[1], ordered[1])];
      if (controlled) onZoom?.(next); else setLocalZoom(next);
    }
    selectionRef.current = undefined;
    setSelection(undefined);
  }
  return { activeDomain: zoom ?? domain, zoom, selection, start, move, finish, reset: () => controlled ? onZoom?.(undefined) : setLocalZoom(undefined) };
}

export function useLegendFilter(keys: string[], controlledHidden?: Set<string>, onHiddenKeys?: (hidden: Set<string>) => void) {
  const [localHidden, setLocalHidden] = useState<Set<string>>(() => new Set());
  const controlled = onHiddenKeys !== undefined;
  const hidden = controlled ? new Set([...(controlledHidden ?? [])].filter((key) => keys.includes(key))) : localHidden;
  const signature = keys.join("|");
  useEffect(() => {
    if (!controlled) setLocalHidden((current) => new Set([...current].filter((key) => keys.includes(key))));
  }, [signature]);
  const visibleKeys = useMemo(() => keys.filter((key) => !hidden.has(key)), [hidden, signature]);
  function toggle(key: string) {
    if (!keys.includes(key)) return;
    const update = (current: Set<string>) => {
      if (!current.has(key) && keys.length - current.size <= 1) return current;
      const next = new Set(current);
      if (next.has(key)) next.delete(key); else next.add(key);
      return next;
    };
    if (controlled) onHiddenKeys?.(update(hidden)); else setLocalHidden(update);
  }
  return { hidden, visibleKeys, toggle };
}

export function ChartHeadingMeta({ unit, zoom, onReset, clipped, clippedCount = 0, includeOutliers = false, onToggleOutliers, mode = "date" }: { unit: string; zoom?: [number, number]; onReset: () => void; clipped?: boolean; clippedCount?: number; includeOutliers?: boolean; onToggleOutliers?: () => void; mode?: "date" | "year" }) {
	const count = clippedCount || (clipped ? 1 : 0);
  return <div className="chart-heading-meta">
    <span>{zoom ? domainLabel(zoom, mode) : unit}</span>
		{count > 0 && (onToggleOutliers ? <button type="button" className="chart-outlier-button" onClick={onToggleOutliers} title={includeOutliers ? "Fit axis to typical observations" : "Include isolated extreme observations in the axis range"}><CircleAlert size={13} />{includeOutliers ? "Fit typical" : `${count} clipped / include`}</button> : <span className="chart-domain-alert" title="Axis excludes isolated extreme outliers"><CircleAlert size={13} aria-label="Axis excludes isolated extreme outliers" /></span>)}
    {zoom && <button type="button" className="icon-button" onClick={onReset} title="Reset selected period" aria-label="Reset selected period"><RotateCcw size={13} /></button>}
  </div>;
}

export function useFittedYDomain(rows: Array<Record<string, unknown>>, domain: [number, number], keys: string[], xKey: string, options: { log?: boolean; includeZero?: boolean } = {}) {
	const [includeOutliers, setIncludeOutliers] = useState(false);
	const fitted = fittedYDomain(rows, domain, keys, xKey, options);
	return {
		...fitted,
		domain: includeOutliers ? fitted.fullDomain : fitted.domain,
		includeOutliers,
		toggleOutliers: () => setIncludeOutliers((value) => !value),
	};
}

export function fittedYDomain(rows: Array<Record<string, unknown>>, domain: [number, number], keys: string[], xKey: string, options: { log?: boolean; includeZero?: boolean } = {}) {
  const observations = rows.flatMap((row) => {
    const x = Number(row[xKey]);
    if (!Number.isFinite(x) || x < domain[0] || x > domain[1]) return [];
    return keys.flatMap((key) => {
      const value = row[key];
      return typeof value === "number" && Number.isFinite(value) && (!options.log || value > 0) ? [{ key, x, value }] : [];
    });
  });
  if (!observations.length) return { domain: ["auto", "auto"] as ["auto", "auto"], fullDomain: ["auto", "auto"] as ["auto", "auto"], clipped: false, clippedCount: 0 };

  const excluded = new Set<string>();
  for (const key of keys) {
    const series = observations.filter((item) => item.key === key).sort((left, right) => left.x - right.x);
    if (series.length < 8) continue;
    const transformed = series.map((item) => options.log ? Math.log(item.value) : item.value).sort((left, right) => left - right);
    const q1 = quantile(transformed, 0.25);
    const q3 = quantile(transformed, 0.75);
    const spread = q3 - q1;
    if (spread <= 0) continue;
    const lowFence = q1 - 3 * spread;
    const highFence = q3 + 3 * spread;
    for (let index = 1; index < series.length - 1; index++) {
      const previous = options.log ? Math.log(series[index - 1].value) : series[index - 1].value;
      const current = options.log ? Math.log(series[index].value) : series[index].value;
      const next = options.log ? Math.log(series[index + 1].value) : series[index + 1].value;
      const outside = current < lowFence || current > highFence;
      const neighborsInside = previous >= lowFence && previous <= highFence && next >= lowFence && next <= highFence;
      if (outside && neighborsInside && Math.abs(current - previous) > 3 * spread && Math.abs(current - next) > 3 * spread) excluded.add(`${key}|${series[index].x}`);
    }
  }
	const fitted = observations.filter((item) => !excluded.has(`${item.key}|${item.x}`)).map((item) => item.value).sort((left, right) => left - right);
	const all = observations.map((item) => item.value).sort((left, right) => left - right);
	const fittedDomain = paddedDomain(fitted, options);
	const fullDomain = paddedDomain(all, options);
	return { domain: fittedDomain, fullDomain, clipped: excluded.size > 0, clippedCount: excluded.size };
}

function paddedDomain(values: number[], options: { log?: boolean; includeZero?: boolean }) {
	let low = values[0];
	let high = values[values.length-1];
  if (options.includeZero) {
    if (low > 0) low = 0;
    if (high < 0) high = 0;
  }
  if (options.log) {
    const factor = low === high ? 1.08 : 1.06;
    low = Math.max(Number.MIN_VALUE, low / factor);
    high *= factor;
  } else {
    const padding = low === high ? Math.max(Math.abs(high) * 0.06, 0.5) : (high - low) * 0.08;
    low -= padding;
    high += padding;
  }
	return [low, high] as [number, number];
}

function quantile(sorted: number[], position: number) {
  const index = (sorted.length - 1) * position;
  const lower = Math.floor(index);
  const weight = index - lower;
  return sorted[lower + 1] === undefined ? sorted[lower] : sorted[lower] * (1 - weight) + sorted[lower + 1] * weight;
}

function eventCoordinate(event: ChartPointer) {
  const value = Number(event?.activeLabel);
  return Number.isFinite(value) ? value : undefined;
}

function domainLabel(domain: [number, number], mode: "date" | "year") {
  if (mode === "year") return `${Math.round(domain[0])}-${Math.round(domain[1])}`;
  const format = (value: number) => new Date(value).toLocaleDateString("en-US", { month: "short", year: "2-digit", timeZone: "UTC" });
  return `${format(domain[0])} - ${format(domain[1])}`;
}
