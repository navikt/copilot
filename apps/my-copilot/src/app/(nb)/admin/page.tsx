import type { Metadata } from "next";
import { BarChartIcon, TasklistIcon } from "@navikt/aksel-icons";
import { BodyLong, Box, VStack } from "@navikt/ds-react";
import { NavCard } from "@/components/navigation/nav-card";
import { PageHero } from "@/components/page-hero";
import { requireAdmin } from "@/lib/auth";

export const metadata: Metadata = {
  title: "Admin",
  description: "Sider for team copilot.",
  robots: { index: false, follow: false },
};

// New admin pages go under /admin and get a card here.
const pages = [
  {
    href: "/admin/kostnad",
    icon: <BarChartIcon aria-hidden fontSize="1.75rem" />,
    title: "Kostnadskontroll",
    description: "Forbruk og grenser for Copilot i Nav.",
  },
  {
    href: "/admin/undersokelser",
    icon: <TasklistIcon aria-hidden fontSize="1.75rem" />,
    title: "Undersøkelser",
    description: "Resultater fra brukerundersøkelsene.",
  },
];

export default async function AdminPage() {
  // The layout is the gate. This call keeps the page out of site search and guards it if the layout changes.
  await requireAdmin();
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero title="Admin" description="Sider for team copilot." />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-16">
            <BodyLong>Sidene her er bare for team copilot. Alle andre får «Fant ikke siden».</BodyLong>
            <div className="grid gap-4 sm:grid-cols-2">
              {pages.map((p) => (
                <NavCard key={p.href} {...p} />
              ))}
            </div>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
