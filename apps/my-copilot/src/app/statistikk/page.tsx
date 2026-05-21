import React, { Suspense } from "react";
import { getCachedPremiumRequestUsage } from "@/lib/cached-github";
import {
  getCachedBigQueryUsage,
  getCachedTeamUsage,
  getCachedUserMetrics,
  getCachedMonthlyTrends,
  getCachedUserWeeklyTrends,
} from "@/lib/cached-bigquery";
import type { EnterpriseMetrics } from "@/lib/types";
import Tabs from "@/components/tabs";
import TeamUsageTable from "@/components/team-usage-table";
import TrendChart from "@/components/charts/TrendChart";
import ModelUsageChart from "@/components/charts/ModelUsageChart";
import AdoptionTrendChart from "@/components/charts/AdoptionTrendChart";
import GenerationModeChart from "@/components/charts/GenerationModeChart";
import MonthlyTrendsChart from "@/components/charts/MonthlyTrendsChart";
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
  getPRMetrics,
  buildTrendData,
  buildAdoptionTrendData,
  buildModelChartData,
  getGenerationModeSummary,
  buildGenerationModeTrendData,
} from "@/lib/data-utils";
import type { LanguageData, EditorData, ModelData } from "@/lib/types";
import { formatNumber } from "@/lib/format";
import { getUser } from "@/lib/auth";
import { getUsernameByScim } from "@/lib/github";

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

// Cached premium data component — tries current month, falls back to previous month
async function PremiumUsageData({ currentYear, currentMonth }: { currentYear: number; currentMonth: number }) {
  "use cache";
  const { cacheLife, cacheTag } = await import("next/cache");
  cacheLife({ stale: 300 });
  cacheTag("premium-usage-navikt");

  const { usage: premiumUsage } = await getCachedPremiumRequestUsage("navikt", currentYear, currentMonth);

  if (premiumUsage?.usageItems?.length) {
    return <PremiumRequestsContent metrics={calculatePremiumMetrics(premiumUsage)} />;
  }

  // Fallback to previous month if current month has no data yet
  const prevMonth = currentMonth === 1 ? 12 : currentMonth - 1;
  const prevYear = currentMonth === 1 ? currentYear - 1 : currentYear;
  const { usage: prevUsage } = await getCachedPremiumRequestUsage("navikt", prevYear, prevMonth);

  if (prevUsage?.usageItems?.length) {
    return (
      <>
        <BodyShort className="text-gray-500 mb-2">
          Viser forrige måned — data for denne måneden er ikke tilgjengelig ennå.
        </BodyShort>
        <PremiumRequestsContent metrics={calculatePremiumMetrics(prevUsage)} />
      </>
    );
  }

  return <BodyShort className="text-gray-500">Ingen data om premiumforespørsler tilgjengelig.</BodyShort>;
}

