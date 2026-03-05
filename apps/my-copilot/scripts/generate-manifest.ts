import fs from "node:fs";
import path from "node:path";

const REPO_ROOT = path.resolve(import.meta.dirname, "../../..");
const GITHUB_DIR = path.join(REPO_ROOT, ".github");
const RAW_BASE = "https://raw.githubusercontent.com/navikt/copilot/main/.github";
const OUTPUT = path.resolve(import.meta.dirname, "../src/lib/copilot-manifest.json");

type Domain = "platform" | "frontend" | "backend" | "auth" | "observability" | "general";

const DOMAIN_MAP: Record<string, Domain> = {
  "nais-agent": "platform",
  "auth-agent": "auth",
  "kafka-agent": "backend",
  "aksel-agent": "frontend",
  "observability-agent": "observability",
  "security-champion-agent": "auth",
  "research-agent": "general",
  "aksel-component": "frontend",
  "kafka-topic": "backend",
  "nais-manifest": "platform",
  "aksel-spacing": "frontend",
  "flyway-migration": "backend",
  "kotlin-app-config": "backend",
  "observability-setup": "observability",
  "tokenx-auth": "auth",
  database: "backend",
  "kotlin-ktor": "backend",
  "nextjs-aksel": "frontend",
  testing: "general",
};

function parseFrontmatter(content: string): { data: Record<string, string | string[]>; body: string } {
  const match = content.match(/^---\s*\n([\s\S]*?)\n---\s*\n([\s\S]*)$/);
  if (!match) return { data: {}, body: content };

  const raw = match[1];
  const body = match[2];
  const data: Record<string, string | string[]> = {};

  let currentKey: string | null = null;
  let arrayValues: string[] = [];

  for (const line of raw.split("\n")) {
    const keyValue = line.match(/^(\w[\w-]*)\s*:\s*"?([^"]*)"?\s*$/);
    if (keyValue) {
      if (currentKey && arrayValues.length > 0) {
        data[currentKey] = arrayValues;
        arrayValues = [];
      }
      currentKey = keyValue[1];
      const value = keyValue[2].trim();
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

interface ManifestItem {
  id: string;
  name: string;
  description: string;
  type: "agent" | "instruction" | "prompt" | "skill";
  domain: Domain;
  filePath: string;
  rawGitHubUrl: string;
  installUrl: string | null;
  insidersInstallUrl: string | null;
  tools?: string[];
  applyTo?: string;
  invocation?: string;
}

function getAgents(): ManifestItem[] {
  const dir = path.join(GITHUB_DIR, "agents");
  if (!fs.existsSync(dir)) return [];

  return fs
    .readdirSync(dir)
    .filter((f) => f.endsWith(".agent.md"))
    .map((file) => {
      const content = fs.readFileSync(path.join(dir, file), "utf-8");
      const { data, body } = parseFrontmatter(content);
      const name = (data.name as string) || file.replace(".agent.md", "");
      const rawUrl = `${RAW_BASE}/agents/${file}`;
      const description = (data.description as string) || "";
      const tools = Array.isArray(data.tools) ? data.tools : [];

      const firstParagraph = body
        .split("\n\n")
        .find((p) => p.trim() && !p.startsWith("#"))
        ?.trim();

      return {
        id: name,
        name,
        description: firstParagraph || description,
        type: "agent" as const,
        domain: DOMAIN_MAP[name] || "general",
        filePath: `.github/agents/${file}`,
        rawGitHubUrl: rawUrl,
        installUrl: buildInstallUrl("agent", rawUrl),
        insidersInstallUrl: buildInsidersInstallUrl("agent", rawUrl),
        tools,
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
      const { data, body } = parseFrontmatter(content);
      const id = file.replace(".instructions.md", "");
      const applyTo = (data.applyTo as string) || "";

      const heading = body.match(/^#\s+(.+)$/m);
      const displayName = heading ? heading[1] : id;

      const firstParagraph = body
        .split("\n\n")
        .find((p) => p.trim() && !p.startsWith("#"))
        ?.trim();

      const rawUrl = `${RAW_BASE}/instructions/${file}`;

      return {
        id,
        name: displayName,
        description: firstParagraph || `Instruksjoner for ${applyTo}`,
        type: "instruction" as const,
        domain: DOMAIN_MAP[id] || "general",
        filePath: `.github/instructions/${file}`,
        rawGitHubUrl: rawUrl,
        installUrl: buildInstallUrl("instructions", rawUrl),
        insidersInstallUrl: buildInsidersInstallUrl("instructions", rawUrl),
        applyTo,
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

      return {
        id: name,
        name,
        description: (data.description as string) || "",
        type: "prompt" as const,
        domain: DOMAIN_MAP[name] || "general",
        filePath: `.github/prompts/${file}`,
        rawGitHubUrl: rawUrl,
        installUrl: buildInstallUrl("prompt", rawUrl),
        insidersInstallUrl: buildInsidersInstallUrl("prompt", rawUrl),
        invocation: `#${name}`,
      };
    });
}

const EXCLUDED_SKILLS = new Set(["ai-news-research"]);

function getSkills(): ManifestItem[] {
  const dir = path.join(GITHUB_DIR, "skills");
  if (!fs.existsSync(dir)) return [];

  return fs
    .readdirSync(dir)
    .filter((f) => {
      if (EXCLUDED_SKILLS.has(f)) return false;
      const skillFile = path.join(dir, f, "SKILL.md");
      return fs.existsSync(skillFile);
    })
    .map((folder) => {
      const content = fs.readFileSync(path.join(dir, folder, "SKILL.md"), "utf-8");
      const { data } = parseFrontmatter(content);
      const name = (data.name as string) || folder;

      return {
        id: name,
        name,
        description: (data.description as string) || "",
        type: "skill" as const,
        domain: DOMAIN_MAP[name] || "general",
        filePath: `.github/skills/${folder}/SKILL.md`,
        rawGitHubUrl: `${RAW_BASE}/skills/${folder}/SKILL.md`,
        installUrl: null,
        insidersInstallUrl: null,
      };
    });
}

const manifest = {
  version: "1.0.0",
  generatedAt: new Date().toISOString(),
  items: [...getAgents(), ...getInstructions(), ...getPrompts(), ...getSkills()],
};

fs.writeFileSync(OUTPUT, JSON.stringify(manifest, null, 2) + "\n");
console.log(`✅ Generated ${OUTPUT} with ${manifest.items.length} customizations`);
