import { cacheLife, cacheTag } from "next/cache";
import type { Domain, McpServerCustomization } from "./customization-types";
import type { UsageExample } from "./manifest-types";

const MCP_REGISTRY_URL = process.env.MCP_REGISTRY_URL || "https://mcp-registry.nav.no";
const MCP_REGISTRY_PUBLIC_URL = process.env.MCP_REGISTRY_PUBLIC_URL || MCP_REGISTRY_URL;

interface ServerResponse {
  server: {
    name: string;
    description: string;
    version: string;
    websiteUrl?: string;
    repository?: { url: string; source: string; subfolder?: string };
    remotes?: { type: string; url: string }[];
    packages?: {
      registryType: string;
      identifier: string;
      runtimeHint?: string;
      transport: { type: string };
      packageArguments?: { type: string; name?: string; value?: string; description?: string }[];
      environmentVariables?: { name: string; description?: string; isRequired?: boolean; isSecret?: boolean }[];
    }[];
  };
  _meta: {
    "io.modelcontextprotocol.registry/official"?: {
      status: string;
      publishedAt: string;
      isLatest: boolean;
    };
    "io.github.navikt/registry"?: {
      tools?: string[];
      tags?: string[];
      examples?: UsageExample[];
    };
  };
}

interface ServerListResponse {
  servers: ServerResponse[];
  metadata: { count: number };
}

const TAG_TO_DOMAIN: Record<string, Domain> = {
  frontend: "frontend",
  nextjs: "frontend",
  svelte: "frontend",
  design: "design",
  figma: "design",
  testing: "testing",
  "browser-automation": "testing",
  "developer-tools": "general",
  "nav-internal": "platform",
  onboarding: "platform",
  github: "general",
  "version-control": "general",
  documentation: "general",
};

function deriveDomain(tags: string[]): Domain {
  for (const tag of tags) {
    const domain = TAG_TO_DOMAIN[tag];
    if (domain) return domain;
  }
  return "general";
}

function formatServerName(name: string): string {
  const parts = name.split("/");
  return parts.length > 1 ? parts[1] : name;
}

function getMcpRegistryHost(): string {
  const url = MCP_REGISTRY_PUBLIC_URL.replace(/^https?:\/\//, "");
  return url.replace(/\/$/, "");
}

function buildMcpInstallUrl(serverName: string, scheme: "vscode" | "vscode-insiders"): string {
  const host = getMcpRegistryHost();
  const encoded = encodeURIComponent(serverName);
  return `${scheme}:mcp/${host}/v0.1/servers/${encoded}/versions/latest`;
}

export async function getMcpServers(): Promise<McpServerCustomization[]> {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("mcp-servers");

  try {
    const res = await fetch(`${MCP_REGISTRY_URL}/v0.1/servers`);
    if (!res.ok) {
      console.error(`MCP registry returned ${res.status}`);
      return [];
    }

    const data: ServerListResponse = await res.json();
    return data.servers
      .filter((s) => s._meta["io.modelcontextprotocol.registry/official"]?.status === "active")
      .map((s) => {
        const navMeta = s._meta["io.github.navikt/registry"];
        const tags = navMeta?.tags ?? [];
        const examples = navMeta?.examples;
        return {
          id: `mcp-${s.server.name}`,
          name: formatServerName(s.server.name),
          description: s.server.description,
          type: "mcp" as const,
          domain: deriveDomain(tags),
          filePath: "",
          rawGitHubUrl: "",
          installUrl: buildMcpInstallUrl(s.server.name, "vscode"),
          insidersInstallUrl: buildMcpInstallUrl(s.server.name, "vscode-insiders"),
          version: s.server.version,
          remotes: s.server.remotes ?? [],
          websiteUrl: s.server.websiteUrl,
          repository: s.server.repository,
          tools: navMeta?.tools,
          tags,
          packages: s.server.packages,
          ...(examples && examples.length > 0 && { examples }),
        };
      });
  } catch (error) {
    console.error("Failed to fetch MCP servers:", error);
    return [];
  }
}
