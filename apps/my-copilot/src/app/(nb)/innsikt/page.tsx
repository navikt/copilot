import { CurrencyExchangeIcon, PadlockLockedIcon } from "@navikt/aksel-icons";
import { Box, Heading, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import { NavCard } from "@/components/navigation/nav-card";
import { PageHero } from "@/components/page-hero";

export const metadata: Metadata = {
  title: "Innsikt",
  description: "Tall om Copilot i Nav: bruk, adopsjon, kostnad og modellpriser.",
};

// LinkCard hides the icon from screen readers, so the description says «Krever innlogging» too.
const lock = <PadlockLockedIcon aria-hidden fontSize="1.75rem" />;

// The overview page for the group Innsikt (§6.2 in docs/nav-pilot-dokumentasjon-forslag.md).
// The private pages are not prefetched: without a login, the prefetch only fetches a redirect.
export default function Innsikt() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero title="Innsikt" description="Tall om Copilot i Nav: bruk, adopsjon, kostnad og modellpriser." />
      <Box
        paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <nav aria-labelledby="sider-i-denne-delen">
          <VStack gap="space-16">
            <Heading size="medium" level="2" id="sider-i-denne-delen">
              Sider i denne delen
            </Heading>
            <div className="grid gap-4 sm:grid-cols-2">
              <NavCard
                href="/statistikk"
                prefetch={false}
                icon={lock}
                title="Statistikk"
                description="Bruksdata og trender for GitHub Copilot i Nav. Krever innlogging."
              />
              <NavCard
                href="/adopsjon"
                prefetch={false}
                icon={lock}
                title="Adopsjon"
                description="AI-tilpasninger i navikt-repoene. Krever innlogging."
              />
              <NavCard
                href="/kostnad"
                prefetch={false}
                icon={lock}
                title="Kostnad"
                description="Hva modellene har kostet så langt denne måneden. Krever innlogging."
              />
              <NavCard
                href="/priser"
                icon={<CurrencyExchangeIcon aria-hidden fontSize="1.75rem" />}
                title="Modellpriser"
                description="Hva hver modell koster per forespørsel."
              />
            </div>
          </VStack>
        </nav>
      </Box>
    </main>
  );
}
