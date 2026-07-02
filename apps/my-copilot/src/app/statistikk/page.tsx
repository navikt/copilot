// Force dynamic rendering is no longer needed since cacheComponents (Dynamic IO) is enabled.

import React, { Suspense } from "react";
import {
  getCopilotUsageMetrics,
  getTeamUsage,
  getUserMetrics,
  getMonthlyTrends,
  getMonthlyBillingUsage,
  getBillingModelDaily,
  getBillingModelForecast,
  getUserWeeklyTrends,
  getAdoptionCohorts,
  getBillingMonthlyTrend,
  getBillingModelBreakdown,
  getDailySummary,
  getUsageDistribution,
} from "@/lib/cached-bigquery";
import type { EnterpriseMetrics } from "@/lib/types";
import Tabs from "@/components/tabs";
import TeamUsageTable from "@/components/team-usage-table";
import TrendChart from "@/components/charts/TrendChart";
import ModelUsageChart from "@/components/charts/ModelUsageChart";

import GenerationModeChart from "@/components/charts/GenerationModeChart";
import MonthlyTrendsChart from "@/components/charts/MonthlyTrendsChart";
import BillingMonthNowChart from "@/components/charts/BillingMonthNowChart";
import AdoptionCohortsChart from "@/components/charts/AdoptionCohortsChart";
import BillingModelBreakdownChart from "@/components/charts/BillingModelBreakdownChart";
import UsageDistributionChart from "@/components/charts/UsageDistributionChart";
import MetricCard from "@/components/metric-card";
import ErrorState from "@/components/error-state";
import { Table, BodyShort, Heading, HGrid, Box, HelpText, Skeleton, VStack, HStack } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { PageHero } from "@/components/page-hero";
import { LinkableHeading } from "@/components/linkable-heading";
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
import { backendRequest } from "@/lib/backend-api";

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
  const [{ teams, error }, user, { distribution: usageDistribution, error: distributionError }] = await Promise.all([
    getTeamUsage(token),
    getUser(),
    getUsageDistribution(token),
  ]);

  if (distributionError) {
    console.error("[statistikk] Usage distribution failed:", distributionError);
  }

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
    <VStack gap="space-24">
      <Box background="neutral-soft" padding="space-24" borderRadius="12">
        <UsageDistributionChart distribution={usageDistribution} />
      </Box>
      <TeamUsageTable
        teams={visibleTeams}
        userTeams={userTeams}
        userMetrics={userMetrics}
        userWeeklyTrends={userWeeklyTrends}
        allowAllTeams={ALLOW_ALL_TEAMS}
      />
    </VStack>
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

  // Prefer current month for "Måned hittil"; fall back to latest complete month if needed.
  const currentBillingMonth = new Date().toISOString().slice(0, 7);

  // Fetch all datasets in parallel — daily/forecast for current month included upfront
  const [
    { trends: monthlyTrends, error: monthlyError },
    { usage: billingUsage, error: billingError },
    { cohorts: adoptionCohorts, error: cohortsError },
    { trend: billingMonthlyTrend, error: billingTrendError },
    { breakdown: billingModelBreakdown, error: billingBreakdownError },
    { summary: dailySummary, error: dailySummaryError },
    { usage: billingModelDailyInit, error: billingModelDailyInitError },
    { forecast: billingModelForecastInit, error: billingModelForecastInitError },
  ] = await Promise.all([
    getMonthlyTrends(token),
    getMonthlyBillingUsage(token),
    getAdoptionCohorts(token),
    getBillingMonthlyTrend(token),
    getBillingModelBreakdown(token),
    getDailySummary(token),
    getBillingModelDaily(token, currentBillingMonth),
    getBillingModelForecast(token, currentBillingMonth),
  ]);
  if (monthlyError) {
    console.error("[statistikk] Monthly trends failed:", monthlyError);
  }
  if (billingError) {
    console.error("[statistikk] Monthly billing usage failed:", billingError);
  }
  if (cohortsError) {
    console.error("[statistikk] Adoption cohorts failed:", cohortsError);
  }
  if (billingTrendError) {
    console.error("[statistikk] Billing monthly trend failed:", billingTrendError);
  }
  if (billingBreakdownError) {
    console.error("[statistikk] Billing model breakdown failed:", billingBreakdownError);
  }
  if (dailySummaryError) {
    console.error("[statistikk] Daily summary failed:", dailySummaryError);
  }
  // Latest complete billing month summary
  const latestBillingMonth = (() => {
    if (!billingUsage || billingUsage.length === 0) return null;
    const months = [...new Set(billingUsage.map((r) => r.month))].sort();
    const latest = months[months.length - 1];
    const rows = billingUsage.filter((r) => r.month === latest);
    const netAmount = rows.reduce((sum, r) => sum + r.net_amount, 0);
    const grossAmount = rows.reduce((sum, r) => sum + r.gross_amount, 0);
    const label = new Date(latest + "-01").toLocaleDateString("nb-NO", { month: "long", year: "numeric" });
    return { month: latest, label, netAmount, grossAmount };
  })();

  // Daily/forecast already fetched in parallel above; fall back to latest complete month if current has no data
  let billingModelDaily = billingModelDailyInit;
  let billingModelForecast = billingModelForecastInit;
  let billingModelDailyError = billingModelDailyInitError;
  let billingModelForecastError = billingModelForecastInitError;

  if (billingModelDaily.length === 0 && latestBillingMonth?.month && latestBillingMonth.month !== currentBillingMonth) {
    const [fallbackDaily, fallbackForecast] = await Promise.all([
      getBillingModelDaily(token, latestBillingMonth.month),
      getBillingModelForecast(token, latestBillingMonth.month),
    ]);
    billingModelDaily = fallbackDaily.usage;
    billingModelForecast = fallbackForecast.forecast;
    billingModelDailyError = billingModelDailyError ?? fallbackDaily.error;
    billingModelForecastError = billingModelForecastError ?? fallbackForecast.error;
  }

  if (billingModelDailyError) console.error("[statistikk] Billing model daily failed:", billingModelDailyError);
  if (billingModelForecastError)
    console.error("[statistikk] Billing model forecast failed:", billingModelForecastError);

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
    <VStack id="dashboard" gap="space-24">
      {/* Hero metrics — the 4 numbers that matter */}
      {heroMonthLabel && (
        <BodyShort size="small" className="text-gray-500">
          Nøkkeltall for {heroMonthLabel}
          {currentMonth && ` • Hittil i ${currentMonth.month}: ${formatNumber(currentMonth.unique_users)} brukere`}
        </BodyShort>
      )}
      <HGrid id="nokkeltall" columns={{ xs: 1, sm: 2, md: 4 }} gap="space-16">
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

      {/* Daily snapshot from v_daily_summary — CLI and PR metrics */}
      {dailySummary && (
        <div id="daglig-oversikt">
          <BodyShort size="small" className="text-gray-500 mb-2">
            Daglig snapshot{" "}
            {new Date(dailySummary.date).toLocaleDateString("nb-NO", {
              day: "numeric",
              month: "long",
              year: "numeric",
            })}
          </BodyShort>
          <HGrid columns={{ xs: 2, sm: 4 }} gap="space-16">
            <MetricCard
              value={formatNumber(dailySummary.daily_active_cli_users)}
              label="CLI-brukere (i dag)"
              helpTitle="CLI-brukere"
              helpText="Unike brukere med Copilot CLI-aktivitet i dag. Kilde: v_daily_summary."
            />
            <MetricCard
              value={formatNumber(dailySummary.pr_reviewed_by_copilot)}
              label="PR-er gjennomgått av Copilot"
              helpTitle="PR-er gjennomgått av Copilot"
              helpText="Antall pull requests gjennomgått av GitHub Copilot i dag. Kilde: v_daily_summary."
            />
            <MetricCard
              value={formatNumber(dailySummary.pr_created_by_copilot)}
              label="PR-er opprettet av Copilot"
              helpTitle="PR-er opprettet av Copilot"
              helpText="Antall pull requests opprettet av GitHub Copilot i dag. Kilde: v_daily_summary."
            />
            <MetricCard
              value={formatNumber(dailySummary.monthly_active_chat_users)}
              label="Chat-brukere (måned)"
              helpTitle="Chat-brukere"
              helpText="Unike brukere som har brukt Copilot Chat i inneværende måned. Kilde: v_daily_summary."
            />
          </HGrid>
        </div>
      )}

      {/* Latest billing month summary — replaced by richer view data when available */}
      {billingMonthlyTrend.length > 0 ? (
        (() => {
          const latest = billingMonthlyTrend[billingMonthlyTrend.length - 1];
          const label = new Date(latest.year_month + "-01").toLocaleDateString("nb-NO", {
            month: "long",
            year: "numeric",
          });
          return (
            <Box background="default" padding="space-16" borderRadius="8" className="border border-gray-200">
              <HStack gap="space-8" align="center" justify="space-between">
                <HStack gap="space-8" align="center">
                  <BodyShort className="text-gray-600 text-sm">Fakturert {label}</BodyShort>
                  <HelpText title="Fakturert beløp">
                    Faktisk fakturert beløp fra GitHub for premium AI-modellforespørsler (etter Nav-rabatt). Kilde:{" "}
                    v_billing_monthly_trend.
                  </HelpText>
                </HStack>
                <HStack gap="space-16" align="center">
                  <BodyShort className="text-gray-500 text-sm">
                    Brutto: {formatNumber(Math.round(latest.total_gross_amount))} USD
                  </BodyShort>
                  <BodyShort className="text-gray-500 text-sm">
                    Rabatt: {Math.round(latest.discount_rate_pct)} %
                  </BodyShort>
                  <BodyShort className="text-gray-800 text-sm font-semibold">
                    Netto: {formatNumber(Math.round(latest.total_net_amount))} USD
                  </BodyShort>
                  <BodyShort className="text-gray-500 text-sm">{latest.distinct_models} modeller</BodyShort>
                </HStack>
              </HStack>
            </Box>
          );
        })()
      ) : latestBillingMonth ? (
        <Box background="default" padding="space-16" borderRadius="8" className="border border-gray-200">
          <HStack gap="space-8" align="center" justify="space-between">
            <HStack gap="space-8" align="center">
              <BodyShort className="text-gray-600 text-sm">Fakturert {latestBillingMonth.label}</BodyShort>
              <HelpText title="Fakturert beløp">
                Faktisk fakturert beløp fra GitHub for premium AI-modellforespørsler (etter Nav-rabatt). Kilde: GitHub
                Enhanced Billing API.
              </HelpText>
            </HStack>
            <HStack gap="space-16" align="center">
              <BodyShort className="text-gray-500 text-sm">
                Brutto: {formatNumber(Math.round(latestBillingMonth.grossAmount))} USD
              </BodyShort>
              <BodyShort className="text-gray-800 text-sm font-semibold">
                Netto: {formatNumber(Math.round(latestBillingMonth.netAmount))} USD
              </BodyShort>
            </HStack>
          </HStack>
        </Box>
      ) : null}

      {/* Monthly trends — THE story */}
      {monthlyTrends.length > 0 && (
        <div id="manedlige-trender">
          <MonthlyTrendsChart data={monthlyTrends} />
        </div>
      )}

      {/* Current month model cost + forecast */}
      {Array.isArray(billingModelDaily) && billingModelDaily.length > 0 && billingModelForecast ? (
        <BillingMonthNowChart dailyData={billingModelDaily} forecast={billingModelForecast} />
      ) : (
        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <BodyShort size="small" className="text-gray-600">
            Måned hittil-grafene er ikke tilgjengelige ennå.
            {billingModelDailyError || billingModelForecastError
              ? " Nye API-endepunkter eller data er ikke tilgjengelige i dette miljøet ennå."
              : " Daglige modellkost-data for inneværende måned er ikke ingestert ennå."}
          </BodyShort>
        </Box>
      )}

      {billingModelBreakdown.length > 0 && (
        <BillingModelBreakdownChart
          breakdown={billingModelBreakdown}
          trend={billingMonthlyTrend}
          dailyData={billingModelDaily ?? undefined}
          forecast={billingModelForecast}
        />
      )}

      {/* Generation mode: user-initiated vs agent-initiated */}
      {generationModeSummary && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-16">
            <div className="flex items-center gap-2">
              <LinkableHeading size="small" level="3">
                Bruker vs. agent
              </LinkableHeading>
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
              <LinkableHeading size="small" level="3">
                AI-adopsjonsfaser
              </LinkableHeading>
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
              <LinkableHeading size="small" level="3">
                Pull requests og code review
              </LinkableHeading>
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
    <VStack id="details" gap="space-24">
      {/* Daily Activity Trend */}
      <div id="daglig-aktivitet">
        <TrendChart data={trendData} />
      </div>

      {/* Code Suggestions */}
      <Box id="kodeforslag" background="neutral-soft" padding="space-24" borderRadius="12">
        <VStack gap="space-16">
          <LinkableHeading size="small" level="3">
            Kodeforslag
          </LinkableHeading>
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
      <HGrid id="sprak-og-verktoy" columns={{ xs: 1, md: 2 }} gap="space-24">
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-12">
            <LinkableHeading size="small" level="3">
              Topp-språk
            </LinkableHeading>
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
            <LinkableHeading size="small" level="3">
              Verktøy
            </LinkableHeading>
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
        <Box id="ai-modeller-i-bruk" background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-16">
            <LinkableHeading size="small" level="3">
              AI-modeller i bruk
            </LinkableHeading>
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
    {
      id: "dashboard",
      label: "Oversikt222",
      content: dashboardContent,
      hashIds: [
        "dashboard",
        "nokkeltall",
        "månedlige-trender",
        "måned-hittil-modeller-og-kostnad",
        "ai-modeller-over-tid",
        "bruker-vs-agent",
        "ai-adopsjonsfaser",
        "pull-requests-og-code-review",
      ],
    },
    {
      id: "team",
      label: "Meg og team",
      content: (
        <div id="team">
          <div id="meg-og-team">
            <Suspense fallback={<Skeleton variant="rectangle" height={200} />}>
              <TeamUsageContent token={token} />
            </Suspense>
          </div>
        </div>
      ),
      hashIds: ["team", "meg-og-team"],
    },
    {
      id: "details",
      label: "Utforsking",
      content: detailsContent,
      hashIds: [
        "details",
        "daglig-aktivitet",
        "kodeforslag",
        "sprak-og-verktoy",
        "ai-modeller-i-bruk",
        "topp-språk",
        "verktøy",
      ],
    },
  ];

  return (
    <>
      <VStack gap="space-24">
        <BodyShort className="text-gray-600">
          Periode: {dateRange.start} - {dateRange.end} ({formatNumber(usage.length)} dager)
        </BodyShort>
        <Tabs tabs={tabs} defaultTab="dashboard" enableHashNavigation />
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
