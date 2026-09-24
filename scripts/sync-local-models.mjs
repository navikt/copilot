#!/usr/bin/env node
/**
 * Refreshes the fallback copy of the local-model table on ki-utvikling.nav.no.
 *
 * The nav-pilot docs page fetches navikt/mlx-workspace's manifest at run time
 * (apps/my-copilot/src/lib/local-models.ts, revalidated hourly). When that fetch
 * fails or the manifest does not validate, it renders
 * apps/my-copilot/src/lib/local-models.json instead, which this script writes.
 * Validation and mapping live in apps/my-copilot/src/lib/local-models-manifest.ts,
 * shared with the page, and are tested there with vitest.
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

import {
  MANIFEST_URL,
  buildTable,
} from "../apps/my-copilot/src/lib/local-models-manifest.ts";

const OUT_FILE = new URL(
  "../apps/my-copilot/src/lib/local-models.json",
  import.meta.url,
);

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
      console.error(
        "local-models.json is out of date. Run: mise run local-models:sync",
      );
      process.exit(2);
    }
    console.log("✓ local-models.json is up to date");
    return;
  }
  writeFileSync(outPath, next, "utf-8");
  console.log(`✓ Wrote ${outPath}`);
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  main().catch((err) => {
    console.error("Failed:", err.message);
    process.exit(1);
  });
}
