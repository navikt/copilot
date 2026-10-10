import { Suspense, cache, type ReactNode } from "react";
import type { Metadata } from "next";
import { BodyShort, Box, HGrid, Skeleton } from "@navikt/ds-react";
import {
  getCopilotUsageMetrics,
  getMonthlyTrends,
  getMonthlyBillingUsage,
  getBillingModelDaily,
  getBillingModelForecast,
  getAdoptionCohorts,
  getBillingMonthlyTrend,
  getBillingModelBreakdown,
  getDailySummary,
  getRepositoryUsage,
} from "@/lib/cached-bigquery";
import { InsightPage, InsightSection } from "@/components/insight-page";
import RepositoryUsageTable from "@/components/repository-usage-table";
import TrendChart from "@/components/charts/TrendChart";
import MonthlyTrendsChart from "@/components/charts/MonthlyTrendsChart";
import BillingMonthNowChart from "@/components/charts/BillingMonthNowChart";
import AdoptionCohortsChart from "@/components/charts/AdoptionCohortsChart";
import BillingModelBreakdownChart from "@/components/charts/BillingModelBreakdownChart";
import MetricCard from "@/components/metric-card";
import ErrorState from "@/components/error-state";
import { getPRMetrics, buildTrendData } from "@/lib/data-utils";
import { currentMonthUTC, previousMonth, selectCompleteMonths } from "@/lib/month-utils";
import { formatNumber, formatMinutes } from "@/lib/format";
import { formatDate } from "@/lib/local-model-results";
import { getUser, getUserToken } from "@/lib/auth";
import type { EnterpriseMetrics } from "@/lib/types";

export const metadata: Metadata = {
  title: "Bruk og kostnad",
  description: "Hvor mange som bruker Copilot i Nav, hva det koster, og hvilke repositorier som bruker det mest.",
};

// Shared by the PR section and «Sist oppdatert», so the request runs once per render.
const dailySummaryFor = cache(getDailySummary);
const usageFor = cache(getCopilotUsageMetrics);

const fallback = <Skeleton variant="rectangle" height={300} />;

function momChange(current: number, previous: number | undefined): string | undefined {
  if (!previous) return undefined;
  const change = Math.round(((current - previous) / previous) * 100);
  return change > 0
    ? `↑ ${change} % fra forrige måned`
    : change < 0
      ? `↓ ${Math.abs(change)} % fra forrige måned`
      : "Uendret";
}

async function KeyFigures({ token }: { token: string }) {
  const { trends, error } = await getMonthlyTrends(token);
  if (error) return <BodyShort>{`Kunne ikke hente månedstall: ${error}`}</BodyShort>;
  const { latestComplete: latest, prevComplete: prev } = selectCompleteMonths(trends, currentMonthUTC());
  if (!latest) return <BodyShort>Ingen hel måned med data ennå.</BodyShort>;

  const activity = (m: typeof latest) => m.code_generations + m.ide_interactions + m.cli_requests;
  const agentShare = (m: typeof latest) =>
    m.unique_users > 0 ? Math.round((m.agent_users / m.unique_users) * 100) : 0;
  const month = new Date(latest.month + "-01").toLocaleDateString("nb-NO", { month: "long", year: "numeric" });

  return (
    <>
      <BodyShort size="small" textColor="subtle">
        Tallene gjelder {month}, siste hele måned.
      </BodyShort>
      <HGrid columns={{ xs: 1, sm: 3 }} gap="space-16">
        <MetricCard
          value={formatNumber(latest.unique_users)}
          label="Aktive brukere"
          helpTitle="Aktive brukere"
          helpText="Unike brukere med kodeforslag, chat, agent eller CLI i løpet av måneden."
          subtitle={prev ? momChange(latest.unique_users, prev.unique_users) : undefined}
        />
        <MetricCard
          value={formatNumber(activity(latest))}
          label="Copilot-aktivitet"
          helpTitle="Copilot-aktivitet"
          helpText="Kodeforslag, chat- og agentinteraksjoner og CLI-forespørsler i løpet av måneden, lagt sammen."
          subtitle={prev ? momChange(activity(latest), activity(prev)) : undefined}
        />
        <MetricCard
          value={`${agentShare(latest)} %`}
          label="Bruker agent"
          helpTitle="Andel som bruker agent"
          helpText="Andelen av de aktive brukerne som brukte agentmodus minst én gang i løpet av måneden."
          subtitle={prev ? momChange(agentShare(latest), agentShare(prev)) : undefined}
        />
      </HGrid>
      {trends.length > 0 && <MonthlyTrendsChart data={trends} />}
    </>
  );
}

