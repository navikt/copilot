import { propagateExtraOrigins } from "./apm";

const matchesAny = (url: string) => propagateExtraOrigins.some((pattern) => pattern.test(url));

describe("propagateExtraOrigins", () => {
  it("matches nav.cloud.nais.io and its subdomains", () => {
    expect(matchesAny("https://nav.cloud.nais.io")).toBe(true);
    expect(matchesAny("https://my-app.nav.cloud.nais.io/api")).toBe(true);
    expect(matchesAny("https://deep.sub.nav.cloud.nais.io:8443/path")).toBe(true);
  });

  it("does not match lookalike hosts that merely contain the trusted domain", () => {
    expect(matchesAny("https://app.nav.cloud.nais.io.evil.com")).toBe(false);
    expect(matchesAny("https://app.nav.cloud.nais.io.evil.com/nav.cloud.nais.io")).toBe(false);
    expect(matchesAny("https://evilnav.cloud.nais.io.attacker.com")).toBe(false);
  });

  it("does not match untrusted schemes or embedded URLs", () => {
    expect(matchesAny("http://nav.cloud.nais.io")).toBe(false);
    expect(matchesAny("https://evil.com/?u=https://nav.cloud.nais.io")).toBe(false);
  });

  it("does not cover nav.no — that is the SDK's own propagation floor, not an extra origin", () => {
    expect(matchesAny("https://nav.no")).toBe(false);
    expect(matchesAny("https://telemetry.nav.no/collect")).toBe(false);
  });
});
