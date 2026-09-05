#!/usr/bin/env node
/**
 * Regenerates the Copilot model picker from the models.dev github-copilot
 * catalog and writes cli/nav-pilot/internal/domain/known_models_gen.go.
 *
 * Why a generated Go file rather than a data file the picker reads at runtime:
 * the picker IS this table once it compiles in, so it cannot drift from what
 * ships, and a stale commit is caught loudly by `--check` diffing a fresh
 * generation. A runtime-read JSON would add a load-and-parse path that can go
 * missing, read stale bytes, or fail in the field, and would need its own
 * validation for no gain. The table has none of that surface.
 *
 * Usage:
 *   node scripts/sync-copilot-models.mjs          # regenerate the Go file
 *   node scripts/sync-copilot-models.mjs --check  # CI mode, writes nothing
 *
 * Exit codes mirror sync-model-pricing.mjs: 0 up to date, 2 the catalog moved
 * (the file would change), 1 the check could not be made (fetch failed, the
 * feed was malformed, or the catalog came back below the floor). 2 is kept
 * separate from 1 so a caller can act on real drift without also acting on a
 * check that never got an answer. A generator that emits an empty or shrunken
 * list on a bad feed silently shrinks the picker; failing loudly is the
 * no-silent-drop rule applied to the generator itself.
 */

import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";

const CATALOG_URL = "https://models.dev/api.json";
const PROVIDER_ID = "github-copilot";
const GO_FILE = new URL(
  "../cli/nav-pilot/internal/domain/known_models_gen.go",
  import.meta.url,
);
const PRICING_FILE = new URL(
  "../apps/my-copilot/src/lib/model-pricing.ts",
  import.meta.url,
);

// Pinned entries that are never in the models.dev catalog but must stay in the
// picker. `auto` is Copilot's server-side pseudo-model. Delisted-but-working
// models are retained here on purpose: dropping a model that still launches is
// the exact picker-drift bug. A model leaves the picker only by leaving the
// catalog AND not being pinned here, which is an explicit human edit, never a
// silent catalog drop. Keep this list short and justify every entry.
const PINNED = [
  { id: "auto", label: "Auto (let Copilot pick)" },
  // Delisted from GitHub's price list 2026-09-05 but still launches; see
  // docs/modellvalg.md. Remove once it stops resolving at launch.
  { id: "claude-opus-4.6", label: "Claude Opus 4.6" },
];

// A real github-copilot catalog carries dozens of models. Anything below this
// is a broken or truncated feed, not a real shrink, and must not overwrite the
// table. Checked against the catalog count alone, before PINNED is merged in,
// so the pinned entries can never make an empty feed look populated.
const MIN_CATALOG_MODELS = 5;

/** Extract {id,label} for the github-copilot provider. Throws on a bad feed. */
function parseCatalog(catalog) {
  if (!catalog || typeof catalog !== "object") {
    throw new Error("catalog is not an object");
  }
  const provider = catalog[PROVIDER_ID];
  if (!provider || typeof provider !== "object") {
    throw new Error(`no ${PROVIDER_ID} provider in the catalog`);
  }
  const models = provider.models;
  if (!models || typeof models !== "object") {
    throw new Error(`${PROVIDER_ID} provider has no models object`);
  }
  const entries = [];
  for (const [id, model] of Object.entries(models)) {
    if (typeof id !== "string" || id === "") continue;
    const label =
      model && typeof model.name === "string" && model.name.trim()
        ? model.name.trim()
        : id;
    entries.push({ id, label });
  }
  return entries;
}

/**
 * Merge the catalog with PINNED and sort by id. Catalog labels win over a
 * pinned label for the same id, so a model that returns to the catalog keeps
 * its fresh name. Throws if the catalog is below the floor, so a broken feed
 * cannot shrink the picker to just the pinned entries.
 */
function buildTable(catalogEntries) {
  if (catalogEntries.length < MIN_CATALOG_MODELS) {
    throw new Error(
      `catalog returned ${catalogEntries.length} models, below the floor of ${MIN_CATALOG_MODELS}; refusing to shrink the picker`,
    );
  }
  const byId = new Map();
  for (const p of PINNED) byId.set(p.id, { id: p.id, label: p.label });
  for (const e of catalogEntries) byId.set(e.id, { id: e.id, label: e.label });
  return [...byId.values()].sort((a, b) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
}

/** Slugify a pricing display name to a candidate catalog id. */
function priceNameToId(name) {
  return name
    .split(" (")[0] // drop variant / status suffixes: "GPT-5.4 (Default, ...)"
    .trim()
    .toLowerCase()
    .replace(/\s+/g, "-");
}

/** Candidate ids derived from the pricing file's model display names. */
function parsePricingIds(tsSource) {
  const ids = new Set();
  for (const [, name] of tsSource.matchAll(/^\s+model: "([^"]+)",/gm)) {
    ids.add(priceNameToId(name));
  }
  return ids;
}

