import fs from "node:fs";
import path from "node:path";
import type { GoldenSummary } from "./golden-baselines";

// docs/golden-baselines/summary.json at the repo root, written by the golden
// harness. Like the news articles it lives outside the app: the runner image
// copies it in (Dockerfile), and scripts/prepare-standalone.mjs does the same
// for a local `pnpm start`. Server only; the helpers in golden-baselines.ts are
// shared with the client.
const SUMMARY_REPO_PATH = "docs/golden-baselines/summary.json";

function summaryPath(): string {
  const local = path.join(process.cwd(), SUMMARY_REPO_PATH);
  return fs.existsSync(local) ? local : path.join(process.cwd(), "..", "..", SUMMARY_REPO_PATH);
}

/** The published summary, or null when there is none. Never falls back to made-up data. */
export function loadGoldenSummary(file = summaryPath()): GoldenSummary | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(fs.readFileSync(file, "utf-8"));
  } catch {
    return null;
  }
  const summary = parsed as GoldenSummary;
  if (!Array.isArray(summary?.runs) || summary.runs.length === 0) return null;
  return summary;
}
