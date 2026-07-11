import { describe, expect, it } from "vitest";
import { formatBillions, formatValuation, quarterLabel, valuationRows } from "./valuationData";

describe("valuation presentation", () => {
  it("keeps the requested priority order", () => {
    expect(valuationRows.map((row) => row.label)).toEqual([
      "P/E",
      "EV / EBITDA",
      "EV / EBIT",
      "OCF / market cap",
      "FCF / market cap",
      "FCF / EV",
      "Net debt / EBITDA",
      "Dividend / FCF",
    ]);
  });

  it("formats valuation and quarterly values", () => {
    expect(formatValuation(22.345, "multiple")).toBe("22.3x");
    expect(formatValuation(0.0521, "yield")).toBe("5.2%");
    expect(formatValuation(-2, "multiple")).toBe("n/m");
    expect(formatValuation(-0.21, "leverage")).toBe("-0.2x");
    expect(formatBillions(-12.45)).toBe("-$12.5B");
    expect(quarterLabel({ fiscalYear: 2025, fiscalQuarter: "Q4", periodEnd: "2025-12-31", derived: true })).toBe("FY25 Q4*");
  });
});
