import { useRef, useState } from "react";

export interface RangeInteraction {
  rangeSelected?: boolean;
  onSelectDomain?: (domain: [number, number]) => void;
  onResetDomain?: () => void;
}

export function useRangeSelection(domain: [number, number], onSelect: ((domain: [number, number]) => void) | undefined, onPin?: (date: number) => void) {
  const [selection, setSelection] = useState<[number, number]>();
  const selectionRef = useRef<[number, number]>();

  function start(event: unknown) {
    const value = eventDate(event);
    if (value === undefined) return;
    selectionRef.current = [value, value];
    setSelection(selectionRef.current);
  }

  function move(event: unknown) {
    const value = eventDate(event);
    if (value === undefined || !selectionRef.current) return;
    selectionRef.current = [selectionRef.current[0], value];
    setSelection(selectionRef.current);
  }

  function finish() {
    const current = selectionRef.current;
    selectionRef.current = undefined;
    setSelection(undefined);
    if (!current) return;
    const next: [number, number] = current[0] <= current[1] ? current : [current[1], current[0]];
    if (next[1] - next[0] >= 20 * 24 * 60 * 60 * 1000) {
      onSelect?.([Math.max(domain[0], next[0]), Math.min(domain[1], next[1])]);
    } else {
      onPin?.(next[1]);
    }
  }

  return { selection, start, move, finish };
}

export function eventDate(event: unknown) {
  if (!event || typeof event !== "object" || !("activeLabel" in event)) return undefined;
  const value = Number((event as { activeLabel?: unknown }).activeLabel);
  return Number.isFinite(value) ? value : undefined;
}
