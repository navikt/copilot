import { SiteShell, type ShellLabels } from "@/components/site-shell";
import type { Metadata } from "next";
import "../globals.css";

const description = "News, practice and tooling for AI-assisted development at Nav.";

export const metadata: Metadata = {
  title: {
    template: "%s — Oh-My-Nav",
    default: "Oh-My-Nav",
  },
  description,
  metadataBase: new URL("https://ki-utvikling.nav.no"),
  openGraph: {
    type: "website",
    locale: "en_GB",
    siteName: "Oh-My-Nav",
    title: "Oh-My-Nav",
    description,
  },
};

// The rest of the site is Norwegian. English pages keep the same chrome but
// declare lang="en", and the two legal pages stay Norwegian, marked with
// hreflang so a screen reader and a search engine both know what they get.
const labels: ShellLabels = {
  subscription: "Subscription",
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
