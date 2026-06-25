import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import type { Domain, ExampleItem } from "../src/lib/manifest-types.ts";
import { VALID_DOMAINS } from "../src/lib/manifest-types.ts";

const REPO_ROOT = path.resolve(import.meta.dirname, "../../..");
const GITHUB_DIR = REPO_ROOT;
const COLLECTIONS_DIR = path.join(GITHUB_DIR, "collections");
const RAW_BASE = "https://raw.githubusercontent.com/navikt/copilot/main";
const OUTPUT = path.resolve(import.meta.dirname, "../src/lib/copilot-manifest.json");

interface Metadata {
  displayName?: string;
  description?: string;
  domain?: Domain;
  tags?: string[];
  references?: string[];
  excluded?: boolean;
  examples?: ExampleItem[];
  deprecated?: boolean;
  deprecatedMessage?: string;
}

function loadMetadata(metadataPath: string): Metadata {
  if (!fs.existsSync(metadataPath)) return {};
  const raw = JSON.parse(fs.readFileSync(metadataPath, "utf-8")) as Metadata;
  if (raw.domain && !VALID_DOMAINS.includes(raw.domain)) {
    throw new Error(`Invalid domain "${raw.domain}" in ${metadataPath}. Valid: ${VALID_DOMAINS.join(", ")}`);
  }
  return raw;
}

function parseFrontmatter(content: string): { data: Record<string, string | string[]>; body: string } {
  const match = content.match(/^---\s*\n([\s\S]*?)\n---\s*\n([\s\S]*)$/);
  if (!match) return { data: {}, body: content };

  const raw = match[1];
  const body = match[2];
  const data: Record<string, string | string[]> = {};

  let currentKey: string | null = null;
  let arrayValues: string[] = [];

  for (const line of raw.split("\n")) {
    const keyMatch = line.match(/^(\w[\w-]*)\s*:\s*(.*)$/);
    if (keyMatch) {
      if (currentKey && arrayValues.length > 0) {
        data[currentKey] = arrayValues;
        arrayValues = [];
      }
      currentKey = keyMatch[1];
      let value = keyMatch[2].trim();
      // Strip matching quotes (single or double)
      if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
        value = value.slice(1, -1);
      }
      if (value) {
        data[currentKey] = value;
        currentKey = null;
      }
    } else {
      const arrayItem = line.match(/^\s+-\s+(.+)$/);
      if (arrayItem && currentKey) {
        arrayValues.push(arrayItem[1].trim());
      }
    }
  }
  if (currentKey && arrayValues.length > 0) {
    data[currentKey] = arrayValues;
  }

  return { data, body };
}

function buildInstallUrl(type: "agent" | "instructions" | "prompt", rawUrl: string): string {
  const vscodeUrl = `vscode:chat-${type}/install?url=${rawUrl}`;
  return `/install/${type}?url=${encodeURIComponent(vscodeUrl)}`;
}

function buildInsidersInstallUrl(type: "agent" | "instructions" | "prompt", rawUrl: string): string {
  const vscodeUrl = `vscode-insiders:chat-${type}/install?url=${rawUrl}`;
  return `/install/${type}?url=${encodeURIComponent(vscodeUrl)}`;
}

function contentHash(filePath: string): string {
  const content = fs.readFileSync(filePath);
  return crypto.createHash("sha256").update(content).digest("hex");
}

interface ManifestItem {
  id: string;
  name: string;
  description: string;
  type: "agent" | "instruction" | "prompt" | "skill";
  domain: Domain;
  filePath: string;
  rawGitHubUrl: string;
  contentHash: string;
  installUrl: string | null;
  insidersInstallUrl: string | null;
  tools?: string[];
  agentReferences?: string[];
  applyTo?: string;
  invocation?: string;
  tags?: string[];
  examples?: ExampleItem[];
  references?: { path: string; rawUrl: string }[];
  model?: string[];
  deprecated?: boolean;
  deprecatedMessage?: string;
  collections?: string[];
}

interface CollectionManifest {
  name: string;
  agents?: string[];
  skills?: string[];
  instructions?: string[];
  prompts?: string[];
}

/**
 * Read all collection manifest.json files and build a reverse index:
 * "type:id" → string[] of collection names that include that item.
 */
