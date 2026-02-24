import React, { Suspense } from "react";
import { getCachedCopilotUsage, getCachedPremiumRequestUsage } from "@/lib/cached-github";
import { CopilotMetrics } from "@/lib/github";
import Tabs from "@/components/tabs";
import TrendChart from "@/components/charts/TrendChart";
import LanguagesChart from "@/components/charts/LanguagesChart";
import EditorsChart from "@/components/charts/EditorsChart";
import ChatChart from "@/components/charts/ChatChart";
import ModelUsageChart from "@/components/charts/ModelUsageChart";
import LinesOfCodeChart from "@/components/charts/LinesOfCodeChart";
import LanguageDistributionChart from "@/components/charts/LanguageDistributionChart";
import MetricCard from "@/components/metric-card";
import ErrorState from "@/components/error-state";
import PremiumRequestsContent from "@/components/premium-requests-content";
import TimeframeSelector from "@/components/timeframe-selector";
import { calculatePremiumMetrics } from "@/lib/billing-utils";
import { Table, BodyShort, Heading, HGrid, Box, HelpText, Skeleton, HStack, VStack } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import {
  calculateAcceptanceRate,
  getTopLanguages,
  getEditorStats,
  getLanguageAcceptanceData,
  getLatestUsage,
  getModelUsageMetrics,
  getDateRange,
  getAggregatedMetrics,
  getAggregatedChatStats,
  getAggregatedFeatureAdoption,
  getAggregatedPRSummary,
} from "@/lib/data-utils";
import { LanguageData, EditorData, RepositoryData, ModelData } from "@/lib/types";
import { formatNumber } from "@/lib/format";

// Static header component (automatically prerendered)
function UsageHeader() {
  return (
    <HStack justify="space-between" align="start" gap="space-16" wrap={false}>
      <VStack gap="space-8">
        <Heading size="xlarge" level="1">
          Copilot Bruksstatistikk
        </Heading>
        <BodyShort className="text-gray-600">
          Viser organisasjonens bruk av GitHub Copilot med oppdaterte data
        </BodyShort>
      </VStack>
      <Suspense fallback={<Skeleton variant="rectangle" width={192} height={40} />}>
        <TimeframeSelector />
      </Suspense>
    </HStack>
  );
}

// Cached data component
async function CachedUsageData({ days }: { days: number }) {
  const { usage, error } = await getCachedCopilotUsage("navikt");

  if (error) return <ErrorState message={`Feil ved henting av bruksdata: ${error}`} />;
  if (!usage || usage.length === 0) return <ErrorState message="Ingen bruksdata tilgjengelig" />;

  // Filter to requested timeframe
  const filteredUsage = usage.slice(-days);

  return <UsageContent usage={filteredUsage} />;
}

// Cached premium data component
async function PremiumUsageData({ currentYear, currentMonth }: { currentYear: number; currentMonth: number }) {
  "use cache";
  const { cacheLife, cacheTag } = await import("next/cache");
  cacheLife({ stale: 300 });
  cacheTag("premium-usage-navikt");

  const { usage: premiumUsage } = await getCachedPremiumRequestUsage("navikt", currentYear, currentMonth);

  const premiumRequestsContent = premiumUsage?.usageItems?.length ? (
    <PremiumRequestsContent metrics={calculatePremiumMetrics(premiumUsage)} />
  ) : (
    <BodyShort className="text-gray-500">Ingen premium forespørsel data tilgjengelig for denne måneden</BodyShort>
  );

  return premiumRequestsContent;
}

