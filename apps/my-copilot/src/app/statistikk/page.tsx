import React, { Suspense } from "react";
import { getCachedPremiumRequestUsage, getCachedCopilotBilling } from "@/lib/cached-github";
import { getCachedBigQueryUsage } from "@/lib/cached-bigquery";
import type { EnterpriseMetrics } from "@/lib/types";
import Tabs from "@/components/tabs";
import TrendChart from "@/components/charts/TrendChart";
import LanguagesChart from "@/components/charts/LanguagesChart";
import EditorsChart from "@/components/charts/EditorsChart";
import ChatChart from "@/components/charts/ChatChart";
import ModelUsageChart from "@/components/charts/ModelUsageChart";
import LinesOfCodeChart from "@/components/charts/LinesOfCodeChart";
import LanguageDistributionChart from "@/components/charts/LanguageDistributionChart";
import AdoptionTrendChart from "@/components/charts/AdoptionTrendChart";
import GenerationModeChart from "@/components/charts/GenerationModeChart";
import MetricCard from "@/components/metric-card";
import ErrorState from "@/components/error-state";
import PremiumRequestsContent from "@/components/premium-requests-content";
import TimeframeSelector from "@/components/timeframe-selector";
import { calculatePremiumMetrics } from "@/lib/billing-utils";
import { Table, BodyShort, Heading, HGrid, Box, HelpText, Skeleton, VStack } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { PageHero } from "@/components/page-hero";
import {
  getTopLanguages,
  getEditorStats,
  getModelUsageMetrics,
  getDateRange,
  getAggregatedMetrics,
  getFeatureAdoption,
  getPRMetrics,
  getCLIMetrics,
  buildTrendData,
  buildAdoptionTrendData,
  buildLanguageChartData,
  buildEditorChartData,
  buildFeatureChartData,
  buildLinesOfCodeData,
  buildModelChartData,
  getGenerationModeSummary,
  buildGenerationModeTrendData,
} from "@/lib/data-utils";
import type { LanguageData, EditorData, ModelData } from "@/lib/types";
import { formatNumber } from "@/lib/format";
import { getUser } from "@/lib/auth";

function formatMinutes(minutes: number): string {
  if (minutes < 60) return `${Math.round(minutes)} min`;
  const hours = Math.floor(minutes / 60);
  const mins = Math.round(minutes % 60);
  if (hours < 24) return mins > 0 ? `${hours}t ${mins}m` : `${hours}t`;
  const days = Math.floor(hours / 24);
  const remainingHours = hours % 24;
  return remainingHours > 0 ? `${days}d ${remainingHours}t` : `${days}d`;
}

// Static header component (automatically prerendered)
function UsageHeader() {
  return (
    <PageHero
      title="Statistikk"
      description="Bruksdata og trender for GitHub Copilot i Nav."
      actions={
        <Suspense fallback={<Skeleton variant="rectangle" width={192} height={40} />}>
          <TimeframeSelector />
        </Suspense>
      }
    />
  );
}

// Cached data component
async function CachedUsageData({ days }: { days: number }) {
  const { usage, error } = await getCachedBigQueryUsage();

  if (error) return <ErrorState message={`Feil ved henting av bruksdata: ${error}`} />;
  if (!usage || usage.length === 0) return <ErrorState message="Ingen bruksdata tilgjengelig" />;

  const filteredUsage = days > 0 ? usage.slice(-days) : usage;

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
    <BodyShort className="text-gray-500">Ingen data om premiumforespørsler tilgjengelig for denne måneden.</BodyShort>
  );

  return premiumRequestsContent;
}

function currencyFormat(num: number) {
  return `$${num.toFixed(2).replace(/(\d)(?=(\d{3})+(?!\d))/g, "$1,")} USD`;
}

