import type { Agent, AnyCustomization, CustomizationType } from "./customization-types";

export const INSTALL_DIRS: Record<Exclude<CustomizationType, "mcp">, string> = {
  agent: ".github/agents",
  instruction: ".github/instructions",
  prompt: ".github/prompts",
  skill: ".github/skills",
};

export const CLIENT_SUPPORT: Record<CustomizationType, string[]> = {
  agent: ["vscode", "nav-pilot", "github"],
  instruction: ["vscode", "nav-pilot", "github"],
  prompt: ["vscode", "nav-pilot"],
  skill: ["nav-pilot", "gh", "github"],
  mcp: ["vscode", "intellij", "cli", "github"],
};

export const CLIENT_LABELS: Record<string, string> = {
  vscode: "VS Code",
  intellij: "IntelliJ",
  cli: "Copilot CLI",
  "nav-pilot": "nav-pilot",
  gh: "GitHub CLI",
  github: "GitHub.com",
};

export function transportLabel(type: string): string {
  switch (type) {
    case "streamable-http":
      return "Streamable HTTP";
    case "sse":
      return "SSE";
    case "stdio":
      return "stdio";
    default:
      return type;
  }
}

export function getToolCount(item: AnyCustomization): number {
  if (item.type === "agent") return item.tools.length;
  if (item.type === "mcp") return item.tools?.length ?? 0;
  return 0;
}

export function getManualInstallCommand(item: AnyCustomization, allItems?: AnyCustomization[]): string {
  if (item.type === "mcp") return "";
  if (item.type === "skill") {
    const skillDir = `.github/skills/${item.name}`;
    const cmds = [`mkdir -p "${skillDir}"`, `curl -fsSL -o "${skillDir}/SKILL.md" "${item.rawGitHubUrl}"`];
    if (item.references && item.references.length > 0) {
      const refDirs = new Set<string>();
      for (const ref of item.references) {
        const dir = ref.path.substring(0, ref.path.lastIndexOf("/"));
        if (dir) refDirs.add(dir);
      }
      for (const dir of refDirs) {
        cmds.splice(1, 0, `mkdir -p "${skillDir}/${dir}"`);
      }
      for (const ref of item.references) {
        cmds.push(`curl -fsSL -o "${skillDir}/${ref.path}" "${ref.rawUrl}"`);
      }
    }
    return cmds.join(" && \\\n  ");
  }
  const dir = INSTALL_DIRS[item.type];
  const cmds = [
    `mkdir -p "${dir}" && curl -fsSL -o "${dir}/$(basename "${item.rawGitHubUrl}")" "${item.rawGitHubUrl}"`,
  ];

  if (item.type === "agent" && item.agentReferences && item.agentReferences.length > 0 && allItems) {
    const refUrls = resolveAgentReferenceUrls(item, allItems);
    for (const url of refUrls) {
      cmds.push(`curl -fsSL -o "${dir}/$(basename "${url}")" "${url}"`);
    }
  }

  return cmds.join(" && \\\n  ");
}

/**
 * Generate `gh skill install` command for a skill.
 * Uses short-name form — skills live at root `skills/` which is
 * auto-discovered by `gh skill` (agentskills.io convention).
 * Requires gh CLI ≥2.90.0.
 */
export function getGhSkillInstallCommand(item: AnyCustomization): string {
  if (item.type !== "skill") return "";
  return `gh skill install navikt/copilot ${item.name}`;
}

/**
 * Generate `nav-pilot install` command for a static customization.
 * Uses `item.id` which matches the stem name nav-pilot expects
 * (e.g., "github-actions" resolves to "github-actions.instructions.md").
 */
export function getNavPilotAddCommand(item: AnyCustomization): { repo: string; user: string } | null {
  if (item.type === "mcp") return null;
  const cmd = `nav-pilot install ${item.id}`;
  return { repo: cmd, user: `${cmd} --user` };
}

