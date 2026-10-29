import { ignoreErrors, propagateExtraOrigins, scrubEvent } from "./apm";

const matchesAny = (url: string) => propagateExtraOrigins.some((pattern) => pattern.test(url));

describe("propagateExtraOrigins", () => {
  it("matches nav.cloud.nais.io and its subdomains", () => {
    expect(matchesAny("https://nav.cloud.nais.io")).toBe(true);
    expect(matchesAny("https://my-app.nav.cloud.nais.io/api")).toBe(true);
    expect(matchesAny("https://deep.sub.nav.cloud.nais.io:8443/path")).toBe(true);
  });

  it("does not match lookalike hosts that merely contain a trusted domain", () => {
    expect(matchesAny("https://app.nav.cloud.nais.io.evil.com")).toBe(false);
    expect(matchesAny("https://evilnav.cloud.nais.io")).toBe(false);
    expect(matchesAny("https://x.evil.com/nav.cloud.nais.io")).toBe(false);
  });

  it("does not match untrusted schemes or embedded URLs", () => {
    expect(matchesAny("http://nav.cloud.nais.io")).toBe(false);
    expect(matchesAny("https://evil.com/?u=https://nav.cloud.nais.io")).toBe(false);
  });
});

describe("ignoreErrors", () => {
  const ignored = (text: string) => ignoreErrors.some((pattern) => pattern.test(text));

  it("drops errors thrown by wallet extensions", () => {
    expect(ignored("Failed to connect to MetaMask")).toBe(true);
    expect(ignored("TypeError: Cannot read properties of undefined (reading 'ethereum')")).toBe(true);
    expect(ignored("Cannot redefine property: ethereum")).toBe(true);
  });

  it("keeps our own errors", () => {
    expect(ignored("Minified React error #418")).toBe(false);
    expect(ignored("Failed to fetch RSC payload")).toBe(false);
  });
});

describe("scrubEvent", () => {
  it("drops query strings and redacts 11-digit numbers in page and navigation URLs", () => {
    const event = {
      type: "event",
      meta: { page: { url: "https://ki-utvikling.nav.no/a/12345678901?id=1" } },
      payload: {
        name: "faro.navigation",
        attributes: {
          fromUrl: "https://x.nav.no/b?u=2",
          toUrl: "https://x.nav.no/12345678901",
          toRoute: "/c/12345678901",
        },
      },
    };
    const out = scrubEvent(event);
    expect(out.meta.page.url).toBe("https://ki-utvikling.nav.no/a/[REDACTED]");
    expect(out.payload.attributes.fromUrl).toBe("https://x.nav.no/b");
    expect(out.payload.attributes.toUrl).toBe("https://x.nav.no/[REDACTED]");
    expect(out.payload.attributes.toRoute).toBe("/c/[REDACTED]");
  });

  it("scrubs request URLs in performance and tracing events, whatever the key", () => {
    const event = {
      type: "event",
      meta: {},
      payload: {
        name: "faro.tracing.fetch",
        attributes: {
          "url.full": "https://ki-utvikling.nav.no/api/usage?username=ola",
          name: "/api/contributors?id=12345678901",
          "http.method": "GET",
          "http.user_agent": "Mozilla/5.0 (X11; Linux) 12345678901?",
        },
      },
    };
    const out = scrubEvent(event);
    expect(out.payload.attributes["url.full"]).toBe("https://ki-utvikling.nav.no/api/usage");
    expect(out.payload.attributes.name).toBe("/api/contributors");
    expect(out.payload.attributes["http.method"]).toBe("GET");
    expect(out.payload.attributes["http.user_agent"]).toBe("Mozilla/5.0 (X11; Linux) 12345678901?");
  });

  it("scrubs fetch span attributes inside trace items", () => {
    const event = {
      type: "trace",
      meta: {},
      payload: {
        resourceSpans: [
          {
            scopeSpans: [
              {
                spans: [
                  {
                    attributes: [
                      { key: "url.full", value: { stringValue: "https://x.nav.no/api/team-year?team=12345678901" } },
                      { key: "http.status_code", value: { intValue: 200 } },
                    ],
                  },
                ],
              },
            ],
          },
        ],
      },
    };
    const out = scrubEvent(event);
    expect(out.payload.resourceSpans[0].scopeSpans[0].spans[0].attributes[0].value.stringValue).toBe(
      "https://x.nav.no/api/team-year"
    );
  });
});
