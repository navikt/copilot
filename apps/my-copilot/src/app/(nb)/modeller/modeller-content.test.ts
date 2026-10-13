import { describe, expect, it } from "vitest";
import type { GoldenRun } from "@/lib/golden-baselines";
import { formatDuration, latestRuns, partialRuns } from "./modeller-content";

describe("formatDuration", () => {
  it("shows seconds alone under a minute", () => {
    expect(formatDuration(48)).toBe("48 s");
    expect(formatDuration(59.4)).toBe("59 s");
  });
  it("shows minutes and seconds from a minute up", () => {
    expect(formatDuration(59.6)).toBe("1 min 0 s");
    expect(formatDuration(83)).toBe("1 min 23 s");
  });
});

describe("latestRuns", () => {
  const run = (date: string, checks = ["c1"], effort = "low") =>
    ({ date, model: "gpt-6-sol", effort, checks: checks.map((id) => ({ id })) }) as unknown as GoldenRun;
  it("keeps the newest run of each model, effort and check set, and same-day siblings", () => {
    const old = run("2026-09-30");
    const a = run("2026-10-07");
    const b = run("2026-10-07");
    const otherChecks = run("2026-09-30", ["rv1"]);
    const otherEffort = run("2026-09-01", ["c1"], "high");
    const { latest, older } = latestRuns([old, a, b, otherChecks, otherEffort]);
    expect(latest).toEqual([a, b, otherChecks, otherEffort]);
    expect(older).toEqual([old]);
  });
});

describe("partialRuns", () => {
  const run = (checks: string[], model = "gpt-6-sol", smoke = false) =>
    ({ model, effort: "low", smoke, checks: checks.map((id) => ({ id })) }) as unknown as GoldenRun;
  it("marks a run whose checks are a strict subset of another run of the same model and effort", () => {
    const full = run(["rv1", "rv2", "rv5", "rv7"]);
    const rerun = run(["rv1", "rv2"]);
    const otherModel = run(["rv1"], "gpt-6.1-sol");
    const disjoint = run(["rv9"]);
    const smoke = run(["rv1"], "gpt-6-sol", true);
    expect([...partialRuns([full, rerun, otherModel, disjoint, smoke])]).toEqual([rerun]);
  });
});
