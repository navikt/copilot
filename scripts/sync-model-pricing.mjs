#!/usr/bin/env node
/**
 * Fetches GitHub Copilot model pricing from the official docs page and
 * updates apps/my-copilot/src/lib/model-pricing.ts with current data.
 *
 * Usage:
 *   node scripts/sync-model-pricing.mjs          # update the file
 *   node scripts/sync-model-pricing.mjs --check  # exit 1 if out of date (CI mode)
 */

const PRICING_URL =
  "https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing";
const TARGET_FILE = new URL(
  "../apps/my-copilot/src/lib/model-pricing.ts",
  import.meta.url,
);

import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

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

  // Extract provider sections by matching h3 headers followed by tables
  const sections = [
    { provider: "OpenAI", anchorId: "openai" },
    { provider: "Anthropic", anchorId: "anthropic" },
    { provider: "Google", anchorId: "google" },
    { provider: "GitHub", anchorId: "fine-tuned-github" },
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
      const cells = [...rows[i][1].matchAll(/<t[dh][^>]*>([\s\S]*?)<\/t[dh]>/g)].map(
       (m) => stripHtml(m[1]).trim()
      );
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

function stripHtml(html) {
  return html
    .replace(/<sup[^>]*>.*?<\/sup>/g, "")
    .replace(/<a[^>]*>.*?<\/a>/g, "")
    .replace(/<[^>]+>/g, "")
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

  const providerOrder = ["OpenAI", "Anthropic", "Google", "GitHub"];
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
  provider: "OpenAI" | "Anthropic" | "Google" | "GitHub";
  category: "Lightweight" | "Versatile" | "Powerful";
  status: "GA" | "Public preview";
  input: number;
  cachedInput: number;
  cacheWrite?: number;
  output: number;
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

  if (models.length === 0) {
    console.error("ERROR: Could not parse any models from the pricing page.");
    console.error("The page structure may have changed. Manual update required.");
    process.exit(1);
  }

  console.log(`Parsed ${models.length} models:`);
  for (const m of models) {
    console.log(`  ${m.provider}/${m.model}: in=$${m.input} cached=$${m.cachedInput} out=$${m.output}`);
  }

  const newContent = generateTypeScript(models);
  const targetPath = fileURLToPath(TARGET_FILE);

  if (checkOnly) {
    const current = readFileSync(targetPath, "utf-8");
    // Compare ignoring the date line (last-updated changes daily)
    const normalize = (s) =>
      s
        .replace(/Last updated: \d{4}-\d{2}-\d{2}/g, "Last updated: DATE")
        .replace(/PRICING_LAST_UPDATED = "[^"]+"/g, 'PRICING_LAST_UPDATED = "DATE"');

    if (normalize(current) !== normalize(newContent)) {
      console.error("\nERROR: model-pricing.ts is out of date!");
      console.error("Run: node scripts/sync-model-pricing.mjs");
      process.exit(1);
    } else {
      console.log("\n✓ model-pricing.ts is up to date");
    }
  } else {
    writeFileSync(targetPath, newContent, "utf-8");
    console.log(`\n✓ Updated ${targetPath}`);
  }
}

main().catch((err) => {
  console.error("Failed:", err.message);
  process.exit(1);
});
