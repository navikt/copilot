import { EXPLANATION_PAGES, GUIDE_PAGES, type DocLink } from "@/components/nav-pilot/doc-pages";

// The site menu: V6, V7 and V8 in docs/nav-pilot-dokumentasjon-forslag.md.

export type NavLink = { href: string; label: string };
export type NavGroup = {
  label: string;
  /** A group with href is a plain link. A group with items is a disclosure button. */
  href?: string;
  /** The overview page of a group with items. It opens the group, but is not listed in it. */
  overview?: string;
  items?: NavLink[];
};

// Five flat links in the header. English pages show the English labels but
// link to the Norwegian pages.
export const TOP_LINKS: { href: string; nb: string; en: string }[] = [
  { href: "/kom-i-gang", nb: "Kom i gang", en: "Get started" },
  { href: "/nav-pilot", nb: "nav-pilot", en: "nav-pilot" },
  { href: "/verktoy", nb: "Tilpasning", en: "Customisation" },
  { href: "/praksis", nb: "Praksis og regler", en: "Practice and rules" },
  { href: "/innsikt", nb: "Innsikt", en: "Insights" },
];

const under = (pathname: string, href: string) => pathname === href || pathname.startsWith(href + "/");

/** The href of the header link whose group this page belongs to, if any. */
export function activeTop(pathname: string): string | undefined {
  const owns = (...hrefs: string[]) => hrefs.some((h) => under(pathname, h));
  if (owns("/kom-i-gang", "/nav-pilot/lokal")) return "/kom-i-gang";
  if (owns("/verktoy", "/nav-pilot/agentpakker")) return "/verktoy";
  if (owns("/nav-pilot", "/cplt")) return "/nav-pilot";
  if (owns("/praksis", "/retningslinjer")) return "/praksis";
  if (owns("/innsikt", "/statistikk", "/adopsjon", "/kostnad", "/priser", "/modeller")) return "/innsikt";
}

/** Pages under the nav-pilot umbrella, which have the section menu. */
export const inSection = (pathname: string) =>
  ["/kom-i-gang", "/verktoy", "/nav-pilot"].some((h) => under(pathname, h));

const fromDocs = (pages: DocLink[]): NavLink[] => pages.map((p) => ({ href: p.href, label: p.title }));

export const SECTION: NavGroup[] = [
  { label: "Oversikt", href: "/nav-pilot" },
  {
    label: "Kom i gang",
    items: [
      { label: "Copilot og nav-pilot", href: "/kom-i-gang" },
      { label: "Lokal modell på Mac", href: "/nav-pilot/lokal" },
      { label: "Egen server", href: "/nav-pilot/lokal/egen-server" },
      { label: "Din første decide-hook", href: "/nav-pilot/lokal/decide" },
    ],
  },
  { label: "Guider", overview: "/nav-pilot/guider", items: fromDocs(GUIDE_PAGES) },
  {
    label: "Tilpasning",
    items: [
      { label: "Verktøykatalog", href: "/verktoy" },
      { label: "Agentpakker", href: "/nav-pilot/agentpakker" },
    ],
  },
  {
    label: "Referanse",
    items: [
      { label: "Kommandoer og konfig", href: "/nav-pilot/referanse" },
      { label: "Klienter", href: "/nav-pilot/klienter" },
      { label: "Kjente begrensninger i cplt", href: "/nav-pilot/referanse/cplt-begrensninger" },
    ],
  },
  { label: "Forklaring", overview: "/nav-pilot/forklaring", items: fromDocs(EXPLANATION_PAGES) },
  { label: "Sandkassen (cplt)", href: "/cplt" },
];

/** The section-menu group that holds this page. It starts open, and names the label line above the title. */
export const sectionGroup = (pathname: string): NavGroup | undefined =>
  SECTION.find((g) => g.overview === pathname || g.items?.some((i) => i.href === pathname));
