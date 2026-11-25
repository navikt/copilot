import { vi } from "vitest";

const exchange = vi.fn(async () => "obo-token");
vi.mock("@/lib/backend-api", async (orig) => ({
  ...(await orig<typeof import("@/lib/backend-api")>()),
  exchangeTokenFor: (...a: unknown[]) => exchange(...(a as [])),
  isLocalDev: () => false,
}));

import { DiscoveryError, MAX_BODY_BYTES, discoverTools, parseTools } from "./fleet-tools";

const json = (result: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify({ jsonrpc: "2.0", id: 1, result }), {
    headers: { "content-type": "application/json", ...headers },
  });

const kind = (p: Promise<unknown>) =>
  p.then(
    () => "ok",
    (e) => (e instanceof DiscoveryError ? e.kind : String(e))
  );

describe("parseTools", () => {
  it("pre-fills risk from annotations and never trusts more than a hint", () => {
    expect(
      parseTools({
        tools: [
          { name: "hent_kode", description: " Henter én kode. ", annotations: { readOnlyHint: true } },
          { name: "slett_kode", annotations: { destructiveHint: true } },
          { name: "lag.notat", annotations: { readOnlyHint: "true", destructiveHint: false } },
        ],
      })
    ).toEqual([
      { name: "hent_kode", description: "Henter én kode.", suggestedRisk: "read", destructive: false },
      { name: "slett_kode", suggestedRisk: "write", destructive: true },
      // A string "true" is not a boolean hint: write.
      { name: "lag.notat", suggestedRisk: "write", destructive: false },
    ]);
  });

  it.each([
    ["no list", {}],
    ["a name with a slash", { tools: [{ name: "a/b" }] }],
    ["a name with a space", { tools: [{ name: "a b" }] }],
    ["a non-ASCII name", { tools: [{ name: "søk" }] }],
    ["an empty name", { tools: [{ name: "" }] }],
    ["a name over 128", { tools: [{ name: "a".repeat(129) }] }],
    ["a duplicate", { tools: [{ name: "a" }, { name: "a" }] }],
    ["too many tools", { tools: Array.from({ length: 257 }, (_, i) => ({ name: `t${i}` })) }],
  ])("refuses %s", (_, result) => {
    expect(() => parseTools(result)).toThrow(DiscoveryError);
  });

  it("clips long descriptions to 1 KiB", () => {
    const [t] = parseTools({ tools: [{ name: "a", description: "æ".repeat(2000) }] });
    expect(new TextEncoder().encode(t.description).length).toBeLessThanOrEqual(1024);
  });
});

describe("discoverTools", () => {
  let fetch: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    vi.unstubAllEnvs();
    vi.stubEnv("FLEET_GATEWAY_URL", "https://gw");
    exchange.mockClear();
    fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
  });

  it("calls tools/list through the gateway with the person's OBO token and the session id", async () => {
    fetch
      .mockResolvedValueOnce(json({ protocolVersion: "2025-06-18" }, { "mcp-session-id": "s1" }))
      .mockResolvedValueOnce(new Response(null, { status: 202 }))
      .mockResolvedValueOnce(
        new Response('event: message\ndata: {"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"a"}]}}\n\n', {
          headers: { "content-type": "text/event-stream" },
        })
      );
    await expect(discoverTools("user-token", "team-a", "docs")).resolves.toEqual([
      { name: "a", suggestedRisk: "write", destructive: true },
    ]);
    expect(exchange).toHaveBeenCalledWith("user-token", "fleet-gateway");
    const [url, init] = fetch.mock.calls[2];
    expect(url).toBe("https://gw/mcp/team-a/docs");
    expect(init.headers.Authorization).toBe("Bearer obo-token");
    expect(init.headers["Mcp-Session-Id"]).toBe("s1");
    expect(JSON.parse(init.body).method).toBe("tools/list");
  });

  it.each([
    [403, "access"],
    [502, "access"],
    [503, "access"],
    [401, "auth"],
    [404, "noroute"],
    [400, "invalid"],
  ])("answers %i as %s", async (status, want) => {
    fetch.mockResolvedValueOnce(new Response("{}", { status }));
    expect(await kind(discoverTools("t", "team-a", "docs"))).toBe(want);
  });

  it("treats a connection error as the accessPolicy check", async () => {
    fetch.mockRejectedValueOnce(new TypeError("fetch failed"));
    expect(await kind(discoverTools("t", "team-a", "docs"))).toBe("access");
  });

  it("caps the response size, by header and by body", async () => {
    fetch.mockResolvedValueOnce(new Response("{}", { headers: { "content-length": String(MAX_BODY_BYTES + 1) } }));
    expect(await kind(discoverTools("t", "team-a", "docs"))).toBe("toolarge");
    fetch.mockResolvedValueOnce(new Response("x".repeat(MAX_BODY_BYTES + 1)));
    expect(await kind(discoverTools("t", "team-a", "docs"))).toBe("toolarge");
  });

  it("is unavailable without FLEET_GATEWAY_URL, and refuses bad names, before any call", async () => {
    expect(await kind(discoverTools("t", "../x", "docs"))).toBe("invalid");
    vi.stubEnv("FLEET_GATEWAY_URL", "");
    expect(await kind(discoverTools("t", "team-a", "docs"))).toBe("unavailable");
    expect(fetch).not.toHaveBeenCalled();
    expect(exchange).not.toHaveBeenCalled();
  });

  it("fails when the token exchange fails", async () => {
    exchange.mockRejectedValueOnce(new Error("no"));
    expect(await kind(discoverTools("t", "team-a", "docs"))).toBe("auth");
    expect(fetch).not.toHaveBeenCalled();
  });
});
