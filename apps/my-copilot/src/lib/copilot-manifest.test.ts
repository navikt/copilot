import { readFileSync } from "node:fs";
import { join } from "node:path";
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
// The launch asks for the read-only GitHub MCP tools with --add-github-mcp-tool instead.
// Reads the agent files, not the manifest: the generator writes [] for a flow list.
describe("agents nav-pilot launches by default", () => {
  it.each(["nav-pilot", "nav-pilot-opus"])("%s has no tools allowlist", (agentId) => {
    const entries = toolsEntries(readFileSync(join(AGENTS_DIR, `${agentId}.agent.md`), "utf8"));
    expect(entries === null || entries.every((t) => t === "*"), `${agentId} restricts tools: ${entries}`).toBe(true);
  });

  it("toolsEntries reads block and flow lists", () => {
    expect(toolsEntries("---\nname: a\n---\n")).toBeNull();
    expect(toolsEntries("---\ntools: [read, 'github/x']\n---\n")).toEqual(["read", "github/x"]);
    expect(toolsEntries('---\ntools:\n  - read\n  - "*"\nmodel: x\n---\n')).toEqual(["read", "*"]);
    expect(toolsEntries("---\ntools: ['*']\n---\n")).toEqual(["*"]);
  });
});

const AGENTS_DIR = join(__dirname, "../../../../agents");

// toolsEntries returns the frontmatter tools: entries (block or flow list, or a scalar),
// or null when there is no tools: key.
function toolsEntries(src: string): string[] | null {
  const fm = src.match(/^---\n([\s\S]*?)\n---/)?.[1] ?? "";
  const lines = fm.split("\n");
  const i = lines.findIndex((l) => /^tools\s*:/.test(l));
  if (i < 0) return null;
  const unquote = (t: string) => t.trim().replace(/^['"]|['"]$/g, "");
  const inline = lines[i].replace(/^tools\s*:/, "").trim();
  if (inline)
    return inline
      .replace(/^\[|\]$/g, "")
      .split(",")
      .map(unquote)
      .filter(Boolean);
  const out: string[] = [];
  for (const l of lines.slice(i + 1)) {
    const m = l.match(/^\s+-\s*(.*)$/);
    if (!m) break;
    out.push(unquote(m[1]));
  }
  return out;
}
