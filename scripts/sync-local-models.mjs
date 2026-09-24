#!/usr/bin/env node
/**
 * Regenerates the local-model table on ki-utvikling.nav.no from the
 * navikt/mlx-workspace manifest, the same manifest nav-pilot reads at
 * `alpha local init` and `start`. Writes apps/my-copilot/src/lib/local-models.json,
 * which the nav-pilot docs page renders.
 *
 * Only the fields the page shows are kept: names, default, context and reply
 * size, memory, weights, minimum nav-pilot version, sampling, and the
 * delegate/local verdict per task class. The per-class run counts are left out
 * so that a benchmark rerun that changes no verdict does not open a PR.
 *
 * The manifest's `key` is written as `id`, because gitleaks reads
 * `"key": "<slug>"` as an API key. The page keeps its Norwegian
 * descriptions keyed by `id`; every number on it comes from this file.
 *
 * Usage:
 *   node scripts/sync-local-models.mjs          # regenerate the JSON file
 *   node scripts/sync-local-models.mjs --check  # CI mode, writes nothing
 *
 * Exit codes as in sync-copilot-models.mjs: 0 up to date, 2 the manifest moved,
 * 1 the check could not be made (fetch failed or the manifest was malformed).
 */

import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";

const MANIFEST_URL = "https://raw.githubusercontent.com/navikt/mlx-workspace/main/manifest/models.json";
const OUT_FILE = new URL("../apps/my-copilot/src/lib/local-models.json", import.meta.url);

function isPlainObject(v) {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

// nav-pilot's release-version format (cli/nav-pilot/internal/agentpakke/version.go).
const RELEASE_VERSION = /^\d{4}\.\d{2}\.\d{2}-\d{6}(-[0-9a-zA-Z][0-9a-zA-Z.-]*)?$/;

/** A manifest param as a number, or null when unset. Params are strings; anything else throws. */
function num(params, name) {
  const v = params[name];
  if (v === undefined) return null;
  const n = typeof v === "string" && v.trim() !== "" ? Number(v) : NaN;
  if (!Number.isFinite(n)) throw new Error(`param ${name}=${JSON.stringify(v)} is not a number string`);
  return n;
}

/** A required positive integer field. */
function int(e, name) {
  const v = e[name];
  if (!Number.isInteger(v) || v <= 0) throw new Error(`${e.key} has no valid ${name}`);
  return v;
}

/**
 * min_nav_pilot: null when omitted. nav-pilot withholds an entry whose value is
 * present but unreadable, so the table must not show it as available: throw.
 */
function minVersion(e) {
  if (!("min_nav_pilot" in e)) return null;
  const v = e.min_nav_pilot;
  if (typeof v !== "string" || !RELEASE_VERSION.test(v)) {
    throw new Error(`${e.key} has an unreadable min_nav_pilot ${JSON.stringify(v)}`);
  }
  return v;
}

/** Project one manifest entry onto what the page renders. Throws on a bad entry. */
function projectEntry(e) {
  if (!isPlainObject(e)) throw new Error("model entry is not an object");
  for (const f of ["key", "name", "model", "role"]) {
    if (typeof e[f] !== "string" || !e[f]) throw new Error(`model entry lacks ${f}`);
  }
  const params = isPlainObject(e.params) ? e.params : {};
  const context = num(params, "MLX_OPENCODE_CONTEXT");
  const output = num(params, "MLX_OPENCODE_OUTPUT");
  if (context === null || output === null) {
    throw new Error(`${e.key} has no MLX_OPENCODE_CONTEXT/OUTPUT`);
  }
  const classes = {};
  const raw = isPlainObject(e.capabilities) && isPlainObject(e.capabilities.classes) ? e.capabilities.classes : {};
  for (const [id, c] of Object.entries(raw)) {
    if (!isPlainObject(c)) continue;
    classes[id] = { delegate: String(c.delegate ?? ""), local: String(c.local ?? "") };
  }
  return {
    id: e.key,
    name: e.name,
    model: e.model,
    default: e.default === true,
    role: e.role,
    weights_gb: int(e, "weights_gb"),
    min_ram_gb: int(e, "min_ram_gb"),
    min_nav_pilot: minVersion(e),
    context,
    output,
    temperature: num(params, "MLX_NAV_PILOT_TEMPERATURE"),
    top_p: num(params, "MLX_NAV_PILOT_TOP_P"),
    prefill_step: num(params, "MLX_PREFILL_STEP_SIZE"),
    classes,
  };
}

/** Project the whole manifest. Refuses an empty list or one without exactly one default. */
function buildTable(manifest) {
  if (!isPlainObject(manifest) || !Array.isArray(manifest.models)) {
    throw new Error("manifest has no models list");
  }
  const models = manifest.models.map(projectEntry);
  if (models.length === 0) throw new Error("manifest lists no models; refusing to empty the table");
  const defaults = models.filter((m) => m.default).length;
  if (defaults !== 1) throw new Error(`manifest has ${defaults} default models, want 1`);
  return { source: MANIFEST_URL, models };
}

function render(table) {
  return JSON.stringify(table, null, 2) + "\n";
}

async function main() {
  const checkOnly = process.argv.includes("--check");
  const res = await fetch(MANIFEST_URL);
  if (!res.ok) throw new Error(`HTTP ${res.status} fetching ${MANIFEST_URL}`);
  const next = render(buildTable(await res.json()));
  const outPath = fileURLToPath(OUT_FILE);

  if (checkOnly) {
    if (readFileSync(outPath, "utf-8") !== next) {
      console.error("local-models.json is out of date. Run: mise run local-models:sync");
      process.exit(2);
    }
    console.log("✓ local-models.json is up to date");
    return;
  }
  writeFileSync(outPath, next, "utf-8");
  console.log(`✓ Wrote ${outPath}`);
}

export { buildTable, projectEntry, render };

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((err) => {
    console.error("Failed:", err.message);
    process.exit(1);
  });
}