function buildCollectionsIndex(): Map<string, string[]> {
  const index = new Map<string, string[]>();
  if (!fs.existsSync(COLLECTIONS_DIR)) return index;

  for (const entry of fs.readdirSync(COLLECTIONS_DIR, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    const manifestPath = path.join(COLLECTIONS_DIR, entry.name, "manifest.json");
    if (!fs.existsSync(manifestPath)) continue;
    const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf-8")) as CollectionManifest;
    const collectionId = entry.name;

    for (const id of manifest.agents ?? []) {
      const key = `agent:${id}`;
      index.set(key, [...(index.get(key) ?? []), collectionId]);
    }
    for (const id of manifest.skills ?? []) {
      const key = `skill:${id}`;
      index.set(key, [...(index.get(key) ?? []), collectionId]);
    }
    for (const id of manifest.instructions ?? []) {
      const key = `instruction:${id}`;
      index.set(key, [...(index.get(key) ?? []), collectionId]);
    }
    for (const id of manifest.prompts ?? []) {
      const key = `prompt:${id}`;
      index.set(key, [...(index.get(key) ?? []), collectionId]);
    }
  }
  return index;
}

/**
 * Extract @xxx-agent references from markdown body text.
 * Matches backtick-wrapped or bare @agent-name patterns followed by
 * whitespace, punctuation, or end-of-string (avoids false positives
 * on compound words like "@auth-agent-style").
 * Returns deduplicated agent IDs (without the @ prefix).
 */
function extractAgentReferences(body: string, selfName: string): string[] {
  const refs = new Set<string>();
  for (const match of body.matchAll(/@([a-z][-a-z]*-agent)(?=[\s.,;:!?`)\]]|$)/gm)) {
    const ref = match[1];
    if (ref !== selfName) refs.add(ref);
  }
  return [...refs].sort();
}

function getAgents(): ManifestItem[] {
  const dir = path.join(GITHUB_DIR, "agents");
  if (!fs.existsSync(dir)) return [];

  const agentFiles = fs.readdirSync(dir).filter((f) => f.endsWith(".agent.md"));

  // Parse all agents in one pass, collecting names for reference validation
  const parsed = agentFiles.map((file) => {
    const filePath = path.join(dir, file);
    const content = fs.readFileSync(filePath, "utf-8");
    const { data, body } = parseFrontmatter(content);
    const name = (data.name as string) || file.replace(".agent.md", "");
    const meta = loadMetadata(path.join(dir, file.replace(".agent.md", ".metadata.json")));
    return { file, filePath, data, body, name, meta };
  });

  const knownAgentIds = new Set(parsed.map((p) => p.name));

  return parsed.map(({ file, filePath, data, body, name, meta }) => {
    const rawUrl = `${RAW_BASE}/agents/${file}`;
    const description = (data.description as string) || "";
    const tools = Array.isArray(data.tools) ? data.tools : [];
    const model = data.model
      ? Array.isArray(data.model)
        ? (data.model as string[])
        : [data.model as string]
      : undefined;
    const agentReferences = extractAgentReferences(body, name).filter((ref) => knownAgentIds.has(ref));

    return {
      id: name,
      name,
      description,
      type: "agent" as const,
      domain: meta.domain || "general",
      filePath: `.github/agents/${file}`,
      rawGitHubUrl: rawUrl,
      contentHash: contentHash(filePath),
      installUrl: buildInstallUrl("agent", rawUrl),
      insidersInstallUrl: buildInsidersInstallUrl("agent", rawUrl),
      tools,
      ...(model && { model }),
      ...(agentReferences.length > 0 && { agentReferences }),
      ...(meta.tags && { tags: meta.tags }),
      ...(meta.examples && { examples: meta.examples }),
      ...(meta.deprecated && { deprecated: true }),
      ...(meta.deprecatedMessage && { deprecatedMessage: meta.deprecatedMessage }),
    };
  });
}

function getInstructions(): ManifestItem[] {
  const dir = path.join(GITHUB_DIR, "instructions");
  if (!fs.existsSync(dir)) return [];

  return fs
    .readdirSync(dir)
    .filter((f) => f.endsWith(".instructions.md"))
    .map((file) => {
      const content = fs.readFileSync(path.join(dir, file), "utf-8");
      const { data } = parseFrontmatter(content);
      const id = file.replace(".instructions.md", "");
      const applyTo = (data.applyTo as string) || "";
      const meta = loadMetadata(path.join(dir, file.replace(".instructions.md", ".metadata.json")));

      const rawUrl = `${RAW_BASE}/instructions/${file}`;

      return {
        id,
        name: meta.displayName || id,
        description: meta.description || `Instruksjoner for ${applyTo}`,
        type: "instruction" as const,
        domain: meta.domain || "general",
        filePath: `.github/instructions/${file}`,
        rawGitHubUrl: rawUrl,
        contentHash: contentHash(path.join(dir, file)),
        installUrl: buildInstallUrl("instructions", rawUrl),
        insidersInstallUrl: buildInsidersInstallUrl("instructions", rawUrl),
        applyTo,
        ...(meta.tags && { tags: meta.tags }),
        ...(meta.examples && { examples: meta.examples }),
        ...(meta.deprecated && { deprecated: true }),
        ...(meta.deprecatedMessage && { deprecatedMessage: meta.deprecatedMessage }),
      };
    });
}

function getPrompts(): ManifestItem[] {
  const dir = path.join(GITHUB_DIR, "prompts");
  if (!fs.existsSync(dir)) return [];

  return fs
    .readdirSync(dir)
    .filter((f) => f.endsWith(".prompt.md"))
    .map((file) => {
      const content = fs.readFileSync(path.join(dir, file), "utf-8");
      const { data } = parseFrontmatter(content);
      const name = (data.name as string) || file.replace(".prompt.md", "");
      const rawUrl = `${RAW_BASE}/prompts/${file}`;
      const meta = loadMetadata(path.join(dir, file.replace(".prompt.md", ".metadata.json")));
      const model = data.model
        ? Array.isArray(data.model)
          ? (data.model as string[])
          : [data.model as string]
        : undefined;

      return {
        id: name,
        name,
        description: (data.description as string) || "",
        type: "prompt" as const,
        domain: meta.domain || "general",
        filePath: `.github/prompts/${file}`,
        rawGitHubUrl: rawUrl,
        contentHash: contentHash(path.join(dir, file)),
        installUrl: buildInstallUrl("prompt", rawUrl),
        insidersInstallUrl: buildInsidersInstallUrl("prompt", rawUrl),
        invocation: `#${name}`,
        ...(model && { model }),
        ...(meta.tags && { tags: meta.tags }),
        ...(meta.examples && { examples: meta.examples }),
        ...(meta.deprecated && { deprecated: true }),
        ...(meta.deprecatedMessage && { deprecatedMessage: meta.deprecatedMessage }),
      };
    });
}

function getSkills(): ManifestItem[] {
  const dir = path.join(GITHUB_DIR, "skills");
  if (!fs.existsSync(dir)) return [];

  return fs
    .readdirSync(dir)
    .filter((f) => fs.existsSync(path.join(dir, f, "SKILL.md")))
    .map((folder) => {
      const meta = loadMetadata(path.join(dir, folder, "metadata.json"));
      return { folder, meta };
    })
    .filter(({ meta }) => !meta.excluded)
    .map(({ folder, meta }) => {
      const content = fs.readFileSync(path.join(dir, folder, "SKILL.md"), "utf-8");
      const { data } = parseFrontmatter(content);
      const name = (data.name as string) || folder;

      const references = meta.references?.map((ref) => ({
        path: ref,
        rawUrl: `${RAW_BASE}/skills/${folder}/${ref}`,
      }));

      return {
        id: name,
        name,
        description: (data.description as string) || "",
        type: "skill" as const,
        domain: meta.domain || "general",
        filePath: `skills/${folder}/SKILL.md`,
        rawGitHubUrl: `${RAW_BASE}/skills/${folder}/SKILL.md`,
        contentHash: contentHash(path.join(dir, folder, "SKILL.md")),
        installUrl: null,
        insidersInstallUrl: null,
        ...(meta.tags && { tags: meta.tags }),
        ...(meta.examples && { examples: meta.examples }),
        ...(references && references.length > 0 && { references }),
        ...(meta.deprecated && { deprecated: true }),
        ...(meta.deprecatedMessage && { deprecatedMessage: meta.deprecatedMessage }),
      };
    });
}

const collectionsIndex = buildCollectionsIndex();

function withCollections(item: ManifestItem): ManifestItem {
  const key = `${item.type}:${item.id}`;
  const itemCollections = collectionsIndex.get(key);
  if (itemCollections && itemCollections.length > 0) {
    return { ...item, collections: itemCollections.sort() };
  }
  return item;
}

const items = [...getAgents(), ...getInstructions(), ...getPrompts(), ...getSkills()].map(withCollections);

let existing: { items: unknown; generatedAt: string } | null = null;
try {
  existing = JSON.parse(fs.readFileSync(OUTPUT, "utf-8"));
} catch {
  // File doesn't exist or is invalid JSON
}
const itemsChanged = !existing || JSON.stringify(existing.items) !== JSON.stringify(items);

const manifest = {
  version: "1.0.0",
  generatedAt: itemsChanged ? new Date().toISOString() : existing!.generatedAt,
  items,
};

fs.writeFileSync(OUTPUT, JSON.stringify(manifest, null, 2) + "\n");
console.log(`✅ Generated ${OUTPUT} with ${manifest.items.length} customizations`);