async function CostThisMonth({ token }: { token: string }) {
  const current = currentMonthUTC();
  const previous = previousMonth(current);
  // Both months up front: early in a month the current one often has no data yet.
  const [billing, dailyNow, forecastNow, dailyPrev, forecastPrev] = await Promise.all([
    getMonthlyBillingUsage(token),
    getBillingModelDaily(token, current),
    getBillingModelForecast(token, current),
    getBillingModelDaily(token, previous),
    getBillingModelForecast(token, previous),
  ]);
  const latestBilled = billing.usage
    .map((r) => r.month)
    .sort()
    .at(-1);

  let daily = dailyNow;
  let forecast = forecastNow;
  if (daily.usage.length === 0 && latestBilled && latestBilled !== current) {
    if (latestBilled === previous) {
      daily = dailyPrev;
      forecast = forecastPrev;
    } else {
      [daily, forecast] = await Promise.all([
        getBillingModelDaily(token, latestBilled),
        getBillingModelForecast(token, latestBilled),
      ]);
    }
  }
  if (daily.error) console.error("[innsikt/bruk] Billing model daily failed:", daily.error);
  if (forecast.error) console.error("[innsikt/bruk] Billing model forecast failed:", forecast.error);

  if (daily.usage.length === 0 || !forecast.forecast) {
    return (
      <Box background="neutral-soft" padding="space-16" borderRadius="8">
        <BodyShort size="small">
          Kostnaden for denne måneden er ikke tilgjengelig ennå.
          {daily.error || forecast.error
            ? " Dataene kunne ikke hentes nå."
            : " Dagens kostnadstall er ikke lest inn ennå."}
        </BodyShort>
      </Box>
    );
  }
  return <BillingMonthNowChart dailyData={daily.usage} forecast={forecast.forecast} />;
}

async function CostOverTime({ token }: { token: string }) {
  const current = currentMonthUTC();
  const [{ breakdown, error }, { trend, error: trendError }, { forecast }] = await Promise.all([
    getBillingModelBreakdown(token),
    getBillingMonthlyTrend(token),
    getBillingModelForecast(token, current),
  ]);
  if (error) return <BodyShort>{`Kunne ikke hente kostnad per modell: ${error}`}</BodyShort>;
  // Without the trend the chart would draw net cost as zero.
  if (trendError) return <BodyShort>{`Kunne ikke hente fakturert kostnad: ${trendError}`}</BodyShort>;
  if (breakdown.length === 0) return <BodyShort>Ingen kostnadsdata ennå.</BodyShort>;
  return <BillingModelBreakdownChart breakdown={breakdown} trend={trend} forecast={forecast} />;
}

async function AdoptionPhases({ token }: { token: string }) {
  const { cohorts, error } = await getAdoptionCohorts(token);
  if (error) return <BodyShort>{`Kunne ikke hente adopsjonsfaser: ${error}`}</BodyShort>;
  if (cohorts.length === 0) return <BodyShort>Ingen data om adopsjonsfaser ennå.</BodyShort>;
  return <AdoptionCohortsChart data={cohorts} />;
}

