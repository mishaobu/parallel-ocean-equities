import { describe, expect, it } from "vitest";
import { fittedYDomain, touchDomainRange } from "./chartInteraction";

describe("interactive chart domains", () => {
  it("fits the selected x-axis window", () => {
    const rows = [{ x: 1, value: 100 }, { x: 2, value: 10 }, { x: 3, value: 20 }];
    const result = fittedYDomain(rows, [2, 3], ["value"], "x");
    expect(result.domain[0]).toBeLessThan(10);
    expect(result.domain[1]).toBeGreaterThan(20);
  });

  it("clips an isolated extreme point that would flatten the chart", () => {
    const rows = Array.from({ length: 20 }, (_, index) => ({ x: index, value: index === 10 ? 1000 : 10+index/10 }));
    const result = fittedYDomain(rows, [0, 20], ["value"], "x");
    expect(result.clipped).toBe(true);
    expect(Number(result.domain[1])).toBeLessThan(20);
  });

  it("keeps a sustained compounding trend in view", () => {
    const rows = Array.from({ length: 20 }, (_, index) => ({ x: index, value: Math.pow(1.3, index) }));
    const result = fittedYDomain(rows, [0, 20], ["value"], "x", { log: true });
    expect(result.clipped).toBe(false);
    expect(Number(result.domain[1])).toBeGreaterThan(rows[19].value);
  });
});

describe("touch chart gestures", () => {
  it("maps two touches to an ordered range in the chart domain", () => {
    expect(touchDomainRange([0, 100], [{ clientX: 175 }, { clientX: 125 }], { left: 100, width: 100 })).toEqual([25, 75]);
  });

  it("clamps touches to the plotted x-axis", () => {
    expect(touchDomainRange([10, 20], [{ clientX: 50 }, { clientX: 250 }], { left: 100, width: 100 })).toEqual([10, 20]);
  });

  it("does not start a selection from one finger", () => {
    expect(touchDomainRange([0, 100], [{ clientX: 150 }], { left: 100, width: 100 })).toBeUndefined();
  });
});
