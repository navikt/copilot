#!/usr/bin/env node
/**
 * Fetches GitHub Copilot model pricing from the official docs page and
 * updates apps/my-copilot/src/lib/model-pricing.ts with current data.
 *
 * Usage:
 *   node scripts/sync-model-pricing.mjs          # update the file
 *   node scripts/sync-model-pricing.mjs --check  # CI mode, writes nothing
 *
 * --check exit codes: 0 up to date (only the timestamp would move), 2 prices
 * moved, 1 the check could not be made (fetch failed, parse floor tripped, a
 * model came back unpriced, or a promotion footnote did not resolve to an end
 * date). 2 is separate from 1 so a caller can act on real drift without also
 * acting on a check that never got an answer.
 */

const PRICING_URL =
  "https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing";
const TARGET_FILE = new URL(
  "../apps/my-copilot/src/lib/model-pricing.ts",
  import.meta.url,
);
const DOC_FILE = new URL("../docs/modellvalg.md", import.meta.url);

// The one sentence in docs/modellvalg.md that timestamps the price table.
// Anchored on its wording so the editorial dates elsewhere in the file stay put.
const DOC_DATE_RE = /(GitHubs listepriser slik de sto \*\*)([^*]+)(\*\*)/;

const NB_MONTHS = [
  "januar", "februar", "mars", "april", "mai", "juni",
  "juli", "august", "september", "oktober", "november", "desember",
];

/** 2026-09-03 -> "3. september 2026". */
function toNorwegianDate(iso) {
  const [y, m, d] = iso.split("-").map(Number);
  return `${d}. ${NB_MONTHS[m - 1]} ${y}`;
}

/** Rewrite the pricing date in that sentence. Throws if the sentence moved. */
function setDocPricingDate(text, iso) {
  if (!DOC_DATE_RE.test(text)) {
    throw new Error(`the pricing-date sentence is gone from ${DOC_FILE.pathname}`);
  }
  return text.replace(DOC_DATE_RE, `$1${toNorwegianDate(iso)}$3`);
}

import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";

// --- Fetch and parse ---

async function fetchPricingPage() {
  const res = await fetch(PRICING_URL);
  if (!res.ok) throw new Error(`HTTP ${res.status} fetching pricing page`);
  return res.text();
}

/**
 * Parse HTML tables into structured pricing data.
 * The page has tables for OpenAI, Anthropic, Google, and GitHub.
 */
function parsePricingTables(html) {
  const models = [];
  const footnotes = parseFootnotes(html);

  // Extract provider sections by matching h3 headers followed by tables
  const sections = [
    { provider: "OpenAI", anchorId: "openai" },
    { provider: "Anthropic", anchorId: "anthropic" },
    { provider: "Google", anchorId: "google" },
    { provider: "GitHub", anchorId: "fine-tuned-github" },
    { provider: "Microsoft", anchorId: "microsoft" },
    { provider: "Moonshot AI", anchorId: "moonshot-ai" },
  ];

  for (const section of sections) {
    const sectionStart = html.indexOf(`id="${section.anchorId}"`);
    if (sectionStart === -1) {
      console.warn(`Warning: Could not find section for ${section.provider}`);
      continue;
    }

    // Find the table after this anchor
    const tableStart = html.indexOf("<table", sectionStart);
    if (tableStart === -1) continue;
    const tableEnd = html.indexOf("</table>", tableStart);
    if (tableEnd === -1) continue;
    const tableHtml = html.substring(tableStart, tableEnd + 8);

    const rows = [...tableHtml.matchAll(/<tr[^>]*>([\s\S]*?)<\/tr>/g)];
    const headerCells =
      rows.length > 0
        ? [...rows[0][1].matchAll(/<t[dh][^>]*>([\s\S]*?)<\/t[dh]>/g)].map((m) => stripHtml(m[1]).trim())
        : [];
    const headerIndices = buildHeaderIndexMap(headerCells);

    const requiredColumns = {
      model: findHeaderIndex(headerIndices, ["model"]),
      input: findHeaderIndex(headerIndices, ["input"]),
      cachedInput: findHeaderIndex(headerIndices, ["cached input"]),
      output: findHeaderIndex(headerIndices, ["output"]),
    };

    if (Object.values(requiredColumns).some((index) => index < 0)) {
      console.warn(`Warning: Missing required columns in ${section.provider} table, skipping section`);
      continue;
    }

    for (let i = 1; i < rows.length; i++) {
      const rawCells = [...rows[i][1].matchAll(/<t[dh][^>]*>([\s\S]*?)<\/t[dh]>/g)].map((m) => m[1]);
      const cells = rawCells.map((c) => stripHtml(c).trim());
      const model = getCell(cells, requiredColumns.model);
      if (!model) continue;

      const status = getCell(cells, findHeaderIndex(headerIndices, ["release status", "status"]));
      const category = getCell(cells, findHeaderIndex(headerIndices, ["category"]));
      const tier = getCell(cells, findHeaderIndex(headerIndices, ["tier"]));
      const threshold = getCell(cells, findHeaderIndex(headerIndices, ["threshold (input tokens)", "threshold"]));
      const input = parsePrice(getCell(cells, requiredColumns.input));
      const cachedInput = parsePrice(getCell(cells, requiredColumns.cachedInput));
      const output = parsePrice(getCell(cells, requiredColumns.output));
      const cacheWrite = parsePrice(getCell(cells, findHeaderIndex(headerIndices, ["cache write"])));

      if (input === undefined || cachedInput === undefined || output === undefined) {
        continue;
      }

      const entry = {
        model: formatModelName(cleanModelName(model), tier, threshold),
        provider: section.provider,
        category: normalizeCategory(category),
        status: normalizeStatus(status),
        input,
        cachedInput,
        output,
      };

      if (cacheWrite !== undefined) {
        entry.cacheWrite = cacheWrite;
      }

      // Promotional prices live in footnotes the model cell links to. The
      // price itself parses fine without them, but it is the promotional one,
      // so a row without its footnote reads as permanent when it is not.
      const footnoteId = findFootnoteId(getCell(rawCells, requiredColumns.model));
      if (footnoteId) {
        // Kept on the record even when the footnote does not resolve, so the
        // guard below can see the reference and refuse to publish the row.
        entry.footnoteId = footnoteId;
        const note = footnotes.get(footnoteId);
        if (note) {
          entry.note = note;
          const endsOn = parsePromotionEndDate(note);
          if (endsOn) entry.promotionEndsOn = endsOn;
        }
      }

      models.push(entry);
    }
  }

  return models;
}

