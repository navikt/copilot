// Shape of docs/golden-baselines/summary.json and pure helpers over it, shared
// with the client. Reading the file is server only: see golden-summary-file.ts.
import { MODEL_PRICING } from "./model-pricing";
import { normalizeModelName } from "./model-policy";

export type Suite = "planning" | "review" | "norsk" | "coding" | "research" | "kafka" | "rust";
export type Effort = "low" | "medium" | "high" | "default";

export interface GoldenRun {
  date: string;
  suite: Suite;
  /** Model id as the CLI takes it, e.g. gpt-6-sol. */
  model: string;
  /** The effort that was requested. */
  effort: Effort;
  /** The effort the run was observed at, when known. */
  ran_at?: string;
  smoke?: boolean;
  cli_version: string;
  n: number;
  /** passed is how many of the n runs passed the check. */
  checks: { id: string; description: string; passed: number }[];
  /** null when usage was not recorded. */
  credits: { median: number; mean: number } | null;
  /** false when some usage events are missing, so credits undercount. */
  usage_complete?: boolean;
  /** false when the run had no usage rows, so the model that answered is unknown. */
  model_verified?: boolean;
  /** Other model ids that subagents used during the run. */
  subagent_models?: string[];
  wall_seconds: { median: number };
  source: string;
}

export interface GoldenSummary {
  generated: string;
  runs: GoldenRun[];
}

const REPO_BLOB = "https://github.com/navikt/copilot/blob/main/";

const slug = (name: string) => name.trim().toLowerCase().replace(/\s+/g, "-");
// The pricing catalogue names every model Nav can use; the ids the CLI takes are those names slugged.
const DISPLAY_NAMES = new Map(
  MODEL_PRICING.map((m) => normalizeModelName(m.model)).map((name) => [slug(name), name] as const)
);

/** Display name for a model id, from the pricing catalogue; the raw id when the catalogue does not have it. */
export function modelName(id: string): string {
  return DISPLAY_NAMES.get(slug(id)) ?? id;
}

/** Median credits, or null when usage is missing, incomplete or the model unverified: an undercount is not a number. */
export function knownCredits(run: GoldenRun): number | null {
  return run.credits && run.usage_complete !== false && run.model_verified !== false ? run.credits.median : null;
}

/** Share of all check outcomes that passed: sum(passed) / (n × checks). */
export function passRate(run: GoldenRun): number {
  const total = run.n * run.checks.length;
  if (total === 0) return 0;
  return run.checks.reduce((sum, c) => sum + c.passed, 0) / total;
}

/** Wilson 95 % interval for k successes out of n, as shares in [0, 1]. */
export function wilson(k: number, n: number, z = 1.96): [number, number] {
  if (n === 0) return [0, 1];
  const p = k / n;
  const z2 = z * z;
  const mid = (p + z2 / (2 * n)) / (1 + z2 / n);
  const half = (z / (1 + z2 / n)) * Math.sqrt((p * (1 - p)) / n + z2 / (4 * n * n));
  return [Math.max(0, mid - half), Math.min(1, mid + half)];
}

/** Wilson interval for passRate, over all n × checks outcomes. */
export function passInterval(run: GoldenRun): [number, number] {
  const passed = run.checks.reduce((sum, c) => sum + c.passed, 0);
  return wilson(passed, run.n * run.checks.length);
}

/** pass^k: share of checks that passed in every one of the n repeats. */
export function passAll(run: GoldenRun): number {
  return run.checks.length === 0 ? 0 : run.checks.filter((c) => c.passed === run.n).length / run.checks.length;
}

/** Median credits divided by pass rate; null when credits are unknown or nothing passed. */
export function creditsPerSolved(run: GoldenRun): number | null {
  const credits = knownCredits(run);
  const rate = passRate(run);
  return credits === null || rate === 0 ? null : credits / rate;
}

export function sourceUrl(source: string): string {
  return /^https?:\/\//.test(source) ? source : REPO_BLOB + source.replace(/^\/+/, "");
}

export function runsBySuite(runs: GoldenRun[]): [Suite, GoldenRun[]][] {
  const bySuite = new Map<Suite, GoldenRun[]>();
  for (const run of runs) bySuite.set(run.suite, [...(bySuite.get(run.suite) ?? []), run]);
  return [...bySuite];
}
