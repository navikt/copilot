import { act, fireEvent, render, screen } from "@testing-library/react";
import { CONSENT_KEY, setConsent } from "@/lib/innblikk-consent";
import { Innblikk, InnblikkChoice, innblikkBeforeSend } from "./innblikk";

const nav = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("next/navigation", () => ({ usePathname: () => nav.pathname }));
// next/script injects from an effect; a plain tag makes the render observable.
vi.mock("next/script", () => ({ default: (props: Record<string, string>) => <script {...props} /> }));

const SRC = "https://cdn.nav.no/team-researchops/sporing/sporing.js";
const scriptIn = (c: HTMLElement) => c.querySelector(`script[src="${SRC}"]`);

const banner = () => screen.queryByRole("region", { name: "Nettstatistikk" });

beforeEach(() => {
  localStorage.clear();
  setConsent(true);
});

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

describe("consent", () => {
  beforeEach(() => {
    localStorage.clear();
    nav.pathname = "/nyheter";
  });

  it("shows the banner and no script while no choice is stored", () => {
    const { container } = render(<Innblikk websiteId="abc" />);
    expect(banner()).not.toBeNull();
    expect(scriptIn(container)).toBeNull();
  });

  it("loads the script after Ja (control: Nei never loads it and sets the opt-out flags)", () => {
    const yes = render(<Innblikk websiteId="abc" />);
    fireEvent.click(screen.getByRole("button", { name: "Ja" }));
    expect(scriptIn(yes.container)).not.toBeNull();
    expect(banner()).toBeNull();
    expect(JSON.parse(localStorage.getItem(CONSENT_KEY)!)).toMatchObject({ analytics: true, version: 1 });
    yes.unmount();

    localStorage.clear();
    const no = render(<Innblikk websiteId="abc" />);
    fireEvent.click(screen.getByRole("button", { name: "Nei" }));
    expect(scriptIn(no.container)).toBeNull();
    expect(banner()).toBeNull();
    expect(localStorage.getItem("umami.disabled")).toBe("1");
    expect(localStorage.getItem("sporing.disabled")).toBe("1");
  });

  it.each(["/admin", "/innsikt/team", "/%61dmin"])(
    "never shows the banner on logged-in page %s (control: /nyheter shows it)",
    (p) => {
      nav.pathname = p;
      render(<Innblikk websiteId="abc" />);
      expect(banner()).toBeNull();
    }
  );

  it("never shows the banner without an ID (control: with an ID it shows)", () => {
    const none = render(<Innblikk websiteId={undefined} />);
    expect(banner()).toBeNull();
    none.unmount();
    render(<Innblikk websiteId="abc" />);
    expect(banner()).not.toBeNull();
  });

  it("is a labelled region with keyboard-reachable buttons and a privacy link, in English on /en", () => {
    render(<Innblikk websiteId="abc" lang="en" />);
    const region = screen.getByRole("region", { name: "Web statistics" });
    for (const name of ["Yes", "No"]) expect(screen.getByRole("button", { name }).tabIndex).toBe(0);
    expect(screen.getByRole("link", { name: /privacy/i }).getAttribute("href")).toBe("/personvern");
    expect(region.querySelector('[aria-modal="true"]')).toBeNull();
  });

  it("the hook returns false once consent is withdrawn (control: true while given)", () => {
    const url = "https://ki-utvikling.nav.no/nyheter";
    act(() => setConsent(true));
    expect(innblikkBeforeSend("event", { url })).not.toBe(false);
    act(() => setConsent(false));
    expect(innblikkBeforeSend("event", { url })).toBe(false);
    localStorage.clear();
    expect(innblikkBeforeSend("event", { url })).toBe(false);
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

describe("InnblikkChoice", () => {
  it("shows the stored choice and changes it (control: starts undecided)", () => {
    localStorage.clear();
    render(<InnblikkChoice />);
    expect(screen.getByText(/ikke valgt/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Nei" }));
    expect(screen.getByText(/sagt nei/)).toBeTruthy();
    expect(localStorage.getItem("umami.disabled")).toBe("1");
    fireEvent.click(screen.getByRole("button", { name: /Ja/ }));
    expect(screen.getByText(/sagt ja/)).toBeTruthy();
    expect(localStorage.getItem("umami.disabled")).toBeNull();
  });
});
