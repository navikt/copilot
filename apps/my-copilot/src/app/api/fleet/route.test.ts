import { vi } from "vitest";

vi.mock("next/headers", () => ({ headers: vi.fn(async () => new Headers()) }));
vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
  redirect: vi.fn((to: string) => {
    throw new Error(`NEXT_REDIRECT ${to}`);
  }),
}));

import { POST } from "./route";

const SAME = { origin: "https://my.nav.no", host: "my.nav.no" };
const post = (body: unknown, headers: Record<string, string> = SAME) =>
  // A plain object: undici's Request drops Origin and Host.
  POST({ headers: new Headers(headers), json: async () => body } as unknown as Request);

describe("POST /api/fleet", () => {
  let fetch: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    vi.unstubAllEnvs();
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("FLEET_CONTROL_URL", "http://fc");
    fetch = vi.fn(async () => new Response("{}"));
    vi.stubGlobal("fetch", fetch);
  });

  it.each([
    ["no Origin", { host: "my.nav.no" }],
    ["another site", { origin: "https://evil.example", host: "my.nav.no" }],
    ["a garbled Origin", { origin: "null", host: "my.nav.no" }],
  ])("refuses a request with %s before any call", async (_, headers) => {
    const res = await post({ action: "revoke", namespace: "team-a", name: "docs", reason: "x" }, headers);
    expect(res.status).toBe(403);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("is 404 without FLEET_CONTROL_URL", async () => {
    vi.stubEnv("FLEET_CONTROL_URL", "");
    await expect(post({})).rejects.toThrow("NEXT_NOT_FOUND");
  });

  it("approves with the generation and content hash, and answers with the page", async () => {
    const res = await post({
      action: "approve",
      namespace: "team-a",
      name: "docs",
      generation: "3",
      contentHash: "h3",
    });
    await expect(res.json()).resolves.toEqual({ href: "/fleet/mcp-servere/team-a/docs" });
    const [url, init] = fetch.mock.calls[0];
    expect(url).toBe("http://fc/apis/fleet.nav.no/v1alpha1/namespaces/team-a/mcpservers/docs/approval");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ generation: 3, contentHash: "h3" });
  });

  it("registers an MCP server as a v1alpha1 resource", async () => {
    const res = await post({
      action: "register",
      namespace: "team-a",
      name: "docs",
      app: "docs-app",
      url: "http://docs-app/mcp",
      tools: "search\nread",
      dataCategory: "Intern",
    });
    expect(res.status).toBe(200);
    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({
      apiVersion: "fleet.nav.no/v1alpha1",
      kind: "MCPServer",
      metadata: { namespace: "team-a", name: "docs" },
      spec: {
        dataCategory: "Intern",
        upstream: { app: "docs-app", url: "http://docs-app/mcp" },
        allowedTools: ["search", "read"],
      },
    });
  });

  it("refuses names that are not DNS labels before any call", async () => {
    for (const bad of [
      { namespace: "../admin", name: "docs" },
      { namespace: "team-a", name: "docs", app: "A/b" },
    ]) {
      const res = await post({ action: "register", ...bad });
      expect(res.status).toBe(400);
    }
    expect(fetch).not.toHaveBeenCalled();
  });

  it("treats only 409 as a conflict and shows fleet-control's message on 422", async () => {
    const body = { action: "revoke", namespace: "team-a", name: "docs", reason: "lekker data" };
    fetch.mockResolvedValueOnce(new Response("{}", { status: 409 }));
    let res = await post(body);
    expect(res.status).toBe(409);
    fetch.mockResolvedValueOnce(new Response("{}", { status: 500 }));
    res = await post(body);
    expect(res.status).toBe(502);
    await expect(res.json()).resolves.toEqual({ error: expect.stringMatching(/får ikke kontakt/) });
    fetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ kind: "Status", reason: "Invalid", message: "spec.dataCategory must be one of" }), {
        status: 422,
      })
    );
    res = await post(body);
    await expect(res.json()).resolves.toEqual({
      error: "Registeret godtok ikke endringen. Sjekk feltene og prøv igjen.",
      detail: "spec.dataCategory must be one of",
    });
    fetch.mockResolvedValueOnce(new Response("{}", { status: 404 }));
    res = await post(body);
    await expect(res.json()).resolves.toEqual({ error: "Fant ikke MCP-serveren, eller du har ikke tilgang." });
  });

  it("needs a reason to revoke, and a known action", async () => {
    expect((await post({ action: "revoke", namespace: "team-a", name: "docs", reason: " " })).status).toBe(400);
    expect((await post({ action: "delete", namespace: "team-a", name: "docs" })).status).toBe(400);
    expect(fetch).not.toHaveBeenCalled();
  });
});