/**
 * Reconcile the catalog with pricing by id, for cost display. The two sets
 * legitimately diverge, so this surfaces the difference in explicit buckets
 * rather than silently dropping either side. Pinned ids are reported on their
 * own line because they are intentionally not catalog-derived.
 */
function reconcile(catalogIds, pricedIds) {
  const pinnedIds = new Set(PINNED.map((p) => p.id));
  const catalog = new Set(catalogIds);
  const matched = [];
  const catalogNotPriced = [];
  for (const id of catalogIds) {
    if (pricedIds.has(id)) matched.push(id);
    else catalogNotPriced.push(id);
  }
  const pricedNotInCatalog = [...pricedIds].filter(
    (id) => !catalog.has(id) && !pinnedIds.has(id),
  );
  return {
    matched: matched.sort(),
    catalogNotPriced: catalogNotPriced.sort(),
    pricedNotInCatalog: pricedNotInCatalog.sort(),
    pinned: [...pinnedIds].sort(),
  };
}

function printReconciliation(rec) {
  console.log(`\nReconciliation with pricing (by id, for cost display):`);
  console.log(`  matched (catalog id has a price): ${rec.matched.length}`);
  console.log(
    `  catalog, not priced (no cost shown): ${rec.catalogNotPriced.length ? rec.catalogNotPriced.join(", ") : "none"}`,
  );
  console.log(
    `  priced, not in catalog (price with no picker entry): ${rec.pricedNotInCatalog.length ? rec.pricedNotInCatalog.join(", ") : "none"}`,
  );
  console.log(`  pinned (not catalog-derived): ${rec.pinned.join(", ")}`);
}

function generateGo(table) {
  let entries = "";
  for (const m of table) {
    entries += `\t{ID: ${JSON.stringify(m.id)}, Label: ${JSON.stringify(m.label)}},\n`;
  }
  return `// Code generated by scripts/sync-copilot-models.mjs; DO NOT EDIT.
// Regenerate with: mise run models:sync  (source: models.dev ${PROVIDER_ID} provider)

package domain

// KnownCopilotModels is the curated Copilot model list, generated from the
// models.dev ${PROVIDER_ID} catalog plus a short pinned set (see the generator's
// PINNED list: the "auto" pseudo-model and delisted-but-working models kept on
// purpose).
//
// It lives in domain rather than internal/provider because two packages need
// the same pairing and cannot import each other: provider builds the launch
// flags and the pickers from it, and internal/artifacts resolves an agent's
// frontmatter model label against it while materializing opencode agents.
// provider imports artifacts, so domain, which both import, is the only home
// that keeps one table instead of two.
var KnownCopilotModels = []ModelChoice{
${entries}}
`;
}

async function fetchCatalog() {
  const res = await fetch(CATALOG_URL);
  if (!res.ok) throw new Error(`HTTP ${res.status} fetching ${CATALOG_URL}`);
  return res.json();
}

async function main() {
  const checkOnly = process.argv.includes("--check");

  console.log(`Fetching model catalog from ${CATALOG_URL} ...`);
  const catalog = await fetchCatalog();
  const catalogEntries = parseCatalog(catalog);
  const table = buildTable(catalogEntries);
  const newContent = generateGo(table);

  const goPath = fileURLToPath(GO_FILE);

  if (checkOnly) {
    const current = readFileSync(goPath, "utf-8");
    if (current !== newContent) {
      console.error("\nERROR: known_models_gen.go is out of date!");
      console.error("Run: mise run models:sync");
      // 2, not 1: the catalog really moved. Every other non-zero exit here is
      // a check that failed to run, and a caller must not confuse the two.
      process.exit(2);
    }
    console.log("\n✓ known_models_gen.go is up to date");
    return;
  }

  writeFileSync(goPath, newContent, "utf-8");
  console.log(`\n✓ Wrote ${table.length} models to ${goPath}`);

  const pricedIds = parsePricingIds(readFileSync(fileURLToPath(PRICING_FILE), "utf-8"));
  const catalogIds = catalogEntries.map((e) => e.id).sort();
  printReconciliation(reconcile(catalogIds, pricedIds));
}

export {
  parseCatalog,
  buildTable,
  priceNameToId,
  parsePricingIds,
  reconcile,
  generateGo,
  MIN_CATALOG_MODELS,
};

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((err) => {
    console.error("Failed:", err.message);
    process.exit(1);
  });
}
