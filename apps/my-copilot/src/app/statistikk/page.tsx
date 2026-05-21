import React, { Suspense } from "react";
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

import GenerationModeChart from "@/components/charts/GenerationModeChart";
import MonthlyTrendsChart from "@/components/charts/MonthlyTrendsChart";
import MetricCard from "@/components/metric-card";
import ErrorState from "@/components/error-state";
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
  return <PageHero title="Statistikk" description="Bruksdata og trender for GitHub Copilot i Nav." />;
}

// Cached data component — uses last 28 days of entity-level data
async function CachedUsageData() {
  const { usage, error } = await getCachedBigQueryUsage();

  if (error) return <ErrorState message={`Feil ved henting av bruksdata: ${error}`} />;
  if (!usage || usage.length === 0) return <ErrorState message="Ingen bruksdata tilgjengelig" />;

  const filteredUsage = usage.slice(-28);

  return <UsageContent usage={filteredUsage} />;
}

// Cached team usage data component — resolves user's teams for highlighting
async function TeamUsageContent() {
  const [{ teams, error }, user] = await Promise.all([getCachedTeamUsage(), getUser(false)]);

  if (error) return <ErrorState message={`Feil ved henting av teamdata: ${error}`} />;
  if (!teams || teams.length === 0) return <ErrorState message="Ingen teamdata tilgjengelig ennå." />;

  // Filter out the catch-all org team — it contains all users and skews comparisons
  const IGNORED_TEAMS = new Set(["nav-it-github-users"]);
  const filteredTeams = teams.filter((t) => !IGNORED_TEAMS.has(t.team_slug));

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
        userTeams = metrics.teams.filter((t) => !IGNORED_TEAMS.has(t));
        userMetrics = metrics;
      }
      if (weeklyTrends.length > 0) {
        userWeeklyTrends = weeklyTrends;
      }
    }
  }

  return (
    <TeamUsageTable
      teams={filteredTeams}
      userTeams={userTeams}
      userMetrics={userMetrics}
      userWeeklyTrends={userWeeklyTrends}
    />
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
  const modelChartData = buildModelChartData(usage);
  const generationModeTrendData = buildGenerationModeTrendData(usage);

  // Fetch monthly trends for the dashboard
  const { trends: monthlyTrends } = await getCachedMonthlyTrends();

  // Find the last COMPLETE month (not the current partial month)
  // A month is "complete" if it has 28+ days of data or isn't the current calendar month
  const now = new Date();
  const currentMonthStr = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
  const completeMonths = monthlyTrends.filter((m) => m.month !== currentMonthStr);
  const currentMonth = monthlyTrends.find((m) => m.month === currentMonthStr);

  // Use last complete month for hero, with MoM comparison to the one before
  const latestComplete = completeMonths.length > 0 ? completeMonths[completeMonths.length - 1] : null;
  const prevComplete = completeMonths.length > 1 ? completeMonths[completeMonths.length - 2] : null;

  function momChange(current: number, previous: number | undefined): string | undefined {
    if (!previous || previous === 0) return undefined;
    const change = Math.round(((current - previous) / previous) * 100);
    return change > 0
      ? `↑ ${change} % fra forrige måned`
      : change < 0
        ? `↓ ${Math.abs(change)} % fra forrige måned`
        : "Uendret";
  }

  // Agent share: what % of active users are using agent mode
  const agentShare =
    latestComplete && latestComplete.unique_users > 0
      ? Math.round((latestComplete.agent_users / latestComplete.unique_users) * 100)
      : 0;

  // Total activity: completions + chat/interactions + CLI
  const totalActivityLatest = latestComplete
    ? latestComplete.code_generations + latestComplete.ide_interactions + latestComplete.cli_requests
    : 0;
  const totalActivityPrev = prevComplete
    ? prevComplete.code_generations + prevComplete.ide_interactions + prevComplete.cli_requests
    : 0;

  // Month label for hero subtitle
  const heroMonthLabel = latestComplete
    ? new Date(latestComplete.month + "-01").toLocaleDateString("nb-NO", { month: "long", year: "numeric" })
    : undefined;

  // ─── TAB 1: DASHBOARD (Key Indicators + Trends) ───
  const dashboardContent = (
    <VStack gap="space-24">
      {/* Hero metrics — the 4 numbers that matter */}
      {heroMonthLabel && (
        <BodyShort size="small" className="text-gray-500">
          Nøkkeltall for {heroMonthLabel}
          {currentMonth && ` • Hittil i ${currentMonthStr}: ${formatNumber(currentMonth.unique_users)} brukere`}
        </BodyShort>
      )}
      <HGrid columns={{ xs: 1, sm: 2, md: 4 }} gap="space-16">
        <MetricCard
          value={formatNumber(latestComplete?.unique_users ?? aggregatedMetrics.monthlyActiveUsers)}
          label="Aktive brukere"
          helpTitle="Aktive brukere"
          helpText="Unike brukere med faktisk aktivitet (kodeforslag, chat-interaksjoner eller CLI-bruk) i siste hele måned."
          subtitle={
            latestComplete && prevComplete
              ? momChange(latestComplete.unique_users, prevComplete.unique_users)
              : undefined
          }
        />
        <MetricCard
          value={formatNumber(totalActivityLatest || aggregatedMetrics.totalInteractions)}
          label="Copilot-aktivitet"
          helpTitle="Copilot-aktivitet"
          helpText="Sum av kodeforslag generert, chat/agent-interaksjoner og CLI-forespørsler i siste hele måned."
          subtitle={totalActivityPrev ? momChange(totalActivityLatest, totalActivityPrev) : undefined}
        />
        <MetricCard
          value={`${agentShare} %`}
          label="Agent-adopsjon"
          helpTitle="Agent-adopsjon"
          helpText="Andel av aktive brukere som brukte agent mode minst én gang i siste hele måned."
          subtitle={
            prevComplete && prevComplete.unique_users > 0
              ? momChange(agentShare, Math.round((prevComplete.agent_users / prevComplete.unique_users) * 100))
              : undefined
          }
        />
        <MetricCard
          value={`${aggregatedMetrics.overallAcceptanceRate} %`}
          label="Aksepteringsrate"
          helpTitle="Aksepteringsrate"
          helpText={`Andel av kodeforslag i editoren som utviklerne aksepterer (siste ${usage.length} dager). Gode rater ligger mellom 20–40 %.`}
          subtitle={`Siste ${usage.length} dager`}
        />
      </HGrid>

      {/* Monthly trends — THE story */}
      {monthlyTrends.length > 0 && <MonthlyTrendsChart data={monthlyTrends} />}

      {/* Generation mode: user-initiated vs agent-initiated */}
      {generationModeSummary && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-16">
            <div className="flex items-center gap-2">
              <Heading size="small" level="3">
                Bruker vs. agent
              </Heading>
              <HelpText title="Genereringsmodus" placement="top">
                Fordeling mellom kode generert av brukeren (forslag, inline chat) og kode generert autonomt av agenten.
                Høyere agent-andel betyr mer autonomt AI-arbeid.
              </HelpText>
            </div>
            <HGrid columns={{ xs: 1, sm: 3 }} gap="space-16">
              <Box background="info-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(generationModeSummary.userInitiatedGenerations)}
                  </Heading>
                  <BodyShort className="text-gray-600">Bruker</BodyShort>
                </div>
              </Box>
              <Box background="accent-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(generationModeSummary.agentInitiatedGenerations)}
                  </Heading>
                  <BodyShort className="text-gray-600">Agent</BodyShort>
                </div>
              </Box>
              <Box background="success-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {generationModeSummary.agentShare} %
                  </Heading>
                  <BodyShort className="text-gray-600">Agent-andel</BodyShort>
                </div>
              </Box>
            </HGrid>
            <GenerationModeChart data={generationModeTrendData} />
          </VStack>
        </Box>
      )}

      {/* PR & Code Review */}
      {prMetrics && prMetrics.totalCreated > 0 && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-16">
            <div className="flex items-center gap-2">
              <Heading size="small" level="3">
                Pull requests og code review
              </Heading>
              <HelpText title="Pull requests" placement="top">
                PR-er der Copilot var involvert som forfatter (coding agent) eller reviewer (code review). Basert på
                siste {usage.length} dager med data.
              </HelpText>
            </div>

            {/* Copilot as author (coding agent) */}
            <BodyShort weight="semibold" size="small" className="text-gray-600">
              Copilot som forfatter
            </BodyShort>
            <HGrid columns={{ xs: 2, sm: 4 }} gap="space-16">
              <Box background="accent-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalCreatedByCopilot)}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    PR-er opprettet
                  </BodyShort>
                </div>
              </Box>
              <Box background="success-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalMergedCreatedByCopilot)}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Merget
                  </BodyShort>
                </div>
              </Box>
              <Box background="info-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatMinutes(prMetrics.medianMinutesToMergeCopilotAuthored)}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Median tid til merge
                  </BodyShort>
                </div>
              </Box>
              <Box background="warning-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {prMetrics.totalCreated > 0
                      ? `${Math.round((prMetrics.totalCreatedByCopilot / prMetrics.totalCreated) * 100)} %`
                      : "—"}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Andel av alle PR-er
                  </BodyShort>
                </div>
              </Box>
            </HGrid>

            {/* Copilot as reviewer (code review) */}
            <BodyShort weight="semibold" size="small" className="text-gray-600">
              Copilot code review
            </BodyShort>
            <HGrid columns={{ xs: 2, sm: 4 }} gap="space-16">
              <Box background="accent-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalReviewedByCopilot)}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    PR-er reviewet
                  </BodyShort>
                </div>
              </Box>
              <Box background="success-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {prMetrics.totalReviewed > 0
                      ? `${Math.round((prMetrics.totalReviewedByCopilot / prMetrics.totalReviewed) * 100)} %`
                      : "—"}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Andel av reviews
                  </BodyShort>
                </div>
              </Box>
              <Box background="info-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(prMetrics.totalCopilotSuggestions)}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Forslag gitt
                  </BodyShort>
                </div>
              </Box>
              <Box background="warning-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {prMetrics.totalCopilotSuggestions > 0
                      ? `${Math.round((prMetrics.totalCopilotAppliedSuggestions / prMetrics.totalCopilotSuggestions) * 100)} %`
                      : "—"}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Forslag brukt
                  </BodyShort>
                </div>
              </Box>
            </HGrid>

            {/* Comparison: merge time */}
            <BodyShort weight="semibold" size="small" className="text-gray-600">
              Tid til merge (median)
            </BodyShort>
            <HGrid columns={{ xs: 1, sm: 3 }} gap="space-16">
              <Box background="neutral-moderate" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatMinutes(prMetrics.medianMinutesToMerge)}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Alle PR-er
                  </BodyShort>
                </div>
              </Box>
              <Box background="accent-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatMinutes(prMetrics.medianMinutesToMergeCopilotAuthored)}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Copilot-forfattet
                  </BodyShort>
                </div>
              </Box>
              <Box background="success-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatMinutes(prMetrics.medianMinutesToMergeCopilotReviewed)}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Copilot-reviewet
                  </BodyShort>
                </div>
              </Box>
            </HGrid>
          </VStack>
        </Box>
      )}
    </VStack>
  );

  // ─── TAB 2: DETALJER (Deep dives) ───
  const detailsContent = (
    <VStack gap="space-24">
      {/* Daily Activity Trend */}
      <TrendChart data={trendData} />

      {/* Code Suggestions */}
      <Box background="neutral-soft" padding="space-24" borderRadius="12">
        <VStack gap="space-16">
          <Heading size="small" level="3">
            Kodeforslag
          </Heading>
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

      {/* Languages & Editors */}
      <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-12">
            <Heading size="small" level="3">
              Topp-språk
            </Heading>
            <Table size="small">
              <TableBody>
                {topLanguages.slice(0, 8).map((lang: LanguageData, i: number) => (
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
              Verktøy
            </Heading>
            <Table size="small">
              <TableBody>
                {editorStats.map((editor: EditorData, i: number) => (
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
    </VStack>
  );

  const tabs = [
    { id: "dashboard", label: "Oversikt", content: dashboardContent },
    {
      id: "team",
      label: "Meg og team",
      content: (
        <Suspense fallback={<Skeleton variant="rectangle" height={200} />}>
          <TeamUsageContent />
        </Suspense>
      ),
    },
    { id: "details", label: "Utforsking", content: detailsContent },
  ];

  return (
    <>
      <VStack gap="space-24">
        <BodyShort className="text-gray-600">
          Periode: {dateRange.start} - {dateRange.end} ({formatNumber(usage.length)} dager)
        </BodyShort>
        <Tabs tabs={tabs} defaultTab="dashboard" />
      </VStack>
    </>
  );
}

// Main page component using Partial Prerendering
export default async function Usage() {
  await getUser();

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
              <CachedUsageData />
            </Suspense>
          </section>
        </Box>
      </div>
    </main>
  );
}
