import type { Metadata } from "next";
import { Box, VStack, Heading, BodyShort, BodyLong, HGrid, Link } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import {
  PersonGroupIcon,
  LaptopIcon,
  CheckmarkCircleIcon,
  XMarkOctagonIcon,
  ShieldLockIcon,
  ClockIcon,
} from "@navikt/aksel-icons";
import NextLink from "next/link";

export const metadata: Metadata = {
  title: "Retningslinjer",
  description: "Regler og rammer for bruk av GitHub Copilot og AI-verktøy i Nav.",
};

export default function RetningslinjerPage() {
  return (
    <main>
      <PageHero title="Retningslinjer" description="Regler og rammer for bruk av GitHub Copilot og AI-verktøy i Nav." />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap={{ xs: "space-24", md: "space-32" }}>
            {/* Quick start callout */}
            <Box background="info-soft" padding="space-16" borderRadius="8">
              <BodyShort>
                <strong>Ny bruker?</strong> Start med{" "}
                <NextLink href="/kom-i-gang" className="text-blue-600 hover:underline">
                  Kom i gang
                </NextLink>{" "}
                for en trinnvis guide til å sette opp Copilot.
              </BodyShort>
            </Box>

            {/* Hvem kan bruke det */}
            <Section icon={<PersonGroupIcon aria-hidden />} title="Hvem kan bruke GitHub Copilot?">
              <VStack gap="space-8">
                <BodyLong>
                  GitHub Copilot Business er tilgjengelig for alle ansatte i Utvikling og Data, så lenge vi har lisenser
                  tilgjengelig. Konsulenter kan også få tilgang — se egen seksjon under.
                </BodyLong>
                <BodyLong>
                  Du kan sjekke din tilgang på{" "}
                  <Link href="https://github.com/settings/copilot" target="_blank" rel="noopener noreferrer">
                    GitHub Copilot-innstillinger
                  </Link>
                  . Det skal stå &laquo;Copilot Business&raquo; og at lisensen kommer fra navikt-organisasjonen.
                </BodyLong>
              </VStack>
            </Section>

            {/* Hvor kan det brukes */}
            <Section icon={<LaptopIcon aria-hidden />} title="Hvor kan Copilot brukes?">
              <BodyLong>
                Du kan bruke Copilot i alle GitHub-prosjekter som ikke inneholder personopplysninger eller
                skjermingsverdig informasjon. Kildekode inneholder normalt ikke personopplysninger. Personvern avklares
                i egne prosesser, uavhengig av Copilot.
              </BodyLong>
            </Section>

            {/* Hva er tillatt */}
            <Section icon={<CheckmarkCircleIcon aria-hidden className="text-green-600" />} title="Hva er tillatt?">
              <VStack gap="space-12">
                <HGrid columns={{ xs: 1, md: 2 }} gap="space-12">
                  <AllowedItem title="GitHub Copilot Business">
                    Kodeforslag, Chat, Agent mode og Copilot Workspace — alt som er tilgjengelig gjennom GitHub Copilot
                    i editoren din.
                  </AllowedItem>
                  <AllowedItem title="Alle modeller i Copilot">
                    Du kan bruke alle modeller som er tilgjengelige gjennom GitHub Copilot, inkludert de fra Anthropic,
                    Google og OpenAI. Bruk &laquo;Auto&raquo; for å la Copilot velge optimal modell.
                  </AllowedItem>
                  <AllowedItem title="Agent mode i editoren">
                    Du kan bruke Agent mode til autonome redigeringer lokalt. Du godkjenner terminalkommandoer og
                    vurderer endringene før commit — samme ansvar som for all annen kode.
                  </AllowedItem>
                  <AllowedItem title="Nav-godkjente MCP-servere">
                    MCP-servere fra{" "}
                    <NextLink href="/verktoy" className="text-blue-600 hover:underline">
                      Verktøy-katalogen
                    </NextLink>{" "}
                    er godkjent for bruk. De utvider Copilot med Nav-spesifikke verktøy.
                  </AllowedItem>
                  <AllowedItem title="Copilot coding agent (cloud)">
                    Coding agent kan brukes til avgrensede oppgaver (bugfiks, tester, dokumentasjon). Agenten lager en
                    PR som må gjennomgås og godkjennes av et menneske før merge.
                  </AllowedItem>
                  <AllowedItem title="Custom instructions og agenter">
                    Team kan lage egne instruksjoner, agenter og skills i sine repoer. Org-nivå agenter publiseres av
                    plattformteamet.
                  </AllowedItem>
                </HGrid>
              </VStack>
            </Section>

            {/* Hva er IKKE tillatt */}
            <Section icon={<XMarkOctagonIcon aria-hidden className="text-red-600" />} title="Hva er IKKE tillatt?">
              <VStack gap="space-12">
                <HGrid columns={{ xs: 1, md: 2 }} gap="space-12">
                  <ForbiddenItem title="Private Copilot-abonnement">
                    Copilot Individual eller andre personlige abonnement skal ikke brukes til Nav-arbeid. Det er uklart
                    hvilke data som samles inn i disse versjonene.
                  </ForbiddenItem>
                  <ForbiddenItem title="ChatGPT, Claude Code og lignende">
                    Frittstående AI-kodeverktøy utenfor GitHub Copilot er ikke tillatt. Du kan bruke Claude- og
                    GPT-modellene via Copilot.
                  </ForbiddenItem>
                  <ForbiddenItem title="Privat bruk på Nav-lisens">
                    Nav-lisensen er kun for Nav-relatert arbeid. Unntaket er opplæring på fagtorsdag og lignende.
                  </ForbiddenItem>
                  <ForbiddenItem title="Tredjeparters MCP-servere uten godkjenning">
                    MCP-servere utenfor Nav-katalogen kan sende kode og kontekst til eksterne tjenester. Bruk kun
                    godkjente servere fra Verktøy-siden.
                  </ForbiddenItem>
                </HGrid>
              </VStack>
            </Section>

            {/* Personvern */}
            <Section icon={<ShieldLockIcon aria-hidden />} title="Personvern og datainnsamling">
              <VStack gap="space-8">
                <BodyLong>
                  GitHub Copilot Business samler bruksdata — for eksempel om du godtar eller avviser forslag, hvor lenge
                  du venter, og hvilke funksjoner du bruker. Dataene kan knyttes til pseudonyme IDer.
                </BodyLong>
                <BodyLong>
                  Copilot Business beholder ikke ledetekster (prompts) eller forslag (suggestions) etter at de er
                  levert. Koden din brukes ikke til å trene modeller.
                </BodyLong>
                <BodyLong>
                  Les mer i{" "}
                  <Link
                    href="https://docs.github.com/en/site-policy/privacy-policies/github-copilot-for-business-privacy-statement"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    GitHub Copilot Privacy Statement
                  </Link>
                  .
                </BodyLong>
              </VStack>
            </Section>

            {/* Ditt ansvar */}
            <Section icon={<PersonGroupIcon aria-hidden />} title="Ditt ansvar som utvikler">
              <VStack gap="space-8">
                <BodyLong>
                  Copilot er et verktøy — du er ansvarlig for koden som går i produksjon. De samme kravene gjelder
                  uansett om koden er skrevet av deg, generert av AI, eller hentet fra andre kilder:
                </BodyLong>
                <ol className="list-decimal list-inside space-y-2">
                  <li>
                    <BodyShort as="span">Vurder alltid forslagene kritisk og bruk sunn fornuft</BodyShort>
                  </li>
                  <li>
                    <BodyShort as="span">Skriv tester for å verifisere at koden fungerer som forventet</BodyShort>
                  </li>
                  <li>
                    <BodyShort as="span">Bruk code review — Copilot erstatter ikke en annen utviklers blikk</BodyShort>
                  </li>
                  <li>
                    <BodyShort as="span">
                      Bruk sikkerhetsverktøy som GitHub Code Scanning for kjente sårbarheter
                    </BodyShort>
                  </li>
                </ol>
                <BodyLong>
                  Se også{" "}
                  <NextLink href="/praksis" className="text-blue-600 hover:underline">
                    God praksis
                  </NextLink>{" "}
                  for konkrete tips om hvordan du jobber effektivt og trygt med Copilot.
                </BodyLong>
              </VStack>
            </Section>

            {/* Bevisst AI-bruk */}
            <Section icon={<LaptopIcon aria-hidden />} title="Bevisst AI-bruk">
              <VStack gap="space-8">
                <BodyLong>
                  Forskning viser at utviklere som bruker AI bevisst lærer mer enn de som delegerer blindt. Nav
                  oppfordrer til &laquo;generer-så-forstå&raquo;-mønsteret: la AI generere, men still spørsmål om
                  hvorfor, verifiser at du forstår, og tilpass aktivt.
                </BodyLong>
                <BodyLong>
                  Vær spesielt bevisst i &laquo;rød sone&raquo;: debugging, nye konsepter, kjernelogikk og
                  sikkerhetskritisk kode. Her bør du prøve selv først og bruke AI som støtte — ikke omvendt.
                </BodyLong>
              </VStack>
            </Section>

            {/* Konsulenter */}
            <Section icon={<PersonGroupIcon aria-hidden />} title="Konsulenter">
              <VStack gap="space-8">
                <BodyLong>
                  Konsulenter kan få lisens gjennom Nav, eller bruke Copilot Business-lisenser fra egen arbeidsgiver.
                  Samme regler gjelder som for ansatte.
                </BodyLong>
                <BodyLong>
                  Det er ikke tillatt å bruke Copilot Individual (selv om det er kjøpt av arbeidsgiver) til Nav-arbeid.
                </BodyLong>
              </VStack>
            </Section>

            {/* Endringslogg */}
            <Section icon={<ClockIcon aria-hidden />} title="Endringslogg">
              <div className="overflow-x-auto -mx-4 px-4 sm:mx-0 sm:px-0">
                <table className="text-sm w-full min-w-max">
                  <thead>
                    <tr className="border-b">
                      <th className="text-left py-2 pr-4 font-medium">Dato</th>
                      <th className="text-left py-2 font-medium">Endring</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr className="border-b">
                      <td className="py-2 pr-4 whitespace-nowrap">2026-05-05</td>
                      <td className="py-2">
                        Lagt til retningslinjer for agent mode, coding agent, MCP-servere og BYOK. Fjernet krav om
                        &laquo;block public code matching&raquo;. Ny seksjon om bevisst AI-bruk.
                      </td>
                    </tr>
                    <tr className="border-b">
                      <td className="py-2 pr-4 whitespace-nowrap">2025-09-02</td>
                      <td className="py-2">Copilot tilgjengelig for konsulenter</td>
                    </tr>
                    <tr className="border-b">
                      <td className="py-2 pr-4 whitespace-nowrap">2024-05-29</td>
                      <td className="py-2">Copilot for konsulenter lagt til</td>
                    </tr>
                    <tr>
                      <td className="py-2 pr-4 whitespace-nowrap">2023-09-05</td>
                      <td className="py-2">Første versjon</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </Section>

            {/* Spørsmål */}
            <Box background="neutral-soft" padding="space-16" borderRadius="8">
              <VStack gap="space-8">
                <Heading size="small" level="2">
                  Har du spørsmål?
                </Heading>
                <BodyLong>
                  Ta kontakt i{" "}
                  <Link href="https://nav-it.slack.com/archives/C055TNXBM17" target="_blank" rel="noopener noreferrer">
                    #github-copilot
                  </Link>{" "}
                  på Slack.
                </BodyLong>
              </VStack>
            </Box>
          </VStack>
        </Box>
      </div>
    </main>
  );
}

function Section({ icon, title, children }: { icon: React.ReactNode; title: string; children: React.ReactNode }) {
  return (
    <VStack gap="space-12">
      <div className="flex items-center gap-3">
        <span className="text-xl">{icon}</span>
        <Heading size="small" level="2">
          {title}
        </Heading>
      </div>
      {children}
    </VStack>
  );
}

function AllowedItem({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Box background="success-soft" padding="space-12" borderRadius="8">
      <VStack gap="space-4">
        <Heading size="xsmall" level="3">
          {title}
        </Heading>
        <BodyShort size="small">{children}</BodyShort>
      </VStack>
    </Box>
  );
}

function ForbiddenItem({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Box background="danger-soft" padding="space-12" borderRadius="8">
      <VStack gap="space-4">
        <Heading size="xsmall" level="3">
          {title}
        </Heading>
        <BodyShort size="small">{children}</BodyShort>
      </VStack>
    </Box>
  );
}
