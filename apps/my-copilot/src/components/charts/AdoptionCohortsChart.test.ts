import { describe, it, expect } from "vitest";
import { transformCohortData } from "./AdoptionCohortsChart";

describe("transformCohortData", () => {
  it("pivots weekly rows into one series per phase, sorted by week", () => {
    const result = transformCohortData([
      { week: "2026-09-14", phase: 1, user_count: 210 },
      { week: "2026-09-07", phase: 0, user_count: 50 },
      { week: "2026-09-07", phase: 1, user_count: 200 },
      { week: "2026-09-07", phase: 2, user_count: 100 },
      { week: "2026-09-07", phase: 3, user_count: 30 },
      { week: "2026-09-14", phase: 3, user_count: 35 },
    ]);
    expect(result.weeks).toEqual(["2026-09-07", "2026-09-14"]);
    expect(result.phase0).toEqual([50, null]);
    expect(result.phase1).toEqual([200, 210]);
    expect(result.phase2).toEqual([100, null]);
    expect(result.phase3).toEqual([30, 35]);
  });

  it("returns empty series for empty input", () => {
    expect(transformCohortData([])).toEqual({ weeks: [], phase0: [], phase1: [], phase2: [], phase3: [] });
  });
});
