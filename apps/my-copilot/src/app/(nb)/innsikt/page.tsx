import { CurrencyExchangeIcon, PadlockLockedIcon, CpuIcon, MonitorIcon, ClockIcon } from "@navikt/aksel-icons";
import { Box, Heading, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import { NavCard } from "@/components/navigation/nav-card";
import { PageHero } from "@/components/page-hero";

export const metadata: Metadata = {
  title: "Innsikt",
  description: "Tall om Copilot i Nav: bruk, adopsjon, kostnad, modellpriser, modellvalg og lokale modeller.",
};

// LinkCard hides the icon from screen readers, so the description says «Krever innlogging» too.
const lock = <PadlockLockedIcon aria-hidden fontSize="1.75rem" />;

// The overview page for the group Innsikt (§6.2 in docs/nav-pilot-dokumentasjon-forslag.md).
// The private pages are not prefetched: without a login, the prefetch only fetches a redirect.
export default function Innsikt() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero
        title="Innsikt"
        description="Tall om Copilot i Nav: bruk, adopsjon, kostnad, modellpriser og modellvalg."
      />
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
                href="/innsikt/bruk"
                prefetch={false}
                icon={lock}
                title="Bruk og kostnad"
                description="Hvor mange som bruker Copilot, hva det koster, og hvor det brukes. Krever innlogging."
              />
              <NavCard
                href="/innsikt/team"
                prefetch={false}
                icon={lock}
                title="Teaminnsikt"
                description="Copilot-bruk og kostnad per team. Krever innlogging."
              />
              <NavCard
                href="/innsikt/tilpasninger"
                prefetch={false}
                icon={lock}
                title="Tilpasninger"
                description="KI-tilpasninger i navikt-repoene. Krever innlogging."
              />
              <NavCard
                href="/priser"
                icon={<CurrencyExchangeIcon aria-hidden fontSize="1.75rem" />}
                title="Modellpriser"
                description="Hva hver modell koster per forespørsel."
              />
              <NavCard
                href="/modeller"
                icon={<CpuIcon aria-hidden fontSize="1.75rem" />}
                title="Modellvalg"
                description="Hvilken modell agentene bruker, og hvorfor."
              />
              <NavCard
                href="/innsikt/lokale-modeller"
                icon={<MonitorIcon aria-hidden fontSize="1.75rem" />}
                title="Lokale modeller"
                description="Hvilke modeller nav-pilot kan kjøre på din Mac, og hva målingene viser."
              />
              <NavCard
                href="/reisen"
                icon={<ClockIcon aria-hidden fontSize="1.75rem" />}
                title="Reisen"
                description="Hva Nav har bygget med KI-agenter, steg for steg."
              />
            </div>
          </VStack>
        </nav>
      </Box>
    </main>
  );
}
