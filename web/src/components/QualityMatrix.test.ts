import { describe, expect, it } from "vitest";
import type { Equity } from "../types";
import { sortQualityEquities } from "./QualityMatrix";

const equities = [
  { ticker: "AAA", status: "ready", annuals: [], current: {}, quality: { roic: 0.2 } },
  { ticker: "BBB", status: "ready", annuals: [], current: {}, quality: { roic: 0.1 } },
  { ticker: "CCC", status: "ready", annuals: [], current: {}, quality: {} },
] satisfies Equity[];

describe("quality table sorting", () => {
  it("sorts any quality column with missing values last", () => {
    expect(sortQualityEquities(equities, { key: "roic", direction: "desc" }).map((row) => row.ticker)).toEqual(["AAA", "BBB", "CCC"]);
    expect(sortQualityEquities(equities, { key: "roic", direction: "asc" }).map((row) => row.ticker)).toEqual(["BBB", "AAA", "CCC"]);
  });
});
