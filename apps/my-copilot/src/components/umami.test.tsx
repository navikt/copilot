import { render } from "@testing-library/react";
import { Umami, umamiBeforeSend } from "./umami";

const nav = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("next/navigation", () => ({ usePathname: () => nav.pathname }));
// next/script injects from an effect; a plain tag makes the render observable.
vi.mock("next/script", () => ({ default: (props: Record<string, string>) => <script {...props} /> }));

const SRC = "https://cdn.nav.no/team-researchops/sporing/sporing.js";
const scriptIn = (c: HTMLElement) => c.querySelector(`script[src="${SRC}"]`);

describe("Umami", () => {
  it.each(["/", "/nyheter", "/innsikt"])("renders on public page %s when the ID is set", (p) => {
    nav.pathname = p;
    const el = scriptIn(render(<Umami websiteId="abc" />).container);
    expect(el?.getAttribute("data-website-id")).toBe("abc");
    expect(el?.getAttribute("data-domains")).toBe("ki-utvikling.nav.no");
    expect(el?.getAttribute("data-exclude-search")).toBe("true");
  });

  it("renders nothing when the ID is unset (control: same page with ID renders)", () => {
    nav.pathname = "/nyheter";
    expect(scriptIn(render(<Umami websiteId={undefined} />).container)).toBeNull();
    expect(scriptIn(render(<Umami websiteId="" />).container)).toBeNull();
    expect(scriptIn(render(<Umami websiteId="abc" />).container)).not.toBeNull();
  });

  it.each(["/abonnement", "/admin", "/admin/kostnad", "/innsikt/team", "/statistikk", "/nav-pilot/undersokelse"])(
    "never renders on logged-in page %s, even with the ID set",
    (p) => {
      nav.pathname = p;
      expect(scriptIn(render(<Umami websiteId="abc" />).container)).toBeNull();
    }
  );
});

describe("umamiBeforeSend", () => {
  it("cancels events from logged-in pages (control: public page passes)", () => {
    expect(umamiBeforeSend("event", { url: "https://ki-utvikling.nav.no/admin/verdi" })).toBe(false);
    expect(umamiBeforeSend("event", { url: "https://ki-utvikling.nav.no/nyheter" })).not.toBe(false);
  });

  it("drops the hash from url and referrer (control: plain URL unchanged)", () => {
    expect(
      umamiBeforeSend("event", { url: "https://ki-utvikling.nav.no/ordbok#token", referrer: "https://x.no/a#b" })
    ).toEqual({ url: "https://ki-utvikling.nav.no/ordbok", referrer: "https://x.no/a" });
    expect(umamiBeforeSend("event", { url: "https://ki-utvikling.nav.no/ordbok" })).toEqual({
      url: "https://ki-utvikling.nav.no/ordbok",
      referrer: undefined,
    });
  });
});
