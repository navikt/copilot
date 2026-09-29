import type { Agent, AnyCustomization, CustomizationType } from "./customization-types";

// The nav-pilot and cplt install commands, shared by every page that shows them
// so the landing page, docs, lokal and the setup wizard can't drift apart.
export const NAV_PILOT_BREW_INSTALL = "brew install navikt/tap/nav-pilot navikt/tap/cplt";
export const NAV_PILOT_QUICKSTART = `${NAV_PILOT_BREW_INSTALL} && nav-pilot`;
export const CPLT_BREW_INSTALL = "brew install navikt/tap/cplt";
export const NAV_PILOT_BREW_UPGRADE = "brew upgrade navikt/tap/nav-pilot";
// The first nav-pilot release with `nav-pilot mcp` (navikt/copilot#1322):
// nav-pilot/2026.09.29-150447-7214b7d.
export const NAV_PILOT_MCP_MIN_VERSION = "2026.09.29-150447";
export const NAV_PILOT_INSTALL_SCRIPT =
  "curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash";
export const CPLT_INSTALL_SCRIPT = "curl -fsSL https://raw.githubusercontent.com/navikt/cplt/main/install.sh | bash";

const APT_KEYRING = "/usr/share/keyrings/navikt-archive-keyring.gpg";
const APT_KEYRING_TMP = "/tmp/navikt-archive-keyring.gpg";

// One && chain: each step runs only if the one before it worked. A blocked download
// used to leave a 0-byte keyring and end in "Unable to locate package" (#1099); now the
// chain stops and the || branch points at the install script. No `set -e`, because the
// block is pasted into the user's interactive shell.
function aptInstall(packages: string, failed: string): string {
  return [
    `curl -fsSL -o ${APT_KEYRING_TMP} https://navikt.github.io/apt/keyring/navikt-archive-keyring.gpg \\`,
    `  && test -s ${APT_KEYRING_TMP} \\`,
    `  && sudo install -m 644 ${APT_KEYRING_TMP} ${APT_KEYRING} \\`,
    `  && echo "deb [signed-by=${APT_KEYRING}] https://navikt.github.io/apt stable main" \\`,
    `    | sudo tee /etc/apt/sources.list.d/navikt.list >/dev/null \\`,
    `  && sudo apt update && sudo apt install ${packages} \\`,
    `  || echo "${failed}" >&2`,
  ].join("\n");
}

export const NAV_PILOT_APT_INSTALL = aptInstall(
  "nav-pilot cplt",
  `Klarte ikke å installere fra apt-arkivet. Sjekk at du når https://navikt.github.io/apt, eller bruk installasjonsskriptet: ${NAV_PILOT_INSTALL_SCRIPT}`
);
// In English: only the English /cplt pages show it.
export const CPLT_APT_INSTALL = aptInstall(
  "cplt",
  `Could not install from the apt archive. Check that you can reach https://navikt.github.io/apt, or use the install script: ${CPLT_INSTALL_SCRIPT}`
);

export type InstallOs = "mac" | "linux" | "windows";

// Upgrade nav-pilot, cplt and the clients, by how they were installed. brew and
// apt own their binaries, so nav-pilot upgrade and cplt update only print the
// package manager's command there; the self-updaters are for script installs.
// Copilot CLI and opencode update themselves whichever way nav-pilot installed them.
const CLIENT_UPGRADE = `copilot update    # Copilot CLI
opencode upgrade  # hvis du bruker opencode`;
export type UpgradeMethod = "brew" | "apt" | "script";
export const UPGRADE_COMMANDS: Record<UpgradeMethod, string> = {
  brew: `brew update
brew upgrade navikt/tap/nav-pilot navikt/tap/cplt
${CLIENT_UPGRADE}`,
  apt: `sudo apt update && sudo apt upgrade nav-pilot cplt
${CLIENT_UPGRADE}`,
  script: `nav-pilot upgrade
cplt update
${CLIENT_UPGRADE}`,
};

