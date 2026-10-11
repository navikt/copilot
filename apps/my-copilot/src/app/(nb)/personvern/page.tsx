import { Heading, BodyLong, Link, VStack, Box } from "@navikt/ds-react";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Personvern",
  description: "Slik behandler ki-utvikling.nav.no og nav-pilot personopplysningene dine.",
};

export default function PersonvernPage() {
  return (
    <main id="hovedinnhold" tabIndex={-1} className="max-w-3xl mx-auto">
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
              Vi bruker Grafana Faro for feilovervåking, ytelsesmåling og bruksstatistikk. Faro samler inn teknisk
              informasjon om nettleser, operativsystem, feilmeldinger og hvilke sider du besøker. Vi lagrer ikke
              parametere i nettadressen. Vi kobler ikke disse dataene til navn eller brukerkonto.
            </BodyLong>
            <BodyLong>
              For innloggede Nav-ansatte henter vi navn fra Azure AD-tokenet for å vise det i brukergrensesnittet. Denne
              informasjonen lagres ikke utover sesjonen.
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <Heading size="medium" level="2">
              Bruksdata om Copilot og nav-pilot
            </Heading>
            <ul className="list-disc list-inside">
              <li>
                <BodyLong as="span">
                  Fra GitHub henter vi hver dag hvor mye hver ansatt bruker GitHub Copilot: aktivitet, KI-modeller og
                  funksjoner, knyttet til GitHub-brukernavnet.
                </BodyLong>
              </li>
              <li>
                <BodyLong as="span">
                  Vi henter også forbruk og kostnad per person per måned, eventuelle egne budsjetter og hvilke
                  GitHub-team du er med i.
                </BodyLong>
              </li>
              <li>
                <BodyLong as="span">Vi henter ikke det du skriver til Copilot, svarene eller koden din.</BodyLong>
              </li>
              <li>
                <BodyLong as="span">
                  nav-pilot sender bruksmålinger til Nav: kommandoer, feil, innstillinger, et maskinnummer regnet ut fra
                  maskinen, og navnet på repoet når du jobber i et repo i navikt. For opencode-økter også KI-modell og
                  antall tokens. Ikke navn, brukernavn eller innhold.
                </BodyLong>
              </li>
              <li>
                <BodyLong as="span">
                  Når nav-pilot starter Copilot eller opencode, slår den på deres egne målinger, med samme mottaker.
                </BodyLong>
              </li>
              <li>
                <BodyLong as="span">Tall om deg ser du selv når du er logget inn.</BodyLong>
              </li>
              <li>
                <BodyLong as="span">
                  Slå av nav-pilots målinger: <code>export DO_NOT_TRACK=1</code> eller{" "}
                  <code>export NAV_PILOT_TELEMETRY_ENABLED=false</code>.
                </BodyLong>
              </li>
            </ul>
          </VStack>

          <VStack gap="space-8">
            <Heading size="medium" level="2">
              Informasjonskapsler
            </Heading>
            <BodyLong>Vi bruker kun teknisk nødvendige informasjonskapsler:</BodyLong>
            <ul className="list-disc list-inside">
              <li>
                <BodyLong as="span">
                  <strong>Sesjonskapsel</strong> holder deg innlogget hvis du har logget inn med Nav-konto
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
