import { SiteShell, type ShellLabels } from "@/components/site-shell";
import type { Metadata } from "next";
import "../globals.css";

const description = "Nyheter, beste praksis og verktøy for KI-drevet utvikling i Nav.";

// The site is «KI-produktutvikling i Nav» (the home page H1); nav-pilot is one part of it (#1192).
export const metadata: Metadata = {
  title: {
    template: "%s — KI-produktutvikling i Nav",
    default: "KI-produktutvikling i Nav",
  },
  description,
  // Without metadataBase, Next resolves og:image against http://localhost:3000
  // off Vercel, and every link preview breaks silently. Each root layout needs
  // its own copy; nothing is inherited across route groups.
  metadataBase: new URL("https://ki-utvikling.nav.no"),
  openGraph: {
    type: "website",
    locale: "nb_NO",
    siteName: "KI-produktutvikling i Nav",
    // No title or description here: Next fills og:title and og:description
    // from each page's own, so a shared link previews as that page.
  },
};

const labels: ShellLabels = {
  siteName: "KI-produktutvikling i Nav",
  mainMenu: "Hovedmeny",
  skip: "Hopp til innhold",
  menu: "Meny",
  back: "Tilbake",
  showSection: "Vis menyen for nav-pilot",
  search: "Søk",
  glossary: "Ordbok",
  otherLang: "English",
  otherLangHref: "/en/news",
  subscription: "Copilot-abonnement",
  subscriptionHref: "/abonnement",
  userMenu: "Brukermeny",
  budgetUsed: "Du har brukt {pct} % av AI-kredittene denne måneden",
  signIn: "Logg inn",
  privacy: "Personvern",
  privacyHref: "/personvern",
  privacyHrefLang: undefined,
  accessibility: "Tilgjengelighet",
  accessibilityHref: "/tilgjengelighet",
  accessibilityHrefLang: undefined,
};

export default function NorwegianLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <SiteShell lang="nb" labels={labels}>
      {children}
    </SiteShell>
  );
}
