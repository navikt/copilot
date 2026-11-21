import { afterEach, describe, expect, it } from "vitest";
import type { AddressInfo } from "node:net";
import type { Server } from "node:http";
import { extractGatewayClientId } from "./auth.js";
import { createApp, MAX_REQUEST_BYTES, MAX_RESPONSE_BYTES, type Config } from "./server.js";

const GW = "gw-client-123";
const user = { active: true, idtyp: "user", NAVident: "Z123456", azp: GW };
let open: Server | undefined;
afterEach(() => void open?.close());

// Fake Texas and downstream API behind one fetch.
function setup(claims: object, over: Partial<Config> = {}) {
  const downstream: { auth?: string }[] = [];
  const fakeFetch = (async (url: string | URL | Request, init?: RequestInit) => {
    const u = String(url);
    if (u.endsWith("/introspect")) return Response.json(claims);
    if (u.endsWith("/token")) return Response.json({ access_token: "obo-token" });
    downstream.push({ auth: (init?.headers as Record<string, string>).Authorization });
    return new Response("x".repeat(MAX_RESPONSE_BYTES * 4));
  }) as typeof fetch;
  const cfg: Config = {
    introspectionEndpoint: "http://texas/introspect", exchangeEndpoint: "http://texas/token", gatewayClientId: GW,
    downstreamUrl: "http://kodeverk", downstreamScope: "api://x/.default", fetch: fakeFetch, ...over,
  };
  const { server, metrics } = createApp(cfg);
  open = server.listen(0);
  const url = `http://127.0.0.1:${(server.address() as AddressInfo).port}/`;
  const post = (body: string, token: string | null = "inbound-token") =>
    fetch(url, {
      method: "POST", body,
      headers: { "Content-Type": "application/json", Accept: "application/json, text/event-stream", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    });
  return { post, metrics, downstream };
}
const call = (name: string, args: object) => JSON.stringify({ jsonrpc: "2.0", id: 1, method: "tools/call", params: { name, arguments: args } });

describe("inbound token validation", () => {
  it.each([
    ["missing header", null, user, 401],
    ["inactive", "t", { active: false }, 401],
    ["idtyp=app", "t", { ...user, idtyp: "app" }, 403],
    ["access_as_application role", "t", { ...user, roles: ["access_as_application"] }, 403],
    ["missing NAVident", "t", { ...user, NAVident: undefined }, 403],
    ["wrong azp", "t", { ...user, azp: "other" }, 403],
  ])("%s", async (_n, token, claims, status) => {
    const { post } = setup(claims);
    expect((await post("{}", token)).status).toBe(status);
  });

  it("fails closed without gateway client ID", async () => {
    const { post } = setup(user, { gatewayClientId: "" });
    expect((await post("{}")).status).toBe(503);
  });

  it("limits request size", async () => {
    const { post } = setup(user);
    expect((await post("x".repeat(MAX_REQUEST_BYTES + 1))).status).toBe(413);
  });

  it("finds exactly one gateway client ID", () => {
    const apps = JSON.stringify([{ name: "dev-gcp:copilot:fleet-gateway", clientId: GW }, { name: "dev-gcp:x:y", clientId: "z" }]);
    expect(extractGatewayClientId(apps, "dev-gcp")).toBe(GW);
    expect(extractGatewayClientId(apps, "prod-gcp")).toBe("");
    expect(extractGatewayClientId("", "dev-gcp")).toBe("");
  });
});

describe("tool call", () => {
  it("does OBO, caps the response and counts the call", async () => {
    const { post, metrics, downstream } = setup(user);
    const res = await post(call("list_codes", { kodeverksnavn: "Landkoder" }));
    expect(res.status).toBe(200);
    const text = await res.text();
    expect(text).toContain("[truncated]");
    expect(text.length).toBeLessThan(MAX_RESPONSE_BYTES * 2);
    // The downstream call carries the OBO token, never the inbound one.
    expect(downstream).toEqual([{ auth: "Bearer obo-token" }]);
    expect(metrics.render()).toContain('mcp_tool_calls_total{tool="list_codes",outcome="ok"} 1');
  });

  it("advertises readOnlyHint", async () => {
    const { post } = setup(user);
    const text = await (await post(JSON.stringify({ jsonrpc: "2.0", id: 1, method: "tools/list" }))).text();
    expect(text).toContain('"readOnlyHint":true');
  });
});
