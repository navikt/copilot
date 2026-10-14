// Wonderwall lets the private routes through (app.yaml), so the proxy is the
// only gate in front of them. A request with no session or token must not
// reach the page or the data.
import { NextRequest } from "next/server";
import { globSync } from "node:fs";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { PRIVATE_ROUTES } from "../scripts/auto-login-ignore-paths.mjs";
import { config, isPrivatePath, PRIVATE_API_PATHS, PRIVATE_PAGE_PATHS, PUBLIC_API_PATHS, proxy } from "./proxy";

const API_ROUTES = new Set(["/statistikk/json"]);

const request = (path: string, headers: Record<string, string> = {}) =>
  new NextRequest(`https://ki-utvikling.nav.no${path}`, { headers });

beforeAll(() => {
  process.env.NAIS_TOKEN_INTROSPECTION_ENDPOINT = "http://texas.invalid/introspect";
});

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
  process.env.NAIS_TOKEN_INTROSPECTION_ENDPOINT = "http://texas.invalid/introspect";
});

describe("isPrivatePath and config.matcher", () => {
  it("does not match prefix collisions", () => {
    expect(isPrivatePath("/statistikkfoo")).toBe(false);
    expect(isPrivatePath("/innsikt/teams")).toBe(false);
    expect(isPrivatePath("/adopsjon-test")).toBe(false);
    expect(isPrivatePath("/kostnadsfri")).toBe(false);
  });

  it("matcher covers every private page and API path", () => {
    const matcherBases = config.matcher.map((m) => m.replace("/:path*", ""));
    for (const path of [...PRIVATE_PAGE_PATHS, ...PRIVATE_API_PATHS]) {
      expect(matcherBases.some((b) => path === b || path.startsWith(b + "/"))).toBe(true);
    }
  });
});

describe("proxy without a token", () => {
  it.each(PRIVATE_ROUTES)("guards %s", async (path) => {
    // Only /x/:path* matchers count: an exact one would leave the subroutes unguarded.
    const bases = config.matcher.filter((m) => m.endsWith("/:path*")).map((m) => m.slice(0, -"/:path*".length));
    expect(bases.some((b) => path === b || path.startsWith(b + "/"))).toBe(true);
    const res = await proxy(request(path));
    if (API_ROUTES.has(path)) {
      expect(res.status).toBe(401);
    } else {
      expect(res.status).toBe(307);
      const location = new URL(res.headers.get("location")!);
      expect(location.pathname).toBe("/oauth2/login");
      expect(location.searchParams.get("redirect")).toBe(path);
    }
  });

  it("preserves query string in login redirect", async () => {
    const res = await proxy(request("/statistikk?tab=models"));
    expect(res.status).toBe(307);
    expect(new URL(res.headers.get("location")!).searchParams.get("redirect")).toBe("/statistikk?tab=models");
  });

  it("lets a public route through", async () => {
    const res = await proxy(request("/reisen"));
    expect(res.headers.get("x-middleware-next")).toBe("1");
  });

  // Next resolves the decoded path to the route handler (filesystem.js, "check
  // decoded variant as well"), so an escaped prefix must not slip past the gate.
  it.each(["/%61pi/usage", "/api%2Fusage", "/%73tatistikk/json"])("guards the escaped %s", async (path) => {
    expect((await proxy(request(path))).status).toBe(401);
  });

  it("redirects an escaped page path with the decoded path", async () => {
    const res = await proxy(request("/%61dmin/kostnad"));
    expect(res.status).toBe(307);
    expect(new URL(res.headers.get("location")!).searchParams.get("redirect")).toBe("/admin/kostnad");
  });

  it("falls back to the raw path on a malformed escape", async () => {
    const res = await proxy(request("/reisen/%E0%A4%A"));
    expect(res.headers.get("x-middleware-next")).toBe("1");
  });
});

describe("proxy in development mode", () => {
  it("allows requests without Texas configured", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "");

    const res = await proxy(request("/statistikk"));
    expect(res.headers.get("x-middleware-next")).toBe("1");
  });

  it("enforces auth when Texas is configured", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "http://localhost:8080/introspect");

    const res = await proxy(request("/statistikk"));
    expect(res.status).toBe(307);
  });
});

