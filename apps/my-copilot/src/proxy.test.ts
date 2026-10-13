// Wonderwall lets the private routes through (app.yaml), so the proxy is the
// only gate in front of them. A request with no session or token must not
// reach the page or the data.
import { NextRequest } from "next/server";
import { beforeAll, describe, expect, it } from "vitest";
import { PRIVATE_ROUTES } from "../scripts/auto-login-ignore-paths.mjs";
import { config, proxy } from "./proxy";

const API_ROUTES = new Set(["/statistikk/json"]);

const request = (path: string) => new NextRequest(`https://ki-utvikling.nav.no${path}`);

beforeAll(() => {
  process.env.NAIS_TOKEN_INTROSPECTION_ENDPOINT = "http://texas.invalid/introspect";
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

  it("lets a public route through", async () => {
    const res = await proxy(request("/reisen"));
    expect(res.headers.get("x-middleware-next")).toBe("1");
  });
});