// Cached billing data for cost tab
async function BillingTabContent() {
  "use cache";
  const { cacheLife, cacheTag } = await import("next/cache");
  cacheLife({ stale: 3600 });
  cacheTag("billing-navikt");

  const { billing, error } = await getCachedCopilotBilling("navikt");

  if (error) return <ErrorState message={`Feil ved henting av faktureringsdata: ${error}`} />;
  if (!billing) return <ErrorState message="Ingen faktureringsdata tilgjengelig" />;

  return (
    <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
      <Box background="neutral-soft" padding="space-24" borderRadius="12">
        <Heading size="small" level="2" spacing>
          Lisensfordeling
        </Heading>
        <Table size="small">
          <TableBody>
            <TableRow>
              <TableHeaderCell>Totalt antall lisenser</TableHeaderCell>
              <TableDataCell>{formatNumber(billing.seat_breakdown.total ?? 0)}</TableDataCell>
            </TableRow>
            <TableRow>
              <TableHeaderCell>Aktiv denne perioden</TableHeaderCell>
              <TableDataCell className="text-green-600 font-semibold">
                {formatNumber(billing.seat_breakdown.active_this_cycle ?? 0)}
              </TableDataCell>
            </TableRow>
            <TableRow>
              <TableHeaderCell>Inaktiv denne perioden</TableHeaderCell>
              <TableDataCell>{formatNumber(billing.seat_breakdown.inactive_this_cycle ?? 0)}</TableDataCell>
            </TableRow>
            <TableRow>
              <TableHeaderCell>Lagt til denne perioden</TableHeaderCell>
              <TableDataCell>{formatNumber(billing.seat_breakdown.added_this_cycle ?? 0)}</TableDataCell>
            </TableRow>
            <TableRow>
              <TableHeaderCell>Ventende invitasjon</TableHeaderCell>
              <TableDataCell>{formatNumber(billing.seat_breakdown.pending_invitation ?? 0)}</TableDataCell>
            </TableRow>
            <TableRow>
              <TableHeaderCell>Ventende kansellering</TableHeaderCell>
              <TableDataCell>{formatNumber(billing.seat_breakdown.pending_cancellation ?? 0)}</TableDataCell>
            </TableRow>
          </TableBody>
        </Table>
        <Box background="info-soft" padding="space-12" borderRadius="8" className="mt-4">
          <BodyShort weight="semibold">
            Total kostnad: {currencyFormat((billing.seat_breakdown.total ?? 0) * 19)} / måned
          </BodyShort>
        </Box>
      </Box>

      <Box background="neutral-soft" padding="space-24" borderRadius="12">
        <Heading size="small" level="2" spacing>
          Organisasjonsinnstillinger
        </Heading>
        <Table size="small">
          <TableBody>
            <TableRow>
              <TableHeaderCell>Administrasjon av lisenser</TableHeaderCell>
              <TableDataCell className="capitalize">{billing.seat_management_setting}</TableDataCell>
            </TableRow>
            <TableRow>
              <TableHeaderCell>IDE Chat</TableHeaderCell>
              <TableDataCell className="capitalize">{billing.ide_chat}</TableDataCell>
            </TableRow>
            <TableRow>
              <TableHeaderCell>Plattform Chat</TableHeaderCell>
              <TableDataCell className="capitalize">{billing.platform_chat}</TableDataCell>
            </TableRow>
            <TableRow>
              <TableHeaderCell>CLI</TableHeaderCell>
              <TableDataCell className="capitalize">{billing.cli}</TableDataCell>
            </TableRow>
            <TableRow>
              <TableHeaderCell>Offentlige kodeforslag</TableHeaderCell>
              <TableDataCell className="capitalize">{billing.public_code_suggestions}</TableDataCell>
            </TableRow>
          </TableBody>
        </Table>
      </Box>
    </HGrid>
  );
}

