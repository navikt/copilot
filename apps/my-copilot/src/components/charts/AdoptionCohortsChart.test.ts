import { describe, it, expect } from "vitest";
import { transformCohortData } from "./AdoptionCohortsChart";

describe("transformCohortData", () => {
  it("groups user counts by day and phase", () => {
    const input = [
      {
        day: "2026-05-28",
        phase: 0,
        phase_version: "v1",
        user_count: 50,
        avg_generations: 10,
        avg_acceptances: 5,
        avg_interactions: 3,
        avg_lines_added: 20,
      },
      {
        day: "2026-05-28",
        phase: 1,
        phase_version: "v1",
        user_count: 200,
        avg_generations: 40,
        avg_acceptances: 20,
        avg_interactions: 10,
        avg_lines_added: 80,
      },
      {
        day: "2026-05-28",
        phase: 2,
        phase_version: "v1",
        user_count: 100,
        avg_generations: 60,
        avg_acceptances: 30,
        avg_interactions: 25,
        avg_lines_added: 150,
      },
      {
        day: "2026-05-28",
        phase: 3,
        phase_version: "v1",
        user_count: 30,
        avg_generations: 80,
        avg_acceptances: 40,
        avg_interactions: 50,
        avg_lines_added: 200,
      },
      {
        day: "2026-05-29",
        phase: 1,
        phase_version: "v1",
        user_count: 210,
        avg_generations: 42,
        avg_acceptances: 21,
        avg_interactions: 11,
        avg_lines_added: 85,
      },
      {
        day: "2026-05-29",
        phase: 2,
        phase_version: "v1",
        user_count: 110,
        avg_generations: 62,
        avg_acceptances: 31,
        avg_interactions: 26,
        avg_lines_added: 155,
      },
      {
        day: "2026-05-29",
        phase: 3,
        phase_version: "v1",
        user_count: 35,
        avg_generations: 85,
        avg_acceptances: 42,
        avg_interactions: 55,
        avg_lines_added: 210,
      },
    ];

    const result = transformCohortData(input);

    expect(result.days).toEqual(["2026-05-28", "2026-05-29"]);
    expect(result.phase0).toEqual([50, 0]);
    expect(result.phase1).toEqual([200, 210]);
    expect(result.phase2).toEqual([100, 110]);
    expect(result.phase3).toEqual([30, 35]);
    expect(result.total).toEqual([380, 355]);
  });

  it("returns empty arrays for empty input", () => {
    const result = transformCohortData([]);
    expect(result.days).toEqual([]);
    expect(result.phase1).toEqual([]);
  });

  it("sorts days chronologically", () => {
    const input = [
      {
        day: "2026-05-30",
        phase: 1,
        phase_version: "v1",
        user_count: 5,
        avg_generations: 1,
        avg_acceptances: 1,
        avg_interactions: 1,
        avg_lines_added: 1,
      },
      {
        day: "2026-05-28",
        phase: 1,
        phase_version: "v1",
        user_count: 3,
        avg_generations: 1,
        avg_acceptances: 1,
        avg_interactions: 1,
        avg_lines_added: 1,
      },
    ];

    const result = transformCohortData(input);
    expect(result.days).toEqual(["2026-05-28", "2026-05-30"]);
    expect(result.phase1).toEqual([3, 5]);
  });
});
