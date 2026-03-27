import { Context, trace } from "@opentelemetry/api";
import pino, { Logger } from "pino";

const logger: Logger = pino({
  base: undefined, // remove default fields
  formatters: {
    // display level as a string
    level: (label) => {
      return {
        level: label,
      };
    },
  },
});

export function getLoggerWithTraceContext(context: Context) {
  const current_span = trace.getSpan(context);
  const trace_id = current_span?.spanContext().traceId;
  const span_id = current_span?.spanContext().spanId;

  return logger.child({ trace_id, span_id });
}

export function getTraceId(ctx: Context): string | undefined {
  return trace.getSpan(ctx)?.spanContext().traceId;
}
