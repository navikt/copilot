import { SiteShell, type ShellLabels } from "@/components/site-shell";
import type { Metadata } from "next";
import "../globals.css";

const description = "Nyheter, beste praksis og verktøy for AI-drevet utvikling i Nav.";

export const metadata: Metadata = {
  title: {
    template: "%s — nav-pilot",
    default: "nav-pilot",
  },
  description,
  // Without metadataBase, Next resolves og:image against http://localhost:3000
  // off Vercel, and every link preview breaks silently. Each root layout needs
  // its own copy; nothing is inherited across route groups.
  metadataBase: new URL("https://ki-utvikling.nav.no"),
  openGraph: {
    type: "website",
    locale: "nb_NO",
    siteName: "nav-pilot",
    title: "nav-pilot",
    description,
  },
};

const labels: ShellLabels = {
  tagline: "Copilot i Nav",
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
  signIn: "Logg inn",
  privacy: "Personvern",
  privacyHref: "/personvern",
  privacyHrefLang: undefined,
  accessibility: "Tilgjengelighet",
  accessibilityHref: "/tilgjengelighet",
  accessibilityHrefLang: undefined,
  footerLang: undefined,
};

export default function NorwegianLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <SiteShell lang="nb" labels={labels}>
      {children}
    </SiteShell>
  );
}