function normalizeHeaderName(name) {
  return name
    .toLowerCase()
    .replace(/\s+/g, " ")
    .trim();
}

function buildHeaderIndexMap(headerCells) {
  const map = new Map();
  for (const [index, header] of headerCells.entries()) {
    map.set(normalizeHeaderName(header), index);
  }
  return map;
}

function findHeaderIndex(headerMap, names) {
  for (const name of names) {
    const index = headerMap.get(normalizeHeaderName(name));
    if (index !== undefined) return index;
  }
  return -1;
}

function getCell(cells, index) {
  if (index < 0 || index >= cells.length) return undefined;
  return cells[index];
}

/** Footnote id a model cell references, e.g. "gpt-56-sol-promo". */
function findFootnoteId(rawCell) {
  return rawCell?.match(/#user-content-fn-([\w-]+)/)?.[1];
}

/** Map of footnote id to plain text from the page's <section data-footnotes>. */
function parseFootnotes(html) {
  const notes = new Map();
  const section = html.match(/<section[^>]*data-footnotes[\s\S]*?<\/section>/)?.[0];
  if (!section) return notes;
  for (const [, id, body] of section.matchAll(/<li id="user-content-fn-([\w-]+)">([\s\S]*?)<\/li>/g)) {
    // stripHtml drops the backref anchors along with the rest of the markup.
    const text = stripHtml(body).replace(/\s+/g, " ").trim();
    if (text) notes.set(id, text);
  }
  return notes;
}

const MONTHS = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];

/**
 * ISO date from a footnote's "through September 3, 2026". Date.parse is
 * avoided on purpose: it reads the date as local midnight, which toISOString
 * then shifts a day backwards east of UTC.
 */
function parsePromotionEndDate(note) {
  const match = note.match(new RegExp(`through (${MONTHS.join("|")}) (\\d{1,2}), (\\d{4})`, "i"));
  if (!match) return undefined;
  const month = MONTHS.findIndex((m) => m.toLowerCase() === match[1].toLowerCase()) + 1;
  return `${match[3]}-${String(month).padStart(2, "0")}-${match[2].padStart(2, "0")}`;
}

/**
 * Rows whose footnote reference did not turn into an end date, either because
 * the footnote body is missing from the page or because its date would not
 * parse. Both mean the price is promotional and we cannot say until when, and
 * shipping that is the silent wrong answer, so the caller must fail rather
 * than publish the row.
 */
function findUnresolvedPromotions(models) {
  return models.filter((m) => m.footnoteId && !m.promotionEndsOn);
}

