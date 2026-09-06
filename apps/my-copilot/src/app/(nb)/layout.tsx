import { SiteShell, type ShellLabels } from "@/components/site-shell";
import type { Metadata } from "next";
import "../globals.css";

const description = "Nyheter, beste praksis og verktøy for AI-drevet utvikling i Nav.";

export const metadata: Metadata = {
  title: {
    template: "%s — Oh-My-Nav",
    default: "Oh-My-Nav",
  },
  description,
  // Without metadataBase, Next resolves og:image against http://localhost:3000
  // off Vercel, and every link preview breaks silently. Each root layout needs
  // its own copy; nothing is inherited across route groups.
  metadataBase: new URL("https://ki-utvikling.nav.no"),
  openGraph: {
    type: "website",
    locale: "nb_NO",
    siteName: "Oh-My-Nav",
    title: "Oh-My-Nav",
    description,
  },
};

const labels: ShellLabels = {
  subscription: "Abonnement",
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
