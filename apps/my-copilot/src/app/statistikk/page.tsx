import React, { Suspense } from "react";
import {
  getCopilotUsageMetrics,
  getTeamUsage,
  getUserMetrics,
  getMonthlyTrends,
  getMonthlyModelUsage,
  getMonthlyBillingUsage,
  getUserWeeklyTrends,
  getAdoptionCohorts,
} from "@/lib/cached-bigquery";
import type { EnterpriseMetrics } from "@/lib/types";
import Tabs from "@/components/tabs";
import TeamUsageTable from "@/components/team-usage-table";
import TrendChart from "@/components/charts/TrendChart";
import ModelUsageChart from "@/components/charts/ModelUsageChart";

import GenerationModeChart from "@/components/charts/GenerationModeChart";
import MonthlyTrendsChart from "@/components/charts/MonthlyTrendsChart";
import MonthlyModelChart from "@/components/charts/MonthlyModelChart";
import AdoptionCohortsChart from "@/components/charts/AdoptionCohortsChart";
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
import { getUser, getUserToken } from "@/lib/auth";
import { backendRequest, BackendApiError } from "@/lib/backend-api";

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
async function CachedUsageData({ token }: { token: string }) {
  const { usage, error } = await getCopilotUsageMetrics(token);

  if (error) return <ErrorState message={`Feil ved henting av bruksdata: ${error}`} />;
  if (!usage || usage.length === 0) return <ErrorState message="Ingen bruksdata tilgjengelig" />;

  const filteredUsage = usage.slice(-28);

  return <UsageContent usage={filteredUsage} token={token} />;
}

// Whether to allow viewing all teams (disabled in prod until approved)
const ALLOW_ALL_TEAMS = process.env.NODE_ENV === "development";

// Cached team usage data component — resolves user's teams for highlighting
async function TeamUsageContent({ token }: { token: string }) {
  const [{ teams, error }, user] = await Promise.all([getTeamUsage(token), getUser()]);

  if (error) return <ErrorState message={`Feil ved henting av teamdata: ${error}`} />;
  if (!teams || teams.length === 0) return <ErrorState message="Ingen teamdata tilgjengelig ennå." />;

  // Filter out the catch-all org team — it contains all users and skews comparisons
  const IGNORED_TEAMS = new Set(["nav-it-github-users"]);

  // Resolve user's GitHub username and fetch personal metrics from BigQuery
  let userTeams: string[] = [];
  let userMetrics = null;
  let userWeeklyTrends = null;
  if (user?.email) {
    let ghLogin: string | null = null;

    // DEV_GITHUB_LOGIN bypasses SAML lookup when GitHub App auth is broken locally
    if (process.env.NODE_ENV === "development" && process.env.DEV_GITHUB_LOGIN) {
      ghLogin = process.env.DEV_GITHUB_LOGIN;
    } else {
      try {
        const saml = await backendRequest<{ identity: string; username: string | null }>(
          `/api/v1/copilot/saml/${encodeURIComponent(user.email)}`,
          token
        );
        ghLogin = saml.username;
        // NOTE: SCIM-based username lookup was removed during the backend migration
        // (the previous BFF called getUsernameByScim as a fallback for users who
        // appear in SCIM but not in SAML). That endpoint no longer exists in the
        // backend. Users in SCIM-only will resolve ghLogin=null here and therefore
        // not see their personal metrics tab. This is a known gap pending product
        // confirmation — see issue backlog.
      } catch (err) {
        console.error("[statistikk] SAML lookup failed:", err);
      }
    }

    if (ghLogin) {
      const [{ metrics, error: metricsError }, { trends: weeklyTrends, error: trendsError }] = await Promise.all([
        getUserMetrics(ghLogin, token),
        getUserWeeklyTrends(ghLogin, token),
      ]);
      if (metricsError) console.error("[statistikk] User metrics failed:", metricsError);
      if (trendsError) console.error("[statistikk] User weekly trends failed:", trendsError);
      if (metrics) {
        userTeams = (metrics.teams ?? []).filter((t) => !IGNORED_TEAMS.has(t));
        userMetrics = metrics;
      }
      if (weeklyTrends.length > 0) {
        userWeeklyTrends = weeklyTrends;
      }
    }
  }

  // Server-side access control: only send teams the user belongs to.
  // In prod, we restrict to user's own teams to prevent cross-team data exposure.
  // In dev, all teams are available for debugging.
  const userTeamSet = new Set(userTeams.map((t) => t.toLowerCase()));
  const visibleTeams = teams
    .filter((t) => !IGNORED_TEAMS.has(t.team_slug))
    .filter((t) => ALLOW_ALL_TEAMS || userTeamSet.has(t.team_slug.toLowerCase()));

  return (
    <TeamUsageTable
      teams={visibleTeams}
      userTeams={userTeams}
      userMetrics={userMetrics}
      userWeeklyTrends={userWeeklyTrends}
      allowAllTeams={ALLOW_ALL_TEAMS}
    />
  );
}

