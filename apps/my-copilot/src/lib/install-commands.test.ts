import type { Agent, Instruction, Prompt, Skill, McpServerCustomization } from "./customization-types";
import {
  transportLabel,
  getToolCount,
  getManualInstallCommand,
  getGhSkillInstallCommand,
  getNavPilotAddCommand,
  getMcpServerConfig,
  getVsCodeAddMcpCommand,
  getMcpAddFields,
  resolveAgentReferenceUrls,
  INSTALL_DIRS,
  CLIENT_SUPPORT,
  CLIENT_LABELS,
} from "./install-commands";

const base = {
  id: "1",
  description: "desc",
  domain: "platform" as const,
  filePath: "path",
  rawGitHubUrl: "https://raw.githubusercontent.com/navikt/copilot/main/.github/agents/nais.agent.md",
  installUrl: null,
  insidersInstallUrl: null,
};

const agent: Agent = { ...base, type: "agent", name: "nais-platform", tools: ["run_in_terminal", "read_file"] };
const agentWithRefs: Agent = {
  ...base,
  type: "agent",
  name: "security-champion-agent",
  rawGitHubUrl: "https://raw.githubusercontent.com/navikt/copilot/main/.github/agents/security-champion.agent.md",
  tools: ["run_in_terminal"],
  agentReferences: ["auth-agent", "nais-agent"],
};
const authAgent: Agent = {
  ...base,
  type: "agent",
  name: "auth-agent",
  id: "auth-agent",
  rawGitHubUrl: "https://raw.githubusercontent.com/navikt/copilot/main/.github/agents/auth.agent.md",
  tools: [],
};
const naisAgent: Agent = {
  ...base,
  type: "agent",
  name: "nais-agent",
  id: "nais-agent",
  rawGitHubUrl: "https://raw.githubusercontent.com/navikt/copilot/main/.github/agents/nais.agent.md",
  tools: [],
};
const allItems = [agent, agentWithRefs, authAgent, naisAgent];
const instruction: Instruction = {
  ...base,
  type: "instruction",
  id: "nextjs-aksel",
  name: "Next.js/Aksel Development",
  applyTo: "src/**/*.tsx",
};
const prompt: Prompt = { ...base, type: "prompt", name: "code-review.prompt.md", invocation: "/code-review" };
const skill: Skill = {
  ...base,
  type: "skill",
  name: "aksel-spacing",
  rawGitHubUrl: "https://raw.githubusercontent.com/navikt/copilot/main/.github/skills/aksel-spacing/SKILL.md",
};
const skillWithRefs: Skill = {
  ...base,
  type: "skill",
  name: "observability-setup",
  rawGitHubUrl: "https://raw.githubusercontent.com/navikt/copilot/main/.github/skills/observability-setup/SKILL.md",
  references: [
    {
      path: "references/grafana-queries.md",
      rawUrl:
        "https://raw.githubusercontent.com/navikt/copilot/main/.github/skills/observability-setup/references/grafana-queries.md",
    },
  ],
};

const remoteMcp: McpServerCustomization = {
  ...base,
  type: "mcp",
  name: "io.github.navikt/github-mcp",
  version: "1.0.0",
  remotes: [{ type: "streamable-http", url: "https://mcp.nav.no/mcp" }],
};

const npmMcp: McpServerCustomization = {
  ...base,
  type: "mcp",
  name: "io.github.navikt/figma-mcp",
  version: "1.0.0",
  remotes: [],
  packages: [
    {
      registryType: "npm",
      identifier: "@anthropic/figma-mcp",
      transport: { type: "stdio" },
      packageArguments: [{ type: "positional", name: "--port", value: "3333" }],
      environmentVariables: [
        { name: "FIGMA_TOKEN", isSecret: true, isRequired: true },
        { name: "DEBUG", description: "Enable debug logging", isSecret: false },
      ],
    },
  ],
};

const pypiMcp: McpServerCustomization = {
  ...base,
  type: "mcp",
  name: "io.github.navikt/python-mcp",
  version: "1.0.0",
  remotes: [],
  packages: [
    {
      registryType: "pypi",
      identifier: "mcp-server-python",
      transport: { type: "stdio" },
    },
  ],
};

const emptyMcp: McpServerCustomization = {
  ...base,
  type: "mcp",
  name: "io.github.navikt/empty-mcp",
  version: "1.0.0",
  remotes: [],
};

