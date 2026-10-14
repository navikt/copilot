import { getMcpServers } from "./mcp-registry";

const cacheConfiguration = vi.hoisted(() => ({
  keyParts: [] as string[],
  revalidate: 0,
  tags: [] as string[],
}));

vi.mock("next/cache", () => ({
  unstable_cache: vi.fn((fn: () => unknown, keyParts: string[], options: { revalidate: number; tags: string[] }) => {
    cacheConfiguration.keyParts = keyParts;
    cacheConfiguration.revalidate = options.revalidate;
    cacheConfiguration.tags = options.tags;
    return fn;
  }),
}));

describe("getMcpServers", () => {
  it("caches registry results for one hour", () => {
    expect(cacheConfiguration).toEqual({
      keyParts: ["mcp-servers"],
      revalidate: 3600,
      tags: ["mcp-servers"],
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("preserves full registry IDs while mapping package versions and setup instructions", async () => {
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
      setupInstructions,
    });
    expect(servers[1]).toMatchObject({
      name: "github-mcp",
      serverId: "io.github.navikt/github-mcp",
      remotes: [{ type: "streamable-http", url: "https://mcp.nav.no/mcp" }],
    });
    expect(servers.every((server) => server.installUrl?.startsWith("vscode:mcp/"))).toBe(true);
    expect(servers.every((server) => server.insidersInstallUrl?.startsWith("vscode-insiders:mcp/"))).toBe(true);
  });

  it("maps Nav native launches without inventing packages or registry install URLs", async () => {
    const local = { command: "nais", args: ["mcp", "serve"] };
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          servers: [
            {
              server: { name: "io.nais/cli", description: "Nais CLI.", version: "1.0.0" },
              _meta: {
                "io.modelcontextprotocol.registry/official": { status: "active" },
                "io.github.navikt/registry": { local },
              },
            },
          ],
        }),
      })
    );

    expect(await getMcpServers()).toEqual([
      expect.objectContaining({
        serverId: "io.nais/cli",
        local,
        remotes: [],
        packages: undefined,
        installUrl: null,
        insidersInstallUrl: null,
      }),
    ]);
  });

  it.each([{ command: "native-mcp", args: null }, { command: "native-mcp" }])(
    "normalizes null or missing native args: %j",
    async (local) => {
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue({
          ok: true,
          json: async () => ({
            servers: [
              {
                server: { name: "io.example/native", description: "Native MCP.", version: "1.0.0" },
                _meta: {
                  "io.modelcontextprotocol.registry/official": { status: "active" },
                  "io.github.navikt/registry": { local },
                },
              },
            ],
          }),
        })
      );

      expect(await getMcpServers()).toEqual([expect.objectContaining({ local: { command: "native-mcp", args: [] } })]);
    }
  );

  it("excludes inactive registry entries, including deprecated Nais", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          servers: ["active", "deprecated", "deleted", undefined].map((status) => ({
            server: {
              name: status === "deprecated" ? "io.nais/cli" : `io.example/${status}`,
              description: "MCP server.",
              version: "1.0.0",
            },
            _meta: {
              "io.modelcontextprotocol.registry/official": status ? { status } : undefined,
              "io.github.navikt/registry": { local: { command: "native-mcp", args: [] } },
            },
          })),
        }),
      })
    );

    expect((await getMcpServers()).map((server) => server.serverId)).toEqual(["io.example/active"]);
  });
});
