// Shape of docs/golden-baselines/summary.json and pure helpers over it, shared
// with the client. Reading the file is server only: see golden-summary-file.ts.

export type Suite = "planning" | "review" | "norsk" | "coding";
export type Effort = "low" | "medium" | "high" | "default";

export interface GoldenRun {
  date: string;
  suite: Suite;
  model: string;
  label: string;
  effort: Effort;
  cli_version: string;
  n: number;
  /** Runs out of n that passed the check. The contract leaves the type open, so a boolean counts as all or none. */
  checks: { id: string; description: string; passed: number | boolean }[];
  credits: { median: number; mean: number };
  wall_seconds: { median: number };
  source: string;
}

export interface GoldenSummary {
  generated: string;
  runs: GoldenRun[];
}

const REPO_BLOB = "https://github.com/navikt/copilot/blob/main/";

/** Share of all check outcomes that passed: sum(passed) / (n × checks). */
export function passRate(run: GoldenRun): number {
  const total = run.n * run.checks.length;
  if (total === 0) return 0;
  return (
    run.checks.reduce((sum, c) => sum + (typeof c.passed === "boolean" ? (c.passed ? run.n : 0) : c.passed), 0) / total
  );
}

export function sourceUrl(source: string): string {
  return /^https?:\/\//.test(source) ? source : REPO_BLOB + source.replace(/^\/+/, "");
}

export function runsBySuite(runs: GoldenRun[]): [Suite, GoldenRun[]][] {
  const bySuite = new Map<Suite, GoldenRun[]>();
  for (const run of runs) bySuite.set(run.suite, [...(bySuite.get(run.suite) ?? []), run]);
  return [...bySuite];
}