describe("transportLabel", () => {
  it("maps known transport types", () => {
    expect(transportLabel("streamable-http")).toBe("Streamable HTTP");
    expect(transportLabel("sse")).toBe("SSE");
    expect(transportLabel("stdio")).toBe("stdio");
  });

  it("returns unknown types as-is", () => {
    expect(transportLabel("websocket")).toBe("websocket");
  });
});

describe("INSTALL_DIRS", () => {
  it("maps all non-mcp types", () => {
    expect(INSTALL_DIRS.agent).toBe(".github/agents");
    expect(INSTALL_DIRS.instruction).toBe(".github/instructions");
    expect(INSTALL_DIRS.prompt).toBe(".github/prompts");
    expect(INSTALL_DIRS.skill).toBe(".github/skills");
  });
});

describe("CLIENT_SUPPORT", () => {
  it("includes nav-pilot for all static types", () => {
    expect(CLIENT_SUPPORT.agent).toContain("nav-pilot");
    expect(CLIENT_SUPPORT.instruction).toContain("nav-pilot");
    expect(CLIENT_SUPPORT.prompt).toContain("nav-pilot");
    expect(CLIENT_SUPPORT.skill).toContain("nav-pilot");
  });

  it("includes vscode for agents, instructions, prompts (have installUrl)", () => {
    expect(CLIENT_SUPPORT.agent).toContain("vscode");
    expect(CLIENT_SUPPORT.instruction).toContain("vscode");
    expect(CLIENT_SUPPORT.prompt).toContain("vscode");
  });

  it("does not include vscode for skills (no one-click install)", () => {
    expect(CLIENT_SUPPORT.skill).not.toContain("vscode");
  });

  it("includes gh for skills only", () => {
    expect(CLIENT_SUPPORT.skill).toContain("gh");
    expect(CLIENT_SUPPORT.agent).not.toContain("gh");
  });

  it("does not include intellij or cli for static types", () => {
    for (const type of ["agent", "instruction", "prompt", "skill"] as const) {
      expect(CLIENT_SUPPORT[type]).not.toContain("intellij");
      expect(CLIENT_SUPPORT[type]).not.toContain("cli");
    }
  });

  it("keeps intellij and cli for mcp", () => {
    expect(CLIENT_SUPPORT.mcp).toContain("vscode");
    expect(CLIENT_SUPPORT.mcp).toContain("intellij");
    expect(CLIENT_SUPPORT.mcp).toContain("cli");
  });
});

describe("CLIENT_LABELS", () => {
  it("maps all client keys to display names", () => {
    expect(CLIENT_LABELS.vscode).toBe("VS Code");
    expect(CLIENT_LABELS.intellij).toBe("IntelliJ");
    expect(CLIENT_LABELS.cli).toBe("Copilot CLI");
    expect(CLIENT_LABELS["nav-pilot"]).toBe("nav-pilot");
    expect(CLIENT_LABELS.gh).toBe("GitHub CLI");
    expect(CLIENT_LABELS.github).toBe("GitHub.com");
  });
});

describe("getToolCount", () => {
  it("returns agent tool count", () => {
    expect(getToolCount(agent)).toBe(2);
  });

  it("returns mcp tool count", () => {
    const mcpWithTools = { ...remoteMcp, tools: ["tool1", "tool2", "tool3"] };
    expect(getToolCount(mcpWithTools)).toBe(3);
  });

  it("returns 0 for mcp without tools", () => {
    expect(getToolCount(remoteMcp)).toBe(0);
  });

  it("returns 0 for non-agent/mcp types", () => {
    expect(getToolCount(instruction)).toBe(0);
    expect(getToolCount(prompt)).toBe(0);
    expect(getToolCount(skill)).toBe(0);
  });
});