// Main content component that takes usage data as props
async function UsageContent({ usage }: { usage: EnterpriseMetrics[] }) {
  "use cache";
  const { cacheLife, cacheTag } = await import("next/cache");
  cacheLife({ stale: 3600 });
  cacheTag("usage-navikt");

  const dateRange = getDateRange(usage);
  if (!dateRange) return <ErrorState message="Ingen bruksdata tilgjengelig" />;

  const aggregatedMetrics = getAggregatedMetrics(usage);
  if (!aggregatedMetrics) return <ErrorState message="Kunne ikke beregne nøkkeltall" />;

  const topLanguages = getTopLanguages(usage);
  const editorStats = getEditorStats(usage);
  const featureAdoption = getFeatureAdoption(usage);
  const prMetrics = getPRMetrics(usage);
  const cliMetrics = getCLIMetrics(usage);
  const modelUsageMetrics = getModelUsageMetrics(usage);
  const generationModeSummary = getGenerationModeSummary(usage);

  const trendData = buildTrendData(usage);
  const adoptionTrendData = buildAdoptionTrendData(usage);
  const languageChartData = buildLanguageChartData(usage);
  const editorChartData = buildEditorChartData(usage);
  const featureChartData = buildFeatureChartData(usage);
  const linesOfCodeData = buildLinesOfCodeData(usage);
  const modelChartData = buildModelChartData(usage);
  const generationModeTrendData = buildGenerationModeTrendData(usage);

  // Tab content components
  const overviewContent = (
    <VStack gap="space-24">
      {/* 1. Key Metrics Cards */}
      <Heading size="small">Nøkkeltall</Heading>
      <HGrid columns={{ xs: 1, sm: 2, md: 4 }} gap="space-16">
        <MetricCard
          value={formatNumber(aggregatedMetrics.dailyActiveUsers)}
          label="Daglig aktive brukere"
          helpTitle="Daglig aktive brukere"
          helpText="Antall unike brukere som brukte Copilot siste dag i perioden."
        />
        <MetricCard
          value={formatNumber(aggregatedMetrics.monthlyActiveUsers)}
          label="Månedlig aktive brukere"
          helpTitle="Månedlig aktive brukere"
          helpText="Antall unike brukere som har brukt Copilot siste 30 dager."
        />
        <MetricCard
          value={`${aggregatedMetrics.overallAcceptanceRate}%`}
          label="Aksepteringsrate"
          helpTitle="Aksepteringsrate"
          helpText="Andel av Copilots kodeforslag som utviklerne aksepterer. Gode rater ligger mellom 20–40 %."
        />
        <MetricCard
          value={formatNumber(aggregatedMetrics.totalInteractions)}
          label="Totale interaksjoner"
          helpTitle="Totale interaksjoner"
          helpText="Alle interaksjoner med Copilot i perioden: chat, agent-forespørsler, kodeforslag og andre handlinger."
        />
      </HGrid>

      {/* 2. Adoption Section — Chat, Agent, CLI */}
      <Box background="neutral-soft" padding="space-24" borderRadius="12">
        <VStack gap="space-16">
          <div className="flex items-center gap-2">
            <Heading size="small" level="3">
              Adopsjon
            </Heading>
            <HelpText title="Adopsjon" placement="top">
              Alle serier viser 7-dagers rullerende snitt for enkel sammenligning. Chat og agent er basert på GitHubs
              rullende 30-dagersvindu, CLI på daglige tall.
            </HelpText>
          </div>
          <BodyShort className="text-gray-600">
            Bruk av Copilots funksjoner. Alle grafer viser 7-dagers rullerende snitt.
          </BodyShort>
          <HGrid columns={{ xs: 1, sm: 3 }} gap="space-16">
            <Box background="info-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {formatNumber(aggregatedMetrics.monthlyActiveChatUsers)}
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Chat-brukere</BodyShort>
                  <HelpText title="Chat-brukere" placement="top">
                    Unike brukere som har brukt Copilot Chat siste 30 dager. Inkluderer inline chat, spør-modus og
                    egendefinerte moduser.
                  </HelpText>
                </div>
              </div>
            </Box>
            <Box background="success-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {formatNumber(aggregatedMetrics.monthlyActiveAgentUsers)}
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Agent-brukere</BodyShort>
                  <HelpText title="Agent-brukere" placement="top">
                    Unike brukere som har brukt agent mode siste 30 dager. I agent mode kan Copilot redigere filer,
                    kjøre tester og lage pull requests i flere steg.
                  </HelpText>
                </div>
              </div>
            </Box>
            <Box background="warning-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {formatNumber(aggregatedMetrics.dailyActiveCLIUsers)}
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">CLI-brukere</BodyShort>
                  <HelpText title="CLI-brukere" placement="top">
                    Brukere som brukte Copilot CLI i terminalen. Kortet viser siste dags tall, grafen viser 7-dagers
                    snitt.
                  </HelpText>
                </div>
              </div>
            </Box>
          </HGrid>
          <AdoptionTrendChart data={adoptionTrendData} />
        </VStack>
      </Box>

      {/* 3. Feature Activity */}
      <ChatChart data={featureChartData} />

      {/* 4. Code Suggestions */}
      <Box background="neutral-soft" padding="space-24" borderRadius="12">
        <VStack gap="space-16">
          <div className="flex items-center gap-2">
            <Heading size="small" level="3">
              Kodeforslag
            </Heading>
            <HelpText title="Kodeforslag" placement="top">
              Inline kodeforslag i editoren. Hvor mange Copilot har generert, og hvor mange utviklerne aksepterte.
            </HelpText>
          </div>
          <HGrid columns={{ xs: 1, sm: 3 }} gap="space-16">
            <Box background="info-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {formatNumber(aggregatedMetrics.totalGenerations)}
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Genererte forslag</BodyShort>
                  <HelpText title="Genererte forslag" placement="top">
                    Inline kodeforslag Copilot har vist i editoren i perioden.
                  </HelpText>
                </div>
              </div>
            </Box>
            <Box background="success-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {formatNumber(aggregatedMetrics.totalAcceptances)}
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Aksepterte forslag</BodyShort>
                  <HelpText title="Aksepterte forslag" placement="top">
                    Kodeforslag utviklerne godtok (Tab-tasten) i perioden.
                  </HelpText>
                </div>
              </div>
            </Box>
            <Box background="accent-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {aggregatedMetrics.overallAcceptanceRate}%
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Aksepteringsrate</BodyShort>
                  <HelpText title="Aksepteringsrate" placement="top">
                    Andel aksepterte forslag av totale. Gode rater ligger mellom 20–40 %.
                  </HelpText>
                </div>
              </div>
            </Box>
          </HGrid>
        </VStack>
      </Box>

      {/* 5. Generation Mode Split */}
      {generationModeSummary && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-16">
            <div className="flex items-center gap-2">
              <Heading size="small" level="3">
                Bruker- vs. agentinitiiert
              </Heading>
              <HelpText title="Genereringsmodus" placement="top">
                Fordeling mellom brukerinitiierte handlinger (kodeforslag, inline chat, spør-modus) og agentinitiierte
                (agent-modus, agent-redigering, redigeringsmodus).
              </HelpText>
            </div>
            <BodyShort className="text-gray-600">
              Hvor stor del av Copilots aktivitet som drives av brukeren direkte vs. av agenten.
            </BodyShort>
            <HGrid columns={{ xs: 1, sm: 2, md: 3 }} gap="space-16">
              <Box background="info-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(generationModeSummary.userInitiatedGenerations)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Brukerinitiiert</BodyShort>
                    <HelpText title="Brukerinitiiert" placement="top">
                      Kodeforslag (Tab-completions), inline chat og spør-modus.
                    </HelpText>
                  </div>
                </div>
              </Box>
              <Box background="accent-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(generationModeSummary.agentInitiatedGenerations)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Agentinitiiert</BodyShort>
                    <HelpText title="Agentinitiiert" placement="top">
                      Agent-modus, agent-redigering, redigeringsmodus og egendefinert modus.
                    </HelpText>
                  </div>
                </div>
              </Box>
              <Box background="success-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {generationModeSummary.agentShare}%
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Agent-andel</BodyShort>
                    <HelpText title="Agent-andel" placement="top">
                      Andel av genereringene som er agentinitiierte.
                    </HelpText>
                  </div>
                </div>
              </Box>
            </HGrid>
            <GenerationModeChart data={generationModeTrendData} />
          </VStack>
        </Box>
      )}

      {/* 6. Daily Activity Trend */}
      <TrendChart data={trendData} />
    </VStack>
  );

  const languagesContent = (
    <VStack gap="space-24">
      {/* Programming Languages Table */}
      <VStack gap="space-16">
        <Heading size="small" level="3">
          Statistikk for programmeringsspråk
        </Heading>
        <div className="overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHeaderCell scope="col">
                  <div className="flex items-center gap-1">
                    Rangering
                    <HelpText title="Rangering" placement="top">
                      Rangering ut fra antall genereringer med Copilot.
                    </HelpText>
                  </div>
                </TableHeaderCell>
                <TableHeaderCell scope="col">Språk</TableHeaderCell>
                <TableHeaderCell scope="col">
                  <div className="flex items-center gap-1">
                    Genereringer
                    <HelpText title="Genereringer" placement="top">
                      Antall kodeforslag Copilot har generert for dette språket.
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
                      <BodyShort>{formatNumber(language.generations)}</BodyShort>
                    </TableDataCell>
                    <TableDataCell>
                      <BodyShort weight="semibold">{language.acceptanceRate}%</BodyShort>
                    </TableDataCell>
                    <TableDataCell>
                      <BodyShort>
                        {formatNumber(language.acceptances)} / {formatNumber(language.generations)}
                      </BodyShort>
                    </TableDataCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
      </VStack>

      {/* Languages Chart */}
      <VStack gap="space-16">
        <Heading size="small" level="3">
          Språkutvikling over tid
        </Heading>
        <LanguagesChart data={languageChartData} />
      </VStack>

      {/* Language Distribution */}
      <LanguageDistributionChart data={topLanguages} />
    </VStack>
  );

  const editorsContent = (
    <VStack gap="space-24">
      {/* Editor Statistics Table */}
      <VStack gap="space-16">
        <Heading size="small" level="3">
          Statistikk for utviklingsverktøy
        </Heading>
        <div className="overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHeaderCell scope="col">
                  <div className="flex items-center gap-1">
                    Rangering
                    <HelpText title="Rangering" placement="top">
                      Rangering ut fra aktivitetsnivå i hvert verktøy.
                    </HelpText>
                  </div>
                </TableHeaderCell>
                <TableHeaderCell scope="col">Verktøy</TableHeaderCell>
                <TableHeaderCell scope="col">
                  <div className="flex items-center gap-1">
                    Aktivitet
                    <HelpText title="Aktivitet" placement="top">
                      Antall kodeforslag generert (editorer) eller forespørsler (CLI).
                    </HelpText>
                  </div>
                </TableHeaderCell>
                <TableHeaderCell scope="col">
                  <div className="flex items-center gap-1">
                    Aksepteringsrate
                    <HelpText title="Aksepteringsrate" placement="top">
                      Prosentandel av forslag som blir akseptert. Gjelder ikke CLI.
                    </HelpText>
                  </div>
                </TableHeaderCell>
                <TableHeaderCell scope="col">
                  <div className="flex items-center gap-1">
                    Aksepterte / totale
                    <HelpText title="Aksepterte / totale" placement="top">
                      Antall aksepterte forslag sammenlignet med totalt antall forslag. For CLI vises sesjoner.
                    </HelpText>
                  </div>
                </TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              {editorStats.map((editor: EditorData, index: number) => {
                const isCLI = editor.name === "Copilot CLI";
                return (
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
                      <BodyShort>
                        {formatNumber(editor.generations)}
                        {isCLI && <span className="text-gray-500 text-sm"> forespørsler</span>}
                      </BodyShort>
                    </TableDataCell>
                    <TableDataCell>
                      <BodyShort weight="semibold">{isCLI ? "–" : `${editor.acceptanceRate}%`}</BodyShort>
                    </TableDataCell>
                    <TableDataCell>
                      <BodyShort>
                        {isCLI
                          ? `${formatNumber(editor.interactions)} sesjoner`
                          : `${formatNumber(editor.acceptances)} / ${formatNumber(editor.generations)}`}
                      </BodyShort>
                    </TableDataCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
      </VStack>

      {/* Editors Chart */}
      <VStack gap="space-16">
        <Heading size="small" level="3">
          Aktivitet per verktøy over tid
        </Heading>
        <EditorsChart data={editorChartData} />
      </VStack>
    </VStack>
  );

  const advancedMetricsContent = (
    <VStack gap="space-24">
      {/* Lines of Code Metrics */}
      <Box background="neutral-soft" padding="space-24" borderRadius="12">
        <VStack gap="space-16">
          <div className="flex items-center gap-2">
            <Heading size="small" level="3">
              Kodelinjer
            </Heading>
            <HelpText title="Kodelinjer" placement="top">
              Kodelinjer Copilot har foreslått å legge til eller slette, og hvor mange utviklerne aksepterte.
            </HelpText>
          </div>
          <BodyShort className="text-gray-600">
            Foreslåtte og aksepterte kodelinjer i perioden, fordelt på lagt til og slettet.
          </BodyShort>
          <HGrid columns={{ xs: 1, sm: 2, md: 5 }} gap="space-16">
            <Box background="info-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {formatNumber(aggregatedMetrics.totalLinesSuggested)}
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Foreslått lagt til</BodyShort>
                  <HelpText title="Foreslått lagt til" placement="top">
                    Kodelinjer Copilot har foreslått å legge til i perioden.
                  </HelpText>
                </div>
              </div>
            </Box>
            <Box background="success-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {formatNumber(aggregatedMetrics.totalLinesAccepted)}
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Akseptert lagt til</BodyShort>
                  <HelpText title="Akseptert lagt til" placement="top">
                    Foreslåtte kodelinjer (lagt til) som utviklerne aksepterte.
                  </HelpText>
                </div>
              </div>
            </Box>
            <Box background="warning-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {formatNumber(aggregatedMetrics.totalLinesDeletedSuggested)}
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Foreslått slettet</BodyShort>
                  <HelpText title="Foreslått slettet" placement="top">
                    Kodelinjer Copilot har foreslått å slette i perioden.
                  </HelpText>
                </div>
              </div>
            </Box>
            <Box background="danger-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {formatNumber(aggregatedMetrics.totalLinesDeleted)}
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Akseptert slettet</BodyShort>
                  <HelpText title="Akseptert slettet" placement="top">
                    Foreslåtte kodelinjer (slettet) som utviklerne aksepterte.
                  </HelpText>
                </div>
              </div>
            </Box>
            <Box background="accent-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {aggregatedMetrics.linesAcceptanceRate}%
                </Heading>
                <div className="flex items-center justify-center gap-1">
                  <BodyShort className="text-gray-600">Aksepteringsrate</BodyShort>
                  <HelpText title="Aksepteringsrate (linjer)" placement="top">
                    Andel av foreslåtte kodelinjer (lagt til) som ble akseptert.
                  </HelpText>
                </div>
              </div>
            </Box>
          </HGrid>
        </VStack>
      </Box>

      {/* Lines of Code Chart */}
      <LinesOfCodeChart data={linesOfCodeData} />

      {/* Feature Adoption Breakdown */}
      {featureAdoption && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-16">
            <div className="flex items-center gap-2">
              <Heading size="small" level="3">
                Funksjonsbruk
              </Heading>
              <HelpText title="Funksjonsbruk" placement="top">
                Aktivitet per Copilot-funksjon, målt i genereringer og aksepteringer.
              </HelpText>
            </div>
            <BodyShort className="text-gray-600">Genereringer og aksepteringer per funksjon i perioden.</BodyShort>
            <HGrid columns={{ xs: 1, sm: 2, md: 3 }} gap="space-16">
              {featureAdoption.features.map((feature) => (
                <Box key={feature.name} background="info-soft" padding="space-16" borderRadius="8">
                  <VStack gap="space-4" align="center">
                    <Heading size="large" level="4">
                      {formatNumber(feature.generations)}
                    </Heading>
                    <div className="flex items-center gap-1">
                      <BodyShort className="text-gray-600">{feature.label}</BodyShort>
                      <HelpText title={feature.label} placement="top">
                        Genereringer: antall ganger Copilot produserte et forslag. Aksepteringer: antall ganger brukeren
                        tok forslaget i bruk.
                      </HelpText>
                    </div>
                    <BodyShort className="text-sm text-gray-500">
                      {formatNumber(feature.acceptances)} akseptert
                    </BodyShort>
                  </VStack>
                </Box>
              ))}
            </HGrid>
          </VStack>
        </Box>
      )}

      {/* PR Metrics */}
      {prMetrics && prMetrics.totalCreated > 0 && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-16">
            <div className="flex items-center gap-2">
              <Heading size="small" level="3">
                Pull requests
              </Heading>
              <HelpText title="Pull requests" placement="top">
                PR-aktivitet der Copilot var involvert som forfatter eller reviewer.
              </HelpText>
            </div>
            <BodyShort className="text-gray-600">
              Opprettelse, review og merge-tider for PR-er der Copilot var involvert.
            </BodyShort>

            <BodyShort weight="semibold" className="text-gray-700">
              Opprettelse og merge
            </BodyShort>
            <HGrid columns={{ xs: 1, sm: 2, md: 4 }} gap="space-16">
              <Box background="accent-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalCreated)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Totalt opprettet</BodyShort>
                    <HelpText title="Totalt opprettet" placement="top">
                      PR-er opprettet i organisasjonen i perioden.
                    </HelpText>
                  </div>
                </div>
              </Box>
              <Box background="success-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalCreatedByCopilot)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Opprettet av Copilot</BodyShort>
                    <HelpText title="Opprettet av Copilot" placement="top">
                      PR-er Copilot opprettet i agent mode, basert på oppgavebeskrivelser.
                    </HelpText>
                  </div>
                </div>
              </Box>
              <Box background="info-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalMerged)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Merget</BodyShort>
                    <HelpText title="Merget" placement="top">
                      PR-er som ble merget i perioden.
                    </HelpText>
                  </div>
                </div>
              </Box>
              <Box background="warning-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalMergedCreatedByCopilot)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Copilot-PR-er merget</BodyShort>
                    <HelpText title="Copilot-PR-er merget" placement="top">
                      Copilot-opprettede PR-er som ble merget.
                    </HelpText>
                  </div>
                </div>
              </Box>
              <Box background="info-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalMergedReviewedByCopilot)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Copilot-reviewede merget</BodyShort>
                    <HelpText title="Copilot-reviewede PR-er merget" placement="top">
                      PR-er reviewet av Copilot som deretter ble merget.
                    </HelpText>
                  </div>
                </div>
              </Box>
            </HGrid>

            {(prMetrics.medianMinutesToMerge > 0 ||
              prMetrics.medianMinutesToMergeCopilotAuthored > 0 ||
              prMetrics.medianMinutesToMergeCopilotReviewed > 0) && (
              <>
                <BodyShort weight="semibold" className="text-gray-700">
                  Tider
                </BodyShort>
                <HGrid columns={{ xs: 1, sm: 3 }} gap="space-16">
                  <Box background="neutral-moderate" padding="space-16" borderRadius="8">
                    <div className="text-center">
                      <Heading size="large" level="4">
                        {formatMinutes(prMetrics.medianMinutesToMerge)}
                      </Heading>
                      <div className="flex items-center justify-center gap-1">
                        <BodyShort className="text-gray-600">Median tid til merge</BodyShort>
                        <HelpText title="Median tid til merge" placement="top">
                          Median tid fra PR opprettes til den merges.
                        </HelpText>
                      </div>
                    </div>
                  </Box>
                  <Box background="success-soft" padding="space-16" borderRadius="8">
                    <div className="text-center">
                      <Heading size="large" level="4">
                        {formatMinutes(prMetrics.medianMinutesToMergeCopilotAuthored)}
                      </Heading>
                      <div className="flex items-center justify-center gap-1">
                        <BodyShort className="text-gray-600">Median tid (Copilot-PR)</BodyShort>
                        <HelpText title="Median tid for Copilot-PR" placement="top">
                          Median tid for Copilot-opprettede PR-er. Sammenlign med totalen for å se om de merges raskere.
                        </HelpText>
                      </div>
                    </div>
                  </Box>
                  <Box background="info-soft" padding="space-16" borderRadius="8">
                    <div className="text-center">
                      <Heading size="large" level="4">
                        {formatMinutes(prMetrics.medianMinutesToMergeCopilotReviewed)}
                      </Heading>
                      <div className="flex items-center justify-center gap-1">
                        <BodyShort className="text-gray-600">Median tid (Copilot-review)</BodyShort>
                        <HelpText title="Median tid for Copilot-reviewede PR-er" placement="top">
                          Median tid for PR-er som Copilot har reviewet.
                        </HelpText>
                      </div>
                    </div>
                  </Box>
                </HGrid>
              </>
            )}

            <BodyShort weight="semibold" className="text-gray-700">
              Code review
            </BodyShort>
            <HGrid columns={{ xs: 1, sm: 2, md: 5 }} gap="space-16">
              <Box background="info-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalReviewed)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Reviewed</BodyShort>
                    <HelpText title="Reviewed" placement="top">
                      PR-er som fikk review i perioden.
                    </HelpText>
                  </div>
                </div>
              </Box>
              <Box background="accent-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalReviewedByCopilot)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Reviewed av Copilot</BodyShort>
                    <HelpText title="Reviewed av Copilot" placement="top">
                      PR-er som fikk code review av Copilot, med automatiske forslag til forbedringer.
                    </HelpText>
                  </div>
                </div>
              </Box>
              <Box background="warning-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalCopilotSuggestions)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Copilot review-forslag</BodyShort>
                    <HelpText title="Copilot review-forslag" placement="top">
                      Kodeendringsforslag fra Copilot under review. Utviklere kan godta eller avvise forslagene i PR-en.
                    </HelpText>
                  </div>
                </div>
              </Box>
              <Box background="success-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalAppliedSuggestions)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Anvendte forslag</BodyShort>
                    <HelpText title="Anvendte forslag" placement="top">
                      Review-forslag som utviklerne tok i bruk.
                    </HelpText>
                  </div>
                </div>
              </Box>
              {prMetrics.totalCopilotSuggestions > 0 && (
                <Box background="neutral-moderate" padding="space-16" borderRadius="8">
                  <div className="text-center">
                    <Heading size="large" level="4">
                      {Math.round((prMetrics.totalCopilotAppliedSuggestions / prMetrics.totalCopilotSuggestions) * 100)}
                      %
                    </Heading>
                    <div className="flex items-center justify-center gap-1">
                      <BodyShort className="text-gray-600">Aksepteringsrate</BodyShort>
                      <HelpText title="Aksepteringsrate for review-forslag" placement="top">
                        Andel av Copilots review-forslag som utviklerne tok i bruk.
                      </HelpText>
                    </div>
                  </div>
                </Box>
              )}
            </HGrid>
          </VStack>
        </Box>
      )}

      {/* CLI Metrics */}
      {cliMetrics && cliMetrics.sessionCount > 0 && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-16">
            <div className="flex items-center gap-2">
              <Heading size="small" level="3">
                Copilot CLI
              </Heading>
              <HelpText title="Copilot CLI" placement="top">
                Copilot direkte i terminalen.
              </HelpText>
            </div>
            <BodyShort className="text-gray-600">Sesjoner, forespørsler og tokenforbruk i CLI-et.</BodyShort>

            <BodyShort weight="semibold" className="text-gray-700">
              Bruk
            </BodyShort>
            <HGrid columns={{ xs: 1, sm: 2, md: 3 }} gap="space-16">
              <Box background="info-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(aggregatedMetrics.dailyActiveCLIUsers)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Daglige CLI-brukere</BodyShort>
                    <HelpText title="Daglige CLI-brukere" placement="top">
                      Unike brukere som brukte Copilot CLI siste dag i perioden.
                    </HelpText>
                  </div>
                </div>
              </Box>
              <Box background="accent-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(cliMetrics.sessionCount)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Sesjoner</BodyShort>
                    <HelpText title="CLI-sesjoner" placement="top">
                      CLI-sesjoner i perioden. En sesjon varer fra brukeren starter til avslutter CLI-et.
                    </HelpText>
                  </div>
                </div>
              </Box>
              <Box background="success-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(cliMetrics.requestCount)}
                  </Heading>
                  <div className="flex items-center justify-center gap-1">
                    <BodyShort className="text-gray-600">Forespørsler</BodyShort>
                    <HelpText title="CLI-forespørsler" placement="top">
                      Forespørsler sendt til Copilot via CLI. Én sesjon kan ha flere forespørsler.
                    </HelpText>
                  </div>
                </div>
              </Box>
            </HGrid>
            {cliMetrics.avgTokensPerRequest > 0 && (
              <>
                <BodyShort weight="semibold" className="text-gray-700">
                  Tokenforbruk
                </BodyShort>
                <HGrid columns={{ xs: 1, sm: 3 }} gap="space-16">
                  <Box background="warning-soft" padding="space-16" borderRadius="8">
                    <div className="text-center">
                      <Heading size="large" level="4">
                        {formatNumber(Math.round(cliMetrics.avgTokensPerRequest))}
                      </Heading>
                      <div className="flex items-center justify-center gap-1">
                        <BodyShort className="text-gray-600">Snitt tokens/forespørsel</BodyShort>
                        <HelpText title="Snitt tokens per forespørsel" placement="top">
                          Gjennomsnittlig tokens (input + output) per forespørsel. Én token ≈ 4 tegn på engelsk.
                        </HelpText>
                      </div>
                    </div>
                  </Box>
                  <Box background="neutral-moderate" padding="space-16" borderRadius="8">
                    <div className="text-center">
                      <Heading size="large" level="4">
                        {formatNumber(cliMetrics.promptTokensSum)}
                      </Heading>
                      <div className="flex items-center justify-center gap-1">
                        <BodyShort className="text-gray-600">Input-tokens totalt</BodyShort>
                        <HelpText title="Input-tokens" placement="top">
                          Prompt-tokens (input) sendt til modellen via CLI. Inkluderer spørsmål og kontekst.
                        </HelpText>
                      </div>
                    </div>
                  </Box>
                  <Box background="neutral-moderate" padding="space-16" borderRadius="8">
                    <div className="text-center">
                      <Heading size="large" level="4">
                        {formatNumber(cliMetrics.outputTokensSum)}
                      </Heading>
                      <div className="flex items-center justify-center gap-1">
                        <BodyShort className="text-gray-600">Output-tokens totalt</BodyShort>
                        <HelpText title="Output-tokens" placement="top">
                          Tokens generert av modellen som svar på CLI-forespørsler.
                        </HelpText>
                      </div>
                    </div>
                  </Box>
                </HGrid>
              </>
            )}
          </VStack>
        </Box>
      )}

      {/* Model Usage Information */}
      {modelUsageMetrics && modelUsageMetrics.length > 0 && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-16">
            <div className="flex items-center gap-2">
              <Heading size="small" level="3">
                AI-modeller i bruk
              </Heading>
              <HelpText title="AI-modeller" placement="top">
                Hvilke AI-modeller som brukes og for hvilke Copilot-funksjoner.
              </HelpText>
            </div>
            <BodyShort className="text-gray-600">AI-modeller i bruk og hvilke funksjoner de støtter.</BodyShort>

            <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
              <div className="overflow-hidden">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHeaderCell scope="col">Modell</TableHeaderCell>
                      <TableHeaderCell scope="col">
                        <div className="flex items-center gap-1">
                          Genereringer
                          <HelpText title="Genereringer" placement="top">
                            Ganger modellen genererte et kodeforslag eller svar.
                          </HelpText>
                        </div>
                      </TableHeaderCell>
                      <TableHeaderCell scope="col">
                        <div className="flex items-center gap-1">
                          Funksjoner
                          <HelpText title="Funksjoner" placement="top">
                            Copilot-funksjoner der modellen ble brukt, f.eks. kodeforslag, chat eller agent mode.
                          </HelpText>
                        </div>
                      </TableHeaderCell>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {modelUsageMetrics.map((model: ModelData) => (
                      <TableRow key={model.name}>
                        <TableDataCell>
                          <BodyShort weight="semibold">{model.name}</BodyShort>
                        </TableDataCell>
                        <TableDataCell>
                          <BodyShort>{formatNumber(model.generations)}</BodyShort>
                        </TableDataCell>
                        <TableDataCell>
                          <BodyShort className="text-sm">{model.features.join(", ")}</BodyShort>
                        </TableDataCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              <ModelUsageChart data={modelChartData} />
            </HGrid>
          </VStack>
        </Box>
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
    {
      id: "kostnad",
      label: "Kostnad",
      content: (
        <Suspense fallback={<Skeleton variant="rectangle" height={200} />}>
          <BillingTabContent />
        </Suspense>
      ),
    },
  ];

  return (
    <>
      <VStack gap="space-24">
        <BodyShort className="text-gray-600">
          Periode: {dateRange.start} - {dateRange.end} ({formatNumber(usage.length)} dager) • Viser organisasjonens bruk
          av GitHub Copilot
        </BodyShort>
        <Tabs tabs={tabs} defaultTab="overview" />
      </VStack>
    </>
  );
}

// Main page component using Partial Prerendering
export default async function Usage({ searchParams }: { searchParams: Promise<{ days?: string }> }) {
  await getUser();
  const params = await searchParams;
  const parsedDays = parseInt(params.days || "28", 10);
  const days = isNaN(parsedDays) ? 28 : parsedDays <= 0 ? 0 : Math.min(parsedDays, 365);

  return (
    <main>
      <UsageHeader />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <section>
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
      </div>
    </main>
  );
}
