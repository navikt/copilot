import { render } from "@testing-library/react";
import { Innblikk, innblikkBeforeSend } from "./innblikk";

const nav = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("next/navigation", () => ({ usePathname: () => nav.pathname }));
// next/script injects from an effect; a plain tag makes the render observable.
vi.mock("next/script", () => ({ default: (props: Record<string, string>) => <script {...props} /> }));

const SRC = "https://cdn.nav.no/team-researchops/sporing/sporing.js";
const scriptIn = (c: HTMLElement) => c.querySelector(`script[src="${SRC}"]`);

describe("Innblikk", () => {
  it.each(["/", "/nyheter", "/innsikt"])("renders on public page %s when the ID is set", (p) => {
    nav.pathname = p;
    const el = scriptIn(render(<Innblikk websiteId="abc" />).container);
    expect(el?.getAttribute("data-website-id")).toBe("abc");
    expect(el?.getAttribute("data-domains")).toBe("ki-utvikling.nav.no");
    expect(el?.getAttribute("data-exclude-search")).toBe("true");
    expect(el?.getAttribute("data-do-not-track")).toBe("true");
    expect(el?.getAttribute("data-before-send")).toBe("innblikkBeforeSend");
  });

  it("renders nothing when the ID is unset (control: same page with ID renders)", () => {
    nav.pathname = "/nyheter";
    expect(scriptIn(render(<Innblikk websiteId={undefined} />).container)).toBeNull();
    expect(scriptIn(render(<Innblikk websiteId="" />).container)).toBeNull();
    expect(scriptIn(render(<Innblikk websiteId="abc" />).container)).not.toBeNull();
  });

  it.each([
    "/abonnement",
    "/admin",
    "/admin/kostnad",
    "/innsikt/team",
    "/statistikk",
    "/nav-pilot/undersokelse",
    "/%61dmin",
  ])("never renders on logged-in page %s, even with the ID set", (p) => {
    nav.pathname = p;
    expect(scriptIn(render(<Innblikk websiteId="abc" />).container)).toBeNull();
  });
});

describe("innblikkBeforeSend", () => {
  it("cancels events from logged-in pages (control: public page passes)", () => {
    expect(innblikkBeforeSend("event", { url: "https://ki-utvikling.nav.no/admin/verdi" })).toBe(false);
    expect(innblikkBeforeSend("event", { url: "https://ki-utvikling.nav.no/nyheter" })).not.toBe(false);
  });

  it("drops the hash from url and referrer (control: plain URL unchanged)", () => {
    expect(
      innblikkBeforeSend("event", { url: "https://ki-utvikling.nav.no/ordbok#token", referrer: "https://x.no/a#b" })
    ).toEqual({ url: "https://ki-utvikling.nav.no/ordbok", referrer: "https://x.no/a" });
    expect(innblikkBeforeSend("event", { url: "https://ki-utvikling.nav.no/ordbok" })).toEqual({
      url: "https://ki-utvikling.nav.no/ordbok",
      referrer: undefined,
    });
  });

  it("drops a logged-in referrer (control: public referrer kept)", () => {
    const url = "https://ki-utvikling.nav.no/nyheter";
    expect(innblikkBeforeSend("event", { url, referrer: "https://ki-utvikling.nav.no/admin/kostnad/innsyn" })).toEqual({
      url,
      referrer: undefined,
    });
    expect(innblikkBeforeSend("event", { url, referrer: "https://ki-utvikling.nav.no/ordbok" })).toEqual({
      url,
      referrer: "https://ki-utvikling.nav.no/ordbok",
    });
  });

  it("decodes escaped paths like proxy.ts (control: escaped public path passes)", () => {
    expect(innblikkBeforeSend("event", { url: "https://ki-utvikling.nav.no/%61dmin" })).toBe(false);
    expect(innblikkBeforeSend("event", { url: "https://ki-utvikling.nav.no/%6Eyheter" })).not.toBe(false);
  });

  it("is installed on window at import, before sporing.js can read it", () => {
    expect((window as unknown as Record<string, unknown>).innblikkBeforeSend).toBe(innblikkBeforeSend);
  });
});