describe("getManualInstallCommand", () => {
  it("generates curl command for agent", () => {
    const cmd = getManualInstallCommand(agent);
    expect(cmd).toContain('mkdir -p ".github/agents"');
    expect(cmd).toContain("curl -fsSL");
    expect(cmd).toContain("nais.agent.md");
  });

  it("generates curl command for instruction", () => {
    const cmd = getManualInstallCommand(instruction);
    expect(cmd).toContain('mkdir -p ".github/instructions"');
  });

  it("generates curl command for prompt", () => {
    const cmd = getManualInstallCommand(prompt);
    expect(cmd).toContain('mkdir -p ".github/prompts"');
  });

  it("creates skill-specific subdirectory", () => {
    const cmd = getManualInstallCommand(skill);
    expect(cmd).toContain('mkdir -p ".github/skills/aksel-spacing"');
    expect(cmd).toContain("curl -fsSL");
    expect(cmd).toContain(
      'curl -fsSL -o ".github/skills/aksel-spacing/SKILL.md" "https://raw.githubusercontent.com/navikt/copilot/main/.github/skills/aksel-spacing/SKILL.md"'
    );
    expect(cmd).not.toContain("references");
  });

  it("generates multi-curl for skill with references", () => {
    const cmd = getManualInstallCommand(skillWithRefs);
    expect(cmd).toContain('mkdir -p ".github/skills/observability-setup"');
    expect(cmd).toContain('mkdir -p ".github/skills/observability-setup/references"');
    expect(cmd).toContain("SKILL.md");
    expect(cmd).toContain("grafana-queries.md");
  });

  it("returns empty string for mcp", () => {
    expect(getManualInstallCommand(remoteMcp)).toBe("");
  });

  it("includes referenced agents when allItems is provided", () => {
    const cmd = getManualInstallCommand(agentWithRefs, allItems);
    expect(cmd).toContain("security-champion.agent.md");
    expect(cmd).toContain("auth.agent.md");
    expect(cmd).toContain("nais.agent.md");
    const lines = cmd.split(" && \\\n  ");
    expect(lines).toHaveLength(3);
  });

  it("omits references when allItems is not provided", () => {
    const cmd = getManualInstallCommand(agentWithRefs);
    expect(cmd).toContain("security-champion.agent.md");
    expect(cmd).not.toContain("auth.agent.md");
    expect(cmd).not.toContain("nais.agent.md");
  });

  it("works for agent without references", () => {
    const cmd = getManualInstallCommand(agent, allItems);
    expect(cmd).toContain("nais.agent.md");
    expect(cmd).not.toContain("auth.agent.md");
  });
});

describe("getGhSkillInstallCommand", () => {
  it("generates gh skill install command for skill", () => {
    const cmd = getGhSkillInstallCommand(skill);
    expect(cmd).toBe("gh skill install navikt/copilot .github/skills/aksel-spacing/SKILL.md");
  });

  it("generates gh skill install command for skill with references", () => {
    const cmd = getGhSkillInstallCommand(skillWithRefs);
    expect(cmd).toBe("gh skill install navikt/copilot .github/skills/observability-setup/SKILL.md");
  });

  it("returns empty string for non-skill types", () => {
    expect(getGhSkillInstallCommand(agent)).toBe("");
    expect(getGhSkillInstallCommand(instruction)).toBe("");
    expect(getGhSkillInstallCommand(prompt)).toBe("");
  });
});

describe("getNavPilotAddCommand", () => {
  it("generates nav-pilot add command for agent", () => {
    const result = getNavPilotAddCommand(agent)!;
    expect(result.repo).toBe("nav-pilot add agent 1");
    expect(result.user).toBe("nav-pilot add agent 1 --user");
  });

  it("generates nav-pilot add command for agent with explicit id", () => {
    const result = getNavPilotAddCommand(authAgent)!;
    expect(result.repo).toBe("nav-pilot add agent auth-agent");
    expect(result.user).toBe("nav-pilot add agent auth-agent --user");
  });

  it("generates nav-pilot add command for instruction using id, not display name", () => {
    const result = getNavPilotAddCommand(instruction)!;
    expect(result.repo).toBe("nav-pilot add instruction nextjs-aksel");
    expect(result.user).toBe("nav-pilot add instruction nextjs-aksel --user");
  });

  it("generates nav-pilot add command for prompt", () => {
    const result = getNavPilotAddCommand(prompt)!;
    expect(result.repo).toBe("nav-pilot add prompt 1");
    expect(result.user).toBe("nav-pilot add prompt 1 --user");
  });

  it("generates nav-pilot add command for skill", () => {
    const result = getNavPilotAddCommand(skill)!;
    expect(result.repo).toBe("nav-pilot add skill 1");
    expect(result.user).toBe("nav-pilot add skill 1 --user");
  });

  it("returns null for mcp", () => {
    expect(getNavPilotAddCommand(remoteMcp)).toBeNull();
  });
});

