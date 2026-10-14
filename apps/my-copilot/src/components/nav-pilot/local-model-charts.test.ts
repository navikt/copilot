import { describe, expect, it } from "vitest";
import { FALLBACK_TABLE } from "@/lib/local-models";
import { modelSizeRows, wilson } from "./local-model-charts";

describe("wilson", () => {
  // The lower bounds capabilities.json reports for the default model (lb).
  it.each([
    [27, 27, 0.943],
    [11, 18, 0.461],
    [3, 3, 0.646],
    [3, 18, 0.083],
  ])("%i of %i has the manifest's lower bound %f", (k, n, lb) => {
    const [lo, hi] = wilson(k, n, 0.9);
    expect(lo).toBeCloseTo(lb, 3);
    expect(hi).toBeLessThanOrEqual(1);
    expect(hi).toBeGreaterThan(k / n - 1e-9);
  });
});

describe("modelSizeRows", () => {
  it("sorts models by weights_gb ascending and aggregates verified runs with Wilson intervals", () => {
    const rows = modelSizeRows(FALLBACK_TABLE.models);
    expect(rows.length).toBe(FALLBACK_TABLE.models.length);
    for (let i = 1; i < rows.length; i++) {
      expect(rows[i].weightsGb).toBeGreaterThanOrEqual(rows[i - 1].weightsGb);
    }
    const def = rows.find((r) => r.isDefault)!;
    expect(def.weightsGb).toBe(25);
    expect(def.n).toBeGreaterThan(0);
    expect(def.lo).toBeGreaterThan(0);
    expect(def.hi).toBeLessThanOrEqual(1);
    expect(def.trusted).toContain("mekanisk endring over flere filer (delegert)");
  });
});
