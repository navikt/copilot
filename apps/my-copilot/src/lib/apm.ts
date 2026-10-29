// Browser telemetry config for @nais/apm, initialised in src/instrumentation-client.ts.
// @nais/apm always runs its own PII scrubber (fnr, email, token params) after
// our beforeSend; scrubEvent adds the stricter rules this app had under Faro.

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

function sanitizeUrl(url: string): string {
  return url.split("?")[0].replace(PII_PATTERN, "[REDACTED]");
}

// Every URL-shaped string in a flat attribute map. Key-agnostic because the
// keys differ per event: route_change and faro.navigation carry
// fromUrl/toUrl/toRoute, faro.performance.* carry the request URL as `name`,
// faro.tracing.* carry `url.full`, securitypolicyviolation carries
// documentURI/blockedURI. A query string such as ?username= or ?q= is user
// data in every one of them.
function sanitizeUrls(attrs: Record<string, unknown> | undefined): void {
  if (!attrs) return;
  for (const [key, value] of Object.entries(attrs)) {
    if (typeof value === "string" && URL_SHAPED.test(value)) attrs[key] = sanitizeUrl(value);
  }
}

type Span = { attributes?: { value?: { stringValue?: string } }[] };
type Payload = {
  attributes?: Record<string, unknown>;
  resourceSpans?: { scopeSpans?: { spans?: Span[] }[] }[];
};
type Item = { meta: { page?: { url?: string } }; payload: unknown };

// Strips query strings and fødselsnummer-like numbers from the page URL, from
// every URL-shaped event attribute, and from the fetch/XHR span attributes in
// trace items (OTLP nests those deeper than @nais/apm's own scrubber walks).
export function scrubEvent<T extends Item>(event: T): T {
  if (event.meta.page?.url) {
    event.meta.page.url = sanitizeUrl(event.meta.page.url);
  }
  const payload = event.payload as Payload;
  sanitizeUrls(payload.attributes);
  for (const resourceSpan of payload.resourceSpans ?? []) {
    for (const scopeSpan of resourceSpan.scopeSpans ?? []) {
      for (const span of scopeSpan.spans ?? []) {
        for (const attr of span.attributes ?? []) {
          const value = attr.value;
          if (value?.stringValue && URL_SHAPED.test(value.stringValue)) {
            value.stringValue = sanitizeUrl(value.stringValue);
          }
        }
      }
    }
  }
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
