import { fireEvent, render, screen } from "@testing-library/react";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { McpDetails } from "./mcp-details";

describe("McpDetails", () => {
  it("renders native launch and manual setup without claiming registry installation", () => {
    const item: EnrichedCustomization = {
      id: "mcp-io.nais/cli",
      name: "cli",
      description: "Nais CLI.",
      type: "mcp",
      serverId: "io.nais/cli",
      domain: "platform",
      filePath: "",
      repoPath: "",
      rawGitHubUrl: "",
      installUrl: "vscode:mcp/mcp-registry.nav.no/v0.1/servers/io.nais%2Fcli/versions/latest",
      insidersInstallUrl: null,
      version: "1.0.0",
      remotes: [],
      local: { command: "nais", args: ["mcp", "serve"] },
      usageCount: 0,
      usedBy: [],
    };

    render(<McpDetails item={item} />);

    expect(screen.getByRole("heading", { name: "Lokal kommando" })).toBeInTheDocument();
    expect(screen.getAllByText("nais mcp serve")[0]).toBeVisible();
    expect(screen.getByText(/Krever at/)).toHaveTextContent("Krever at nais er installert på maskinen din.");
    expect(screen.queryByRole("heading", { name: "Pakker" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "VS Code" }));
    expect(screen.queryByRole("link", { name: "Installer fra MCP-registeret" })).not.toBeInTheDocument();
    expect(
      screen.getByText(`code --add-mcp '{"name":"io.nais/cli","command":"nais","args":["mcp","serve"]}'`)
    ).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "IntelliJ" }));
    expect(screen.queryByText(/installere serveren direkte fra registeret/)).not.toBeInTheDocument();
    expect(screen.getByText(/Legg til manuelt i/)).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Copilot CLI" }));
    expect(screen.getByText("STDIO")).toBeVisible();
    expect(screen.getAllByText("nais mcp serve")).toHaveLength(2);
  });

  it("renders setup instructions with duplicate titles and commands using safe keys", () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const command = "cplt config set sandbox.allow_cache_exec ms-playwright";
    const item: EnrichedCustomization = {
      id: "mcp-com.microsoft/playwright-mcp",
      name: "playwright-mcp",
      description: "Browser automation.",
      type: "mcp",
      serverId: "com.microsoft/playwright-mcp",
      domain: "testing",
      filePath: "",
      repoPath: "",
      rawGitHubUrl: "",
      installUrl: null,
      insidersInstallUrl: null,
      version: "0.0.80",
      remotes: [],
      setupInstructions: [
        {
          title: "Gjenta oppsett",
          description: "Første instruksjon.",
          commands: [command, command],
        },
        {
          title: "Gjenta oppsett",
          description: "Andre instruksjon.",
          commands: [command],
        },
      ],
      packages: [
        {
          registryType: "npm",
          identifier: "@playwright/mcp",
          version: "0.0.80",
          transport: { type: "stdio" },
          packageArguments: [{ type: "named", name: "--isolated", description: "Isolated browser state" }],
        },
      ],
      usageCount: 0,
      usedBy: [],
    };

    try {
      render(<McpDetails item={item} />);

      expect(screen.getByRole("heading", { name: "Oppsett" })).toBeInTheDocument();
      expect(screen.getAllByText("Gjenta oppsett")).toHaveLength(2);
      expect(screen.getAllByText(command)).toHaveLength(3);
      expect(screen.getByText("Argumenter:")).toBeInTheDocument();
      expect(consoleError.mock.calls.flat().join(" ")).not.toContain("same key");
    } finally {
      consoleError.mockRestore();
    }
  });

  it("shows nav-pilot mcp enable with the full registry name before the manual setup", () => {
    const item: EnrichedCustomization = {
      id: "mcp-io.github.navikt/github-mcp",
      name: "github-mcp",
      description: "GitHub.",
      type: "mcp",
      serverId: "io.github.navikt/github-mcp",
      domain: "general",
      filePath: "",
      repoPath: "",
      rawGitHubUrl: "",
      installUrl: null,
      insidersInstallUrl: null,
      version: "1.0.0",
      remotes: [{ type: "streamable-http", url: "https://api.githubcopilot.com/mcp/" }],
      usageCount: 0,
      usedBy: [],
    };

    render(<McpDetails item={item} />);

    const primary = screen.getByText("nav-pilot mcp enable io.github.navikt/github-mcp");
    const manual = screen.getByRole("heading", { name: "Manuelt oppsett" });
    expect(primary.compareDocumentPosition(manual) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.getByText("nav-pilot mcp list")).toBeInTheDocument();
    expect(screen.getAllByRole("heading")[0]).toHaveTextContent("Installering");
  });

  it("groups tools by risk and says which are off and how to turn them on", () => {
    const item: EnrichedCustomization = {
      id: "mcp-com.jetbrains/intellij",
      name: "intellij",
      description: "IntelliJ.",
      type: "mcp",
      serverId: "com.jetbrains/intellij",
      domain: "general",
      filePath: "",
      repoPath: "",
      rawGitHubUrl: "",
      installUrl: null,
      insidersInstallUrl: null,
      version: "1.0.0",
      remotes: [{ type: "sse", url: "http://127.0.0.1:64342/sse" }],
      tools: ["read_file", "reformat_file", "execute_terminal_command"],
      toolRisk: { reformat_file: "write", execute_terminal_command: "host-exec" },
      usageCount: 0,
      usedBy: [],
    };

    render(<McpDetails item={item} />);

    expect(screen.getByText("Leser (1)")).toBeInTheDocument();
    expect(screen.getByText("Endrer prosjektet (1)")).toBeInTheDocument();
    expect(screen.getByText("Kjører utenfor sandkassen (1)")).toBeInTheDocument();
    expect(screen.queryByText(/Gjør noe i et annet system/)).not.toBeInTheDocument();
    expect(screen.getByText("--allow-host-exec")).toBeInTheDocument();
    expect(screen.getByText(/Manuelt oppsett slår på alle verktøyene/)).toHaveTextContent("execute_terminal_command");
  });
});
