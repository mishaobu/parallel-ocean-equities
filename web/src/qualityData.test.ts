import { describe, expect, it } from "vitest";
import { formatQuality, qualityRows } from "./qualityData";

describe("quality metrics", () => {
  it("includes every operating-quality measure", () => {
    expect(qualityRows).toHaveLength(13);
    expect(qualityRows.map((row) => row.label)).toContain("Cash conversion cycle");
    expect(qualityRows.map((row) => row.label)).toContain("Incremental ROIC");
  });

  it("formats percentages, days, and multiples", () => {
    expect(formatQuality(0.153, "percent")).toBe("15.3%");
    expect(formatQuality(42.48, "days")).toBe("42.5d");
    expect(formatQuality(1.234, "multiple")).toBe("1.23x");
  });
});
