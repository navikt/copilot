import { CurrencyExchangeIcon, PadlockLockedIcon, CpuIcon, MonitorIcon, ClockIcon } from "@navikt/aksel-icons";
import { Box, Heading, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import { NavCard } from "@/components/navigation/nav-card";
import { PageHero } from "@/components/page-hero";
import { HeroRibbons } from "@/components/hero-ribbons/hero-ribbons";

export const metadata: Metadata = {
  title: "Innsikt",
  description: "Tall om hvordan Nav bruker Copilot og andre KI-verktøy, hva det koster, og hvilke modeller vi velger.",
};

// LinkCard hides the icon from screen readers, so the description says «Krever innlogging» too.
const lock = <PadlockLockedIcon aria-hidden fontSize="1.75rem" />;

// The overview page for the group Innsikt (V7 in copilot-intern beslutningslogg.md).
// The private pages are not prefetched: without a login, the prefetch only fetches a redirect.
export default function Innsikt() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero
        title="Innsikt"
        description="Tall om hvordan Nav bruker Copilot og andre KI-verktøy, hva det koster, og hvilke modeller vi velger."
        background={<HeroRibbons />}
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
                href="/innsikt/trender"
                prefetch={false}
                icon={lock}
                title="Trender"
                description="Hvordan bruken endrer seg måned for måned, og om nye brukere fortsetter. Krever innlogging."
              />
              <NavCard
                href="/innsikt/team"
                prefetch={false}
                icon={lock}
                title="Team"
                description="Copilot-bruk og kostnad per team, med en kostnadsrapport du kan skrive ut. Krever innlogging."
              />
              <NavCard
                href="/innsikt/tilpasninger"
                prefetch={false}
                icon={lock}
                title="Tilpasninger"
                description="Hvor mange navikt-repoer som har instruksjoner, agenter, skills og prompts, fordelt på team og språk. Krever innlogging."
              />
              <NavCard
                href="/innsikt/undersokelser"
                prefetch={false}
                icon={lock}
                title="Undersøkelser"
                description="Resultater fra avsluttede brukerundersøkelser om KI-verktøy i Nav. Krever innlogging."
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
