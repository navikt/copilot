import { describe, expect, it } from "vitest";
import manifest from "./copilot-manifest.json";

const NON_CANONICAL_GITHUB_MCP_PREFIXES = ["io.github.navikt/github-mcp/", "github-mcp-server/"] as const;
const PORTABLE_GITHUB_MCP_PREFIX = "github/";

// Bare names Copilot CLI actually resolves, verified against `copilotToolsFingerprint`
// in a `--log-level all` wire log (CLI 1.0.83). `search` and `web` are VS Code tool-set
// names and resolve to nothing, which is how nine agents silently lost grep and glob.
const RESOLVING_BARE_TOOLS = new Set([
  "agent",
  "execute",
  "read",
  "edit",
  "todo",
  "bash",
  "read_bash",
  "stop_bash",
  "list_bash",
  "view",
  "create",
  "grep",
  "glob",
  "web_fetch",
  "task",
  "read_agent",
  "list_agents",
  "write_agent",
  "skill",
  "sql",
  "update_todo",
  "*",
]);

const AFFECTED_AGENT_IDS = [
  "aksel-agent",
  "code-review",
  "forfatter",
  "kafka-agent",
  "research-agent",
  "rust-agent",
  "security-champion-agent",
] as const;

const EXPECTED_PORTABLE_GITHUB_TOOL_COUNTS: Record<(typeof AFFECTED_AGENT_IDS)[number], number> = {
  "aksel-agent": 11,
  "code-review": 5,
  forfatter: 2,
  "kafka-agent": 9,
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

  it("names only tools the CLI resolves", () => {
    for (const agent of agents) {
      for (const tool of agent.tools ?? []) {
        if (tool.includes("/")) continue;
        expect(
          RESOLVING_BARE_TOOLS.has(tool),
          `${agent.id} names "${tool}", which Copilot CLI resolves to nothing`
        ).toBe(true);
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

// nav-pilot launches Copilot CLI with `--agent nav-pilot`. A `tools:` allowlist on that
// agent hides every MCP server the user has enabled (Figma, Aksel, fleet-gateway) from the
// model, and it is not a security boundary: cplt, fleet-gateway and server-side auth are.
describe("agents nav-pilot launches by default", () => {
  it.each(["nav-pilot", "nav-pilot-opus"])("%s has no tools allowlist", (agentId) => {
    const agent = manifest.items.find((item) => item.type === "agent" && item.id === agentId);
    expect(agent, `${agentId} missing from manifest`).toBeDefined();
    const tools = (agent as { tools?: string[] }).tools;
    // The generator writes [] for an agent without a tools: key.
    expect(!tools?.length || tools.includes("*"), `${agentId} restricts tools: ${tools}`).toBe(true);
  });
});
