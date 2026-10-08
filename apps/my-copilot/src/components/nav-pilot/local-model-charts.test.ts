import { describe, expect, it } from "vitest";
import { wilson } from "./local-model-charts";

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
