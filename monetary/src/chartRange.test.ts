import { describe, expect, it } from "vitest";
import { touchDomainRange } from "./chartRange";

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