// Regression for #1070: Wonderwall is in front with autoLoginIgnorePaths covering
// the private paths, so a client-supplied Authorization header reaches the pod
// untouched when there is no session. The proxy must validate the token with
// Texas rather than checking header presence.
describe("proxy token validation with Texas", () => {
  it("throws when NAIS_TOKEN_INTROSPECTION_ENDPOINT is unset in production", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "");

    await expect(proxy(request("/api/usage", { authorization: "Bearer anything" }))).rejects.toThrow(
      "NAIS_TOKEN_INTROSPECTION_ENDPOINT is not defined"
    );
  });

  it("passes through when Texas says the token is active", async () => {
    const fetchSpy = vi
      .fn()
      .mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ active: true }), { status: 200 })));
    vi.stubGlobal("fetch", fetchSpy);

    const pageRes = await proxy(request("/statistikk", { authorization: "Bearer valid-jwt" }));
    expect(pageRes.headers.get("x-middleware-next")).toBe("1");
    expect(fetchSpy).toHaveBeenCalledWith(
      "http://texas.invalid/introspect",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ identity_provider: "entra_id", token: "valid-jwt" }),
      })
    );

    const apiRes = await proxy(request("/api/usage", { authorization: "Bearer valid-jwt" }));
    expect(apiRes.headers.get("x-middleware-next")).toBe("1");
  });

  it("redirects a private page and returns 401 on a private API route when Texas says inactive", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ active: false }), { status: 200 })))
    );

    const pageRes = await proxy(request("/statistikk", { authorization: "Bearer forged" }));
    expect(pageRes.status).toBe(307);
    expect(new URL(pageRes.headers.get("location")!).pathname).toBe("/oauth2/login");

    const apiRes = await proxy(request("/api/usage", { authorization: "Bearer forged" }));
    expect(apiRes.status).toBe(401);
  });

  it("fails closed when Texas is unreachable", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("connect ECONNREFUSED")));

    const res = await proxy(request("/api/usage", { authorization: "Bearer valid-jwt" }));
    expect(res.status).toBe(401);
  });

  it("rejects a non-Bearer Authorization header without calling Texas", async () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);

    const res = await proxy(request("/api/usage", { authorization: "Basic dXNlcjpwYXNz" }));
    expect(res.status).toBe(401);
    expect(fetchSpy).not.toHaveBeenCalled();
  });
});

const PUBLIC_ROUTE_PATHS = ["/health", "/metrics", "/install"];

// A new route.ts under src/app must be behind the token check or be listed in
// PUBLIC_API_PATHS / PUBLIC_ROUTE_PATHS on purpose. Adding a file fails this test until it is one or the other.
describe("every route.ts under src/app", () => {
  const routes = globSync("src/app/**/route.ts").map(
    (f) =>
      "/" +
      f
        .replace(/^src\/app\//, "")
        .replace(/\/route\.ts$/, "")
        .split("/")
        .filter((s) => !(s.startsWith("(") && s.endsWith(")")))
        .join("/")
        .replace(/\[[^\]]+\]/g, "x")
  );

  it("finds the routes", () => expect(routes.length).toBeGreaterThan(15));

  it.each(routes)("%s is guarded or allowlisted", async (path) => {
    const publicPaths = [...PUBLIC_API_PATHS, ...PUBLIC_ROUTE_PATHS];
    const allowlisted = publicPaths.some((p) => path === p || path.startsWith(p + "/"));
    const matched = config.matcher.some(
      (m) => path === m.replace("/:path*", "") || path.startsWith(m.replace("/:path*", "") + "/")
    );
    if (allowlisted) {
      expect(isPrivatePath(path)).toBe(false);
      return;
    }
    expect(isPrivatePath(path)).toBe(true);
    expect(matched).toBe(true);
    expect((await proxy(request(path))).status).toBe(401);
  });
});

describe("PUBLIC_API_PATHS", () => {
  it("lets an allowlisted API path through without a token", async () => {
    PUBLIC_API_PATHS.push("/api/usage");
    try {
      expect(isPrivatePath("/api/usage")).toBe(false);
      expect((await proxy(request("/api/usage"))).headers.get("x-middleware-next")).toBe("1");
      expect((await proxy(request("/api/credits"))).status).toBe(401);
    } finally {
      PUBLIC_API_PATHS.pop();
    }
  });
});
