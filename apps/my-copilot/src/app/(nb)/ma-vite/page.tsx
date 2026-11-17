import { PageHero } from "@/components/page-hero";
import { BodyLong, Box, Heading, Link, VStack } from "@navikt/ds-react";
import NextLink from "next/link";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Dette må du vite",
  description: "Det viktigste om KI-verktøy i Nav på én side: kom i gang, regler, sandkassen og hvor du får hjelp.",
};

const link = "text-blue-600 hover:underline";

export default function MaVitePage() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero
        title="Dette må du vite"
        description="Det viktigste om KI-verktøy i Nav på én side. Resten kan du lese når du trenger det."
      />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-24" className="max-w-3xl mx-auto">
            <VStack as="section" gap="space-8">
              <Heading size="medium" level="2">
                Kom i gang
              </Heading>
              <BodyLong>
                Bruk GitHub Copilot med Nav-lisensen. Ikke bruk private abonnementer eller verktøy med egen API-nøkkel.{" "}
                <NextLink href="/kom-i-gang" className={link}>
                  Kom i gang
                </NextLink>{" "}
                viser oppsettet steg for steg.
              </BodyLong>
            </VStack>

            <VStack as="section" gap="space-8">
              <Heading size="medium" level="2">
                Regler for sikkerhet og personvern
              </Heading>
              <BodyLong>
                Ikke gi verktøyene hemmeligheter eller personopplysninger, og bruk bare MCP-servere som Nav har
                godkjent. Hele listen over hva som er tillatt, står i{" "}
                <NextLink href="/retningslinjer" className={link}>
                  retningslinjene
                </NextLink>
                .
              </BodyLong>
            </VStack>

            <VStack as="section" gap="space-8">
              <Heading size="medium" level="2">
                Agenter skal kjøre i sandkassen
              </Heading>
              <BodyLong>
                En agent som kjører kommandoer på maskinen din, skal kjøre isolert. Lokalt gjør du det med cplt.{" "}
                <NextLink href="/nav-pilot/forklaring/sandkassen" className={link}>
                  Slik virker sandkassen
                </NextLink>
              </BodyLong>
            </VStack>

            <VStack as="section" gap="space-8">
              <Heading size="medium" level="2">
                Du eier koden
              </Heading>
              <BodyLong>
                Koden agenten skriver, er din når du ber om review. Du må kunne forklare den.{" "}
                <NextLink href="/praksis/eierskap" className={link}>
                  Eierskap og review av KI-kode
                </NextLink>
              </BodyLong>
            </VStack>

            <VStack as="section" gap="space-8">
              <Heading size="medium" level="2">
                Hvor får du hjelp?
              </Heading>
              <BodyLong>
                Spør i{" "}
                <Link href="https://nav-it.slack.com/archives/C0ASJSZK8P8" target="_blank" rel="noopener noreferrer">
                  #ki-produktutvikling-diskusjon
                </Link>{" "}
                på Slack. Tips og mønstre finner du under{" "}
                <NextLink href="/praksis" className={link}>
                  god praksis
                </NextLink>
                .
              </BodyLong>
            </VStack>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
