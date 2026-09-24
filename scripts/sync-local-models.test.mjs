#!/usr/bin/env node
/**
 * Tests for sync-local-models.mjs. Usage: node --test scripts/sync-local-models.test.mjs
 *
 * A malformed manifest must fail the sync, never empty the table on the page.
 */

import { test } from "node:test";
import assert from "node:assert/strict";

import { buildTable } from "./sync-local-models.mjs";

const entry = (over = {}) => ({
  key: "m",
  name: "M",
  model: "org/M",
  role: "r",
  default: true,
  weights_gb: 25,
  min_ram_gb: 48,
  params: { MLX_OPENCODE_CONTEXT: "65536", MLX_OPENCODE_OUTPUT: "16384", MLX_NAV_PILOT_TEMPERATURE: "0.6" },
  capabilities: { classes: { debug: { delegate: "cloud", local: "cloud", local_k: 0, local_n: 3 } } },
  ...over,
});

test("projects the numbers the page shows and drops run counts", () => {
  const [m] = buildTable({ models: [entry({ min_nav_pilot: "2026.09.24-110317-3596754" })] }).models;
  assert.equal(m.context, 65536);
  assert.equal(m.output, 16384);
  assert.equal(m.temperature, 0.6);
  assert.equal(m.top_p, null);
  assert.equal(m.min_nav_pilot, "2026.09.24-110317-3596754");
  assert.deepEqual(m.classes, { debug: { delegate: "cloud", local: "cloud" } });
});

test("refuses a malformed manifest", () => {
  assert.throws(() => buildTable({}));
  assert.throws(() => buildTable({ models: [] }));
  assert.throws(() => buildTable({ models: [entry({ default: false })] }));
  assert.throws(() => buildTable({ models: [entry(), entry({ key: "n" })] }));
  assert.throws(() => buildTable({ models: [entry({ params: {} })] }));
  assert.throws(() => buildTable({ models: [entry({ params: { MLX_OPENCODE_CONTEXT: "x", MLX_OPENCODE_OUTPUT: "1" } })] }));
  assert.throws(() => buildTable({ models: [entry({ params: { MLX_OPENCODE_CONTEXT: null, MLX_OPENCODE_OUTPUT: "1" } })] }));
  assert.throws(() => buildTable({ models: [entry({ min_ram_gb: "48" })] }));
  assert.throws(() => buildTable({ models: [entry({ weights_gb: undefined })] }));
  assert.throws(() => buildTable({ models: [entry({ min_nav_pilot: null })] }));
  assert.throws(() => buildTable({ models: [entry({ min_nav_pilot: "soon" })] }));
});
