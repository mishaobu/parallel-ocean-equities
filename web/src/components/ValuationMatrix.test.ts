import { describe, expect, it } from "vitest";
import type { Equity } from "../types";
import { sortValuationEquities } from "./ValuationMatrix";

const equities = [
  { ticker: "AAA", status: "ready", annuals: [], current: {}, valuation: { pe: 20, forwardPe: 12 } },
  { ticker: "BBB", status: "ready", annuals: [], current: {}, valuation: { pe: 10, forwardPe: 16 } },
  { ticker: "CCC", status: "ready", annuals: [], current: {}, valuation: {} },
] satisfies Equity[];

describe("valuation table sorting", () => {
  it("sorts numeric metrics in either direction with missing values last", () => {
    expect(sortValuationEquities(equities, { key: "pe", basis: "actual", direction: "desc" }).map((equity) => equity.ticker)).toEqual(["AAA", "BBB", "CCC"]);
    expect(sortValuationEquities(equities, { key: "pe", basis: "actual", direction: "asc" }).map((equity) => equity.ticker)).toEqual(["BBB", "AAA", "CCC"]);
  });

  it("sorts model columns independently", () => {
    expect(sortValuationEquities(equities, { key: "pe", basis: "forward", direction: "desc" }).map((equity) => equity.ticker)).toEqual(["BBB", "AAA", "CCC"]);
  });
});