function stripHtml(html) {
  // Single-pass tag removal is bypassable (e.g. "<scr<sup></sup>ipt>" becomes
  // "<script>" after one pass), so loop until the string stops changing.
  let text = html;
  let previous;
  do {
    previous = text;
    text = text
      .replace(/<sup[^>]*>.*?<\/sup>/g, "")
      .replace(/<a[^>]*>.*?<\/a>/g, "")
      .replace(/<[^>]+>/g, "");
  } while (text !== previous);
  // This is a text extractor, not an HTML renderer: drop any leftover angle
  // brackets (unbalanced/malformed markup) entirely.
  return text
    .replace(/[<>]/g, "")
    .replace(/&[a-z]+;/g, "")
    .trim();
}

function cleanModelName(name) {
  return name
    .replace(/\[\d+\]/g, "")
    .replace(/\s+/g, " ")
    .trim();
}

function formatModelName(model, tier, threshold) {
  const cleanTier = tier?.trim();
  const cleanThreshold = threshold?.trim();

  const hasTier = Boolean(cleanTier);
  const hasThreshold = Boolean(cleanThreshold) && !/^not applicable$/i.test(cleanThreshold);

  if (!hasTier && !hasThreshold) return model;

  const variant = [];
  if (hasTier) variant.push(cleanTier);
  if (hasThreshold) variant.push(cleanThreshold);
  return `${model} (${variant.join(", ")})`;
}

function normalizeCategory(cat) {
  const lower = cat?.toLowerCase() || "";
  if (lower.includes("light")) return "Lightweight";
  if (lower.includes("versat")) return "Versatile";
  if (lower.includes("power")) return "Powerful";
  return cat;
}

function normalizeStatus(status) {
  const lower = status?.toLowerCase() || "";
  if (lower.includes("preview")) return "Public preview";
  return "GA";
}

function parsePrice(str) {
  if (!str) return undefined;
  const cleaned = str.replace(/[$,]/g, "").trim();
  if (!cleaned) return undefined;
  const parsed = Number.parseFloat(cleaned);
  return Number.isFinite(parsed) ? parsed : undefined;
}

// --- Generate TypeScript ---

function generateTypeScript(models) {
  const today = new Date().toISOString().split("T")[0];

  const providerOrder = ["OpenAI", "Anthropic", "Google", "GitHub", "Microsoft", "Moonshot AI"];
  const grouped = {};
  for (const m of models) {
    if (!grouped[m.provider]) grouped[m.provider] = [];
    grouped[m.provider].push(m);
  }

  let entries = "";
  for (const provider of providerOrder) {
    const group = grouped[provider];
    if (!group?.length) continue;
    entries += `  // ${provider}\n`;
    for (const m of group) {
      entries += `  {\n`;
      entries += `    model: ${JSON.stringify(m.model)},\n`;
      entries += `    provider: ${JSON.stringify(m.provider)},\n`;
      entries += `    category: ${JSON.stringify(m.category)},\n`;
      entries += `    status: ${JSON.stringify(m.status)},\n`;
      entries += `    input: ${m.input},\n`;
      entries += `    cachedInput: ${m.cachedInput},\n`;
      if (m.cacheWrite !== undefined) {
        entries += `    cacheWrite: ${m.cacheWrite},\n`;
      }
      entries += `    output: ${m.output},\n`;
      if (m.promotionEndsOn) {
        entries += `    promotionEndsOn: ${JSON.stringify(m.promotionEndsOn)},\n`;
      }
      if (m.note) {
        entries += `    note: ${JSON.stringify(m.note)},\n`;
      }
      entries += `  },\n`;
    }
  }

  return `/**
 * GitHub Copilot model pricing data.
 * Source: ${PRICING_URL}
 * Last updated: ${today}
 *
 * All prices are per 1 million tokens in USD.
 * 1 AI credit = $0.01 USD.
 *
 * AUTO-GENERATED by scripts/sync-model-pricing.mjs — do not edit manually.
 */

export interface ModelPrice {
  model: string;
  provider: "OpenAI" | "Anthropic" | "Google" | "GitHub" | "Microsoft" | "Moonshot AI";
  category: "Lightweight" | "Versatile" | "Powerful";
  status: "GA" | "Public preview";
  input: number;
  cachedInput: number;
  cacheWrite?: number;
  output: number;
  /** ISO date the promotional price in the note runs through, if any. */
  promotionEndsOn?: string;
  note?: string;
}

export const MODEL_PRICING: ModelPrice[] = [
${entries}];

export const PRICING_SOURCE_URL = ${JSON.stringify(PRICING_URL)};
export const PRICING_LAST_UPDATED = ${JSON.stringify(today)};
`;
}

