import { Heading, BodyShort, Box, Accordion, VStack } from "@navikt/ds-react";
import { WrenchIcon, InformationSquareIcon } from "@navikt/aksel-icons";

export default function Troubleshooting() {
  return (
    <VStack gap="space-32">
      <section>
        <div className="flex items-center gap-2 mb-3">
          <WrenchIcon className="text-blue-700" aria-hidden fontSize="1.5rem" />
          <Heading size="small" level="3" className="text-blue-700">
            Klassisk Feilsøking (Når Copilot stopper opp)
          </Heading>
        </div>
        <BodyShort className="text-gray-700 mb-4">
          Av og til vil GitHub Copilot slutte å gi forslag, miste konteksten, eller henge seg opp. Her er løsningene på
          de aller vanligste problemene vi ser i Nav.
        </BodyShort>
      </section>

      <Box background="neutral-soft" padding="space-16" borderRadius="8">
        <Accordion>
          <Accordion.Item>
            <Accordion.Header>
              <Heading size="xsmall" level="4">
                1. Sertifikat- eller Proxy-feil (Zscaler / VPN)
              </Heading>
            </Accordion.Header>
            <Accordion.Content>
              <VStack gap="space-16">
                <BodyShort>
                  Dette er den hyppigste årsaken til at Copilot slutter å virke i enterprise-miljøer. Strenge VPN-er og
                  proxyer (som Zscaler) kan blokkere telemetri-endepunktene eller tukle med SSL-sertifikatene.
                </BodyShort>
                <div className="pl-4 border-l-2 border-gray-300">
                  <BodyShort weight="semibold">Løsning:</BodyShort>
                  <ul className="list-disc pl-5 space-y-1 mt-2 text-gray-700 text-sm">
                    <li>Koble fra og til VPN/Zscaler.</li>
                    <li>Sørg for at du har de nyeste Nav-sertifikatene installert på maskinen.</li>
                    <li>
                      Hvis editoren din klager på "Self-signed certificate in certificate chain", må du peke editoren
                      til Navs sertifikat-bundle via innstillingene.
                    </li>
                  </ul>
                </div>
              </VStack>
            </Accordion.Content>
          </Accordion.Item>

          <Accordion.Item>
            <Accordion.Header>
              <Heading size="xsmall" level="4">
                2. Ingen forslag lenger / Agenten henger
              </Heading>
            </Accordion.Header>
            <Accordion.Content>
              <VStack gap="space-16">
                <BodyShort>
                  Hvis nettverket er fint, men Copilot likevel ignorerer deg, har gjerne sesjonen hengt seg opp. Slik
                  tvinger du frem en nullstilling:
                </BodyShort>
                <div className="pl-4 border-l-2 border-gray-300">
                  <BodyShort weight="semibold">I VS Code:</BodyShort>
                  <ol className="list-decimal pl-5 space-y-1 mt-2 text-gray-700 text-sm">
                    <li>
                      Bruk den innebygde kommandoen <strong>/troubleshoot</strong> direkte i Copilot Chat for å la
                      Copilot diagnostisere seg selv.
                    </li>
                    <li>
                      Hvis ikke det fungerer: Trykk på konto-ikonet (nederst til venstre) og velg{" "}
                      <strong>Sign Out</strong> fra GitHub.
                    </li>
                    <li>Lukk VS Code, åpne igjen, og logg inn på nytt.</li>
                  </ol>
                </div>
                <div className="pl-4 border-l-2 border-gray-300">
                  <BodyShort weight="semibold">I IntelliJ / JetBrains:</BodyShort>
                  <ol className="list-decimal pl-5 space-y-1 mt-2 text-gray-700 text-sm">
                    <li>
                      Gå til <strong>Tools &gt; GitHub Copilot &gt; Logout</strong>.
                    </li>
                    <li>
                      <strong>Viktig:</strong> For dype feil, prøv å tømme lokal cache (File &gt; Invalidate Caches...).
                    </li>
                    <li>Restart IDE-en og logg inn på nytt.</li>
                  </ol>
                </div>
                <BodyShort className="text-sm text-gray-600 mt-2">
                  <em>
                    Merk: Sjekk også{" "}
                    <a href="https://www.githubstatus.com" className="text-blue-600 hover:underline">
                      githubstatus.com
                    </a>
                    . Noen ganger er det rett og slett en pågående outage hos GitHub.
                  </em>
                </BodyShort>
              </VStack>
            </Accordion.Content>
          </Accordion.Item>

          <Accordion.Item>
            <Accordion.Header>
              <Heading size="xsmall" level="4">
                3. Agenten sliter med gigantisk kontekst (Token Limits & Kostnader)
              </Heading>
            </Accordion.Header>
            <Accordion.Content>
              <VStack gap="space-16">
                <BodyShort>
                  GitHub Copilot har i dag et massivt kontekstvindu (opptil 1 million tokens for visse modeller). Du vil
                  sjelden oppleve at agenten "krasjer" av minne-årsaker lenger.
                </BodyShort>
                <BodyShort>
                  <strong>Men pass på:</strong> Å mate hele prosjektet inn i konteksten har to store ulemper:
                </BodyShort>
                <ul className="list-disc pl-5 space-y-1 text-gray-700 text-sm">
                  <li>
                    <strong>Dyrere regning:</strong> Med GitHubs bruksbaserte fakturering (fra 2026) brenner store
                    kontekster gjennom organisasjonens "AI Credits" i et forrykende tempo.
                  </li>
                  <li>
                    <strong>Tapt resonneringsevne:</strong> Når AI-modellen drukner i tusenvis av irrelevante filer,
                    "glemmer" den instruksene og gir mye dårligere og tregere svar (og reservert minne for
                    chain-of-thought fylles opp).
                  </li>
                </ul>
                <BodyShort className="text-sm font-semibold">
                  Bruk <code>@</code> for å plukke kun de 1-3 filene som faktisk er relevante for oppgaven din!
                </BodyShort>
              </VStack>
            </Accordion.Content>
          </Accordion.Item>

          <Accordion.Item>
            <Accordion.Header>
              <Heading size="xsmall" level="4">
                4. Finn selve feilmeldingen i Copilot Logs
              </Heading>
            </Accordion.Header>
            <Accordion.Content>
              <VStack gap="space-16">
                <BodyShort>
                  Før du melder inn en feil, er det utrolig nyttig å se akkurat hva som går galt i bakgrunnen (f.eks.
                  hvilken HTTP-kode som returneres).
                </BodyShort>
                <div className="pl-4 border-l-2 border-gray-300">
                  <BodyShort weight="semibold">I VS Code:</BodyShort>
                  <ol className="list-decimal pl-5 space-y-1 mt-2 text-gray-700 text-sm">
                    <li>Åpne Command Palette (Cmd+Shift+P).</li>
                    <li>
                      Søk etter og velg <strong>Developer: Show Logs...</strong>
                    </li>
                    <li>
                      Velg <strong>GitHub Copilot</strong> fra listen.
                    </li>
                    <li>Se etter røde linjer, timeout-feil eller HTTP 401/403/500-koder.</li>
                  </ol>
                </div>
              </VStack>
            </Accordion.Content>
          </Accordion.Item>
        </Accordion>
      </Box>

      <Box background="info-soft" padding="space-16" borderRadius="8">
        <div className="flex items-center gap-2 mb-3">
          <InformationSquareIcon className="text-blue-700" aria-hidden />
          <Heading size="small" level="3" className="text-blue-700">
            Fremdeles problemer?
          </Heading>
        </div>
        <BodyShort className="text-gray-800">
          Hvis trinnene over ikke løser problemet ditt, spør gjerne i <strong>#copilot-hjelp</strong> på Slack. Legg ved
          et utdrag fra Copilot Logs og nevn hvilken Editor (inkludert versjon) du sitter på.
        </BodyShort>
      </Box>
    </VStack>
  );
}