async function UsageMetrics({
  token,
  children,
}: {
  token: string;
  children: (usage: EnterpriseMetrics[]) => ReactNode;
}) {
  const { usage, error } = await usageFor(token);
  if (error) return <BodyShort>{`Kunne ikke hente bruksdata: ${error}`}</BodyShort>;
  if (!usage?.length) return <BodyShort>Ingen bruksdata ennå.</BodyShort>;
  return children(usage.slice(-28));
}

async function PullRequests({ token }: { token: string }) {
  const [{ usage, error }, { summary }] = await Promise.all([usageFor(token), dailySummaryFor(token)]);
  if (error) return <BodyShort>{`Kunne ikke hente PR-data: ${error}`}</BodyShort>;
  const pr = usage ? getPRMetrics(usage.slice(-28)) : null;
  if (!pr || pr.totalCreated === 0) return <BodyShort>Ingen PR-data ennå.</BodyShort>;
  const pct = (part: number, whole: number) => (whole > 0 ? `${Math.round((part / whole) * 100)} %` : "–");
  return (
    <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
      <MetricCard
        value={formatNumber(pr.totalCreatedByCopilot)}
        label="PR-er laget av Copilot"
        helpTitle="PR-er laget av Copilot"
        helpText="Pull requests som Copilot coding agent opprettet de siste 28 dagene."
        subtitle={`${pct(pr.totalCreatedByCopilot, pr.totalCreated)} av alle PR-er`}
      />
      <MetricCard
        value={pct(pr.totalReviewedByCopilot, pr.totalReviewed)}
        label="Gjennomgått av Copilot"
        helpTitle="Andel gjennomgått av Copilot"
        helpText="Andelen av gjennomgåtte pull requests der Copilot code review var med, de siste 28 dagene."
        subtitle={`${formatNumber(pr.totalReviewedByCopilot)} PR-er`}
      />
      <MetricCard
        value={formatMinutes(pr.medianMinutesToMerge)}
        label="Tid til merge"
        helpTitle="Tid til merge"
        helpText="Median tid fra en pull request opprettes til den merges, for alle PR-er de siste 28 dagene."
        subtitle={`Laget av Copilot: ${formatMinutes(pr.medianMinutesToMergeCopilotAuthored)}`}
      />
      <MetricCard
        value={summary ? formatMinutes(summary.pr_avg_minutes_to_review) : "–"}
        label="Tid til første review"
        helpTitle="Tid til første review"
        helpText="Median tid fra en pull request opprettes til den får første review. Bare PR-er som er merget. Snitt over adopsjonsfasene, vektet etter antall PR-er. Målt fra 7. juli 2026."
      />
    </HGrid>
  );
}

async function Repositories({ token }: { token: string }) {
  const { repositories, error } = await getRepositoryUsage(token);
  if (error) return <BodyShort>{`Kunne ikke hente repositorier: ${error}`}</BodyShort>;
  if (!repositories?.length) return <BodyShort>Ingen repositoriedata ennå.</BodyShort>;
  const sum = (key: "pr_created_by_copilot" | "pr_reviewed_by_copilot") => repositories.reduce((s, r) => s + r[key], 0);
  return (
    <>
      <HGrid columns={{ xs: 1, sm: 2 }} gap="space-16">
        <MetricCard
          value={formatNumber(sum("pr_created_by_copilot"))}
          label="PR-er laget av Copilot"
          helpTitle="PR-er laget av Copilot"
          helpText="Pull requests opprettet av Copilot coding agent i repositoriene under, så lenge vi har data."
        />
        <MetricCard
          value={formatNumber(sum("pr_reviewed_by_copilot"))}
          label="PR-er gjennomgått av Copilot"
          helpTitle="PR-er gjennomgått av Copilot"
          helpText="Pull requests gjennomgått av Copilot code review i repositoriene under, så lenge vi har data."
        />
      </HGrid>
      <RepositoryUsageTable repositories={repositories} />
    </>
  );
}

