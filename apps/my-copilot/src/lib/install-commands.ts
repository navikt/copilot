import type { AnyCustomization, CustomizationType } from "./customization-types";

export const INSTALL_DIRS: Record<Exclude<CustomizationType, "mcp">, string> = {
  agent: ".github/agents",
  instruction: ".github/instructions",
  prompt: ".github/prompts",
  skill: ".github/skills",
};

export const CLIENT_SUPPORT: Record<CustomizationType, string[]> = {
  instruction: ["vscode", "intellij", "cli", "github"],
  agent: ["vscode", "intellij", "cli", "github"],
  prompt: ["vscode", "intellij"],
  skill: ["vscode", "intellij", "cli", "github"],
  mcp: ["vscode", "intellij", "cli", "github"],
};

export const CLIENT_LABELS: Record<string, string> = {
  vscode: "VS Code",
  intellij: "IntelliJ",
  cli: "Copilot CLI",
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

export function getManualInstallCommand(item: AnyCustomization): string {
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
  return `mkdir -p "${dir}" && curl -fsSL -o "${dir}/$(basename "${item.rawGitHubUrl}")" "${item.rawGitHubUrl}"`;
}

function buildPackageArgs(pkg: NonNullable<Extract<AnyCustomization, { type: "mcp" }>["packages"]>[0]): {
  runtime: string;
  args: string[];
} | null {
  const runtime = pkg.registryType === "npm" ? "npx" : pkg.registryType === "pypi" ? "uvx" : null;
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
