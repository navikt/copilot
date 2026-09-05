#!/usr/bin/env node
/**
 * Tests for the catalog sync in sync-copilot-models.mjs.
 *
 * Usage: node --test scripts/sync-copilot-models.test.mjs
 *
 * The mutation-check is the point of this file: feed the generator garbage and
 * it must error, never quietly emit an empty or shrunken table. A generator
 * that shrinks the picker on a bad feed is the no-silent-drop rule broken.
 */

import { test } from "node:test";
import assert from "node:assert/strict";

import {
  parseCatalog,
  buildTable,
  priceNameToId,
  parsePricingIds,
  reconcile,
  generateGo,
  MIN_CATALOG_MODELS,
} from "./sync-copilot-models.mjs";

// A trimmed but real-shaped github-copilot feed. `no-name` has no name field,
// so its label must fall back to the id.
const FEED = {
  "github-copilot": {
    id: "github-copilot",
    models: {
      "gpt-5.5": { id: "gpt-5.5", name: "GPT-5.5" },
      "claude-opus-4.8": { id: "claude-opus-4.8", name: "Claude Opus 4.8" },
      "claude-sonnet-4.6": { id: "claude-sonnet-4.6", name: "Claude Sonnet 4.6" },
      "gemini-3.6-flash": { id: "gemini-3.6-flash", name: "Gemini 3.6 Flash" },
      "grok-4.6": { id: "grok-4.6", name: "Grok 4.6" },
      "no-name": { id: "no-name" },
    },
  },
};

test("parseCatalog reads ids and labels, falling back to id for a nameless model", () => {
  const entries = parseCatalog(FEED);
  const byId = Object.fromEntries(entries.map((e) => [e.id, e.label]));
  assert.equal(byId["gpt-5.5"], "GPT-5.5");
  assert.equal(byId["no-name"], "no-name");
  assert.equal(entries.length, 6);
});

test("buildTable is sorted, non-empty, and includes the pinned entries", () => {
  const table = buildTable(parseCatalog(FEED));
  const ids = table.map((m) => m.id);
  assert.ok(ids.includes("auto"), "auto must be pinned in");
  assert.ok(ids.includes("claude-opus-4.6"), "delisted-but-working model must be retained");
  assert.ok(ids.includes("gpt-5.5"), "catalog model must be present");
  assert.deepEqual(ids, [...ids].sort(), "table must be sorted by id");
  assert.ok(table.length > 0);
});

test("buildTable is stable: same feed produces the same table", () => {
  const a = buildTable(parseCatalog(FEED));
  const b = buildTable(parseCatalog(FEED));
  assert.deepEqual(a, b);
  assert.equal(generateGo(a), generateGo(b));
});

test("generateGo emits a DO NOT EDIT header and one line per model", () => {
  const go = generateGo(buildTable(parseCatalog(FEED)));
  assert.match(go, /DO NOT EDIT/);
  assert.match(go, /var KnownCopilotModels = \[\]ModelChoice\{/);
  assert.match(go, /\{ID: "gpt-5.5", Label: "GPT-5.5"\},/);
});

// --- Mutation-checks: a bad feed must error, not empty the list. ---

test("an empty catalog throws rather than emitting a table", () => {
  assert.throws(() => buildTable(parseCatalog({ "github-copilot": { models: {} } })), /floor/);
});

test("a catalog below the floor throws rather than shrinking the picker", () => {
  const models = {};
  for (let i = 0; i < MIN_CATALOG_MODELS - 1; i++) models[`m-${i}`] = { id: `m-${i}` };
  assert.throws(() => buildTable(parseCatalog({ "github-copilot": { models } })), /floor/);
});

test("a malformed feed (no provider / no models) throws", () => {
  assert.throws(() => parseCatalog(null), /not an object/);
  assert.throws(() => parseCatalog({}), /no github-copilot provider/);
  assert.throws(() => parseCatalog({ "github-copilot": {} }), /no models object/);
  assert.throws(() => parseCatalog("garbage"), /not an object/);
});

// An array is a `typeof "object"`, so every shape guard has to reject it by
// hand. A list of models would otherwise be keyed by position and produce a
// picker of "0", "1", "2" — past the floor as soon as the list is long enough.
test("an array-shaped feed throws at every level rather than yielding numeric ids", () => {
  assert.throws(() => parseCatalog([]), /not an object/);
  assert.throws(() => parseCatalog({ "github-copilot": [] }), /no github-copilot provider/);
  assert.throws(
    () => parseCatalog({ "github-copilot": { models: [] } }),
    /no models object/,
  );
  const listed = Array.from({ length: MIN_CATALOG_MODELS + 1 }, (_, i) => ({
    id: `m-${i}`,
    name: `M ${i}`,
  }));
  assert.throws(
    () => parseCatalog({ "github-copilot": { models: listed } }),
    /no models object/,
    "a long enough list would otherwise clear the floor with positional ids",
  );
});

// A model entry that is not an object must fall back to its id, not throw and
// not stringify something odd into the label.
test("a non-object model entry falls back to the id as its label", () => {
  const entries = parseCatalog({
    "github-copilot": { models: { "odd-one": ["GPT-5.5"], "nully": null } },
  });
  assert.deepEqual(entries, [
    { id: "odd-one", label: "odd-one" },
    { id: "nully", label: "nully" },
  ]);
});

// --- Reconciliation ---

test("priceNameToId slugifies display names to catalog ids", () => {
  assert.equal(priceNameToId("GPT-5.4 (Default, ≤ 272K)"), "gpt-5.4");
  assert.equal(priceNameToId("Claude Opus 4.8 (fast mode) (preview)"), "claude-opus-4.8");
  assert.equal(priceNameToId("Kimi K2.7 Code"), "kimi-k2.7-code");
  assert.equal(priceNameToId("GPT-5.3-Codex"), "gpt-5.3-codex");
});

test("reconcile surfaces divergence in explicit buckets, not silent drops", () => {
  const catalogIds = parseCatalog(FEED).map((e) => e.id);
  const priced = parsePricingIds(
    [
      '    model: "GPT-5.5",',
      '    model: "Claude Opus 4.8 (fast mode) (preview)",',
      '    model: "Claude Sonnet 4",', // priced, not in catalog
    ].join("\n"),
  );
  const rec = reconcile(catalogIds, priced);
  assert.ok(rec.matched.includes("gpt-5.5"));
  assert.ok(rec.matched.includes("claude-opus-4.8"));
  assert.ok(rec.pricedNotInCatalog.includes("claude-sonnet-4"));
  assert.ok(rec.catalogNotPriced.includes("grok-4.6"), "unpriced catalog model is surfaced, not dropped");
  assert.deepEqual(rec.pinned, ["auto", "claude-opus-4.6"]);
});
