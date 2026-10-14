import type { Instrumentation } from "next";

// Next.js calls this for every route handler, server action and server render
// that throws. "Failed to find Server Action" never gets here; the
// MyCopilotErrorLogs rule in .nais/app.yaml covers it.
export const onRequestError: Instrumentation.onRequestError = async (err, request, ctx) => {
  if (process.env.NEXT_RUNTIME !== "nodejs") return;
  const { recordRequestError } = await import("@/lib/metrics");
  const { getLoggerWithTraceContext } = await import("@/lib/logger");
  const { context } = await import("@opentelemetry/api");
  recordRequestError(ctx.routePath, ctx.routeType);
  getLoggerWithTraceContext(context.active()).error(
    { err, route: ctx.routePath, route_type: ctx.routeType, method: request.method },
    "Unhandled request error"
  );
};