async function LastUpdated({ token }: { token: string }) {
  const { summary } = await dailySummaryFor(token);
  return summary ? formatDate(summary.date) : "kunne ikke hentes";
}

export default async function BrukPage() {
  await getUser();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;

  return (
    <InsightPage
      title="Bruk og kostnad"
      description="Hvor mange som bruker Copilot i Nav, hva det koster, og hvor det brukes."
      intro="Her ser du hvor mange i Nav som bruker Copilot, hvor mye de bruker agenten, hva det koster, og hvordan Copilot er med i pull requests. Nederst finner du repositoriene med mest Copilot-aktivitet."
      updated={
        <Suspense fallback="henter …">
          <LastUpdated token={token} />
        </Suspense>
      }
      source={
        <>
          Tallene kommer fra copilot-api, som leser GitHubs bruks- og fakturadata fra BigQuery. Nøkkeltall bruker siste
          hele måned fra <code>/usage/trends</code>. Kostnad bruker <code>/billing/model-daily</code>,{" "}
          <code>/billing/model-forecast</code> og <code>/billing/model-breakdown</code>. Kostnaden per modell er brutto
          USD før rabatt, mens prognosen og linjen for fakturert kostnad er netto. Adopsjonsfasene kommer fra{" "}
          <code>/adoption/cohorts</code>. Pull requests og daglig aktivitet bruker de siste 28 dagene fra{" "}
          <code>/usage/metrics</code>, og tid til første review kommer fra <code>/usage/daily-summary</code> (
          <code>v_daily_summary</code>). Repositoriene gjelder hele perioden med data, uten private repositorier og uten
          repositorier med færre enn fem PR-er.
        </>
      }
    >
      <InsightSection id="nokkeltall" title="Nøkkeltall">
        <Suspense fallback={fallback}>
          <KeyFigures token={token} />
        </Suspense>
      </InsightSection>

      <InsightSection id="kostnad" title="Kostnad denne måneden">
        <Suspense fallback={fallback}>
          <CostThisMonth token={token} />
        </Suspense>
      </InsightSection>

      <InsightSection id="kostnad-over-tid" title="Kostnad over tid">
        <Suspense fallback={fallback}>
          <CostOverTime token={token} />
        </Suspense>
      </InsightSection>

      {/* The id keeps the published #ai-adopsjonsfaser link after AI became KI. */}
      <InsightSection id="ai-adopsjonsfaser" title="KI-adopsjonsfaser">
        <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm md:grid-cols-4">
          <div>
            <dt className="font-medium">Fase 0</dt>
            <dd>Ingen KI-bruk siste 28 dager</dd>
          </div>
          <div>
            <dt className="font-medium">Fase 1</dt>
            <dd>Bruker kodeforslag</dd>
          </div>
          <div>
            <dt className="font-medium">Fase 2</dt>
            <dd>Bruker KI-agent i ett verktøy, for eksempel bare Chat</dd>
          </div>
          <div>
            <dt className="font-medium">Fase 3</dt>
            <dd>Bruker KI-agent i flere verktøy, for eksempel Chat og CLI</dd>
          </div>
        </dl>
        <Suspense fallback={fallback}>
          <AdoptionPhases token={token} />
        </Suspense>
      </InsightSection>

      <InsightSection id="pull-requests-og-code-review" title="Pull requests og code review">
        <Suspense fallback={fallback}>
          <PullRequests token={token} />
        </Suspense>
      </InsightSection>

      <InsightSection id="daglig-aktivitet" title="Daglig aktivitet">
        <Suspense fallback={fallback}>
          <UsageMetrics token={token}>{(usage) => <TrendChart data={buildTrendData(usage)} />}</UsageMetrics>
        </Suspense>
      </InsightSection>

      <InsightSection id="repositorier" title="Repositorier">
        <Suspense fallback={fallback}>
          <Repositories token={token} />
        </Suspense>
      </InsightSection>
    </InsightPage>
  );
}
