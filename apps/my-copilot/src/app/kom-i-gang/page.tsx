import type { Metadata } from "next";
import { Box, VStack, Heading, BodyShort, BodyLong, Link, HGrid } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import NextLink from "next/link";
import { ArrowRightIcon } from "@navikt/aksel-icons";

export const metadata: Metadata = {
  title: "Kom i gang",
  description: "Fra null til produktiv med GitHub Copilot i Nav på under 10 minutter.",
};

export default function KomIGangPage() {
  return (
    <main>
      <PageHero title="Kom i gang" description="Fra null til produktiv med GitHub Copilot på under 10 minutter." />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <div className="max-w-2xl mx-auto">
            <VStack gap="space-32">
              <Step number={1} title="Aktiver tilgang">
                <VStack gap="space-8">
                  <BodyLong>
                    Alle utviklere i navikt-organisasjonen kan gi seg selv tilgang til GitHub Copilot Business. Det tar
                    under ett minutt og krever ingen godkjenning.
                  </BodyLong>
                  <Box background="info-soft" padding="space-12" borderRadius="8">
                    <VStack gap="space-4">
                      <BodyShort size="small" weight="semibold">
                        Slik aktiverer du:
                      </BodyShort>
                      <ol className="list-decimal list-inside space-y-1">
                        <li>
                          <BodyShort size="small" as="span">
                            Gå til{" "}
                            <NextLink href="/abonnement" className="text-blue-600 hover:underline">
                              Abonnement
                            </NextLink>{" "}
                            (krever innlogging)
                          </BodyShort>
                        </li>
                        <li>
                          <BodyShort size="small" as="span">
                            Klikk &laquo;Aktiver&raquo; — ferdig!
                          </BodyShort>
                        </li>
                      </ol>
                    </VStack>
                  </Box>
                  <Box background="neutral-soft" padding="space-12" borderRadius="8">
                    <VStack gap="space-4">
                      <BodyShort size="small" weight="semibold">
                        Kort om reglene:
                      </BodyShort>
                      <ul className="list-inside space-y-1">
                        <li>
                          <BodyShort size="small" as="span">
                            Kan brukes i alle Nav-prosjekter uten personopplysninger
                          </BodyShort>
                        </li>
                        <li>
                          <BodyShort size="small" as="span">
                            Du er ansvarlig for å vurdere og teste generert kode
                          </BodyShort>
                        </li>
                        <li>
                          <BodyShort size="small" as="span">
                            Ikke bruk private abonnement eller ChatGPT til Nav-arbeid
                          </BodyShort>
                        </li>
                      </ul>
                      <BodyShort size="small">
                        <NextLink href="/retningslinjer" className="text-blue-600 hover:underline">
                          Les fullstendige retningslinjer →
                        </NextLink>
                      </BodyShort>
                    </VStack>
                  </Box>
                </VStack>
              </Step>

              <Step number={2} title="Installer verktøyene">
                <VStack gap="space-12">
                  <Box background="neutral-soft" padding="space-16" borderRadius="8">
                    <VStack gap="space-8">
                      <div className="flex items-center gap-2">
                        <Heading size="xsmall" level="3">
                          macOS (anbefalt)
                        </Heading>
                        <span className="text-xs bg-blue-100 text-blue-800 px-2 py-0.5 rounded-full">Homebrew</span>
                      </div>
                      <div className="font-mono text-sm bg-gray-900 text-gray-100 rounded-lg p-4 overflow-x-auto">
                        <div className="text-gray-400"># Copilot CLI (agentic terminal)</div>
                        <div>brew install copilot-cli</div>
                        <div className="mt-3 text-gray-400"># Nav-verktøy</div>
                        <div>brew install navikt/tap/nav-pilot</div>
                        <div>brew install navikt/tap/cplt</div>
                      </div>
                      <BodyShort size="small">
                        VS Code har Copilot innebygd — åpne editoren og logg inn med GitHub-kontoen din.
                      </BodyShort>
                    </VStack>
                  </Box>

                  <details className="group">
                    <summary className="cursor-pointer text-sm text-blue-600 hover:underline list-none flex items-center gap-1">
                      <span className="group-open:rotate-90 transition-transform">▶</span>
                      Andre plattformer og editorer
                    </summary>
                    <div className="mt-4">
                      <HGrid columns={{ xs: 1, md: 3 }} gap="space-12">
                        <Box background="neutral-soft" padding="space-12" borderRadius="8">
                          <VStack gap="space-4">
                            <Heading size="xsmall" level="4">
                              Linux
                            </Heading>
                            <BodyShort size="small">
                              Se{" "}
                              <Link
                                href="https://docs.github.com/en/copilot/how-tos/copilot-cli/set-up-copilot-cli/install-copilot-cli"
                                target="_blank"
                                rel="noopener noreferrer"
                              >
                                GitHub Copilot CLI install docs
                              </Link>{" "}
                              for Linux-instruksjoner. Installer deretter nav-pilot via Go eller nedlasting.
                            </BodyShort>
                          </VStack>
                        </Box>
                        <Box background="neutral-soft" padding="space-12" borderRadius="8">
                          <VStack gap="space-4">
                            <Heading size="xsmall" level="4">
                              JetBrains
                            </Heading>
                            <BodyShort size="small">
                              Installer &laquo;GitHub Copilot&raquo;-plugin fra Marketplace. Restart IDE-en og logg inn
                              med GitHub.
                            </BodyShort>
                          </VStack>
                        </Box>
                        <Box background="neutral-soft" padding="space-12" borderRadius="8">
                          <VStack gap="space-4">
                            <Heading size="xsmall" level="4">
                              Windows
                            </Heading>
                            <BodyShort size="small">
                              Installer via{" "}
                              <code className="bg-white/50 px-1 rounded">winget install GitHub.Copilot</code> eller se{" "}
                              <Link
                                href="https://docs.github.com/en/copilot/how-tos/copilot-cli/set-up-copilot-cli/install-copilot-cli"
                                target="_blank"
                                rel="noopener noreferrer"
                              >
                                install docs
                              </Link>
                              .
                            </BodyShort>
                          </VStack>
                        </Box>
                      </HGrid>
                    </div>
                  </details>

                  <Box background="info-soft" padding="space-12" borderRadius="8">
                    <BodyShort size="small">
                      <strong>Verifiser:</strong> Åpne en fil og begynn å skrive. Ser du et grått forslag? Trykk{" "}
                      <code className="bg-white/50 px-1 rounded">Tab</code> for å godta — da fungerer Copilot.
                    </BodyShort>
                  </Box>
                </VStack>
              </Step>

              <Step number={3} title="Sett opp nav-pilot i repoet">
                <VStack gap="space-8">
                  <BodyLong>
                    nav-pilot er Navs egen Copilot-agent. Den kjenner Nav-stacken (Kotlin, Ktor, Nais, Aksel, Kafka) og
                    kan planlegge, arkitektere og bygge Nav-applikasjoner.
                  </BodyLong>
                  <Box background="neutral-soft" padding="space-16" borderRadius="8">
                    <VStack gap="space-8">
                      <div className="font-mono text-sm bg-gray-900 text-gray-100 rounded-lg p-4 overflow-x-auto">
                        <div className="text-gray-400"># Kjør i repoet ditt — interaktiv veiviser</div>
                        <div>nav-pilot</div>
                      </div>
                      <BodyShort size="small">
                        Velger samling, installerer agenter, instruksjoner og skills i repoet ditt. Du kan også{" "}
                        <NextLink href="/install" className="text-blue-600 hover:underline">
                          installere via nettleseren
                        </NextLink>
                        .
                      </BodyShort>
                    </VStack>
                  </Box>
                </VStack>
              </Step>

              <Step number={4} title="Din første samtale med @nav-pilot">
                <VStack gap="space-8">
                  <BodyLong>
                    Åpne Copilot Chat og start en samtale med{" "}
                    <code className="bg-gray-100 px-1 rounded">@nav-pilot</code>. Prøv noe fra ditt eget prosjekt:
                  </BodyLong>
                  <Box background="neutral-soft" padding="space-12" borderRadius="8">
                    <VStack gap="space-8">
                      <div>
                        <BodyShort size="small" weight="semibold">
                          Åpne Chat
                        </BodyShort>
                        <BodyShort size="small">
                          VS Code: <code className="bg-white/50 px-1 rounded">⌘⇧I</code> (Mac) /{" "}
                          <code className="bg-white/50 px-1 rounded">Ctrl+Shift+I</code> (Windows)
                        </BodyShort>
                      </div>
                      <div>
                        <BodyShort size="small" weight="semibold">
                          Eksempler å prøve:
                        </BodyShort>
                        <ul className="list-inside space-y-1 mt-1">
                          <li>
                            <BodyShort size="small" as="span">
                              <code className="bg-white/50 px-1 rounded">@nav-pilot</code> Forklar arkitekturen i dette
                              prosjektet
                            </BodyShort>
                          </li>
                          <li>
                            <BodyShort size="small" as="span">
                              <code className="bg-white/50 px-1 rounded">@nav-pilot</code> Skriv en test for denne
                              funksjonen
                            </BodyShort>
                          </li>
                          <li>
                            <BodyShort size="small" as="span">
                              <code className="bg-white/50 px-1 rounded">@nav-pilot</code> Lag et nytt Ktor-endepunkt
                              med Nais-konfigurasjon
                            </BodyShort>
                          </li>
                        </ul>
                      </div>
                    </VStack>
                  </Box>
                  <BodyShort className="text-gray-600">
                    nav-pilot gir bedre svar enn vanlig Copilot fordi den forstår Nav-konvensjoner, Nais-konfigurasjon
                    og teamets mønstre.
                  </BodyShort>
                </VStack>
              </Step>

              <Step number={5} title="Neste steg" isLast>
                <VStack gap="space-12">
                  <BodyLong>Du er i gang! Her er veien videre:</BodyLong>
                  <HGrid columns={{ xs: 1, md: 2 }} gap="space-12">
                    <NextStepCard
                      href="/praksis"
                      title="God praksis"
                      description="Lær å bruke Copilot effektivt — fra grunnleggende til avansert."
                    />
                    <NextStepCard
                      href="/verktoy"
                      title="Verktøy"
                      description="Utforsk agenter, skills og MCP-servere laget for Nav."
                    />
                    <NextStepCard
                      href="https://nav-it.slack.com/archives/C055TNXBM17"
                      title="#github-copilot"
                      description="Still spørsmål og del erfaringer med andre Nav-utviklere."
                      external
                    />
                    <NextStepCard
                      href="/nav-pilot"
                      title="Mer om nav-pilot"
                      description="Utforsk alle funksjoner og se eksempler på bruk."
                    />
                  </HGrid>
                </VStack>
              </Step>
            </VStack>
          </div>
        </Box>
      </div>
    </main>
  );
}

