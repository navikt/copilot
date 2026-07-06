import { Heading, BodyLong, Link, VStack, Box } from "@navikt/ds-react";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Personvern",
  description: "Slik behandler Oh-My-Nav personopplysningene dine.",
};

export default function PersonvernPage() {
  return (
    <main className="max-w-3xl mx-auto">
      <Box paddingBlock={{ xs: "space-16", md: "space-24" }} paddingInline={{ xs: "space-16", md: "space-40" }}>
        <VStack gap="space-16">
          <Heading size="xlarge" level="1">
            Personvern og informasjonskapsler
          </Heading>

          <VStack gap="space-8">
            <Heading size="medium" level="2">
              Ansvarlig
            </Heading>
            <BodyLong>
              Nav (Arbeids- og velferdsetaten) er behandlingsansvarlig for personopplysninger som samles inn på dette
              nettstedet.
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <Heading size="medium" level="2">
              Hva vi samler inn
            </Heading>
            <BodyLong>
              Vi bruker Navs egen telemetriløsning (@nais/apm, basert på Grafana Faro) for feilovervåking og
              ytelsesmåling. Verktøyet samler inn teknisk informasjon om nettleser, operativsystem og feilmeldinger for
              å forbedre tjenesten. Personidentifiserende informasjon (fødselsnummer og varianter, e-postadresser og
              token-parametere i URL-er) vaskes bort automatisk før noe sendes, og vi samler ikke inn
              personidentifiserende informasjon fra anonyme besøkende.
            </BodyLong>
            <BodyLong>
              For innloggede Nav-ansatte henter vi navn fra Azure AD-tokenet for å vise det i brukergrensesnittet. Denne
              informasjonen lagres ikke utover sesjonen.
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <Heading size="medium" level="2">
              Informasjonskapsler
            </Heading>
            <BodyLong>Vi bruker kun teknisk nødvendige informasjonskapsler:</BodyLong>
            <ul className="list-disc list-inside">
              <li>
                <BodyLong as="span">
                  <strong>Sesjonskapsel</strong> — holder deg innlogget hvis du har logget inn med Nav-konto
                </BodyLong>
              </li>
            </ul>
            <BodyLong>
              Vi bruker ingen tredjeparts informasjonskapsler, reklamesporere eller analysekapsler som krever samtykke.
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <Heading size="medium" level="2">
              Dine rettigheter
            </Heading>
            <BodyLong>
              Du har rett til innsyn, retting og sletting av personopplysninger. Les mer om dine rettigheter på{" "}
              <Link href="https://www.nav.no/personvernerklaering">Navs personvernerklæring</Link>.
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <Heading size="medium" level="2">
              Kontakt
            </Heading>
            <BodyLong>
              Spørsmål om personvern kan rettes til Navs personvernombud. Se{" "}
              <Link href="https://www.nav.no/personvernerklaering">Navs personvernerklæring</Link> for
              kontaktinformasjon.
            </BodyLong>
          </VStack>
        </VStack>
      </Box>
    </main>
  );
}
