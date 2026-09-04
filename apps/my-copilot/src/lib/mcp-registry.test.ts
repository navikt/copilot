import { getMcpServers } from "./mcp-registry";

vi.mock("next/cache", () => ({
  cacheLife: vi.fn(),
  cacheTag: vi.fn(),
}));

describe("getMcpServers", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("preserves full registry IDs globally while retaining short display names", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          servers: [
            {
              server: {
                name: "com.microsoft/playwright-mcp",
                description: "Browser automation.",
                version: "0.0.80",
                packages: [
                  {
                    registryType: "npm",
                    identifier: "@playwright/mcp",
                    version: "0.0.80",
                    transport: { type: "stdio" },
                  },
                ],
              },
              _meta: {
                "io.modelcontextprotocol.registry/official": {
                  status: "active",
                  publishedAt: "2026-03-10T00:00:00Z",
                  isLatest: true,
                },
                "io.github.navikt/registry": { tags: ["testing"] },
              },
            },
            {
              server: {
                name: "io.github.navikt/github-mcp",
                description: "GitHub tools.",
                version: "1.0.0",
                remotes: [{ type: "streamable-http", url: "https://mcp.nav.no/mcp" }],
              },
              _meta: {
                "io.modelcontextprotocol.registry/official": {
                  status: "active",
                  publishedAt: "2026-03-10T00:00:00Z",
                  isLatest: true,
                },
                "io.github.navikt/registry": { tags: ["github"] },
              },
            },
          ],
          metadata: { count: 2 },
        }),
      })
    );

    const servers = await getMcpServers();

    expect(servers).toHaveLength(2);
    expect(servers[0]).toMatchObject({
      name: "playwright-mcp",
      serverId: "com.microsoft/playwright-mcp",
      version: "0.0.80",
      packages: [{ identifier: "@playwright/mcp", version: "0.0.80" }],
    });
    expect(servers[1]).toMatchObject({
      name: "github-mcp",
      serverId: "io.github.navikt/github-mcp",
      remotes: [{ type: "streamable-http", url: "https://mcp.nav.no/mcp" }],
    });
  });
});
