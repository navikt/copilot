import { describe, expect, it } from "vitest";
import manifest from "./copilot-manifest.json";

const NON_CANONICAL_GITHUB_MCP_PREFIXES = ["io.github.navikt/github-mcp/", "github-mcp-server/"] as const;
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

const EXPECTED_PORTABLE_GITHUB_TOOL_COUNTS: Record<(typeof AFFECTED_AGENT_IDS)[number], number> = {
  "aksel-agent": 11,
  "code-review": 5,
  forfatter: 2,
  "kafka-agent": 9,
  "nav-pilot-opus": 11,
  "nav-pilot": 11,
  "research-agent": 15,
  "rust-agent": 3,
  "security-champion-agent": 15,
};

describe("GitHub MCP agent tools", () => {
  const agents = manifest.items.filter((item) => item.type === "agent");

  it("does not use deprecated or CLI-internal GitHub MCP prefixes", () => {
    for (const agent of agents) {
      for (const tool of agent.tools ?? []) {
        for (const prefix of NON_CANONICAL_GITHUB_MCP_PREFIXES) {
          expect(tool.startsWith(prefix), `${agent.id} uses the non-canonical GitHub MCP prefix ${prefix}`).toBe(false);
        }
      }
    }
  });

  it.each(AFFECTED_AGENT_IDS)("%s retains its expected portable GitHub MCP tool count", (agentId) => {
    const agent = agents.find((item) => item.id === agentId);
    const portableTools = agent?.tools?.filter((tool) => tool.startsWith(PORTABLE_GITHUB_MCP_PREFIX)) ?? [];

    expect(portableTools, `${agentId} has an unexpected portable GitHub MCP tool count`).toHaveLength(
      EXPECTED_PORTABLE_GITHUB_TOOL_COUNTS[agentId]
    );
  });
});
