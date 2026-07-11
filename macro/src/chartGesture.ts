import { useRef, useState, type TouchEvent } from "react";

export interface TouchPoint { clientX: number }
export interface TouchBounds { left: number; width: number }

export function useRangeSelection(domain: [number, number], onSelect?: (domain: [number, number]) => void) {
  const [selection, setSelection] = useState<[number, number]>();
  const selectionRef = useRef<[number, number]>();
  const ignoreMouseUntil = useRef(0);

  function start(event: unknown) {
    if (Date.now() < ignoreMouseUntil.current) return;
    const value = eventTimestamp(event);
    if (value === undefined) return;
    selectionRef.current = [value, value];
    setSelection(selectionRef.current);
  }

  function move(event: unknown) {
    if (Date.now() < ignoreMouseUntil.current) return;
    const value = eventTimestamp(event);
    if (value === undefined || !selectionRef.current) return;
    selectionRef.current = [selectionRef.current[0], value];
    setSelection(selectionRef.current);
  }

  function finish() {
    if (Date.now() < ignoreMouseUntil.current) return;
    commitSelection();
  }

  function commitSelection() {
    const current = selectionRef.current;
    selectionRef.current = undefined;
    setSelection(undefined);
    if (!current) return;
    const next: [number, number] = current[0] <= current[1] ? current : [current[1], current[0]];
    if (next[1] - next[0] >= 20 * 24 * 60 * 60 * 1000) {
      onSelect?.([Math.max(domain[0], next[0]), Math.min(domain[1], next[1])]);
    }
  }

  function updateTouchSelection(event: TouchEvent<HTMLElement>) {
    ignoreMouseUntil.current = Date.now() + 800;
    const next = touchDomainRange(domain, touchPoints(event.touches), plotBounds(event.currentTarget));
    if (!next) {
      selectionRef.current = undefined;
      setSelection(undefined);
      return;
    }
    if (event.cancelable) event.preventDefault();
    selectionRef.current = next;
    setSelection(next);
  }

  function finishTouch(event: TouchEvent<HTMLElement>) {
    ignoreMouseUntil.current = Date.now() + 800;
    if (!selectionRef.current) return;
    if (event.cancelable) event.preventDefault();
    commitSelection();
  }

  function cancelTouch() {
    selectionRef.current = undefined;
    setSelection(undefined);
  }

  return {
    selection,
    start,
    move,
    finish,
    touchHandlers: { onTouchStart: updateTouchSelection, onTouchMove: updateTouchSelection, onTouchEnd: finishTouch, onTouchCancel: cancelTouch },
  };
}

export function touchDomainRange(domain: [number, number], touches: TouchPoint[], bounds: TouchBounds): [number, number] | undefined {
  if (touches.length < 2 || !Number.isFinite(bounds.width) || bounds.width <= 0) return undefined;
  const value = (clientX: number) => {
    const ratio = Math.min(1, Math.max(0, (clientX - bounds.left) / bounds.width));
    return domain[0] + ratio * (domain[1] - domain[0]);
  };
  const first = value(touches[0].clientX);
  const second = value(touches[1].clientX);
  return first <= second ? [first, second] : [second, first];
}

function touchPoints(touches: TouchEvent<HTMLElement>["touches"]) {
  const points: TouchPoint[] = [];
  for (let index = 0; index < Math.min(touches.length, 2); index++) points.push({ clientX: touches[index].clientX });
  return points;
}

function plotBounds(element: HTMLElement): TouchBounds {
  const plot = element.querySelector<SVGGraphicsElement>(".recharts-cartesian-grid");
  const rect = plot?.getBoundingClientRect() ?? element.getBoundingClientRect();
  return { left: rect.left, width: rect.width };
}

function eventTimestamp(event: unknown) {
  if (!event || typeof event !== "object" || !("activeLabel" in event)) return undefined;
  const value = Number((event as { activeLabel?: unknown }).activeLabel);
  return Number.isFinite(value) ? value : undefined;
}
