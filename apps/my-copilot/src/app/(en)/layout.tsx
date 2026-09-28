import { SiteShell, type ShellLabels } from "@/components/site-shell";
import type { Metadata } from "next";
import "../globals.css";

const description = "News, practice and tooling for AI-assisted development at Nav.";

export const metadata: Metadata = {
  title: {
    template: "%s — nav-pilot",
    default: "nav-pilot",
  },
  description,
  metadataBase: new URL("https://ki-utvikling.nav.no"),
  openGraph: {
    type: "website",
    locale: "en_GB",
    siteName: "nav-pilot",
    title: "nav-pilot",
    description,
  },
};

// The rest of the site is Norwegian. English pages keep the same chrome but
// declare lang="en", and the two legal pages stay Norwegian, marked with
// hreflang so a screen reader and a search engine both know what they get.
const labels: ShellLabels = {
  tagline: "Copilot at Nav",
  mainMenu: "Main menu",
  skip: "Skip to content",
  menu: "Menu",
  back: "Back",
  showSection: "Show the nav-pilot menu",
  search: "Search",
  glossary: "Glossary (Norwegian)",
  otherLang: "Norsk",
  otherLangHref: "/",
  subscription: "Copilot subscription",
  subscriptionHref: "/abonnement",
  signIn: "Sign in",
  privacy: "Privacy (Norwegian)",
  privacyHref: "/personvern",
  privacyHrefLang: "nb",
  accessibility: "Accessibility (Norwegian)",
  accessibilityHref: "/tilgjengelighet",
  accessibilityHrefLang: "nb",
  footerLang: "nb",
};

export default function EnglishLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <SiteShell lang="en" labels={labels}>
      {children}
    </SiteShell>
  );
}
