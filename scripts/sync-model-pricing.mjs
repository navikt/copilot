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
    // Skip header row
    const headerCells =
      rows.length > 0
       ? [...rows[0][1].matchAll(/<t[dh][^>]*>([\s\S]*?)<\/t[dh]>/g)].map((m) => stripHtml(m[1]).trim())
       : [];
    const headerIndex = (name) => headerCells.findIndex((cell) => cell.toLowerCase() === name.toLowerCase());

    for (let i = 1; i < rows.length; i++) {
      const cells = [...rows[i][1].matchAll(/<t[dh][^>]*>([\s\S]*?)<\/t[dh]>/g)].map(
       (m) => stripHtml(m[1]).trim(),
      );

      if (cells.length < 5) continue;

      const getCell = (name) => {
       const idx = headerIndex(name);
       return idx >= 0 ? cells[idx] : undefined;
      };

      const model = getCell("Model");
      if (!model) continue;
      const status = getCell("Release status");
      const category = getCell("Category");
      const input = getCell("Input");
      const cachedInput = getCell("Cached input");
      const cacheWriteIdx = headerIndex("Cache write");
      const output = getCell("Output");
      const cacheWrite = cacheWriteIdx >= 0 ? cells[cacheWriteIdx] : undefined;

      const entry = {
       model: cleanModelName(model),
       provider: section.provider,
       category: normalizeCategory(category),
       status: normalizeStatus(status),
       input: parsePrice(input),
       cachedInput: parsePrice(cachedInput),
       output: parsePrice(output),
      };

      if (cacheWrite) {
       entry.cacheWrite = parsePrice(cacheWrite);
      }

      // Detect notes from footnotes
      if (model.includes("[1]") || model.includes("1")) {
        if (entry.model.includes("GPT-4.1") || entry.model.includes("GPT-5 mini")) {
          entry.note = "Included model";
        }
      }

      if (!isNaN(entry.input) && !isNaN(entry.output)) {
        models.push(entry);
      }
    }
  }

  return models;
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
  if (!str) return NaN;
  const cleaned = str.replace(/[$,]/g, "").trim();
  return parseFloat(cleaned);
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
