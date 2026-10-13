import type { Metadata } from "next";
import { BodyLong, Box, Link, List, VStack } from "@navikt/ds-react";
import { ListItem } from "@navikt/ds-react/List";
import { PageHero } from "@/components/page-hero";
import { requireAdmin } from "@/lib/auth";

export const metadata: Metadata = {
  title: "Admin",
  description: "Sider for team copilot.",
  robots: { index: false, follow: false },
};

// New admin pages go under /admin and get a line here.
const pages = [
  { href: "/admin/kostnad", title: "Kostnadskontroll", description: "Forbruk og grenser for Copilot i Nav." },
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
            <List>
              {pages.map((p) => (
                <ListItem key={p.href}>
                  <Link href={p.href}>{p.title}</Link>: {p.description}
                </ListItem>
              ))}
            </List>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
