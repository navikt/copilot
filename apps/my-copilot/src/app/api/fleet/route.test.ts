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

const auth = { user: { name: "A123456" } as unknown, token: "t" as string | null };
vi.mock("@/lib/auth", () => ({ getUser: vi.fn(async () => auth.user), getUserToken: vi.fn(async () => auth.token) }));

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
    auth.user = { name: "A123456" };
    auth.token = "t";
  });

  it.each([
    ["no introspected user", () => (auth.user = null)],
    ["no token", () => (auth.token = null)],
  ])("is 401 with %s, before any call", async (_, unset) => {
    unset();
    const res = await post({ action: "revoke", namespace: "team-a", name: "docs", reason: "x" });
    expect(res.status).toBe(401);
    expect(fetch).not.toHaveBeenCalled();
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

  it("sends the readme untrimmed, and refuses one over 64 KiB", async () => {
    const base = {
      action: "register",
      namespace: "team-a",
      name: "docs",
      app: "docs-app",
      url: "http://x/mcp",
      tools: "search",
      dataCategory: "Intern",
    };
    const readme = "    kode\n\n# Docs\n";
    expect((await post({ ...base, readme })).status).toBe(200);
    expect(JSON.parse(fetch.mock.calls[0][1].body).spec.readme).toBe(readme);
    expect((await post({ ...base, readme: "æ".repeat(32 * 1024 + 1) })).status).toBe(400);
    expect(fetch).toHaveBeenCalledTimes(1);
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
      error: "Fleet godtok ikke endringen. Sjekk feltene og prøv igjen.",
      detail: "spec.dataCategory must be one of",
    });
    fetch.mockResolvedValueOnce(new Response("{}", { status: 404 }));
    res = await post(body);
    await expect(res.json()).resolves.toEqual({ error: "Fant ikke MCP-serveren, eller du har ikke tilgang." });
  });

  const status = (code: number, reason: string, message: string, fields: string[] = []) =>
    new Response(
      JSON.stringify({
        kind: "Status",
        code,
        reason,
        message,
        details: { causes: fields.map((field) => ({ field, reason: "FieldValueInvalid", message })) },
      }),
      { status: code }
    );
  const register = { action: "register", namespace: "team-a", name: "docs", app: "docs", url: "x", tools: "search" };
  const approve = { action: "approve", namespace: "team-a", name: "docs", generation: "3", contentHash: "h3" };

  it("tells a taken name apart from a conflicting edit", async () => {
    fetch.mockResolvedValueOnce(status(409, "AlreadyExists", 'mcpservers "docs" already exists'));
    await expect((await post(register)).json()).resolves.toMatchObject({
      error:
        "Det finnes allerede en MCP-server som heter docs i team-a. Velg et annet navn, eller åpne den som finnes.",
    });
    fetch.mockResolvedValueOnce(
      status(409, "Conflict", "generation 3 is not the one awaiting approval", ["generation"])
    );
    await expect((await post(approve)).json()).resolves.toMatchObject({ error: expect.stringMatching(/Noen andre/) });
  });

  it("tells the submitter that someone else must approve, and others how to get access", async () => {
    fetch.mockResolvedValueOnce(status(403, "Forbidden", "the submitter cannot approve their own generation"));
    await expect((await post(approve)).json()).resolves.toMatchObject({
      error: "Du sendte inn versjon 3 selv. En annen på team-a må godkjenne den.",
    });
    fetch.mockResolvedValueOnce(status(403, "Forbidden", "not a member"));
    await expect((await post(approve)).json()).resolves.toMatchObject({ error: expect.stringMatching(/Nais Console/) });
  });

  it("marks the field from details.causes, and falls back to the detail for unknown fields", async () => {
    fetch.mockResolvedValueOnce(status(422, "Invalid", "route.upstream_url: host must be docs.team-a", ["spec"]));
    let out = await (await post(register)).json();
    expect(out.fields.url).toMatch(/intern(e)? adresse/i);
    expect(out.detail).toBeUndefined();
    fetch.mockResolvedValueOnce(
      status(422, "Invalid", "StrengtFortrolig needs its own risk assessment and approval first", ["spec.dataCategory"])
    );
    out = await (await post(register)).json();
    expect(out.fields.dataCategory).toMatch(/Strengt fortrolig/);
    fetch.mockResolvedValueOnce(status(422, "Invalid", "spec is odd", ["spec"]));
    out = await (await post(register)).json();
    expect(out).toEqual({ error: "Fleet godtok ikke endringen. Sjekk feltene og prøv igjen.", detail: "spec is odd" });
  });

  it("needs a reason to revoke, and a known action", async () => {
    expect((await post({ action: "revoke", namespace: "team-a", name: "docs", reason: " " })).status).toBe(400);
    expect((await post({ action: "delete", namespace: "team-a", name: "docs" })).status).toBe(400);
    expect(fetch).not.toHaveBeenCalled();
  });
});
