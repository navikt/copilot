import { SiteShell, type ShellLabels } from "@/components/site-shell";
import type { Metadata } from "next";
import "../globals.css";

export const metadata: Metadata = {
  title: {
    template: "%s — Oh-My-Nav",
    default: "Oh-My-Nav",
  },
  description: "News, practice and tooling for AI-assisted development at Nav.",
};

// The rest of the site is Norwegian. English pages keep the same chrome but
// declare lang="en", so search engines and screen readers get the right
// language, and they link back to the Norwegian pages for the legal texts.
const labels: ShellLabels = {
  subscription: "Subscription",
  subscriptionHref: "/abonnement",
  signIn: "Sign in",
  privacy: "Privacy",
  privacyHref: "/personvern",
  accessibility: "Accessibility",
  accessibilityHref: "/tilgjengelighet",
};

export default function EnglishLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <SiteShell lang="en" labels={labels}>
      {children}
    </SiteShell>
  );
}
