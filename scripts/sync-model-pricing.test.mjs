#!/usr/bin/env node
/**
 * Tests for the footnote parsing in sync-model-pricing.mjs.
 *
 * Usage: node --test scripts/sync-model-pricing.test.mjs
 *
 * The fixtures are copied verbatim from the live pricing page, so a change in
 * how GitHub marks up promotions shows up here as a failing test.
 */

import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  realpathSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import {
  parsePricingTables,
  parseFootnotes,
  parsePromotionEndDate,
  findUnresolvedPromotions,
  setDocPricingDate,
} from "./sync-model-pricing.mjs";

// The fixtures carry two provider tables, not all six, and the parser warns
// once per missing section. Silenced so a green `mise run check` stays green
// looking.
console.warn = () => {};

// Verbatim from docs.github.com, 2026-08-31.
const FOOTNOTES = `<section data-footnotes="" class="footnotes"><h2 class="sr-only" id="footnote-label" tabindex="-1">Footnotes</h2>
<ol>
<li id="user-content-fn-gpt-56-sol-promo">
<p>GPT-5.6 Sol is available at promotional pricing, 50% off standard rates, through September 3, 2026. The default tier is $2.00 per 1M input tokens, $0.20 per 1M cached input tokens, $2.50 per 1M cache write tokens, and $10.00 per 1M output tokens. The long context tier is $4.00 per 1M input tokens, $0.40 per 1M cached input tokens, $5.00 per 1M cache write tokens, and $15.00 per 1M output tokens. <a href="#user-content-fnref-gpt-56-sol-promo" data-footnote-backref="" aria-label="Back to reference 1" class="data-footnote-backref">↩</a> <a href="#user-content-fnref-gpt-56-sol-promo-2" data-footnote-backref="" aria-label="Back to reference 1-2" class="data-footnote-backref">↩<sup>2</sup></a></p>
</li>
<li id="user-content-fn-gemini-flash-promo">
<p>Gemini 3.6 Flash and Gemini 3.7 Flash are available at the promotional pricing of $0.75 per 1M input tokens, $0.075 per 1M cached input tokens, and $3.75 per 1M output tokens through December 31, 2026. <a href="#user-content-fnref-gemini-flash-promo" data-footnote-backref="" aria-label="Back to reference 2" class="data-footnote-backref">↩</a> <a href="#user-content-fnref-gemini-flash-promo-2" data-footnote-backref="" aria-label="Back to reference 2-2" class="data-footnote-backref">↩<sup>2</sup></a></p>
</li>
</ol>
</section>`;

const solRef = (suffix) =>
  `<sup><a href="#user-content-fn-gpt-56-sol-promo" id="user-content-fnref-gpt-56-sol-promo${suffix}" data-footnote-ref="" aria-describedby="footnote-label">1</a></sup>`;
const flashRef = (suffix) =>
  `<sup><a href="#user-content-fn-gemini-flash-promo" id="user-content-fnref-gemini-flash-promo${suffix}" data-footnote-ref="" aria-describedby="footnote-label">2</a></sup>`;

const OPENAI_TABLE = `<h3 id="openai">OpenAI</h3><table aria-labelledby="openai"><thead><tr><th scope="col">Model</th><th scope="col">Release status</th><th scope="col">Category</th><th scope="col">Tier</th><th scope="col">Threshold (input tokens)</th><th scope="col">Input</th><th scope="col">Cached input</th><th scope="col">Cache write</th><th scope="col">Output</th></tr></thead><tbody>
<tr><td>GPT-5.6 Sol${solRef("")}</td><td>GA</td><td>Powerful</td><td>Default</td><td>≤ 272K</td><td>$2.00</td><td>$0.20</td><td>$2.50</td><td>$10.00</td></tr>
<tr><td>GPT-5.6 Sol${solRef("-2")}</td><td>GA</td><td>Powerful</td><td>Long context</td><td>&gt; 272K</td><td>$4.00</td><td>$0.40</td><td>$5.00</td><td>$15.00</td></tr>
<tr><td>GPT-5.6 Luna</td><td>GA</td><td>Versatile</td><td>Default</td><td>Not applicable</td><td>$1.25</td><td>$0.125</td><td>Not applicable</td><td>$10.00</td></tr>
</tbody></table>`;