function Step({
  number,
  title,
  children,
  isLast = false,
}: {
  number: number;
  title: string;
  children: React.ReactNode;
  isLast?: boolean;
}) {
  return (
    <div className="relative flex gap-6">
      {/* Vertical line + number badge */}
      <div className="flex flex-col items-center">
        <div className="flex items-center justify-center w-8 h-8 rounded-full bg-blue-600 text-white text-sm font-bold shrink-0">
          {number}
        </div>
        {!isLast && <div className="w-0.5 flex-1 bg-blue-200 mt-2" />}
      </div>
      {/* Content */}
      <div className="flex-1 pb-4">
        <Heading size="small" level="2" className="mb-3">
          {title}
        </Heading>
        {children}
      </div>
    </div>
  );
}

function NextStepCard({
  href,
  title,
  description,
  external = false,
}: {
  href: string;
  title: string;
  description: string;
  external?: boolean;
}) {
  const isExternal = external;

  const content = (
    <Box
      background="neutral-soft"
      padding="space-12"
      borderRadius="8"
      className="h-full hover:bg-gray-200 transition-colors"
    >
      <VStack gap="space-4">
        <div className="flex items-center gap-2">
          <Heading size="xsmall" level="3">
            {title}
          </Heading>
          <ArrowRightIcon aria-hidden fontSize="1rem" />
        </div>
        <BodyShort size="small">{description}</BodyShort>
      </VStack>
    </Box>
  );

  if (isExternal) {
    return (
      <Link href={href} target="_blank" rel="noopener noreferrer" className="no-underline">
        {content}
      </Link>
    );
  }

  return (
    <NextLink href={href} className="no-underline">
      {content}
    </NextLink>
  );
}