// Main content component that takes usage data as props.
// Individual data fetches are NOT cached at the BFF layer — caching is owned
// by copilot-api (1 h in-memory cache). Each request fetches fresh data from
// the backend, which is the single source of truth for cache lifetime.
async function UsageContent({ usage, token }: { usage: EnterpriseMetrics[]; token: string }) {
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

  // Fetch monthly trends and model usage for the dashboard
  const [
    { trends: monthlyTrends, error: monthlyError },
    { usage: monthlyModelUsage, error: modelUsageError },
    { usage: billingUsage, error: billingError },
    { cohorts: adoptionCohorts, error: cohortsError },
    globalBudget,
  ] = await Promise.all([
    getMonthlyTrends(token),
    getMonthlyModelUsage(token),
    getMonthlyBillingUsage(token),
    getAdoptionCohorts(token),
    backendRequest<{ totalConsumed: number; perUserBudget: number; activeUsers: number }>(
      "/api/v1/copilot/budget/global",
      token
    ).catch((err) => {
      if (!(err instanceof BackendApiError && err.status === 404)) {
        console.error("[statistikk] Global budget fetch failed:", err);
      }
      return null;
    }),
  ]);
  if (monthlyError) {
    console.error("[statistikk] Monthly trends failed:", monthlyError);
  }
  if (modelUsageError) {
    console.error("[statistikk] Monthly model usage failed:", modelUsageError);
  }
  if (billingError) {
    console.error("[statistikk] Monthly billing usage failed:", billingError);
  }
  if (cohortsError) {
    console.error("[statistikk] Adoption cohorts failed:", cohortsError);
  }

  // Find the last COMPLETE month (not the current partial month)
  // A month is "complete" if it has 28+ days of data or isn't the current calendar month
  // Use the latest month in the data as reference to avoid new Date() prerender issues
  const latestMonth = monthlyTrends.length > 0 ? monthlyTrends[monthlyTrends.length - 1].month : null;
  const completeMonths = latestMonth
    ? monthlyTrends.filter((m) => m.month !== latestMonth || m.days_in_month >= 28)
    : monthlyTrends;
  const currentMonth = latestMonth ? monthlyTrends.find((m) => m.month === latestMonth && m.days_in_month < 28) : null;

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
          {currentMonth && ` • Hittil i ${currentMonth.month}: ${formatNumber(currentMonth.unique_users)} brukere`}
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

      {/* Global AI credit budget */}
      {globalBudget && (
        <HGrid columns={{ xs: 1, sm: 2, md: 4 }} gap="space-16">
          <MetricCard
            value={`${formatNumber(Math.round(globalBudget.totalConsumed))} USD`}
            label="Totalt AI-kreditforbruk"
            helpTitle="Totalt AI-kreditforbruk"
            helpText="Sum av AI-kreditforbruk for alle Nav-utviklere denne måneden. Inkluderer brukere med aktivt forbruk i GitHub Copilot."
            subtitle={`${globalBudget.activeUsers} aktive brukere · ${formatNumber(globalBudget.perUserBudget)} USD per bruker`}
          />
        </HGrid>
      )}

      {/* Monthly trends — THE story */}
      {monthlyTrends.length > 0 && <MonthlyTrendsChart data={monthlyTrends} />}

      {/* Monthly model/token usage */}
      {monthlyModelUsage.length > 0 && <MonthlyModelChart data={monthlyModelUsage} billingData={billingUsage} />}

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

      {/* AI Adoption Cohorts */}
      {adoptionCohorts.length > 0 && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-16">
            <div className="flex items-center gap-2">
              <Heading size="small" level="3">
                AI-adopsjonsfaser
              </Heading>
              <HelpText title="Om adopsjonsfasene" placement="top">
                GitHub klassifiserer brukere i faser basert på AI-aktivitet de siste 28 dagene. Grafen viser utviklingen
                over tid.
              </HelpText>
            </div>
            <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm md:grid-cols-4">
              <div>
                <dt className="font-medium text-gray-600">Fase 0</dt>
                <dd>Ingen AI-bruk siste 28 dager</dd>
              </div>
              <div>
                <dt className="font-medium text-gray-600">Fase 1</dt>
                <dd>Bruker kodeforslag (inline completions)</dd>
              </div>
              <div>
                <dt className="font-medium text-gray-600">Fase 2</dt>
                <dd>Bruker AI-agent i ett verktøy (f.eks. bare Chat)</dd>
              </div>
              <div>
                <dt className="font-medium text-gray-600">Fase 3</dt>
                <dd>Bruker AI-agent i flere verktøy (f.eks. Chat + CLI)</dd>
              </div>
            </dl>
            <AdoptionCohortsChart data={adoptionCohorts} />
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
          <TeamUsageContent token={token} />
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
  const token = await getUserToken();

  if (!token) {
    return <ErrorState message="Mangler innloggingstoken" />;
  }

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
              <CachedUsageData token={token} />
            </Suspense>
          </section>
        </Box>
      </div>
    </main>
  );
}
