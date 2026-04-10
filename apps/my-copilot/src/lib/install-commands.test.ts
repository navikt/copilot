import type { Agent, Instruction, Prompt, Skill, McpServerCustomization } from "./customization-types";
import {
  transportLabel,
  getToolCount,
  getManualInstallCommand,
  getMcpServerConfig,
  getVsCodeAddMcpCommand,
  getMcpAddFields,
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
const instruction: Instruction = {
  ...base,
  type: "instruction",
  name: "nextjs-aksel.instructions.md",
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
  it("includes vscode for all types", () => {
    for (const clients of Object.values(CLIENT_SUPPORT)) {
      expect(clients).toContain("vscode");
    }
  });

  it("skill supports vscode, intellij, cli, and github", () => {
    expect(CLIENT_SUPPORT.skill).toEqual(["vscode", "intellij", "cli", "github"]);
  });
});

describe("CLIENT_LABELS", () => {
  it("maps all client keys to display names", () => {
    expect(CLIENT_LABELS.vscode).toBe("VS Code");
    expect(CLIENT_LABELS.intellij).toBe("IntelliJ");
    expect(CLIENT_LABELS.cli).toBe("Copilot CLI");
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
      'curl -fsSL -o ".github/skills/aksel-spacing/SKILL.md" "https://raw.githubusercontent.com/navikt/copilot/main/.github/skills/aksel-spacing/SKILL.md"',
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
