// Runs the host redirects in next.config.ts through the same Next helpers the
// server uses (server/lib/router-utils/resolve-routes.js), so the custom
// regex and the `has: host` condition are checked the way production sees them.
import { describe, it, expect } from "vitest";
import type { IncomingMessage } from "node:http";
import nextConfig from "../next.config";
import { buildCustomRoute } from "next/dist/server/lib/router-utils/filesystem";
import { matchHas, prepareDestination } from "next/dist/shared/lib/router/utils/prepare-destination";
import { formatUrl } from "next/dist/shared/lib/router/utils/format-url";
import { getRedirectStatus } from "next/dist/lib/redirect-status";

const routes = (await nextConfig.redirects!()).map((r) => buildCustomRoute("redirect", r));

function resolve(host: string, url: string): { status: number; location: string } | null {
  const parsed = new URL(url, `https://${host}`);
  const query = Object.fromEntries(parsed.searchParams);
  const req = { headers: { host } } as unknown as IncomingMessage;
  for (const route of routes) {
    let params = route.match(parsed.pathname);
    if (params && (route.has || route.missing)) {
      const hasParams = matchHas(req, query, route.has, route.missing);
      params = hasParams ? Object.assign(params, hasParams) : false;
    }
    if (!params) continue;
    const { parsedDestination } = prepareDestination({
      appendParamsToQuery: false,
      destination: route.destination,
      params,
      query,
    });
    const { query: destQuery, ...rest } = parsedDestination;
    const search = new URLSearchParams(destQuery as Record<string, string>).toString();
    return { status: getRedirectStatus(route), location: formatUrl({ ...rest, search: search && `?${search}` }) };
  }
  return null;
}

describe("old host redirects", () => {
  it.each([
    ["min-copilot.intern.nav.no", "https://ki-utvikling.nav.no"],
    ["min-copilot.ansatt.nav.no", "https://ki-utvikling.nav.no"],
    ["min-copilot.intern.dev.nav.no", "https://ki-utvikling.ekstern.dev.nav.no"],
    ["min-copilot.ansatt.dev.nav.no", "https://ki-utvikling.ekstern.dev.nav.no"],
  ])("%s answers 308 to %s with path and query kept", (host, target) => {
    expect(resolve(host, "/")).toEqual({ status: 308, location: `${target}/` });
    expect(resolve(host, "/praksis/guide/agenter?utm=a&b=1")).toEqual({
      status: 308,
      location: `${target}/praksis/guide/agenter?utm=a&b=1`,
    });
    expect(resolve(host, "/statistikk")).toEqual({ status: 308, location: `${target}/statistikk` });
    // Path names that only start with an excluded word are still redirected.
    expect(resolve(host, "/apikey")?.location).toBe(`${target}/apikey`);
  });

  it.each([
    "/oauth2/callback?code=x",
    "/oauth2/login",
    "/api/copilot",
    "/api",
    "/internal/x",
    "/health",
    "/_next/static/a.js",
  ])("leaves %s on the old host", (path) => {
    expect(resolve("min-copilot.intern.nav.no", path)).toBeNull();
  });

  it("leaves the new host and pod-IP probes alone", () => {
    expect(resolve("ki-utvikling.nav.no", "/praksis")).toBeNull();
    expect(resolve("10.0.0.1:3000", "/health")).toBeNull();
  });
});
