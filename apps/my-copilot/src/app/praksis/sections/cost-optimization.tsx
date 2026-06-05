import { BodyShort, Box, Heading, HGrid, HStack, VStack } from "@navikt/ds-react";
import { LinkableHeading } from "@/components/linkable-heading";
import {
  CurrencyExchangeIcon,
  SparklesIcon,
  PersonGroupIcon,
  CogIcon,
  ArrowRightIcon,
  FileTextIcon,
} from "@navikt/aksel-icons";
import NextLink from "next/link";

export default function CostOptimization() {
  return (
    <Box background="neutral-soft" padding={{ xs: "space-12", sm: "space-16", md: "space-24" }} borderRadius="12">
      <LinkableHeading size="medium" level="2">
        Kostnadsoptimalisering i praksis
      </LinkableHeading>
      <BodyShort size="small" className="text-gray-600">
        Seks tiltak med høy effekt: presise spørsmål, smale sesjoner, riktig agentvalg, kort output, minimale
        instruksjonsfiler og mindre verktøy-overhead.
      </BodyShort>

      <Box paddingBlock="space-16">
        <VStack gap="space-16">
          <Box background="accent-soft" borderRadius="8" padding="space-16">
            <BodyShort size="small" className="text-gray-700">
              Budsjettgrensen i my-copilot er et kostnadstak, ikke en kvote som må brukes opp. Hvis teamet bruker mindre
              enn grensen, mister ikke Nav «tokens» som må tas igjen mot slutten av måneden.
            </BodyShort>
          </Box>

          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
            <Box background="info-soft" padding="space-16" borderRadius="8">
              <VStack gap="space-8">
                <HStack gap="space-8" align="center">
                  <SparklesIcon className="text-blue-700" aria-hidden />
                  <Heading size="small" level="3" className="text-blue-700">
                    1. Presis første melding
                  </Heading>
                </HStack>
                <BodyShort size="small" className="text-gray-700">
                  Ett presist spørsmål slår fem runder avklaring. Oppgi språk, rammeverk, scope og ønsket output.
                </BodyShort>
              </VStack>
            </Box>

            <Box background="success-soft" padding="space-16" borderRadius="8">
              <VStack gap="space-8">
                <HStack gap="space-8" align="center">
                  <CurrencyExchangeIcon className="text-green-700" aria-hidden />
                  <Heading size="small" level="3" className="text-green-700">
                    2. Smale sesjoner
                  </Heading>
                </HStack>
                <BodyShort size="small" className="text-gray-700">
                  Bytt chat når du bytter problem. Bruk <code>/clear</code> og <code>/compact</code> for å unngå
                  irrelevant historikk.
                </BodyShort>
              </VStack>
            </Box>

            <Box background="accent-soft" padding="space-16" borderRadius="8">
              <VStack gap="space-8">
                <HStack gap="space-8" align="center">
                  <PersonGroupIcon className="text-blue-700" aria-hidden />
                  <Heading size="small" level="3" className="text-blue-700">
                    3. Riktig agent for riktig jobb
                  </Heading>
                </HStack>
                <BodyShort size="small" className="text-gray-700">
                  Bruk <code>@research-agent</code> for kartlegging, <code>@nav-pilot</code> for plan/syntese og{" "}
                  <code>@nav-pilot-opus</code> kun for smale høyrisikovurderinger.
                </BodyShort>
              </VStack>
            </Box>

            <Box background="warning-soft" padding="space-16" borderRadius="8">
              <VStack gap="space-8">
                <HStack gap="space-8" align="center">
                  <CogIcon className="text-orange-700" aria-hidden />
                  <Heading size="small" level="3" className="text-orange-700">
                    4. Kort output og mindre overhead
                  </Heading>
                </HStack>
                <BodyShort size="small" className="text-gray-700">
                  Aktiver <code>terse-mode</code> ved rask iterasjon. Bruk CLI for deterministiske steg i stedet for å
                  la LLM gjøre alt.
                </BodyShort>
              </VStack>
            </Box>
          </HGrid>

          <Box background="warning-soft" padding="space-16" borderRadius="8">
            <VStack gap="space-8">
              <HStack gap="space-8" align="center">
                <CogIcon className="text-orange-700" aria-hidden />
                <Heading size="small" level="3" className="text-orange-700">
                  5. Kutt verktøy-overhead
                </Heading>
              </HStack>
              <BodyShort size="small" className="text-gray-700">
                Send relevant utsnitt av logger og diff, ikke alt. Kjør målrettede kommandoer først, og bruk
                deterministiske verktøy (`gh`, `kubectl`, `curl`) før du ber LLM tolke alt i én runde.
              </BodyShort>
            </VStack>
          </Box>

          <Box background="danger-soft" padding="space-16" borderRadius="8">
            <VStack gap="space-8">
              <HStack gap="space-8" align="center">
                <FileTextIcon className="text-red-700" aria-hidden />
                <Heading size="small" level="3" className="text-red-700">
                  6. Hold instruksjonsfiler minimale
                </Heading>
              </HStack>
              <BodyShort size="small" className="text-gray-700">
                <code>AGENTS.md</code> og <code>copilot-instructions.md</code> injiseres i{" "}
                <strong>hver forespørsel</strong>. Skriv kortfattet; bruk <code>applyTo</code>-glob for å begrense
                scope, og legg domeneinnhold i skill- og agent-filer som bare lastes ved eksplisitt bruk.
              </BodyShort>
            </VStack>
          </Box>

          <Box background="neutral-soft" borderWidth="1" borderRadius="8" padding="space-16">
            <Heading size="xsmall" level="3">
              Erfaringer som går igjen
            </Heading>
            <Box paddingBlock="space-8">
              <ul style={{ paddingInlineStart: "1.25rem", margin: 0 }}>
                <li>
                  <BodyShort size="small">Lange sesjoner er sjelden billigst, selv med cache.</BodyShort>
                </li>
                <li>
                  <BodyShort size="small">Ubrukte verktøy i konteksten gir merkbar token-kost over tid.</BodyShort>
                </li>
                <li>
                  <BodyShort size="small">Smal delegering gir bedre svar enn én stor "gjør alt"-prompt.</BodyShort>
                </li>
                <li>
                  <BodyShort size="small">
                    En <code>AGENTS.md</code> på 500 linjer koster like mye som en på 50 — hver eneste gang.
                  </BodyShort>
                </li>
              </ul>
            </Box>
          </Box>

          <Box background="info-soft" borderRadius="8" padding="space-12">
            <HStack gap="space-8" align="center">
              <ArrowRightIcon className="text-blue-700" aria-hidden />
              <BodyShort size="small" className="text-gray-700">
                Dypdykk og kilder:{" "}
                <NextLink href="/nyheter/token-forbruk-verktoy-teknikker" className="text-blue-600 hover:underline">
                  Slik holder du token-forbruket nede
                </NextLink>
              </BodyShort>
            </HStack>
          </Box>

          <Box background="default" borderWidth="1" borderRadius="8" padding="space-12">
            <BodyShort size="small" className="text-gray-700">
              Følg opp effekten i{" "}
              <NextLink href="/statistikk#kostnad" className="text-blue-600 hover:underline">
                Statistikk
              </NextLink>{" "}
              før dere endrer modellvalg eller arbeidsflyt.
            </BodyShort>
          </Box>
        </VStack>
      </Box>
    </Box>
  );
}