// One command per OS, the one the /nav-pilot hero shows; windows means WSL.
// The Kom i gang wizard reads the same map, so the two can't disagree (#1187).
export const NAV_PILOT_INSTALL: Record<InstallOs, string> = {
  mac: NAV_PILOT_BREW_INSTALL,
  linux: NAV_PILOT_INSTALL_SCRIPT,
  windows: NAV_PILOT_INSTALL_SCRIPT,
};

// The same sources nav-pilot's first-run wizard offers (opencodeInstallCommand
// in cli/nav-pilot/internal/cli/config_setup.go), pinned to OpenCodeInstallVersion.
export const OPENCODE_INSTALL: Record<InstallOs, string> = {
  mac: "brew install anomalyco/tap/opencode",
  linux: "curl -fsSL https://opencode.ai/install | bash -s -- --version 1.18.32",
  windows: "curl -fsSL https://opencode.ai/install | bash -s -- --version 1.18.32",
};

// Maps navigator.userAgentData.platform or navigator.userAgent to the install view.
// Anything we can't place (Android, BSD) gets macOS, the most common case.
export function installOsFromPlatform(platform: string): InstallOs {
  const p = platform.toLowerCase();
  if (p.includes("mac")) return "mac";
  if (p.includes("win")) return "windows";
  if (p.includes("cros") || p.includes("chrome os")) return "linux"; // ChromeOS's Linux container is Debian
  if (p.includes("linux") && !p.includes("android")) return "linux";
  return "mac";
}

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
 * Uses artifact file stem when available, matching nav-pilot resolver semantics
 * (e.g., ".github/agents/security-champion.agent.md" => "security-champion"),
 * and includes explicit --type to avoid cross-type name ambiguity.
 */
export function getNavPilotAddCommand(item: AnyCustomization): { repo: string; user: string } | null {
  if (item.type === "mcp") return null;
  const cmd = `nav-pilot install ${getNavPilotInstallName(item)} --type ${item.type}`;
  return { repo: `${cmd} --repo`, user: `${cmd} --user` };
}

function stemFromPath(path: string | undefined, suffix: string): string | null {
  const fileName = path?.split("/").pop();
  if (!fileName || !fileName.endsWith(suffix)) return null;
  return fileName.slice(0, -suffix.length);
}

function getNavPilotInstallName(item: Exclude<AnyCustomization, { type: "mcp" }>): string {
  if (item.type === "skill") return item.name;

  const suffix = item.type === "agent" ? ".agent.md" : item.type === "instruction" ? ".instructions.md" : ".prompt.md";
  return stemFromPath(item.filePath, suffix) ?? stemFromPath(item.rawGitHubUrl, suffix) ?? item.id;
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
  const runtime = pkg.registryType === "npm" ? "pnpm" : pkg.registryType === "pypi" ? "uvx" : null;
  if (!runtime) return null;
  const packageIdentifier =
    pkg.registryType === "npm" && pkg.version ? `${pkg.identifier}@${pkg.version}` : pkg.identifier;
  const args: string[] = pkg.registryType === "npm" ? ["dlx", packageIdentifier] : [packageIdentifier];
  if (pkg.packageArguments) {
    for (const arg of pkg.packageArguments) {
      if (arg.name) args.push(arg.name);
      if (arg.value) args.push(arg.value);
    }
  }
  return { runtime, args };
}

function getRemoteConfigType(remoteType: string): "http" | "sse" {
  return remoteType === "sse" ? "sse" : "http";
}

export function getMcpServerConfig(item: AnyCustomization): string {
  if (item.type !== "mcp") return "";
  const serverName = item.serverId;

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
    return JSON.stringify(
      { [serverName]: { type: getRemoteConfigType(item.remotes[0].type), url: item.remotes[0].url } },
      null,
      2
    );
  }

  return "";
}

export function getVsCodeAddMcpCommand(item: AnyCustomization): string {
  if (item.type !== "mcp") return "";
  const serverName = item.serverId;

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
    return `code --add-mcp '${JSON.stringify({
      name: serverName,
      type: getRemoteConfigType(item.remotes[0].type),
      url: item.remotes[0].url,
    })}'`;
  }

  return "";
}

export function getMcpAddFields(
  item: AnyCustomization
): { name: string; type: string; url?: string; command?: string; env?: string } | null {
  if (item.type !== "mcp") return null;
  const name = item.serverId;

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