// --- Main ---

async function main() {
  const checkOnly = process.argv.includes("--check");

  console.log("Fetching pricing data from GitHub docs...");
  const html = await fetchPricingPage();
  const models = parsePricingTables(html);

  const targetPath = fileURLToPath(TARGET_FILE);
  const current = readFileSync(targetPath, "utf-8");
  const currentCount = (current.match(/^ {4}model: /gm) ?? []).length;

  // A docs reshuffle past the parser looks like a partial parse, not a crash,
  // and the workflow commits whatever comes out. Models do get retired, so the
  // floor is generous: half the models we already know about.
  if (models.length === 0 || models.length * 2 < currentCount) {
    console.error(
      `ERROR: parsed ${models.length} models, down from ${currentCount} in the generated file.`,
    );
    console.error("The page structure may have changed. Manual update required.");
    process.exit(1);
  }

  const unresolvedPromotions = findUnresolvedPromotions(models);
  if (unresolvedPromotions.length > 0) {
    console.error("ERROR: models with a footnote that did not yield a promotion end date:");
    for (const m of unresolvedPromotions) {
      console.error(
        `  ${m.provider}/${m.model}: ${m.note ?? `footnote #${m.footnoteId} not found on the page`}`,
      );
    }
    process.exit(1);
  }

  const unpriced = models.filter((m) => !(m.input > 0) || !(m.output > 0));
  if (unpriced.length > 0) {
    console.error("ERROR: models parsed without a positive input and output price:");
    for (const m of unpriced) {
      console.error(`  ${m.provider}/${m.model}: in=${m.input} out=${m.output}`);
    }
    process.exit(1);
  }

  console.log(`Parsed ${models.length} models:`);
  for (const m of models) {
    console.log(`  ${m.provider}/${m.model}: in=$${m.input} cached=$${m.cachedInput} out=$${m.output}`);
  }

  const newContent = generateTypeScript(models);

  const docPath = fileURLToPath(DOC_FILE);
  const doc = readFileSync(docPath, "utf-8");

  if (checkOnly) {
    // The doc restates PRICING_LAST_UPDATED in Norwegian prose; nothing else
    // keeps the two in step, so a drift here is the same kind of staleness.
    const pricingDate = current.match(/PRICING_LAST_UPDATED = "([^"]+)"/)?.[1];
    if (!pricingDate) {
      console.error("\nERROR: no PRICING_LAST_UPDATED in the generated file.");
      process.exit(1);
    }
    if (setDocPricingDate(doc, pricingDate) !== doc) {
      console.error(`\nERROR: ${docPath} is out of date!`);
      console.error(
        `  says "${doc.match(DOC_DATE_RE)[2]}", PRICING_LAST_UPDATED is ${pricingDate} ` +
          `("${toNorwegianDate(pricingDate)}")`,
      );
      console.error("Run: node scripts/sync-model-pricing.mjs");
      process.exit(2);
    }

    // Compare ignoring the date line (last-updated changes daily)
    const normalize = (s) =>
      s
        .replace(/Last updated: \d{4}-\d{2}-\d{2}/g, "Last updated: DATE")
        .replace(/PRICING_LAST_UPDATED = "[^"]+"/g, 'PRICING_LAST_UPDATED = "DATE"');

    if (normalize(current) !== normalize(newContent)) {
      console.error("\nERROR: model-pricing.ts is out of date!");
      console.error("Run: node scripts/sync-model-pricing.mjs");
      // 2, not 1: this is the one outcome that means the prices really moved.
      // Every other non-zero exit here is a check that failed to run, and a
      // caller must not mistake one for the other.
      process.exit(2);
    } else {
      console.log("\n✓ model-pricing.ts is up to date");
    }
  } else {
    writeFileSync(targetPath, newContent, "utf-8");
    console.log(`\n✓ Updated ${targetPath}`);
    const today = new Date().toISOString().split("T")[0];
    writeFileSync(docPath, setDocPricingDate(doc, today), "utf-8");
    console.log(`✓ Updated ${docPath}`);
  }
}

export {
  parsePricingTables,
  parseFootnotes,
  parsePromotionEndDate,
  findFootnoteId,
  findUnresolvedPromotions,
  setDocPricingDate,
  toNorwegianDate,
};

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((err) => {
    console.error("Failed:", err.message);
    process.exit(1);
  });
}
