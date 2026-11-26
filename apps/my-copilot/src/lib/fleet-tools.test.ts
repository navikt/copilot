import { vi } from "vitest";

const exchange = vi.fn(async () => "obo-token");
vi.mock("@/lib/backend-api", async (orig) => ({
  ...(await orig<typeof import("@/lib/backend-api")>()),
  exchangeTokenFor: (...a: unknown[]) => exchange(...(a as [])),
  isLocalDev: () => false,
}));

import { DiscoveryError, MAX_BODY_BYTES, discoverTools, parseTools } from "./fleet-tools";

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });
const URL_ = "http://docs.team-a/mcp";

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

  it("calls the probe with the person's OBO token, the team and the address", async () => {
    fetch.mockResolvedValueOnce(json({ tools: [{ name: "a", title: "A", annotations: { readOnlyHint: true } }] }));
    await expect(discoverTools("user-token", "team-a", URL_)).resolves.toEqual([
      { name: "a", suggestedRisk: "read", destructive: false },
    ]);
    expect(exchange).toHaveBeenCalledWith("user-token", "fleet-gateway");
    const [url, init] = fetch.mock.calls[0];
    expect(url).toBe("https://gw/probe/v1/tools");
    expect(init.method).toBe("POST");
    expect(init.headers.Authorization).toBe("Bearer obo-token");
    expect(JSON.parse(init.body)).toEqual({ team: "team-a", upstream_url: URL_ });
  });

  it.each(["unavailable", "auth", "access", "noroute", "toolarge", "invalid"])(
    "passes the probe's error kind %s through",
    async (k) => {
      fetch.mockResolvedValueOnce(json({ error: k }, k === "unavailable" ? 503 : 502));
      expect(await kind(discoverTools("t", "team-a", URL_))).toBe(k);
    }
  );

  it.each([
    ["404, the probe is off", new Response("404 page not found", { status: 404 }), "off"],
    ["429, rate limited", json({ error: "unavailable" }, 429), "unavailable"],
    ["an unknown kind", json({ error: "secret detail" }, 502), "unavailable"],
    ["a non-JSON error", new Response("oops", { status: 500 }), "unavailable"],
    ["a 200 without JSON", new Response("oops"), "invalid"],
    ["a 200 with bad tools", json({ tools: [{ name: "a/b" }] }), "invalid"],
  ])("answers %s", async (_, res, want) => {
    fetch.mockResolvedValueOnce(res);
    expect(await kind(discoverTools("t", "team-a", URL_))).toBe(want);
  });

  it("treats a connection error as unavailable", async () => {
    fetch.mockRejectedValueOnce(new TypeError("fetch failed"));
    expect(await kind(discoverTools("t", "team-a", URL_))).toBe("unavailable");
  });

  it("caps the response size, by header and by body", async () => {
    fetch.mockResolvedValueOnce(new Response("{}", { headers: { "content-length": String(MAX_BODY_BYTES + 1) } }));
    expect(await kind(discoverTools("t", "team-a", URL_))).toBe("toolarge");
    fetch.mockResolvedValueOnce(new Response("x".repeat(MAX_BODY_BYTES + 1)));
    expect(await kind(discoverTools("t", "team-a", URL_))).toBe("toolarge");
  });

  it("is off without FLEET_GATEWAY_URL, and refuses bad input, before any call", async () => {
    expect(await kind(discoverTools("t", "../x", URL_))).toBe("invalid");
    expect(await kind(discoverTools("t", "team-a", "not a url"))).toBe("invalid");
    expect(await kind(discoverTools("t", "team-a", "http://x/" + "a".repeat(2048)))).toBe("invalid");
    vi.stubEnv("FLEET_GATEWAY_URL", "");
    expect(await kind(discoverTools("t", "team-a", URL_))).toBe("off");
    expect(fetch).not.toHaveBeenCalled();
    expect(exchange).not.toHaveBeenCalled();
  });

  it("fails when the token exchange fails", async () => {
    exchange.mockRejectedValueOnce(new Error("no"));
    expect(await kind(discoverTools("t", "team-a", URL_))).toBe("auth");
    expect(fetch).not.toHaveBeenCalled();
  });
});
