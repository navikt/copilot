import { describe, expect, it } from "vitest";
import { register } from "@prometheus-io/client";
import { recordHttpRequest, routeFor, routesFromManifest } from "./http-metrics";

// Shape of .next/routes-manifest.json for one static and one dynamic route.
const routes = routesFromManifest({
  staticRoutes: [{ page: "/admin", regex: "^/admin(?:/)?$" }],
  dynamicRoutes: [{ page: "/api/survey/[id]/responses", regex: "^/api/survey/([^/]+?)/responses(?:/)?$" }],
});

describe("http metrics", () => {
  it("labels a dynamic route with its pattern, never the raw path", async () => {
    const route = routeFor("/api/survey/abc123/responses?x=1", routes);
    expect(route).toBe("/api/survey/[id]/responses");
    recordHttpRequest("POST", route, 500, 0.02);
    const out = await register.metrics();
    expect(out).toContain('http_requests_total{method="POST",route="/api/survey/[id]/responses",code="500"} 1');
    expect(out).not.toContain("abc123");
  });

  it("maps unknown paths and methods to fixed values", async () => {
    expect(routeFor("/_next/static/chunk.js", routes)).toBe("unmatched");
    expect(routeFor("/admin", routes)).toBe("/admin");
    recordHttpRequest("PROPFIND", "unmatched", 404, 0);
    expect(await register.metrics()).toContain('http_requests_total{method="other",route="unmatched",code="404"} 1');
  });
});
