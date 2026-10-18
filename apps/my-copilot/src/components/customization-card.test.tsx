import { render, screen } from "@testing-library/react";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { CustomizationCard } from "./customization-card";

describe("CustomizationCard", () => {
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
      render(<CustomizationCard item={item} />);

      expect(screen.getAllByText("Streamable HTTP")).toHaveLength(1);
      expect(screen.getAllByText("SSE")).toHaveLength(1);
      expect(consoleError.mock.calls.flat().join(" ")).not.toContain("same key");
    } finally {
      consoleError.mockRestore();
    }
  });
});
