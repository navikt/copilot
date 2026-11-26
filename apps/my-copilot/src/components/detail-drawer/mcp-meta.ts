// Pure helpers for MCP entries, without "use client", so server pages can call them too.
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import type { McpToolRisk } from "@/lib/customization-types";

const GATEWAY_HOSTS = ["fleet-gateway.intern.nav.no", "fleet-gateway.intern.dev.nav.no"];

// Registry URLs may hold the {{domain_internal}} template, so accept fleet-gateway.<nav domain> there.
export function isGatewayUrl(raw: string): boolean {
  try {
    const host = new URL(raw.replace("{{domain_internal}}", "intern.nav.no")).hostname;
    return GATEWAY_HOSTS.includes(host);
  } catch {
    return false;
  }
}

export function isFleetGatewayMcp(item: EnrichedCustomization): boolean {
  if (item.type !== "mcp") return false;
  return item.remotes.some((r) => isGatewayUrl(r.url)) || (item.tags?.includes("fleet-gateway") ?? false);
}

/** The page a gateway server gets of its own, from its registry name. */
export const serverPagePath = (serverId: string) => `/mcp/servere/${serverId.slice(serverId.lastIndexOf("/") + 1)}`;

export function riskOf(toolRisk: Record<string, McpToolRisk> | undefined, tool: string): McpToolRisk {
  return toolRisk?.[tool] ?? "read";
}

// shortcut: the registry has no local/remote field, so a server is local when no remote is off this machine; add a field if a remote on loopback ever means something else.
export function isLocalMcp(item: EnrichedCustomization): boolean {
  if (item.type !== "mcp") return false;
  // A package runs on this machine, whatever remote the entry also lists.
  if (item.packages?.length) return true;
  return !item.remotes.some((r) => {
    try {
      return !["127.0.0.1", "localhost", "[::1]"].includes(new URL(r.url.replace("{{domain_internal}}", "x")).hostname);
    } catch {
      return true;
    }
  });
}

export function riskCounts(tools: string[], toolRisk?: Record<string, McpToolRisk>) {
  const changes = tools.filter((tool) => riskOf(toolRisk, tool) !== "read").length;
  return `${tools.length} verktøy, ${changes} endrer`;
}
