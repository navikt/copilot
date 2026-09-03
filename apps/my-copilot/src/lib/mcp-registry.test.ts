import { getMcpServers } from "./mcp-registry";

vi.mock("next/cache", () => ({
  cacheLife: vi.fn(),
  cacheTag: vi.fn(),
}));

describe("getMcpServers", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("keeps the display name separate and maps package version and setup instructions", async () => {
    const setupInstructions = [
      {
        title: "Installer Chromium før første bruk",
        description: "Kjør kommandoen utenfor cplt.",
        commands: ["pnpm dlx @playwright/mcp@0.0.80 install-browser chromium"],
      },
    ];
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
                "io.github.navikt/registry": {
                  tags: ["testing"],
                  setupInstructions,
                },
              },
            },
          ],
          metadata: { count: 1 },
        }),
      })
    );

    const servers = await getMcpServers();

    expect(servers).toHaveLength(1);
    expect(servers[0]).toMatchObject({
      name: "playwright-mcp",
      serverId: "com.microsoft/playwright-mcp",
      version: "0.0.80",
      packages: [{ identifier: "@playwright/mcp", version: "0.0.80" }],
      setupInstructions,
    });
  });
});
