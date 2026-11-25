import { subscribe } from "node:diagnostics_channel";
import { readFileSync } from "node:fs";
import type { IncomingMessage, ServerResponse } from "node:http";
import { join } from "node:path";
import { Counter, Histogram } from "@prometheus-io/client";

// Server-side HTTP metrics with the same names and labels as the Go apps
// (copilot-api/http_metrics.go). Next.js has no hook that sees each response's
// status, so this listens on Node's built-in http.server diagnostics channels.

type Route = { page: string; regex: RegExp };
type Manifest = { staticRoutes?: { page: string; regex: string }[]; dynamicRoutes?: { page: string; regex: string }[] };

const METHODS = new Set(["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]);

export function routesFromManifest(manifest: Manifest): Route[] {
  return [...(manifest.staticRoutes ?? []), ...(manifest.dynamicRoutes ?? [])].map((r) => ({
    page: r.page,
    regex: new RegExp(r.regex),
  }));
}

// The route template from routes-manifest.json, never the raw path, so the
// label set stays bounded. Anything else (static assets, 404s) is "unmatched".
export function routeFor(url: string | undefined, routes: Route[]): string {
  const path = (url ?? "/").split("?")[0];
  return routes.find((r) => r.regex.test(path))?.page ?? "unmatched";
}

const requests = new Counter({
  name: "http_requests_total",
  help: "HTTP requests by method, route template and status code.",
  labelNames: ["method", "route", "code"],
});
const duration = new Histogram({
  name: "http_request_duration_seconds",
  help: "HTTP request duration in seconds.",
  labelNames: ["route"],
});

export function recordHttpRequest(method: string | undefined, route: string, code: number, seconds: number) {
  requests.inc({ method: METHODS.has(method ?? "") ? method! : "other", route, code: String(code) });
  duration.observe({ route }, seconds);
}

let started = false;
let failureLogged = false;

// A throw inside a diagnostics_channel subscriber becomes an uncaughtException
// and takes the process down, so metrics failures are swallowed here.
function safely<T>(fn: (m: T) => void) {
  return (m: unknown) => {
    try {
      fn(m as T);
    } catch (err) {
      if (!failureLogged) {
        failureLogged = true;
        console.debug("http metrics: failed to record a request", err);
      }
    }
  };
}

export function startHttpMetrics(manifestPath = join(process.cwd(), ".next", "routes-manifest.json")) {
  if (started) return; // a second subscription would count every request twice
  started = true;
  let routes: Route[] = [];
  try {
    routes = routesFromManifest(JSON.parse(readFileSync(manifestPath, "utf8")));
  } catch {
    // No manifest (unusual dev layout): every request counts as "unmatched".
  }
  const startTimes = new WeakMap<IncomingMessage, bigint>();
  subscribe(
    "http.server.request.start",
    safely<{ request: IncomingMessage }>(({ request }) => startTimes.set(request, process.hrtime.bigint()))
  );
  subscribe(
    "http.server.response.finish",
    safely<{ request: IncomingMessage; response: ServerResponse }>(({ request, response }) => {
      const start = startTimes.get(request);
      const seconds = start === undefined ? 0 : Number(process.hrtime.bigint() - start) / 1e9;
      recordHttpRequest(request.method, routeFor(request.url, routes), response.statusCode, seconds);
    })
  );
}
