#!/usr/bin/env node
/**
 * Refreshes the fallback copy of the local-model table on ki-utvikling.nav.no.
 *
 * /nav-pilot/referanse and /nav-pilot/forklaring/lokal-modell fetch
 * navikt/mlx-workspace's manifest, capabilities.json and reports.json at run time
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
  CAPABILITIES_URL,
  MANIFEST_URL,
  REPORTS_URL,
  buildReports,
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
  const [manifest, capabilities, reports] = await Promise.all(
    [MANIFEST_URL, CAPABILITIES_URL, REPORTS_URL].map(async (url) => {
      const res = await fetch(url);
      if (!res.ok) throw new Error(`HTTP ${res.status} fetching ${url}`);
      return res.json();
    }),
  );
  const next = render({
    ...buildTable(manifest, capabilities),
    reports: buildReports(reports),
  });
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
