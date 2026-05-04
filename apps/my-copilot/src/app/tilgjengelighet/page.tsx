import { Heading, BodyLong, Link, VStack, Box } from "@navikt/ds-react";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Tilgjengelighetserklæring",
  description: "Tilgjengelighetserklæring for Oh-My-Nav (ki-utvikling.nav.no).",
};

export default function TilgjengelighetPage() {
  return (
    <main className="max-w-3xl mx-auto">
      <Box paddingBlock={{ xs: "space-16", md: "space-24" }} paddingInline={{ xs: "space-16", md: "space-40" }}>
        <VStack gap="space-16">
          <Heading size="xlarge" level="1">
            Tilgjengelighetserklæring
          </Heading>

          <VStack gap="space-8">
            <BodyLong>
              Denne erklæringen gjelder nettstedet <strong>ki-utvikling.nav.no</strong> (Oh-My-Nav), som er eid av Nav
              (Arbeids- og velferdsetaten).
            </BodyLong>
            <BodyLong>
              Vi ønsker at så mange som mulig skal kunne bruke nettstedet. Vi jobber kontinuerlig med å forbedre
              tilgjengeligheten og følger kravene i{" "}
              <Link href="https://lovdata.no/dokument/NL/lov/2017-06-16-51">
                likestillings- og diskrimineringsloven
              </Link>{" "}
              og{" "}
              <Link href="https://www.uutilsynet.no/wcag-standarden/wcag-21-standarden/140">
                WCAG 2.1 nivå AA
              </Link>
              .
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <Heading size="medium" level="2">
              Slik har vi testet
            </Heading>
            <BodyLong>
              Nettstedet er bygget med Nav sitt designsystem{" "}
              <Link href="https://aksel.nav.no">Aksel</Link>, som er testet for universell utforming.
              Vi bruker semantisk HTML, ARIA-attributter der nødvendig, og tastaturnavigasjon fungerer på alle
              interaktive elementer.
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <Heading size="medium" level="2">
              Kjente mangler
            </Heading>
            <BodyLong>
              Vi er ikke kjent med vesentlige tilgjengelighetsproblemer på nettstedet. Dersom du oppdager problemer,
              ber vi deg melde fra (se kontaktinformasjon under).
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <Heading size="medium" level="2">
              Tilbakemelding
            </Heading>
            <BodyLong>
              Dersom du opplever problemer med tilgjengeligheten, ta kontakt med oss via{" "}
              <Link href="https://github.com/navikt/copilot/issues">GitHub Issues</Link>. Vi setter pris på
              tilbakemeldinger som hjelper oss å forbedre nettstedet.
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <Heading size="medium" level="2">
              Tilsynsmyndighet
            </Heading>
            <BodyLong>
              Digitaliseringsdirektoratet fører tilsyn med universell utforming av IKT. Dersom du ikke er fornøyd med
              vårt svar, kan du{" "}
              <Link href="https://www.uutilsynet.no/klage/klage-pa-nettlosning/1124">
                klage til Uutilsynet
              </Link>
              .
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <Heading size="small" level="2">
              Oppdatert
            </Heading>
            <BodyLong>Denne erklæringen ble sist oppdatert mai 2026.</BodyLong>
          </VStack>
        </VStack>
      </Box>
    </main>
  );
}