describe("getMcpServerConfig", () => {
  it("returns empty for non-mcp types", () => {
    expect(getMcpServerConfig(agent)).toBe("");
  });

  it("generates http config for remote mcp", () => {
    const config = JSON.parse(getMcpServerConfig(remoteMcp));
    expect(config["github-mcp"]).toEqual({ type: "http", url: "https://mcp.nav.no/mcp" });
  });

  it("generates stdio config for npm package", () => {
    const config = JSON.parse(getMcpServerConfig(npmMcp));
    expect(config["figma-mcp"].command).toBe("npx");
    expect(config["figma-mcp"].args).toContain("-y");
    expect(config["figma-mcp"].args).toContain("@anthropic/figma-mcp");
    expect(config["figma-mcp"].args).toContain("--port");
    expect(config["figma-mcp"].args).toContain("3333");
    expect(config["figma-mcp"].env.FIGMA_TOKEN).toBe("");
    expect(config["figma-mcp"].env.DEBUG).toBe("Enable debug logging");
  });

  it("generates stdio config for pypi package", () => {
    const config = JSON.parse(getMcpServerConfig(pypiMcp));
    expect(config["python-mcp"].command).toBe("uvx");
    expect(config["python-mcp"].args).toEqual(["mcp-server-python"]);
  });

  it("returns empty for mcp with no remotes or packages", () => {
    expect(getMcpServerConfig(emptyMcp)).toBe("");
  });
});

describe("getVsCodeAddMcpCommand", () => {
  it("returns empty for non-mcp types", () => {
    expect(getVsCodeAddMcpCommand(agent)).toBe("");
  });

  it("generates code --add-mcp for remote", () => {
    const cmd = getVsCodeAddMcpCommand(remoteMcp);
    expect(cmd).toContain("code --add-mcp");
    const json = JSON.parse(cmd.replace("code --add-mcp '", "").replace(/'$/, ""));
    expect(json.name).toBe("github-mcp");
    expect(json.type).toBe("http");
    expect(json.url).toBe("https://mcp.nav.no/mcp");
  });

  it("generates code --add-mcp for npm package", () => {
    const cmd = getVsCodeAddMcpCommand(npmMcp);
    const json = JSON.parse(cmd.replace("code --add-mcp '", "").replace(/'$/, ""));
    expect(json.name).toBe("figma-mcp");
    expect(json.command).toBe("npx");
    expect(json.env.FIGMA_TOKEN).toBe("${input:FIGMA_TOKEN}");
    expect(json.env.DEBUG).toBe("Enable debug logging");
  });

  it("returns empty for mcp with no remotes or packages", () => {
    expect(getVsCodeAddMcpCommand(emptyMcp)).toBe("");
  });
});

describe("getMcpAddFields", () => {
  it("returns null for non-mcp types", () => {
    expect(getMcpAddFields(agent)).toBeNull();
  });

  it("returns HTTP fields for remote", () => {
    const fields = getMcpAddFields(remoteMcp);
    expect(fields).toEqual({ name: "github-mcp", type: "HTTP", url: "https://mcp.nav.no/mcp" });
  });

  it("returns STDIO fields for npm package", () => {
    const fields = getMcpAddFields(npmMcp)!;
    expect(fields.name).toBe("figma-mcp");
    expect(fields.type).toBe("STDIO");
    expect(fields.command).toContain("npx");
    expect(fields.command).toContain("@anthropic/figma-mcp");
    expect(fields.env).toContain("FIGMA_TOKEN=...");
    expect(fields.env).toContain("DEBUG=Enable debug logging");
  });

  it("returns STDIO fields for pypi package", () => {
    const fields = getMcpAddFields(pypiMcp)!;
    expect(fields.command).toContain("uvx");
    expect(fields.command).toContain("mcp-server-python");
  });

  it("returns null for mcp with no remotes or packages", () => {
    expect(getMcpAddFields(emptyMcp)).toBeNull();
  });
});

describe("resolveAgentReferenceUrls", () => {
  it("returns URLs for referenced agents", () => {
    const urls = resolveAgentReferenceUrls(agentWithRefs, allItems);
    expect(urls).toEqual([
      "https://raw.githubusercontent.com/navikt/copilot/main/.github/agents/auth.agent.md",
      "https://raw.githubusercontent.com/navikt/copilot/main/.github/agents/nais.agent.md",
    ]);
  });

  it("returns empty array for agent without references", () => {
    expect(resolveAgentReferenceUrls(agent, allItems)).toEqual([]);
  });

  it("filters out references to unknown agents", () => {
    const agentWithUnknownRef: Agent = {
      ...base,
      type: "agent",
      name: "test-agent",
      tools: [],
      agentReferences: ["auth-agent", "nonexistent-agent"],
    };
    const urls = resolveAgentReferenceUrls(agentWithUnknownRef, allItems);
    expect(urls).toEqual(["https://raw.githubusercontent.com/navikt/copilot/main/.github/agents/auth.agent.md"]);
  });
});
