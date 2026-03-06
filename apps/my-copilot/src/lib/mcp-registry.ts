import { cacheLife, cacheTag } from "next/cache";
import type { McpServerCustomization } from "./customization-types";

const MCP_REGISTRY_URL = process.env.MCP_REGISTRY_URL || "https://mcp-registry.nav.no";

interface ServerResponse {
  server: {
    name: string;
    description: string;
    version: string;
    remotes?: { type: string; url: string }[];
  };
  _meta: {
    "io.modelcontextprotocol.registry/official"?: {
      status: string;
      publishedAt: string;
      isLatest: boolean;
    };
  };
}

interface ServerListResponse {
  servers: ServerResponse[];
  metadata: { count: number };
}

function formatServerName(name: string): string {
  const parts = name.split("/");
  return parts.length > 1 ? parts[1] : name;
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
      .map((s) => ({
        id: `mcp-${s.server.name}`,
        name: formatServerName(s.server.name),
        description: s.server.description,
        type: "mcp" as const,
        domain: "general" as const,
        filePath: "",
        rawGitHubUrl: "",
        installUrl: null,
        insidersInstallUrl: null,
        version: s.server.version,
        remotes: s.server.remotes ?? [],
      }));
  } catch (error) {
    console.error("Failed to fetch MCP servers:", error);
    return [];
  }
}