// Main content component that takes usage data as props
async function UsageContent({ usage }: { usage: CopilotMetrics[] }) {
  "use cache";
  const { cacheLife, cacheTag } = await import("next/cache");
  cacheLife({ stale: 3600 });
  cacheTag("usage-navikt");

  const dateRange = getDateRange(usage);
  const latestUsage = getLatestUsage(usage);
  if (!latestUsage || !dateRange) return <ErrorState message="Ingen bruksdata tilgjengelig" />;

  const aggregatedMetrics = getAggregatedMetrics(usage);
  if (!aggregatedMetrics) return <ErrorState message="Kunne ikke beregne nøkkeltall" />;

  const topLanguages = getTopLanguages(usage);
  const editorStats = getEditorStats(usage);
  const chatStats = getAggregatedChatStats(usage);
  const prSummaryMetrics = getAggregatedPRSummary(usage);
  const featureAdoptionMetrics = getAggregatedFeatureAdoption(usage);
  const modelUsageMetrics = getModelUsageMetrics(usage);

  // Tab content components
  const overviewContent = (
    <VStack gap="space-24">
      {/* Header */}
      <Heading size="medium">Oversikt over nøkkeltall</Heading>

      {/* Key Metrics Cards */}
      <HGrid columns={4} gap="space-16">
        <MetricCard
          value={formatNumber(aggregatedMetrics.totalActiveUsers)}
          label="Aktive brukere"
          helpTitle="Aktive brukere"
          helpText="Unike brukere som har brukt GitHub Copilot i organisasjonen i løpet av hele perioden."
          accentColor="blue"
        />
        <MetricCard
          value={formatNumber(aggregatedMetrics.totalEngagedUsers)}
          label="Engasjerte brukere"
          helpTitle="Engasjerte brukere"
          helpText="Brukere som aktivt har interagert med Copilot ved å akseptere kodeforslag eller bruke chat-funksjonen."
          accentColor="green"
        />
        <MetricCard
          value={`${aggregatedMetrics.overallAcceptanceRate}%`}
          label="Aksepteringsrate"
          helpTitle="Aksepteringsrate"
          helpText="Prosentandel av Copilots kodeforslag som blir akseptert av utviklerne over hele perioden. Typisk ligger gode rater mellom 20-40%."
          accentColor="purple"
        />
        <MetricCard
          value={formatNumber(aggregatedMetrics.totalSuggestions)}
          label="Totale kodeforslag"
          helpTitle="Totale kodeforslag"
          helpText="Totalt antall kodeforslag som Copilot har generert over hele perioden, inkludert både aksepterte og avviste forslag."
          accentColor="orange"
        />
      </HGrid>

      {/* Chat Usage Section */}
      {chatStats && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <Heading size="medium" level="3" className="mb-4">
            Chat-funksjoner
          </Heading>
          <BodyShort className="text-gray-600 mb-4">
            Oversikt over hvordan GitHub Copilot Chat brukes i organisasjonen. Dette inkluderer samtaler i IDE-er og på
            GitHub.com, samt hvordan brukerne interagerer med chat-svarene.
          </BodyShort>
          <HGrid columns={4} gap="space-16">
            <div className="text-center">
              <Heading size="large" level="4" className="mb-2 text-blue-600">
                {formatNumber(chatStats.totalChats)}
              </Heading>
              <div className="flex items-center justify-center gap-1">
                {" "}
                <BodyShort className="text-gray-600">Totale samtaler</BodyShort>
                <HelpText title="Totale samtaler" placement="top">
                  Antall chat-samtaler som har blitt startet med Copilot, både i IDE-er og på GitHub.com.
                </HelpText>
              </div>
            </div>
            <div className="text-center">
              <Heading size="large" level="4" className="mb-2 text-green-600">
                {formatNumber(chatStats.totalCopyEvents)}
              </Heading>
              <div className="flex items-center justify-center gap-1">
                <BodyShort className="text-gray-600">Kopieringshendelser</BodyShort>
                <HelpText title="Kopieringshendelser" placement="top">
                  Hvor mange ganger brukere har kopiert kode eller tekst fra Copilot chat-svar.
                </HelpText>
              </div>
            </div>
            <div className="text-center">
              <Heading size="large" level="4" className="mb-2 text-purple-600">
                {formatNumber(chatStats.totalInsertionEvents)}
              </Heading>
              <div className="flex items-center justify-center gap-1">
                <BodyShort className="text-gray-600">Innsettingshendelser</BodyShort>
                <HelpText title="Innsettingshendelser" placement="top">
                  Antall ganger kode fra chat-svar har blitt satt direkte inn i filer.
                </HelpText>
              </div>
            </div>
            <div className="text-center">
              <Heading size="large" level="4" className="mb-2 text-orange-600">
                {formatNumber(chatStats.ideUsers + chatStats.dotcomUsers)}
              </Heading>
              <div className="flex items-center justify-center gap-1">
                {" "}
                <BodyShort className="text-gray-600">Chat-brukere</BodyShort>
                <HelpText title="Chat-brukere" placement="top">
                  Unike brukere som har brukt Copilot chat-funksjonen, enten i IDE-er eller på GitHub.com.
                </HelpText>
              </div>
            </div>
          </HGrid>
        </Box>
      )}

      {/* Chat Chart */}
      {chatStats && (
        <div className="mt-6">
          <ChatChart usage={usage} />
        </div>
      )}

      {/* Code Completion Details */}
      <Box background="neutral-soft" padding="space-24" borderRadius="12">
        <Heading size="medium" level="3" className="mb-4">
          Detaljer om kodeforslag
        </Heading>
        <BodyShort className="text-gray-600 mb-4">
          Detaljert statistikk over GitHub Copilots funksjon for kodeforslag, som viser hvor effektivt AI-assistenten
          bidrar til kodeutviklingen i organisasjonen over hele perioden.
        </BodyShort>
        <HGrid columns={3} gap="space-16">
          <Box background="info-soft" padding="space-16" borderRadius="8">
            <div className="text-center">
              <Heading size="large" level="4" className="mb-2 text-blue-600">
                {formatNumber(aggregatedMetrics.totalAcceptances)}
              </Heading>
              <div className="flex items-center justify-center gap-1">
                {" "}
                <BodyShort className="text-gray-600">Aksepterte forslag</BodyShort>
                <HelpText title="Aksepterte forslag" placement="top">
                  Antall kodeforslag fra Copilot som utviklerne har akseptert og tatt i bruk over hele perioden.
                </HelpText>
              </div>
            </div>
          </Box>
          <Box background="neutral-soft" padding="space-16" borderRadius="8">
            <div className="text-center">
              <Heading size="large" level="4" className="mb-2 text-gray-600">
                {formatNumber(aggregatedMetrics.totalSuggestions)}
              </Heading>
              <div className="flex items-center justify-center gap-1">
                {" "}
                <BodyShort className="text-gray-600">Totale forslag</BodyShort>
                <HelpText title="Totale forslag" placement="top">
                  Totalt antall kodeforslag som Copilot har generert over hele perioden, inkludert både aksepterte og
                  avviste forslag.
                </HelpText>
              </div>
            </div>
          </Box>
          <Box background="success-soft" padding="space-16" borderRadius="8">
            <div className="text-center">
              <Heading size="large" level="4" className="mb-2 text-green-600">
                {formatNumber(aggregatedMetrics.codeCompletionUsers)}
              </Heading>
              <div className="flex items-center justify-center gap-1">
                {" "}
                <BodyShort className="text-gray-600">Aktive utviklere</BodyShort>
                <HelpText title="Aktive utviklere" placement="top">
                  Antall unike utviklere som har mottatt og interagert med kodeforslag fra Copilot.
                </HelpText>
              </div>
            </div>
          </Box>
        </HGrid>
      </Box>

      {/* Charts Section */}
      <VStack gap="space-24">
        <Heading size="medium" level="3">
          Trendanalyse
        </Heading>
        <TrendChart usage={usage} />
      </VStack>
    </VStack>
  );

  const languagesContent = (
    <VStack gap="space-24">
      {/* Programming Languages Table */}
      <div className="overflow-hidden">
        <Heading size="medium" level="3" className="mb-4">
          Statistikk for programmeringsspråk
        </Heading>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHeaderCell scope="col">
                <div className="flex items-center gap-1">
                  Rangering
                  <HelpText title="Rangering" placement="top">
                    Språkenes rangering basert på antall aktive brukere som bruker Copilot med det språket.
                  </HelpText>
                </div>
              </TableHeaderCell>
              <TableHeaderCell scope="col">Språk</TableHeaderCell>
              <TableHeaderCell scope="col">
                <div className="flex items-center gap-1">
                  Brukere
                  <HelpText title="Brukere" placement="top">
                    Antall unike utviklere som har brukt Copilot med dette programmeringsspråket.
                  </HelpText>
                </div>
              </TableHeaderCell>
              <TableHeaderCell scope="col">
                <div className="flex items-center gap-1">
                  Aksepteringsrate
                  <HelpText title="Aksepteringsrate" placement="top">
                    Hvor stor andel av Copilots forslag som aksepteres for dette språket.
                  </HelpText>
                </div>
              </TableHeaderCell>
              <TableHeaderCell scope="col">
                <div className="flex items-center gap-1">
                  Aksepterte / totale
                  <HelpText title="Aksepterte / totale" placement="top">
                    Antall aksepterte forslag sammenlignet med totalt antall forslag for språket.
                  </HelpText>
                </div>
              </TableHeaderCell>
            </TableRow>
          </TableHeader>
          <TableBody>
            {topLanguages.map((language: LanguageData, index: number) => {
              const { acceptances, suggestions } = getLanguageAcceptanceData(usage, language.name);
              const acceptanceRate = calculateAcceptanceRate(acceptances, suggestions);

              return (
                <TableRow key={language.name}>
                  <TableDataCell>
                    <Box
                      background="accent-soft"
                      borderRadius="full"
                      className="flex items-center justify-center w-8 h-8"
                    >
                      <BodyShort weight="semibold" className="text-blue-600">
                        {index + 1}
                      </BodyShort>
                    </Box>
                  </TableDataCell>
                  <TableDataCell>
                    <BodyShort weight="semibold" className="capitalize">
                      {language.name}
                    </BodyShort>
                  </TableDataCell>
                  <TableDataCell>
                    <BodyShort>{formatNumber(language.total_engaged_users)}</BodyShort>
                  </TableDataCell>
                  <TableDataCell>
                    <BodyShort weight="semibold">{acceptanceRate}%</BodyShort>
                  </TableDataCell>
                  <TableDataCell>
                    <BodyShort>
                      {formatNumber(acceptances)} / {formatNumber(suggestions)}
                    </BodyShort>
                  </TableDataCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>

      {/* Languages Chart */}
      <VStack gap="space-16">
        <Heading size="medium" level="3">
          Språkutvikling over tid
        </Heading>
        <LanguagesChart usage={usage} />
      </VStack>

      {/* Language Distribution */}
      <LanguageDistributionChart usage={usage} />
    </VStack>
  );

  const editorsContent = (
    <VStack gap="space-24">
      {/* Editor Statistics Table */}
      <div className="overflow-hidden">
        <Heading size="medium" level="3" className="mb-4">
          Statistikk for editorer
        </Heading>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHeaderCell scope="col">
                <div className="flex items-center gap-1">
                  Rangering
                  <HelpText title="Rangering" placement="top">
                    Editorenes rangering basert på antall aktive brukere som bruker Copilot med editoren.
                  </HelpText>
                </div>
              </TableHeaderCell>
              <TableHeaderCell scope="col">Editor</TableHeaderCell>
              <TableHeaderCell scope="col">
                <div className="flex items-center gap-1">
                  Brukere
                  <HelpText title="Brukere" placement="top">
                    Antall unike utviklere som bruker Copilot i denne editoren.
                  </HelpText>
                </div>
              </TableHeaderCell>
              <TableHeaderCell scope="col">
                <div className="flex items-center gap-1">
                  Aksepteringsrate
                  <HelpText title="Aksepteringsrate" placement="top">
                    Prosentandel av forslag som blir akseptert i denne editoren.
                  </HelpText>
                </div>
              </TableHeaderCell>
              <TableHeaderCell scope="col">
                <div className="flex items-center gap-1">
                  Aksepterte / totale
                  <HelpText title="Aksepterte / totale" placement="top">
                    Antall aksepterte forslag sammenlignet med totalt antall forslag for editoren.
                  </HelpText>
                </div>
              </TableHeaderCell>
            </TableRow>
          </TableHeader>
          <TableBody>
            {editorStats.map((editor: EditorData, index: number) => (
              <TableRow key={editor.name}>
                <TableDataCell>
                  <Box
                    background="accent-soft"
                    borderRadius="full"
                    className="flex items-center justify-center w-8 h-8"
                  >
                    <BodyShort weight="semibold" className="text-blue-600">
                      {index + 1}
                    </BodyShort>
                  </Box>
                </TableDataCell>
                <TableDataCell>
                  <BodyShort weight="semibold">{editor.name}</BodyShort>
                </TableDataCell>
                <TableDataCell>
                  <BodyShort>{formatNumber(editor.users)}</BodyShort>
                </TableDataCell>
                <TableDataCell>
                  <BodyShort weight="semibold">{editor.acceptanceRate}%</BodyShort>
                </TableDataCell>
                <TableDataCell>
                  <BodyShort>
                    {formatNumber(editor.acceptances)} / {formatNumber(editor.suggestions)}
                  </BodyShort>
                </TableDataCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Editors Chart */}
      <VStack gap="space-16">
        <Heading size="medium" level="3">
          Editorbruk over tid
        </Heading>
        <EditorsChart usage={usage} />
      </VStack>
    </VStack>
  );

  const advancedMetricsContent = (
    <VStack gap="space-24">
      {/* Lines of Code Metrics */}
      {aggregatedMetrics && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <Heading size="medium" level="3" className="mb-4">
            Kodelinjer
          </Heading>
          <BodyShort className="text-gray-600 mb-4">
            Detaljert oversikt over kodelinjer som er foreslått og akseptert av Copilot over hele perioden. Dette gir et
            mer detaljert bilde av kodeproduksjonen enn bare antall forslag.
          </BodyShort>
          <HGrid columns={3} gap="space-16">
            <Box background="info-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4" className="mb-2 text-blue-600">
                  {formatNumber(aggregatedMetrics.totalLinesSuggested)}
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Foreslåtte linjer</BodyShort>
                  <HelpText title="Foreslåtte linjer" placement="top">
                    Totalt antall kodelinjer som Copilot har foreslått gjennom hele perioden.
                  </HelpText>
                </div>
              </div>
            </Box>
            <Box background="success-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4" className="mb-2 text-green-600">
                  {formatNumber(aggregatedMetrics.totalLinesAccepted)}
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Aksepterte linjer</BodyShort>
                  <HelpText title="Aksepterte linjer" placement="top">
                    Antall kodelinjer fra Copilot som utviklerne har akseptert og tatt i bruk over hele perioden.
                  </HelpText>
                </div>
              </div>
            </Box>
            <Box background="warning-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4" className="mb-2 text-orange-600">
                  {aggregatedMetrics.linesAcceptanceRate}%
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Linjeaksepteringsrate</BodyShort>
                  <HelpText title="Linjeaksepteringsrate" placement="top">
                    Prosentandel av foreslåtte kodelinjer som ble akseptert over hele perioden. Dette kan avvike fra
                    forslags-aksepteringsraten.
                  </HelpText>
                </div>
              </div>
            </Box>
          </HGrid>
        </Box>
      )}

      {/* Lines of Code Chart */}
      <LinesOfCodeChart usage={usage} />

      {/* Feature Adoption Breakdown */}
      {featureAdoptionMetrics && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <Heading size="medium" level="3" className="mb-4">
            Funksjonsbruk
          </Heading>
          <BodyShort className="text-gray-600 mb-4">
            Oversikt over hvor mange brukere som benytter de ulike Copilot-funksjonene. Dette hjelper deg å forstå
            hvilke funksjoner som gir mest verdi.
          </BodyShort>
          <HGrid columns={4} gap="space-16">
            <div className="text-center">
              <Heading size="large" level="4" className="mb-2 text-blue-600">
                {formatNumber(featureAdoptionMetrics.codeCompletionUsers)}
              </Heading>
              <BodyShort className="text-gray-600 mb-1">Kodeforslag</BodyShort>
              <BodyShort className="text-sm text-gray-500">
                ({featureAdoptionMetrics.adoptionRates.codeCompletion}% av aktive)
              </BodyShort>
            </div>
            <div className="text-center">
              <Heading size="large" level="4" className="mb-2 text-green-600">
                {formatNumber(featureAdoptionMetrics.ideChatUsers)}
              </Heading>
              <BodyShort className="text-gray-600 mb-1">IDE Chat</BodyShort>
              <BodyShort className="text-sm text-gray-500">
                ({featureAdoptionMetrics.adoptionRates.ideChat}% av aktive)
              </BodyShort>
            </div>
            <div className="text-center">
              <Heading size="large" level="4" className="mb-2 text-purple-600">
                {formatNumber(featureAdoptionMetrics.dotcomChatUsers)}
              </Heading>
              <BodyShort className="text-gray-600 mb-1">GitHub Chat</BodyShort>
              <BodyShort className="text-sm text-gray-500">
                ({featureAdoptionMetrics.adoptionRates.dotcomChat}% av aktive)
              </BodyShort>
            </div>
            <div className="text-center">
              <Heading size="large" level="4" className="mb-2 text-orange-600">
                {formatNumber(featureAdoptionMetrics.prSummaryUsers)}
              </Heading>
              <BodyShort className="text-gray-600 mb-1">PR Sammendrag</BodyShort>
              <BodyShort className="text-sm text-gray-500">
                ({featureAdoptionMetrics.adoptionRates.prSummary}% av aktive)
              </BodyShort>
            </div>
          </HGrid>
        </Box>
      )}

      {/* PR Summary Metrics */}
      {prSummaryMetrics && prSummaryMetrics.totalPRSummaries > 0 && (
        <VStack gap="space-16">
          <Heading size="medium" level="3">
            Pull request-sammendrag
          </Heading>
          <BodyShort className="text-gray-600">
            GitHub Copilot kan automatisk generere sammendrag for pull requests. Her ser du hvordan denne funksjonen
            brukes på tvers av repoer.
          </BodyShort>

          <HGrid columns={3} gap="space-16" className="mb-6">
            <Box background="accent-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4" className="mb-2 text-blue-600">
                  {formatNumber(prSummaryMetrics.totalEngagedUsers)}
                </Heading>
                <BodyShort className="text-gray-600">Aktive brukere</BodyShort>
              </div>
            </Box>
            <Box background="success-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4" className="mb-2 text-green-600">
                  {formatNumber(prSummaryMetrics.totalPRSummaries)}
                </Heading>
                <BodyShort className="text-gray-600">Genererte sammendrag</BodyShort>
              </div>
            </Box>
            <Box background="info-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4" className="mb-2 text-purple-600">
                  {formatNumber(prSummaryMetrics.repositoryStats.length)}
                </Heading>
                <BodyShort className="text-gray-600">Repositorier</BodyShort>
              </div>
            </Box>
          </HGrid>

          {prSummaryMetrics.repositoryStats.length > 0 && (
            <div className="overflow-hidden">
              <Heading size="small" level="4" className="mb-4">
                Topp-repoer
              </Heading>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHeaderCell scope="col">Repository</TableHeaderCell>
                    <TableHeaderCell scope="col">Brukere</TableHeaderCell>
                    <TableHeaderCell scope="col">Sammendrag</TableHeaderCell>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {prSummaryMetrics.repositoryStats.slice(0, 10).map((repo: RepositoryData) => (
                    <TableRow key={repo.name}>
                      <TableDataCell>
                        <BodyShort weight="semibold">{repo.name}</BodyShort>
                      </TableDataCell>
                      <TableDataCell>
                        <BodyShort>{formatNumber(repo.users)}</BodyShort>
                      </TableDataCell>
                      <TableDataCell>
                        <BodyShort>{formatNumber(repo.summaries)}</BodyShort>
                      </TableDataCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </VStack>
      )}

      {/* Model Usage Information */}
      {modelUsageMetrics && modelUsageMetrics.length > 0 && (
        <VStack gap="space-16">
          <Heading size="medium" level="3">
            AI-modeller i bruk
          </Heading>
          <BodyShort className="text-gray-600">
            Oversikt over hvilke AI-modeller som brukes og hvilke funksjoner de støtter. Dette inkluderer både standard
            GitHub-modeller og tilpassede modeller.
          </BodyShort>

          <HGrid columns={2} gap="space-24">
            <div className="overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHeaderCell scope="col">Modell</TableHeaderCell>
                    <TableHeaderCell scope="col">Type</TableHeaderCell>
                    <TableHeaderCell scope="col">Brukere</TableHeaderCell>
                    <TableHeaderCell scope="col">Funksjoner</TableHeaderCell>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {modelUsageMetrics.map((model: ModelData) => (
                    <TableRow key={model.name}>
                      <TableDataCell>
                        <BodyShort weight="semibold">{model.name}</BodyShort>
                      </TableDataCell>
                      <TableDataCell>
                        <BodyShort className={model.isCustom ? "text-purple-600" : "text-gray-600"}>
                          {model.isCustom ? "Tilpasset" : "Standard"}
                        </BodyShort>
                      </TableDataCell>
                      <TableDataCell>
                        <BodyShort>{formatNumber(model.users)}</BodyShort>
                      </TableDataCell>
                      <TableDataCell>
                        <BodyShort className="text-sm">{model.features.join(", ")}</BodyShort>
                      </TableDataCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <ModelUsageChart usage={usage} />
          </HGrid>
        </VStack>
      )}
    </VStack>
  );

  const tabs = [
    { id: "overview", label: "Oversikt", content: overviewContent },
    { id: "languages", label: "Språk og teknologier", content: languagesContent },
    { id: "editors", label: "Utviklingsverktøy", content: editorsContent },
    { id: "advanced", label: "Avanserte målinger", content: advancedMetricsContent },
    {
      id: "premium",
      label: "Premiumforespørsler",
      content: (
        <Suspense fallback={<Skeleton variant="rectangle" height={200} />}>
          <PremiumUsageData currentYear={new Date().getFullYear()} currentMonth={new Date().getMonth() + 1} />
        </Suspense>
      ),
    },
  ];

  return (
    <>
      <BodyShort className="text-gray-600 mb-12">
        Periode: {dateRange.start} - {dateRange.end} ({formatNumber(usage.length)} dager) • Viser organisasjonens bruk
        av GitHub Copilot
      </BodyShort>
      <Tabs tabs={tabs} defaultTab="overview" />
    </>
  );
}

// Main page component using Partial Prerendering
export default async function Usage({ searchParams }: { searchParams: Promise<{ days?: string }> }) {
  const params = await searchParams;
  const days = Math.min(Math.max(parseInt(params.days || "28", 10) || 28, 1), 100);

  return (
    <main className="max-w-7xl mx-auto">
      <Box
        paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
      >
        <section>
          {/* Static content - automatically prerendered */}
          <UsageHeader />

          {/* Cached dynamic content - included in static shell */}
          <Suspense
            fallback={
              <div className="space-y-4">
                <Skeleton variant="text" width="60%" />
                <Skeleton variant="rectangle" height={400} />
              </div>
            }
          >
            <CachedUsageData days={days} />
          </Suspense>
        </section>
      </Box>
    </main>
  );
}
