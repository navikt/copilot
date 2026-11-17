import { PageHero } from "@/components/page-hero";
import { BodyLong, Box, Heading, List, VStack } from "@navikt/ds-react";
import { ListItem } from "@navikt/ds-react/List";
import NextLink from "next/link";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Eierskap og review av KI-kode",
  description:
    "Hvem eier koden agenten skriver, hva teamet må forstå, og hvordan dere gjør review av KI-generert kode.",
};

const link = "text-blue-600 hover:underline";

export default function EierskapPage() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero
        title="Eierskap og review av KI-kode"
        description="Koden agenten skriver, eies av teamet som merger den. Slik holder dere på forståelsen."
      />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-24" className="max-w-3xl mx-auto">
            <VStack as="section" gap="space-8">
              <Heading size="medium" level="2">
                Hva må teamet forstå fullt ut?
              </Heading>
              <BodyLong>
                Avtal i teamet hvilken kode noen må kunne forklare linje for linje, og hvilken kode det holder å sikre
                med tester. Bruk sonene fra bevisst KI-bruk som utgangspunkt:
              </BodyLong>
              <List>
                <ListItem>
                  <strong>Forstå fullt ut:</strong> kjernelogikk, sikkerhet (autentisering, tilgangskontroll, validering
                  av input), behandling av personopplysninger og arkitekturvalg.
                </ListItem>
                <ListItem>
                  <strong>Sikre med tester:</strong> boilerplate, konfigurasjon, testdata og refaktorering med kjent
                  mål. Testene må sjekke oppførselen, ikke bare at koden kjører. Konfigurasjon som berører nettverk,
                  hemmeligheter, MCP-servere, produksjon eller persondata, har høy risiko og må forstås fullt ut.
                </ListItem>
              </List>
            </VStack>

            <VStack as="section" gap="space-8">
              <Heading size="medium" level="2">
                Forfatteren eier PR-en
              </Heading>
              <List>
                <ListItem>
                  Du er forfatter av PR-en, uansett hvem som skrev koden. Les den før du ber om review.
                </ListItem>
                <ListItem>
                  Du skal kunne forklare valgene, også de agenten tok. Kan du ikke det, er PR-en ikke klar.
                </ListItem>
                <ListItem>
                  Hold PR-ene små. Del en stor endring fra agenten i flere PR-er som kan leses hver for seg.
                </ListItem>
                <ListItem>
                  Skriv i beskrivelsen hva som er testet, og hva den som gjør review, bør se ekstra nøye på.
                </ListItem>
              </List>
            </VStack>

            <VStack as="section" gap="space-8">
              <Heading size="medium" level="2">
                Review av KI-generert kode
              </Heading>
              <List>
                <ListItem>Gjør review som av annen kode. At en agent skrev den, senker ikke kravene.</ListItem>
                <ListItem>
                  Be forfatteren forklare valgene. Et svar som «agenten foreslo det» er et signal om å ta en runde til.
                </ListItem>
                <ListItem>
                  Se etter det agenter ofte gjør: unødvendig kode, nye avhengigheter, svekket feilhåndtering og tester
                  som er justert så de passer koden.
                </ListItem>
                <ListItem>
                  Bruk gjerne Copilot til en første review, men et menneske godkjenner og merger.{" "}
                  <NextLink href="/praksis/guide/gjennomfore-code-review" className={link}>
                    Code review med Copilot
                  </NextLink>
                </ListItem>
                <ListItem>Er PR-en for stor til å leses ordentlig, be om å få den delt opp.</ListItem>
              </List>
            </VStack>

            <VStack as="section" gap="space-8">
              <Heading size="medium" level="2">
                Les mer
              </Heading>
              <BodyLong>
                Om rød og grønn sone:{" "}
                <NextLink href="/retningslinjer" className={link}>
                  Bevisst KI-bruk i retningslinjene
                </NextLink>
                . Om hvorfor vi prioriterer vedlikehold:{" "}
                <NextLink href="/nyheter/kvalitet-foran-fart" className={link}>
                  Kvalitet foran fart
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
