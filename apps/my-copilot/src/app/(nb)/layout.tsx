import { SiteShell, type ShellLabels } from "@/components/site-shell";
import type { Metadata } from "next";
import "../globals.css";

export const metadata: Metadata = {
  title: {
    template: "%s — Oh-My-Nav",
    default: "Oh-My-Nav",
  },
  description: "Nyheter, beste praksis og verktøy for AI-drevet utvikling i Nav.",
};

const labels: ShellLabels = {
  subscription: "Abonnement",
  subscriptionHref: "/abonnement",
  signIn: "Logg inn",
  privacy: "Personvern",
  privacyHref: "/personvern",
  accessibility: "Tilgjengelighet",
  accessibilityHref: "/tilgjengelighet",
};

export default function NorwegianLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <SiteShell lang="nb" labels={labels}>
      {children}
    </SiteShell>
  );
}