const GOOGLE_TABLE = `<h3 id="google">Google</h3><table aria-labelledby="google"><thead><tr><th scope="col">Model</th><th scope="col">Release status</th><th scope="col">Category</th><th scope="col">Tier</th><th scope="col">Threshold (input tokens)</th><th scope="col">Input</th><th scope="col">Cached input</th><th scope="col">Output</th></tr></thead><tbody>
<tr><td>Gemini 3.5 Flash</td><td>GA</td><td>Lightweight</td><td>Default</td><td>Not applicable</td><td>$1.50</td><td>$0.15</td><td>$9.00</td></tr>
<tr><td>Gemini 3.6 Flash${flashRef("")}</td><td>GA</td><td>Versatile</td><td>Default</td><td>Not applicable</td><td>$0.75</td><td>$0.075</td><td>$3.75</td></tr>
<tr><td>Gemini 3.7 Flash${flashRef("-2")}</td><td>GA</td><td>Versatile</td><td>Default</td><td>Not applicable</td><td>$0.75</td><td>$0.075</td><td>$3.75</td></tr>
</tbody></table>`;

const PAGE = OPENAI_TABLE + GOOGLE_TABLE + FOOTNOTES;
const byModel = (models, name) => models.find((m) => m.model === name);

test("pricing sync regenerates API metadata from the newly written catalog", (t) => {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "pricing-sync-")));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const dir of [
    "scripts",
    "apps/my-copilot/src/lib",
    "apps/copilot-api",
    "docs",
  ]) {
    mkdirSync(join(root, dir), { recursive: true });
  }
  for (const script of [
    "sync-model-pricing.mjs",
    "generate-api-model-metadata.mjs",
  ]) {
    copyFileSync(
      new URL(script, import.meta.url),
      join(root, "scripts", script),
    );
  }
  writeFileSync(join(root, "apps/my-copilot/src/lib/model-pricing.ts"), "");
  writeFileSync(
    join(root, "apps/copilot-api/model_metadata.json"),
    JSON.stringify({ models: [], retired: [] }),
  );
  writeFileSync(
    join(root, "docs/modellvalg.md"),
    "GitHubs listepriser slik de sto **1. januar 2026**",
  );
  const fixture = join(root, "pricing.html");
  writeFileSync(
    fixture,
    PAGE +
      "<!-- `; throw new Error('fixture executed'); // ${process.exit(1)} -->",
  );
  const mock =
    "import { readFileSync } from 'node:fs'; globalThis.fetch = async () => ({ ok: true, text: async () => readFileSync(process.env.PRICING_FIXTURE, 'utf8') });";
  execFileSync(
    process.execPath,
    [
      "--import",
      `data:text/javascript;base64,${Buffer.from(mock).toString("base64")}`,
      join(root, "scripts/sync-model-pricing.mjs"),
    ],
    { stdio: "pipe", env: { ...process.env, PRICING_FIXTURE: fixture } },
  );
  const metadata = JSON.parse(
    readFileSync(join(root, "apps/copilot-api/model_metadata.json"), "utf8"),
  );
  assert.deepEqual(
    metadata.models,
    parsePricingTables(PAGE).map(({ model, provider, category }) => ({
      model,
      provider,
      category,
    })),
  );
  execFileSync(
    process.execPath,
    [join(root, "scripts/generate-api-model-metadata.mjs"), "--check"],
    { stdio: "pipe" },
  );
  // The written catalog must carry the footnote's end date, or the
  // my-copilot promotion invariant test has nothing to check.
  const catalog = readFileSync(
    join(root, "apps/my-copilot/src/lib/model-pricing.ts"),
    "utf8",
  );
  assert.match(
    catalog,
    /Gemini 3\.6 Flash \(Default\)[^}]*promotionEndsOn: "2026-12-31"/,
  );
});

