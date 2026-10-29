// Browser telemetry config for @nais/apm, initialised in src/instrumentation-client.ts.
// @nais/apm always runs its own PII scrubber (fnr, email, token params) after
// our beforeSend; scrubEvent adds the stricter rules this app had under Faro:
// every query string and fragment goes.

const PII_PATTERN = /\b\d{11}\b/g;
const URL_SHAPED = /^(https?:\/\/|\/)/;

// Extra hosts we propagate W3C trace headers to, on top of the SDK's own
// origin + *.nav.no floor. Anchored at the start of the URL and terminated at
// a host boundary (path, port, or end of string) so that lookalike hosts such
// as https://x.nav.cloud.nais.io.evil.com do NOT match (CodeQL alert #31).
export const propagateExtraOrigins = [/^https:\/\/([a-z0-9-]+\.)*nav\.cloud\.nais\.io(\/|:|$)/];

// Crypto-wallet browser extensions (MetaMask and others that inject
// window.ethereum) throw into every page they run on. Not our errors; Faro
// matches these against the message, name and stack of each error.
export const ignoreErrors = [/MetaMask/i, /\bethereum\b/i];

// A URL (absolute or root-relative) followed by a query or fragment, anywhere
// in a string: "GET /api?x=1", a stack frame filename, an exception message.
const URL_QUERY = /((?:https?:\/\/|\/)[^\s?#"'<>()]*)[?#][^\s"'<>()]*/g;

// Drops every query string and fragment, and redacts fødselsnummer-like
// numbers in the path left behind and in strings that are a URL on their own.
function scrub(s: string): string {
  const out = s.replace(URL_QUERY, (_, base: string) => base.replace(PII_PATTERN, "[REDACTED]"));
  return URL_SHAPED.test(out) ? out.replace(PII_PATTERN, "[REDACTED]") : out;
}

// OTLP trace items put span event attribute values at depth ~14
// (resourceSpans/scopeSpans/spans/events/attributes/value/arrayValue/values),
// so the cap sits above @nais/apm's 8 to keep reaching them.
const MAX_DEPTH = 16;

// Walks every string leaf in place. Key- and shape-agnostic: route_change,
// faro.performance.* `name`, `url.full`, exception values, stack frames,
// measurement context and OTLP span attributes all carry URLs in different
// places, and a query string such as ?username= or ?q= is user data in all.
function walk(value: unknown, depth: number): unknown {
  if (typeof value === "string") return scrub(value);
  if (value === null || typeof value !== "object" || depth >= MAX_DEPTH) return value;
  const obj = value as Record<string, unknown>;
  for (const key of Object.keys(obj)) obj[key] = walk(obj[key], depth + 1);
  return value;
}

type Item = { meta: { page?: { url?: string } }; payload: unknown };

// Runs before @nais/apm's own scrubber, which also covers fnr, e-mail and
// token params but keeps other query strings.
export function scrubEvent<T extends Item>(event: T): T {
  if (event.meta.page?.url) event.meta.page.url = scrub(event.meta.page.url);
  walk(event.payload, 0);
  return event;
}

export const apmOptions = {
  app: "my-copilot",
  namespace: "copilot",
  version: process.env.NEXT_PUBLIC_APP_VERSION || undefined,
  ignoreErrors,
  beforeSend: scrubEvent,
  tracing: { propagateExtraOrigins },
};
