import { apmOptions, ignoreErrors, propagateExtraOrigins, scrubEvent } from "./apm";

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
        attributes: { fromUrl: "https://x.nav.no/b?u=2", toUrl: "https://x.nav.no/12345678901", toRoute: "/c/12345678901" },
      },
    };
    const out = scrubEvent(event);
    expect(out.meta.page.url).toBe("https://ki-utvikling.nav.no/a/[REDACTED]");
    expect(out.payload.attributes.fromUrl).toBe("https://x.nav.no/b");
    expect(out.payload.attributes.toUrl).toBe("https://x.nav.no/[REDACTED]");
    expect(out.payload.attributes.toRoute).toBe("/c/[REDACTED]");
  });
});
