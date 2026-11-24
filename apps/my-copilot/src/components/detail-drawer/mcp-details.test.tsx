import { fireEvent, render, screen } from "@testing-library/react";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { McpDetails } from "./mcp-details";
import { riskCounts, serverPagePath } from "./mcp-meta";

describe("McpDetails", () => {
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

  it("shows nav-pilot mcp enable with the full registry name in the default tab", () => {
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

    expect(screen.getByText("nav-pilot mcp enable io.github.navikt/github-mcp")).toBeVisible();
    expect(screen.getByRole("tab", { name: "nav-pilot" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("nav-pilot mcp list")).toBeInTheDocument();
    expect(screen.getAllByRole("heading")[0]).toHaveTextContent("Koble til");
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
    fireEvent.mouseDown(screen.getByRole("tab", { name: "VS Code" }));
    fireEvent.click(screen.getByRole("tab", { name: "VS Code" }));
    expect(screen.getByText(/Manuelt oppsett slår på alle verktøyene/)).toHaveTextContent("execute_terminal_command");
  });

  it("hides automatic installation controls for manual-only entries and renders setup instructions", () => {
    const item: EnrichedCustomization = {
      id: "mcp-io.github.nais/nais-mcp",
      name: "nais-mcp",
      description: "Read Nais platform context through your installed Nais CLI.",
      type: "mcp",
      serverId: "io.github.nais/nais-mcp",
      domain: "platform",
      filePath: "",
      repoPath: "",
      rawGitHubUrl: "",
      installUrl: null,
      insidersInstallUrl: null,
      version: "6.6.1",
      remotes: [],
      setupInstructions: [
        {
          title: "Konfigurer lokal stdio manuelt",
          description: "Klienten starter nais med argumentene alpha, mcp, serve.",
          commands: ["nais login -n"],
        },
      ],
      usageCount: 0,
      usedBy: [],
    };

    render(<McpDetails item={item} />);

    expect(screen.queryByRole("heading", { name: "Koble til" })).not.toBeInTheDocument();
    expect(screen.queryByText(/nav-pilot mcp enable/)).not.toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "VS Code" })).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Oppsett" })).toBeInTheDocument();
    expect(screen.getByText("Konfigurer lokal stdio manuelt")).toBeInTheDocument();
    expect(screen.getByText("nais login -n")).toBeInTheDocument();
  });

  it("renders each transport type once when remotes repeat a type", () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const item: EnrichedCustomization = {
      id: "mcp-example/server",
      name: "server",
      description: "Example.",
      type: "mcp",
      serverId: "example/server",
      domain: "general",
      filePath: "",
      repoPath: "",
      rawGitHubUrl: "",
      installUrl: null,
      insidersInstallUrl: null,
      version: "1.0.0",
      remotes: [
        { type: "streamable-http", url: "https://a.example/mcp" },
        { type: "streamable-http", url: "https://b.example/mcp" },
        { type: "sse", url: "https://a.example/sse" },
      ],
      usageCount: 0,
      usedBy: [],
    };

    try {
      render(<McpDetails item={item} />);

      expect(screen.getAllByText("Streamable HTTP")).toHaveLength(1);
      expect(screen.getAllByText("SSE")).toHaveLength(1);
      expect(consoleError.mock.calls.flat().join(" ")).not.toContain("same key");
    } finally {
      consoleError.mockRestore();
    }
  });

  describe("fleet-gateway hint", () => {
    const base = {
      id: "mcp-x",
      name: "x",
      description: "d",
      type: "mcp",
      serverId: "x",
      domain: "testing",
      filePath: "",
      repoPath: "",
      rawGitHubUrl: "",
      installUrl: null,
      insidersInstallUrl: null,
      version: "1",
      remotes: [],
    } as unknown as EnrichedCustomization;

    it("links to the guide for a gateway server", () => {
      const item = {
        ...base,
        remotes: [{ type: "streamable-http", url: "https://fleet-gateway.intern.nav.no/kodeverk" }],
      } as EnrichedCustomization;
      render(<McpDetails item={item} />);
      expect(screen.getByRole("link", { name: "Bruke Navs MCP-servere" })).toHaveAttribute("href", "/mcp/bruke");
    });

    it.each(["https://evil.example/fleet-gateway", "https://example.com/?x=fleet-gateway"])("ignores %s", (url) => {
      render(<McpDetails item={{ ...base, remotes: [{ type: "streamable-http", url }] } as EnrichedCustomization} />);
      expect(screen.queryByText(/Bruke Navs MCP-servere/)).not.toBeInTheDocument();
    });

    it("accepts the dev host and the domain_internal template", () => {
      for (const url of ["https://fleet-gateway.intern.dev.nav.no/x", "https://fleet-gateway.{{domain_internal}}/x"]) {
        const { unmount } = render(
          <McpDetails item={{ ...base, remotes: [{ type: "streamable-http", url }] } as EnrichedCustomization} />
        );
        expect(screen.getByText(/Bruke Navs MCP-servere/)).toBeInTheDocument();
        unmount();
      }
    });

    it("is hidden for other servers", () => {
      const item = {
        ...base,
        remotes: [{ type: "streamable-http", url: "https://example.com/mcp" }],
      } as EnrichedCustomization;
      render(<McpDetails item={item} />);
      expect(screen.queryByText(/Bruke Navs MCP-servere/)).not.toBeInTheDocument();
    });
  });

  it("gives a gateway server tabs for every client, its public address and the naisdevice note", () => {
    const url = "https://fleet-gateway.intern.nav.no/mcp/copilot/dolly";
    const item = {
      id: "mcp-io.github.navikt/dolly",
      name: "Dolly",
      description: "d",
      type: "mcp",
      serverId: "io.github.navikt/dolly",
      domain: "testing",
      version: "1",
      remotes: [{ type: "streamable-http", url }],
      tools: ["dolly_search", "create_order"],
      toolRisk: { create_order: "external" },
    } as unknown as EnrichedCustomization;
    render(<McpDetails item={item} />);
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual([
      "nav-pilot",
      "VS Code",
      "IntelliJ",
      "Copilot CLI",
      "OpenCode",
      "Andre klienter",
    ]);
    expect(screen.getByText(/Krever naisdevice/)).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Andre klienter" }));
    fireEvent.click(screen.getByRole("tab", { name: "Andre klienter" }));
    expect(screen.getAllByText(url).length).toBeGreaterThan(0);
    expect(riskCounts(item.type === "mcp" ? item.tools! : [], { create_order: "external" })).toBe(
      "2 verktøy, 1 endrer"
    );
    expect(serverPagePath("io.github.navikt/dolly")).toBe("/mcp/servere/dolly");
  });
});