test("metadata generator retires dropped models and restores returning ones", (t) => {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "metadata-retired-")));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const dir of [
    "scripts",
    "apps/my-copilot/src/lib",
    "apps/copilot-api",
  ]) {
    mkdirSync(join(root, dir), { recursive: true });
  }
  copyFileSync(
    new URL("generate-api-model-metadata.mjs", import.meta.url),
    join(root, "scripts/generate-api-model-metadata.mjs"),
  );
  const row = (model, category = "Versatile") => ({
    model,
    provider: "Acme",
    category,
  });
  const metadataPath = join(root, "apps/copilot-api/model_metadata.json");
  const generate = (models) => {
    writeFileSync(
      join(root, "apps/my-copilot/src/lib/model-pricing.ts"),
      `export const PRICING_SOURCE_URL = "x"; export const PRICING_LAST_UPDATED = "2026-01-01";
export const MODEL_PRICING = ${JSON.stringify(models)};`,
    );
    execFileSync(
      process.execPath,
      [join(root, "scripts/generate-api-model-metadata.mjs")],
      { stdio: "pipe" },
    );
    return JSON.parse(readFileSync(metadataPath, "utf8"));
  };
  writeFileSync(
    metadataPath,
    JSON.stringify({
      models: [row("A"), row("B", "Powerful")],
      retired: [row("Old")],
    }),
  );

  let metadata = generate([row("A")]);
  assert.deepEqual(metadata.models, [row("A")]);
  assert.deepEqual(metadata.retired, [row("B", "Powerful"), row("Old")]);

  metadata = generate([row("A")]);
  assert.deepEqual(
    metadata.retired,
    [row("B", "Powerful"), row("Old")],
    "retired survives regeneration",
  );

  metadata = generate([row("A"), row("B", "Powerful")]);
  assert.deepEqual(
    metadata.retired,
    [row("Old")],
    "a returning model leaves retired",
  );
});

test("footnote list parses into id-keyed text without backref arrows", () => {
  const notes = parseFootnotes(PAGE);
  assert.deepEqual(
    [...notes.keys()],
    ["gpt-56-sol-promo", "gemini-flash-promo"],
  );
  assert.match(
    notes.get("gpt-56-sol-promo"),
    /^GPT-5\.6 Sol is available at promotional pricing/,
  );
  assert.ok(!notes.get("gpt-56-sol-promo").includes("↩"));
});

test("both GPT-5.6 Sol rows carry the promotion end date and the note", () => {
  const models = parsePricingTables(PAGE);
  for (const name of [
    "GPT-5.6 Sol (Default, ≤ 272K)",
    "GPT-5.6 Sol (Long context, 272K)",
  ]) {
    const row = byModel(models, name);
    assert.ok(row, `missing row: ${name}`);
    assert.equal(row.promotionEndsOn, "2026-09-03");
    assert.match(row.note, /50% off standard rates/);
  }
});

test("both promoted Gemini Flash rows carry the promotion end date", () => {
  const models = parsePricingTables(PAGE);
  for (const name of [
    "Gemini 3.6 Flash (Default)",
    "Gemini 3.7 Flash (Default)",
  ]) {
    const row = byModel(models, name);
    assert.ok(row, `missing row: ${name}`);
    assert.equal(row.promotionEndsOn, "2026-12-31");
  }
});

test("models without a footnote get neither field", () => {
  const models = parsePricingTables(PAGE);
  for (const name of ["GPT-5.6 Luna (Default)", "Gemini 3.5 Flash (Default)"]) {
    const row = byModel(models, name);
    assert.ok(row, `missing row: ${name}`);
    assert.equal(row.promotionEndsOn, undefined);
    assert.equal(row.note, undefined);
  }
});

const SOL_ROWS = [
  "GPT-5.6 Sol (Default, ≤ 272K)",
  "GPT-5.6 Sol (Long context, 272K)",
];

test("an undatable footnote is flagged, not silently dropped", () => {
  const broken = PAGE.replace(
    "through September 3, 2026",
    "through the end of the promotion",
  );
  const unresolved = findUnresolvedPromotions(parsePricingTables(broken));
  assert.deepEqual(
    unresolved.map((m) => m.model),
    SOL_ROWS,
  );
  // The note survives so the error message can name what went unparsed.
  assert.match(unresolved[0].note, /through the end of the promotion/);
});

