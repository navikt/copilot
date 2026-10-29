// Browser telemetry config for @nais/apm, initialised in src/instrumentation-client.ts.
// @nais/apm always runs its own PII scrubber (fnr, email, token params) after
// our beforeSend; scrubEvent adds the stricter rules this app had under Faro.

const PII_PATTERN = /\b\d{11}\b/g;

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

type Item = { meta: { page?: { url?: string } }; payload: unknown };

// Strips query strings and fødselsnummer-like numbers from the page URL and
// from the URL attributes that navigation and route_change events carry.
export function scrubEvent<T extends Item>(event: T): T {
  if (event.meta.page?.url) {
    event.meta.page.url = sanitizeUrl(event.meta.page.url);
  }
  const attrs = (event.payload as { attributes?: Record<string, string> }).attributes;
  for (const key of ["fromUrl", "toUrl", "toRoute"]) {
    if (attrs?.[key]) attrs[key] = sanitizeUrl(attrs[key]);
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
