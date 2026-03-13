import { enrichWithUsage } from "./enrich-customizations";
import type { AnyCustomization } from "./customization-types";
import type { CustomizationUsage } from "./types";

function makeItem(overrides: Partial<AnyCustomization> & { type: "agent"; name: string }): AnyCustomization {
  return {
    id: overrides.name,
    description: "desc",
    domain: "general",
    filePath: overrides.filePath ?? `.github/agents/${overrides.name}.agent.md`,
    rawGitHubUrl: "",
    installUrl: "",
    insidersInstallUrl: null,
    tags: [],
    examples: [],
    tools: [],
    ...overrides,
  };
}

describe("enrichWithUsage", () => {
  const usage: CustomizationUsage[] = [
    { category: "agents", file_name: "nais-platform.agent.md", repo_count: 42, sample_repos: ["repo-a", "repo-b"] },
    { category: "instructions", file_name: "kotlin-ktor.instructions.md", repo_count: 10, sample_repos: ["repo-c"] },
  ];

  it("matches items by category and file name", () => {
    const items: AnyCustomization[] = [
      makeItem({ type: "agent", name: "nais-platform", filePath: ".github/agents/nais-platform.agent.md" }),
    ];

    const result = enrichWithUsage(items, usage);
    expect(result).toHaveLength(1);
    expect(result[0].usageCount).toBe(42);
    expect(result[0].usedBy).toEqual(["repo-a", "repo-b"]);
  });

  it("returns zero usage for unmatched items", () => {
    const items: AnyCustomization[] = [
      makeItem({ type: "agent", name: "unknown-agent", filePath: ".github/agents/unknown-agent.agent.md" }),
    ];

    const result = enrichWithUsage(items, usage);
    expect(result[0].usageCount).toBe(0);
    expect(result[0].usedBy).toEqual([]);
  });

  it("handles empty usage data", () => {
    const items: AnyCustomization[] = [
      makeItem({ type: "agent", name: "test", filePath: ".github/agents/test.agent.md" }),
    ];

    const result = enrichWithUsage(items, []);
    expect(result[0].usageCount).toBe(0);
  });

  it("handles items without matching filePath pattern", () => {
    const items: AnyCustomization[] = [
      makeItem({ type: "agent", name: "mcp-server", filePath: "some/other/path.md" }),
    ];

    const result = enrichWithUsage(items, usage);
    expect(result[0].usageCount).toBe(0);
  });

  it("preserves original item properties", () => {
    const items: AnyCustomization[] = [
      makeItem({ type: "agent", name: "nais-platform", filePath: ".github/agents/nais-platform.agent.md" }),
    ];

    const result = enrichWithUsage(items, usage);
    expect(result[0].name).toBe("nais-platform");
    expect(result[0].type).toBe("agent");
    expect(result[0].description).toBe("desc");
  });

  it("matches skills by directory name, not SKILL.md", () => {
    const skillUsage: CustomizationUsage[] = [
      { category: "skills", file_name: "observability-setup", repo_count: 7, sample_repos: ["repo-x"] },
    ];
    const items: AnyCustomization[] = [
      makeItem({
        type: "agent" as never,
        name: "observability-setup",
        filePath: ".github/skills/observability-setup/SKILL.md",
      }),
    ];

    const result = enrichWithUsage(items, skillUsage);
    expect(result[0].usageCount).toBe(7);
    expect(result[0].usedBy).toEqual(["repo-x"]);
  });
});