test("a reference to a footnote that is not on the page is flagged too", () => {
  // The whole footnotes section fails to match, which is what an upstream
  // markup change looks like. Every reference is then dangling.
  const unresolved = findUnresolvedPromotions(
    parsePricingTables(OPENAI_TABLE + GOOGLE_TABLE),
  );
  assert.deepEqual(
    unresolved.map((m) => m.model),
    [...SOL_ROWS, "Gemini 3.6 Flash (Default)", "Gemini 3.7 Flash (Default)"],
  );
  assert.equal(unresolved[0].note, undefined);
  assert.equal(unresolved[0].footnoteId, "gpt-56-sol-promo");
});

test("a reference to a footnote id missing from the list is flagged", () => {
  const renamed = PAGE.replace(
    '<li id="user-content-fn-gpt-56-sol-promo">',
    '<li id="user-content-fn-sol-promo-2027">',
  );
  const unresolved = findUnresolvedPromotions(parsePricingTables(renamed));
  assert.deepEqual(
    unresolved.map((m) => m.model),
    SOL_ROWS,
  );
  // Gemini still resolves, so the guard is not just firing on everything.
  const gemini = parsePricingTables(renamed).find(
    (m) => m.model === "Gemini 3.6 Flash (Default)",
  );
  assert.equal(gemini.promotionEndsOn, "2026-12-31");
});

test("date parsing handles the shapes the page uses", () => {
  assert.equal(
    parsePromotionEndDate("... through September 3, 2026."),
    "2026-09-03",
  );
  assert.equal(
    parsePromotionEndDate("... through December 31, 2026."),
    "2026-12-31",
  );
  assert.equal(parsePromotionEndDate("... through Smarch 3, 2026."), undefined);
  assert.equal(parsePromotionEndDate("no end date at all"), undefined);
});

test("the doc's pricing date is rewritten, and the editorial dates are not", () => {
  const doc = [
    "GitHubs listepriser slik de sto **30. august 2026**, og speiler",
    "`apps/my-copilot/src/lib/model-pricing.ts`.",
    "Per 31. august 2026 gjelder det:",
    "- **GPT-5.6 Sol:** 50 % av standardpris t.o.m. 3. september 2026.",
    "## Målinger (august 2026)",
  ].join("\n");

  const out = setDocPricingDate(doc, "2026-09-03");

  assert.match(out, /slik de sto \*\*3\. september 2026\*\*/);
  assert.equal(
    out.split("\n").slice(1).join("\n"),
    doc.split("\n").slice(1).join("\n"),
  );
});

test("the pricing date is found when the sentence wraps before the date", () => {
  // How docs/modellvalg.md actually reads: prettier put a newline between
  // "sto" and the bolded date, and matching a literal space made the guard
  // throw "the pricing-date sentence is gone" on a sentence that was there.
  const doc = [
    "GitHub priser. Prisene under er GitHubs listepriser slik de sto",
    "**30. august 2026**, hentet fra `apps/my-copilot/src/lib/model-pricing.ts`.",
  ].join("\n");

  const out = setDocPricingDate(doc, "2026-09-03");

  assert.match(out, /\*\*3\. september 2026\*\*/);
  assert.equal(out.split("\n")[0], doc.split("\n")[0]);
});

test("a doc without the sentence is an error, not a silent no-op", () => {
  assert.throws(() => setDocPricingDate("ingen priser her", "2026-09-03"));
});

test("API model metadata: a tier suffix rename is not a retirement, a removal is", async () => {
  const { retiredModels } = await import("./generate-api-model-metadata.mjs");
  const m = (model) => ({ model, provider: "Anthropic", category: "Powerful" });
  const previous = {
    models: [
      m("Claude Opus 5.5"),
      m("Claude Opus 4.8 (fast mode) (preview)"),
      m("Claude Gone 1"),
    ],
    retired: [],
  };
  const models = [
    m("Claude Opus 5.5 (Default)"),
    m("Claude Opus 4.8 (fast mode) (preview) (Default)"),
  ];
  assert.deepEqual(retiredModels(previous, models), [m("Claude Gone 1")]);
});
