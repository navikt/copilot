import { readFileSync } from "node:fs";
import { join } from "node:path";
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

  const spec = (over: Record<string, unknown> = {}) => ({
    dataCategory: "Intern",
    upstream: { app: "docs-app", url: "http://docs-app/mcp" },
    allowedTools: ["search", "read"],
    ...over,
  });

  it("registers an MCP server as a v1alpha1 resource, with risk and tool docs", async () => {
    const full = spec({
      tools: [{ name: "search", description: "Søker" }],
      defaultRisk: "read",
      toolRisk: { read: "propose" },
      rosRef: "https://ros.example/1",
    });
    const res = await post({ action: "register", namespace: "team-a", name: "docs", spec: full });
    expect(res.status).toBe(200);
    // Register and edit open the MCP server page with the «sendt til godkjenning» receipt; approve does not (below).
    await expect(res.clone().json()).resolves.toEqual({ href: "/fleet/mcp-servere/team-a/docs?sendt=1" });
    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({
      apiVersion: "fleet.nav.no/v1alpha1",
      kind: "MCPServer",
      metadata: { namespace: "team-a", name: "docs" },
      spec: full,
    });
  });

  it("sends the readme untrimmed, and refuses one over 64 KiB", async () => {
    const base = { action: "register", namespace: "team-a", name: "docs" };
    const readme = "    kode\n\n# Docs\n";
    expect((await post({ ...base, spec: spec({ readme }) })).status).toBe(200);
    expect(JSON.parse(fetch.mock.calls[0][1].body).spec.readme).toBe(readme);
    expect((await post({ ...base, spec: spec({ readme: "æ".repeat(32 * 1024 + 1) }) })).status).toBe(400);
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("refuses names that are not DNS labels before any call", async () => {
    for (const bad of [
      { namespace: "../admin", name: "docs", spec: spec() },
      { namespace: "team-a", name: "docs", spec: spec({ upstream: { app: "A/b", url: "http://x/mcp" } }) },
    ]) {
      const res = await post({ action: "register", ...bad });
      expect(res.status).toBe(400);
    }
    expect(fetch).not.toHaveBeenCalled();
  });

  it.each([
    ["no spec", undefined],
    ["no tools", spec({ allowedTools: [] })],
    ["tool outside the gateway charset", spec({ allowedTools: ["søk"] })],
    ["tool with a slash", spec({ allowedTools: ["a/b"] })],
    ["duplicate tool", spec({ allowedTools: ["a", "a"] })],
    ["too many tools", spec({ allowedTools: Array.from({ length: 257 }, (_, i) => `t${i}`) })],
    ["unknown defaultRisk", spec({ defaultRisk: "admin" })],
    ["toolRisk for a tool not allowed", spec({ toolRisk: { other: "read" } })],
    ["unknown toolRisk", spec({ toolRisk: { search: "root" } })],
    ["tool doc for a tool not allowed", spec({ tools: [{ name: "other" }] })],
  ])("refuses a spec with %s before any call", async (_, bad) => {
    const res = await post({ action: "register", namespace: "team-a", name: "docs", spec: bad });
    expect(res.status).toBe(400);
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
  const register = {
    action: "register",
    namespace: "team-a",
    name: "docs",
    spec: { dataCategory: "Intern", upstream: { app: "docs", url: "x" }, allowedTools: ["search"] },
  };
  // The edit page sends the whole spec, like the wizard.
  const editForm = { ...register, action: "edit" };
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

  it("maps a 403 by its reason, and others to how to get access", async () => {
    fetch.mockResolvedValueOnce(status(403, "SubmitterNotPerson", "reworded"));
    await expect((await post(approve)).json()).resolves.toMatchObject({
      error: "Ingen person sendte inn versjon 3. Noen på team-a må sende den inn på nytt.",
    });
    fetch.mockResolvedValueOnce(status(403, "ApproverNoOid", "reworded"));
    await expect((await post(approve)).json()).resolves.toMatchObject({ error: expect.stringMatching(/Logg ut/) });
    fetch.mockResolvedValueOnce(status(403, "NotPlatformTeam", "the approver has no object id"));
    await expect((await post(approve)).json()).resolves.toMatchObject({
      error: "Team copilot godkjenner MCP-servere. Versjon 3 venter på dem.",
    });
    fetch.mockResolvedValueOnce(status(403, "Forbidden", "only the platform team approves MCP server versions"));
    await expect((await post(approve)).json()).resolves.toMatchObject({
      error: expect.stringMatching(/^Team copilot/),
    });
    fetch.mockResolvedValueOnce(status(403, "Forbidden", "not a member"));
    await expect((await post(approve)).json()).resolves.toMatchObject({ error: expect.stringMatching(/Nais Console/) });
  });

  it("falls back to the message from a fleet-control without reasons", async () => {
    fetch.mockResolvedValueOnce(
      status(403, "Forbidden", "the pending generation has no person as submitter; a person must submit it again")
    );
    await expect((await post(approve)).json()).resolves.toMatchObject({ error: expect.stringMatching(/Ingen person/) });
    fetch.mockResolvedValueOnce(status(403, "Forbidden", "the approver has no object id"));
    await expect((await post(approve)).json()).resolves.toMatchObject({ error: expect.stringMatching(/Logg ut/) });
  });

  it("matches only reasons fleet-control defines", () => {
    const pinned: string[] = JSON.parse(
      readFileSync(join(__dirname, "../../../../../fleet-control/internal/api/reasons.json"), "utf8")
    );
    const used = [
      ...readFileSync(join(__dirname, "../../../lib/fleet-errors.ts"), "utf8").matchAll(/reason === "(\w+)"/g),
    ].map((m) => m[1]);
    expect(used.length).toBeGreaterThan(0);
    for (const r of used) if (r !== "Forbidden") expect(pinned).toContain(r);
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
    fetch.mockResolvedValueOnce(status(422, "Invalid", "MCP-serveren har skriveverktøy", ["spec.rosRef"]));
    out = await (await post(register)).json();
    expect(out.fields.rosRef).toMatch(/ROS/);
    fetch.mockResolvedValueOnce(status(422, "Invalid", "spec is odd", ["spec"]));
    out = await (await post(register)).json();
    expect(out).toEqual({ error: "Fleet godtok ikke endringen. Sjekk feltene og prøv igjen.", detail: "spec is odd" });
  });

  it("needs a reason to revoke, and a known action", async () => {
    expect((await post({ action: "revoke", namespace: "team-a", name: "docs", reason: " " })).status).toBe(400);
    expect((await post({ action: "delete", namespace: "team-a", name: "docs" })).status).toBe(400);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("needs a data category, and sends none by default", async () => {
    expect(
      (await post({ ...editForm, resourceVersion: "7", spec: { ...register.spec, dataCategory: "" } })).status
    ).toBe(400);
    expect((await post({ ...register, spec: { ...register.spec, dataCategory: undefined } })).status).toBe(400);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("edits with PUT and the resourceVersion, which makes a new version", async () => {
    fetch.mockResolvedValueOnce(Response.json(current));
    const res = await post({
      ...editForm,
      resourceVersion: "7",
      spec: { ...register.spec, allowedTools: ["search", "read"] },
    });
    await expect(res.json()).resolves.toEqual({ href: "/fleet/mcp-servere/team-a/docs?sendt=1" });
    const [url, init] = fetch.mock.calls[1];
    expect(url).toBe("http://fc/apis/fleet.nav.no/v1alpha1/namespaces/team-a/mcpservers/docs");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toMatchObject({
      metadata: { namespace: "team-a", name: "docs", resourceVersion: "7" },
      spec: { allowedTools: ["search", "read"], dataCategory: "Intern" },
    });
    expect((await post(editForm)).status).toBe(400);
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  // What fleet-control holds: fields the edit form does not show must survive an edit.
  const current = {
    metadata: { namespace: "team-a", name: "docs", resourceVersion: "7" },
    spec: {
      dataCategory: "Intern",
      upstream: { app: "docs", url: "x" },
      allowedTools: ["search", "read"],
      readme: "old",
      tools: [
        { name: "search", description: "Søk" },
        { name: "read", description: "Les" },
      ],
      defaultRisk: "read",
      toolRisk: { search: "read", read: "propose" },
      rosRef: "ROS-1",
      registry: { version: "1.0.0" },
    },
    status: {},
  };
  const editTo = async (spec: Record<string, unknown>) => {
    fetch.mockResolvedValueOnce(Response.json(current));
    const res = await post({ ...editForm, resourceVersion: "7", spec });
    expect(res.status).toBe(200);
    const [url, init] = fetch.mock.calls[1];
    expect(url).toBe("http://fc/apis/fleet.nav.no/v1alpha1/namespaces/team-a/mcpservers/docs");
    expect(init.method).toBe("PUT");
    return JSON.parse(init.body).spec;
  };

  it("sends the page's spec, and keeps registry from what fleet-control holds, never the client's", async () => {
    expect(await editTo({ ...current.spec, readme: "new", registry: { version: "9.9.9" } })).toEqual({
      ...current.spec,
      readme: "new",
    });
  });

  it("refuses a bad app name or a risk level for a tool that is not allowed, before any call", async () => {
    const bad = (spec: Record<string, unknown>) => post({ ...editForm, resourceVersion: "7", spec });
    expect((await bad({ ...register.spec, upstream: { app: "Bad_App", url: "x" } })).status).toBe(400);
    expect((await bad({ ...register.spec, toolRisk: { other: "read" } })).status).toBe(400);
    expect(fetch).not.toHaveBeenCalled();
  });

  const grant = {
    action: "grant",
    namespace: "team-b",
    name: "agent",
    serverNamespace: "team-a",
    serverName: "docs",
    generation: "2",
    contentHash: "g2",
  };

  it("gives access with a typed server ref, and opens the agent profile", async () => {
    const res = await post(grant);
    await expect(res.json()).resolves.toEqual({ href: "/fleet/agenter/team-b/agent" });
    const [url, init] = fetch.mock.calls[0];
    expect(url).toBe("http://fc/apis/fleet.nav.no/v1alpha1/namespaces/team-b/agentprofiles/agent/approval");
    expect(JSON.parse(init.body)).toEqual({
      generation: 2,
      serverRef: { group: "fleet.nav.no", kind: "MCPServer", namespace: "team-a", name: "docs" },
      contentHash: "g2",
    });
  });

  it("maps grant errors to the MCP server's team", async () => {
    fetch.mockResolvedValueOnce(status(403, "SelfApproval", "reworded"));
    await expect((await post(grant)).json()).resolves.toMatchObject({
      error: "Du sendte inn agentprofilen selv. En annen på team-a må gi tilgang.",
    });
    fetch.mockResolvedValueOnce(status(403, "Forbidden", "the submitter cannot grant access to their own profile"));
    await expect((await post(grant)).json()).resolves.toMatchObject({
      error: expect.stringMatching(/agentprofilen selv/),
    });
    fetch.mockResolvedValueOnce(status(403, "Forbidden", "only the owner team of MCP server team-a/docs can act"));
    await expect((await post(grant)).json()).resolves.toMatchObject({ error: expect.stringMatching(/team-a/) });
    fetch.mockResolvedValueOnce(status(404, "NotFound", "not found"));
    await expect((await post(grant)).json()).resolves.toMatchObject({ error: expect.stringMatching(/agentprofilen/) });
    expect((await post({ ...grant, serverName: "../x" })).status).toBe(400);
  });

  it("maps a failed edit: stale resourceVersion is a conflict, a bad field is marked", async () => {
    const edit = { ...editForm, resourceVersion: "7" };
    fetch.mockResolvedValueOnce(Response.json(current));
    fetch.mockResolvedValueOnce(status(409, "Conflict", "resourceVersion is stale", ["metadata.resourceVersion"]));
    let res = await post(edit);
    expect(res.status).toBe(409);
    await expect(res.json()).resolves.toMatchObject({ error: expect.stringMatching(/Noen andre har endret/) });
    fetch.mockResolvedValueOnce(Response.json(current));
    fetch.mockResolvedValueOnce(status(422, "Invalid", "tools must be unique", ["spec.allowedTools"]));
    res = await post(edit);
    expect(res.status).toBe(422);
    expect((await res.json()).fields.tools).toMatch(/verktøy/i);
  });
});
