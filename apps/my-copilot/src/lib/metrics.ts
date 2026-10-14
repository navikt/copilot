import { Counter, register } from "prom-client";

// Errors Next.js reports through onRequestError (src/instrumentation.ts): a
// route handler, server action or server render that throws. route is the
// route template (/api/survey/[id]/responses), so the label set stays small.
// Reuses an existing registration so a second module instance does not throw.
const requestErrors =
  (register.getSingleMetric("my_copilot_request_errors_total") as Counter<"route" | "route_type"> | undefined) ??
  new Counter({
    name: "my_copilot_request_errors_total",
    help: "Unhandled errors in route handlers, server actions and server renders",
    labelNames: ["route", "route_type"],
  });

export function recordRequestError(route: string, routeType: string) {
  requestErrors.inc({ route, route_type: routeType });
}

export { register };
