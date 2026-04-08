import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import type { Domain, ExampleItem } from "../src/lib/manifest-types.ts";
import { VALID_DOMAINS } from "../src/lib/manifest-types.ts";

const REPO_ROOT = path.resolve(import.meta.dirname, "../../..");
const GITHUB_DIR = path.join(REPO_ROOT, ".github");
const RAW_BASE = "https://raw.githubusercontent.com/navikt/copilot/main/.github";
const OUTPUT = path.resolve(import.meta.dirname, "../src/lib/copilot-manifest.json");

interface Metadata {
  displayName?: string;
  description?: string;
  domain?: Domain;
  tags?: string[];
  references?: string[];
  excluded?: boolean;
  examples?: ExampleItem[];
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
  applyTo?: string;
  invocation?: string;
  tags?: string[];
  examples?: ExampleItem[];
  references?: { path: string; rawUrl: string }[];
}

function getAgents(): ManifestItem[] {
  const dir = path.join(GITHUB_DIR, "agents");
  if (!fs.existsSync(dir)) return [];

  return fs
    .readdirSync(dir)
    .filter((f) => f.endsWith(".agent.md"))
    .map((file) => {
      const content = fs.readFileSync(path.join(dir, file), "utf-8");
      const { data } = parseFrontmatter(content);
      const name = (data.name as string) || file.replace(".agent.md", "");
      const rawUrl = `${RAW_BASE}/agents/${file}`;
      const description = (data.description as string) || "";
      const tools = Array.isArray(data.tools) ? data.tools : [];
      const meta = loadMetadata(path.join(dir, file.replace(".agent.md", ".metadata.json")));

      return {
        id: name,
        name,
        description,
        type: "agent" as const,
        domain: meta.domain || "general",
        filePath: `.github/agents/${file}`,
        rawGitHubUrl: rawUrl,
        contentHash: contentHash(path.join(dir, file)),
        installUrl: buildInstallUrl("agent", rawUrl),
        insidersInstallUrl: buildInsidersInstallUrl("agent", rawUrl),
        tools,
        ...(meta.tags && { tags: meta.tags }),
        ...(meta.examples && { examples: meta.examples }),
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
        ...(meta.tags && { tags: meta.tags }),
        ...(meta.examples && { examples: meta.examples }),
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
        filePath: `.github/skills/${folder}/SKILL.md`,
        rawGitHubUrl: `${RAW_BASE}/skills/${folder}/SKILL.md`,
        contentHash: contentHash(path.join(dir, folder, "SKILL.md")),
        installUrl: null,
        insidersInstallUrl: null,
        ...(meta.tags && { tags: meta.tags }),
        ...(meta.examples && { examples: meta.examples }),
        ...(references && references.length > 0 && { references }),
      };
    });
}

const items = [...getAgents(), ...getInstructions(), ...getPrompts(), ...getSkills()];

const existing = fs.existsSync(OUTPUT) ? JSON.parse(fs.readFileSync(OUTPUT, "utf-8")) : null;
const itemsChanged = !existing || JSON.stringify(existing.items) !== JSON.stringify(items);

const manifest = {
  version: "1.0.0",
  generatedAt: itemsChanged ? new Date().toISOString() : existing.generatedAt,
  items,
};

fs.writeFileSync(OUTPUT, JSON.stringify(manifest, null, 2) + "\n");
console.log(`✅ Generated ${OUTPUT} with ${manifest.items.length} customizations`);
