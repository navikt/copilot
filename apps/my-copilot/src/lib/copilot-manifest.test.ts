import { describe, expect, it } from "vitest";
import manifest from "./copilot-manifest.json";

const DEPRECATED_GITHUB_MCP_PREFIX = "io.github.navikt/github-mcp/";
const PORTABLE_GITHUB_MCP_PREFIX = "github/";

const AFFECTED_AGENT_IDS = [
  "aksel-agent",
  "code-review",
  "forfatter",
  "kafka-agent",
  "nav-pilot-opus",
  "nav-pilot",
  "research-agent",
  "rust-agent",
  "security-champion-agent",
] as const;

describe("GitHub MCP agent tools", () => {
  const agents = manifest.items.filter((item) => item.type === "agent");

  it("does not use the deprecated Nav registry server prefix", () => {
    for (const agent of agents) {
      for (const tool of agent.tools ?? []) {
        expect(
          tool.startsWith(DEPRECATED_GITHUB_MCP_PREFIX),
          `${agent.id} still uses the deprecated GitHub MCP server`
        ).toBe(false);
      }
    }
  });

  it.each(AFFECTED_AGENT_IDS)("%s retains a portable GitHub MCP tool", (agentId) => {
    const agent = agents.find((item) => item.id === agentId);
    const portableTools = agent?.tools?.filter((tool) => tool.startsWith(PORTABLE_GITHUB_MCP_PREFIX)) ?? [];

    expect(portableTools, `${agentId} has no portable GitHub MCP tools`).not.toHaveLength(0);
  });
});
