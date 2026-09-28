import { PageHero } from "@/components/page-hero";
import { BodyLong, Box, VStack } from "@navikt/ds-react";
import NextLink from "next/link";
import { LinkableHeading } from "@/components/linkable-heading";
import type { Metadata } from "next";
import { InteractiveSetupWizard } from "@/components/nav-pilot/interactive-setup-wizard";

export const metadata: Metadata = {
  title: "Kom i gang",
  description: "Fra null til produktiv med GitHub Copilot i Nav på under 10 minutter.",
};

export default function KomIGangPage() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero title="Kom i gang" description="Fra null til produktiv med GitHub Copilot på under 10 minutter." />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
          marginInline="auto"
        >
          <VStack gap="space-32" className="max-w-3xl mx-auto">
            <VStack as="section" gap="space-8">
              <LinkableHeading id="hva-er-nav-pilot" size="medium" level="2">
                Hva er nav-pilot?
              </LinkableHeading>
              <BodyLong>
                nav-pilot er et CLI. Det installerer agenter, skills og instruksjoner fra Nav i repoet ditt, og starter
                Copilot CLI eller opencode i sandkassen cplt. Agenten{" "}
                <code className="font-mono text-xs">@nav-pilot</code> bruker kunnskapen til å planlegge apper på Nais.
                Selve modellen kjører i GitHub Copilot.{" "}
                <NextLink href="/nav-pilot" className="text-blue-600 hover:underline">
                  Mer om nav-pilot
                </NextLink>
              </BodyLong>
            </VStack>
            <div id="installer">
              <InteractiveSetupWizard />
            </div>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