// Cached team usage data component — resolves user's teams for highlighting
async function TeamUsageContent() {
  const [{ teams, error }, user, { trends: monthlyTrends }] = await Promise.all([
    getCachedTeamUsage(),
    getUser(false),
    getCachedMonthlyTrends(),
  ]);

  if (error) return <ErrorState message={`Feil ved henting av teamdata: ${error}`} />;
  if (!teams || teams.length === 0) return <ErrorState message="Ingen teamdata tilgjengelig ennå." />;

  // Resolve user's GitHub username via SCIM and fetch personal metrics from BigQuery
  let userTeams: string[] = [];
  let userMetrics = null;
  let userWeeklyTrends = null;
  if (user?.email) {
    let ghLogin: string | null = null;

    // DEV_GITHUB_LOGIN bypasses SCIM when GitHub App auth is broken locally
    if (process.env.DEV_GITHUB_LOGIN) {
      ghLogin = process.env.DEV_GITHUB_LOGIN;
    } else {
      const { user: resolved } = await getUsernameByScim(user.email);
      ghLogin = resolved;
    }

    if (ghLogin) {
      const [{ metrics }, { trends: weeklyTrends }] = await Promise.all([
        getCachedUserMetrics(ghLogin),
        getCachedUserWeeklyTrends(ghLogin),
      ]);
      if (metrics) {
        userTeams = metrics.teams;
        userMetrics = metrics;
      }
      if (weeklyTrends.length > 0) {
        userWeeklyTrends = weeklyTrends;
      }
    }
  }

  return (
    <VStack gap="space-24">
      <TeamUsageTable
        teams={teams}
        userTeams={userTeams}
        userMetrics={userMetrics}
        userWeeklyTrends={userWeeklyTrends}
      />
      {monthlyTrends.length > 0 && <MonthlyTrendsChart data={monthlyTrends} />}
    </VStack>
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
  const prMetrics = getPRMetrics(usage);
  const modelUsageMetrics = getModelUsageMetrics(usage);
  const generationModeSummary = getGenerationModeSummary(usage);

  const trendData = buildTrendData(usage);
  const adoptionTrendData = buildAdoptionTrendData(usage);
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

      {/* 3. Daily Activity Trend */}
      <TrendChart data={trendData} />

      {/* 4. Compact Top Languages & Editors */}
      <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-12">
            <Heading size="small" level="3">
              Topp-språk
            </Heading>
            <Table size="small">
              <TableBody>
                {topLanguages.slice(0, 5).map((lang: LanguageData, i: number) => (
                  <TableRow key={lang.name}>
                    <TableDataCell className="w-8">
                      <BodyShort className="text-gray-500">{i + 1}.</BodyShort>
                    </TableDataCell>
                    <TableDataCell>
                      <BodyShort weight="semibold" className="capitalize">
                        {lang.name}
                      </BodyShort>
                    </TableDataCell>
                    <TableDataCell align="right">
                      <BodyShort className="text-gray-600">{lang.acceptanceRate}%</BodyShort>
                    </TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </VStack>
        </Box>
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-12">
            <Heading size="small" level="3">
              Topp-verktøy
            </Heading>
            <Table size="small">
              <TableBody>
                {editorStats.slice(0, 4).map((editor: EditorData, i: number) => (
                  <TableRow key={editor.name}>
                    <TableDataCell className="w-8">
                      <BodyShort className="text-gray-500">{i + 1}.</BodyShort>
                    </TableDataCell>
                    <TableDataCell>
                      <BodyShort weight="semibold">{editor.name}</BodyShort>
                    </TableDataCell>
                    <TableDataCell align="right">
                      <BodyShort className="text-gray-600">{formatNumber(editor.generations)}</BodyShort>
                    </TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </VStack>
        </Box>
      </HGrid>
    </VStack>
  );

  const detailsContent = (
    <VStack gap="space-24">
      {/* Code Suggestions */}
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
                <BodyShort className="text-gray-600">Genererte forslag</BodyShort>
              </div>
            </Box>
            <Box background="success-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {formatNumber(aggregatedMetrics.totalAcceptances)}
                </Heading>
                <BodyShort className="text-gray-600">Aksepterte forslag</BodyShort>
              </div>
            </Box>
            <Box background="accent-soft" padding="space-16" borderRadius="8">
              <div className="text-center">
                <Heading size="large" level="4">
                  {aggregatedMetrics.overallAcceptanceRate}%
                </Heading>
                <BodyShort className="text-gray-600">Aksepteringsrate</BodyShort>
              </div>
            </Box>
          </HGrid>
        </VStack>
      </Box>

      {/* Generation Mode Split */}
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
            <HGrid columns={{ xs: 1, sm: 3 }} gap="space-16">
              <Box background="info-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(generationModeSummary.userInitiatedGenerations)}
                  </Heading>
                  <BodyShort className="text-gray-600">Brukerinitiiert</BodyShort>
                </div>
              </Box>
              <Box background="accent-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(generationModeSummary.agentInitiatedGenerations)}
                  </Heading>
                  <BodyShort className="text-gray-600">Agentinitiiert</BodyShort>
                </div>
              </Box>
              <Box background="success-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {generationModeSummary.agentShare}%
                  </Heading>
                  <BodyShort className="text-gray-600">Agent-andel</BodyShort>
                </div>
              </Box>
            </HGrid>
            <GenerationModeChart data={generationModeTrendData} />
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

            <HGrid columns={{ xs: 1, sm: 2, md: 4 }} gap="space-16">
              <Box background="accent-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalCreated)}
                  </Heading>
                  <BodyShort className="text-gray-600">Totalt opprettet</BodyShort>
                </div>
              </Box>
              <Box background="success-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalCreatedByCopilot)}
                  </Heading>
                  <BodyShort className="text-gray-600">Opprettet av Copilot</BodyShort>
                </div>
              </Box>
              <Box background="info-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalMerged)}
                  </Heading>
                  <BodyShort className="text-gray-600">Merget</BodyShort>
                </div>
              </Box>
              <Box background="warning-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalReviewedByCopilot)}
                  </Heading>
                  <BodyShort className="text-gray-600">Reviewed av Copilot</BodyShort>
                </div>
              </Box>
            </HGrid>

            {(prMetrics.medianMinutesToMerge > 0 || prMetrics.medianMinutesToMergeCopilotAuthored > 0) && (
              <HGrid columns={{ xs: 1, sm: 3 }} gap="space-16">
                <Box background="neutral-moderate" padding="space-16" borderRadius="8">
                  <div className="text-center">
                    <Heading size="large" level="4">
                      {formatMinutes(prMetrics.medianMinutesToMerge)}
                    </Heading>
                    <BodyShort className="text-gray-600">Median tid til merge</BodyShort>
                  </div>
                </Box>
                <Box background="success-soft" padding="space-16" borderRadius="8">
                  <div className="text-center">
                    <Heading size="large" level="4">
                      {formatMinutes(prMetrics.medianMinutesToMergeCopilotAuthored)}
                    </Heading>
                    <BodyShort className="text-gray-600">Median tid (Copilot-PR)</BodyShort>
                  </div>
                </Box>
                <Box background="info-soft" padding="space-16" borderRadius="8">
                  <div className="text-center">
                    <Heading size="large" level="4">
                      {formatMinutes(prMetrics.medianMinutesToMergeCopilotReviewed)}
                    </Heading>
                    <BodyShort className="text-gray-600">Median tid (Copilot-review)</BodyShort>
                  </div>
                </Box>
              </HGrid>
            )}
          </VStack>
        </Box>
      )}

      {/* Model Usage */}
      {modelUsageMetrics && modelUsageMetrics.length > 0 && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-16">
            <Heading size="small" level="3">
              AI-modeller i bruk
            </Heading>
            <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
              <div className="overflow-hidden">
                <Table size="small">
                  <TableHeader>
                    <TableRow>
                      <TableHeaderCell scope="col">Modell</TableHeaderCell>
                      <TableHeaderCell scope="col">Genereringer</TableHeaderCell>
                      <TableHeaderCell scope="col">Funksjoner</TableHeaderCell>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {modelUsageMetrics.map((model: ModelData) => (
                      <TableRow key={model.name}>
                        <TableDataCell>
                          <BodyShort weight="semibold">{model.name}</BodyShort>
                        </TableDataCell>
                        <TableDataCell>{formatNumber(model.generations)}</TableDataCell>
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

      {/* Premium Requests */}
      <Box background="neutral-soft" padding="space-24" borderRadius="12">
        <VStack gap="space-16">
          <Heading size="small" level="3">
            Premiumforespørsler
          </Heading>
          <Suspense fallback={<Skeleton variant="rectangle" height={100} />}>
            <PremiumUsageData currentYear={new Date().getFullYear()} currentMonth={new Date().getMonth() + 1} />
          </Suspense>
        </VStack>
      </Box>
    </VStack>
  );

  const tabs = [
    { id: "overview", label: "Oversikt", content: overviewContent },
    {
      id: "team",
      label: "Teamoversikt",
      content: (
        <Suspense fallback={<Skeleton variant="rectangle" height={200} />}>
          <TeamUsageContent />
        </Suspense>
      ),
    },
    { id: "details", label: "Detaljer", content: detailsContent },
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
