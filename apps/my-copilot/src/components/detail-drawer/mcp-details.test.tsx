import { render, screen } from "@testing-library/react";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { McpDetails } from "./mcp-details";

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
});