/**
 * Resolve agentReferences to raw GitHub URLs using the full manifest.
 * Returns URLs for referenced agents that exist in allItems.
 */
export function resolveAgentReferenceUrls(agent: Agent, allItems: AnyCustomization[]): string[] {
  if (!agent.agentReferences || agent.agentReferences.length === 0) return [];

  const agentMap = new Map<string, AnyCustomization>();
  for (const item of allItems) {
    if (item.type === "agent") agentMap.set(item.id, item);
  }

  return agent.agentReferences.filter((ref) => agentMap.has(ref)).map((ref) => agentMap.get(ref)!.rawGitHubUrl);
}

function buildPackageArgs(pkg: NonNullable<Extract<AnyCustomization, { type: "mcp" }>["packages"]>[0]): {
  runtime: string;
  args: string[];
} | null {
  const runtime = pkg.registryType === "npm" ? "pnx" : pkg.registryType === "pypi" ? "uvx" : null;
  if (!runtime) return null;
  const args: string[] = pkg.registryType === "npm" ? ["-y", pkg.identifier] : [pkg.identifier];
  if (pkg.packageArguments) {
    for (const arg of pkg.packageArguments) {
      if (arg.name) args.push(arg.name);
      if (arg.value) args.push(arg.value);
    }
  }
  return { runtime, args };
}

function getServerName(item: AnyCustomization): string {
  return item.name.split("/").pop() ?? item.name;
}

export function getMcpServerConfig(item: AnyCustomization): string {
  if (item.type !== "mcp") return "";
  const serverName = getServerName(item);

  if (item.packages && item.packages.length > 0) {
    const result = buildPackageArgs(item.packages[0]);
    if (!result) return "";
    const entry: Record<string, unknown> = { command: result.runtime, args: result.args };
    if (item.packages[0].environmentVariables) {
      const env: Record<string, string> = {};
      for (const v of item.packages[0].environmentVariables) {
        env[v.name] = v.isSecret ? "" : (v.description ?? "");
      }
      if (Object.keys(env).length > 0) entry.env = env;
    }
    return JSON.stringify({ [serverName]: entry }, null, 2);
  }

  if (item.remotes.length > 0) {
    return JSON.stringify({ [serverName]: { type: "http", url: item.remotes[0].url } }, null, 2);
  }

  return "";
}

export function getVsCodeAddMcpCommand(item: AnyCustomization): string {
  if (item.type !== "mcp") return "";
  const serverName = getServerName(item);

  if (item.packages && item.packages.length > 0) {
    const result = buildPackageArgs(item.packages[0]);
    if (!result) return "";
    const config: Record<string, unknown> = { name: serverName, command: result.runtime, args: result.args };
    if (item.packages[0].environmentVariables) {
      const env: Record<string, string> = {};
      for (const v of item.packages[0].environmentVariables) {
        env[v.name] = v.isSecret ? `\${input:${v.name}}` : (v.description ?? "");
      }
      if (Object.keys(env).length > 0) config.env = env;
    }
    return `code --add-mcp '${JSON.stringify(config)}'`;
  }

  if (item.remotes.length > 0) {
    return `code --add-mcp '${JSON.stringify({ name: serverName, type: "http", url: item.remotes[0].url })}'`;
  }

  return "";
}

export function getMcpAddFields(
  item: AnyCustomization
): { name: string; type: string; url?: string; command?: string; env?: string } | null {
  if (item.type !== "mcp") return null;
  const name = getServerName(item);

  if (item.remotes.length > 0) {
    return { name, type: "HTTP", url: item.remotes[0].url };
  }

  if (item.packages && item.packages.length > 0) {
    const result = buildPackageArgs(item.packages[0]);
    if (!result) return null;
    const envVars = item.packages[0].environmentVariables
      ?.map((v) => `${v.name}=${v.isSecret ? "..." : (v.description ?? "")}`)
      .join(", ");
    return { name, type: "STDIO", command: `${result.runtime} ${result.args.join(" ")}`, env: envVars };
  }

  return null;
}
