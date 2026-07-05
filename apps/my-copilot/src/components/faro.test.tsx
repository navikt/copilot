import { propagateTraceHeaderCorsUrls } from "./faro";

const matchesAny = (url: string) => propagateTraceHeaderCorsUrls.some((pattern) => pattern.test(url));

describe("propagateTraceHeaderCorsUrls", () => {
  it("matches nav.no and its subdomains", () => {
    expect(matchesAny("https://nav.no")).toBe(true);
    expect(matchesAny("https://nav.no/some/path")).toBe(true);
    expect(matchesAny("https://www.nav.no")).toBe(true);
    expect(matchesAny("https://telemetry.nav.no/collect")).toBe(true);
    expect(matchesAny("https://deep.sub.domain.nav.no:8443/path")).toBe(true);
  });

  it("matches nav.cloud.nais.io and its subdomains", () => {
    expect(matchesAny("https://nav.cloud.nais.io")).toBe(true);
    expect(matchesAny("https://my-app.nav.cloud.nais.io/api")).toBe(true);
  });

  it("does not match lookalike hosts that merely contain a trusted domain", () => {
    expect(matchesAny("https://x.nav.no.evil.com")).toBe(false);
    expect(matchesAny("https://x.nav.no.evil.com/nav.no")).toBe(false);
    expect(matchesAny("https://app.nav.cloud.nais.io.evil.com")).toBe(false);
    expect(matchesAny("https://evilnav.no")).toBe(false);
    expect(matchesAny("https://nav.noevil.com")).toBe(false);
  });

  it("does not match untrusted schemes or embedded URLs", () => {
    expect(matchesAny("http://nav.no")).toBe(false);
    expect(matchesAny("https://evil.com/?u=https://nav.no")).toBe(false);
  });
});
