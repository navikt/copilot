import { Suspense, cache, type ReactNode } from "react";
import type { Metadata } from "next";
import { BodyShort, HGrid, Skeleton, VStack } from "@navikt/ds-react";
import {
  getCopilotUsageMetrics,
  getMonthlyTrends,
  getBillingModelDaily,
  getAdoptionCohorts,
  getBillingMonthlyTrend,
  getDailySummary,
  getRepositoryUsage,
} from "@/lib/cached-bigquery";
import { InsightPage, InsightSection } from "@/components/insight-page";
import RepositoryUsageTable from "@/components/repository-usage-table";
import TrendChart from "@/components/charts/TrendChart";
import MonthlyTrendsChart from "@/components/charts/MonthlyTrendsChart";
import AdoptionCohortsChart from "@/components/charts/AdoptionCohortsChart";
import { ChartFigure, NumbersFold, PlainTable } from "@/components/charts/chart-figure";
import { EventChart } from "@/components/charts/EventChart";
import MetricCard from "@/components/metric-card";
import ErrorState from "@/components/error-state";
import { getPRMetrics, buildTrendData } from "@/lib/data-utils";
import { currentMonthUTC, selectCompleteMonths } from "@/lib/month-utils";
import { formatNumber, formatMinutes, formatPercent, formatShare, formatUSD } from "@/lib/format";
import { formatPp } from "@/lib/trends";
import { getUser, getUserToken } from "@/lib/auth";
import type { EnterpriseMetrics } from "@/lib/types";

export const metadata: Metadata = {
  title: "Bruk og kostnad",
  description: "Hvor mange som bruker Copilot i Nav, hva det koster, og hvilke repoer som bruker det mest.",
};

// Shared by the PR section and «Sist oppdatert», so the request runs once per render.
const dailySummaryFor = cache(getDailySummary);
const usageFor = cache(getCopilotUsageMetrics);

const fallback = <Skeleton variant="rectangle" height={300} />;

function momChange(current: number, previous: number | undefined): string | undefined {
  if (!previous) return undefined;
  const change = Math.round(((current - previous) / previous) * 100);
  return change > 0
    ? `↑ ${formatPercent(change)} fra forrige måned`
    : change < 0
      ? `↓ ${formatPercent(Math.abs(change))} fra forrige måned`
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
          value={formatPercent(agentShare(latest))}
          label="Bruker agent"
          helpTitle="Andel som bruker agent"
          helpText="Andelen av de aktive brukerne som brukte agentmodus minst én gang i løpet av måneden."
          subtitle={prev ? `${formatPp(agentShare(latest) - agentShare(prev))} fra forrige måned` : undefined}
        />
      </HGrid>
      {trends.length > 0 && <MonthlyTrendsChart data={trends} />}
    </>
  );
}

const monthLong = (m: string) =>
  new Date(`${m}-01T00:00:00Z`).toLocaleDateString("nb-NO", { month: "long", year: "numeric", timeZone: "UTC" });

/** This month day by day and this year month by month, actual figures only. Net: estimated billed, as on /innsikt/kostnad. */
async function Cost({ token }: { token: string }) {
  const current = currentMonthUTC();
  const [daily, { trend, error }] = await Promise.all([
    getBillingModelDaily(token, current),
    getBillingMonthlyTrend(token),
  ]);
  if (daily.error) console.error("[innsikt/bruk] Billing model daily failed:", daily.error);
  if (error) console.error("[innsikt/bruk] Billing monthly trend failed:", error);

  const netByDay = new Map<string, number>();
  for (const r of daily.usage) netByDay.set(r.day, (netByDay.get(r.day) ?? 0) + r.net_amount);
  const days = [...netByDay.keys()].sort();
  const cumulative: number[] = [];
  for (const d of days) cumulative.push((cumulative.at(-1) ?? 0) + netByDay.get(d)!);
  const sum = cumulative.at(-1) ?? 0;

  const year = current.slice(0, 4);
  // One figure for the current month in both charts: the sum of the daily estimated billed amounts.
  const months = trend
    .filter((t) => t.year_month.startsWith(year))
    .map((t) => (t.year_month === current && days.length ? { ...t, total_net_amount: sum } : t))
    .sort((a, b) => a.year_month.localeCompare(b.year_month));
  const keys = months.map((t) => t.year_month);
  const amount = (closed: boolean) =>
    months.map((t) => (t.year_month < current === closed ? t.total_net_amount : null));
  const monthShort = (m: string) =>
    new Date(`${m}-01T00:00:00Z`).toLocaleDateString("nb-NO", { month: "short", timeZone: "UTC" });

  return (
    <VStack gap="space-16">
      <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
        {days.length ? (
          <ChartFigure
            title="Hva har Copilot kostet hittil i måneden?"
            summary={`Foreløpig forbruk i ${monthLong(current)} er ${formatUSD(sum)} til og med ${Number(days.at(-1)!.slice(8))}. ${monthLong(current).split(" ")[0]}.`}
            chart={
              <EventChart
                kind="line"
                unit="usd"
                keys={days}
                labels={days.map((d) => d.slice(8))}
                series={[{ label: "Foreløpig forbruk, samlet", color: 0, data: cumulative }]}
              />
            }
          />
        ) : (
          <BodyShort>
            {daily.error
              ? "Kunne ikke hente kostnaden for denne måneden."
              : `Ingen kostnadstall for ${monthLong(current)} ennå.`}
          </BodyShort>
        )}
        {months.length === 0 ? (
          <BodyShort>
            {error ? "Kunne ikke hente kostnaden per måned." : `Ingen kostnadstall for ${year} ennå.`}
          </BodyShort>
        ) : (
          <ChartFigure
            title={`Hva har Copilot kostet per måned i ${year}?`}
            summary={months
              .map(
                (t) =>
                  `${monthLong(t.year_month)}: ${formatUSD(t.total_net_amount)}${t.year_month < current ? "" : " (foreløpig)"}`
              )
              .join(", ")}
            chart={
              <EventChart
                kind="bar"
                stacked
                unit="usd"
                keys={keys}
                labels={keys.map((k) => monthShort(k) + (k === current ? " (foreløpig)" : ""))}
                series={[
                  { label: "Fakturert", color: 0, data: amount(true) },
                  { label: "Foreløpig forbruk", color: 1, data: amount(false) },
                ]}
              />
            }
          />
        )}
      </HGrid>
      {months.length > 0 && (
        <NumbersFold>
          <PlainTable
            rows={[
              ["Måned", "Beløp", "Grunnlag"],
              ...months.map((t) => [
                monthLong(t.year_month),
                formatUSD(t.total_net_amount),
                t.year_month < current ? "Fakturert" : "Foreløpig forbruk",
              ]),
            ]}
          />
        </NumbersFold>
      )}
      <NumbersFold summary="Slik leser du tallene">
        <BodyShort size="small">
          Fakturert er beløpet på fakturaen for en avsluttet måned, etter rabatt. Foreløpig forbruk er det vi regner med
          å bli fakturert for så langt i måneden. Beløpet kan endre seg til fakturaen kommer. Grafene viser bare faktisk
          forbruk, ikke prognoser.
        </BodyShort>
      </NumbersFold>
    </VStack>
  );
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
  const pct = (part: number, whole: number) => (whole > 0 ? formatShare(part / whole) : "–");
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
        label="Snitt tid til første review"
        helpTitle="Snitt tid til første review, siste dag med data"
        helpText="Gjennomsnittlig tid fra en pull request opprettes til den får første review, for siste dag med data, ikke de siste 28 dagene. Bare PR-er som er merget. GitHub oppgir et snitt per KI-adopsjonsfase, og vi vekter snittene etter antall mergede PR-er. Målt fra 7. juli 2026."
      />
    </HGrid>
  );
}

async function Repositories({ token }: { token: string }) {
  const { repositories, error } = await getRepositoryUsage(token);
  if (error) return <BodyShort>{`Kunne ikke hente repoene: ${error}`}</BodyShort>;
  if (!repositories?.length) return <BodyShort>Ingen data om repoer ennå.</BodyShort>;
  const sum = (key: "pr_created_by_copilot" | "pr_reviewed_by_copilot") => repositories.reduce((s, r) => s + r[key], 0);
  return (
    <>
      <HGrid columns={{ xs: 1, sm: 2 }} gap="space-16">
        <MetricCard
          value={formatNumber(sum("pr_created_by_copilot"))}
          label="PR-er laget av Copilot, hele perioden"
          helpTitle="PR-er laget av Copilot, hele perioden"
          helpText="Pull requests opprettet av Copilot coding agent i repoene under, så lenge vi har data."
        />
        <MetricCard
          value={formatNumber(sum("pr_reviewed_by_copilot"))}
          label="PR-er gjennomgått av Copilot, hele perioden"
          helpTitle="PR-er gjennomgått av Copilot, hele perioden"
          helpText="Pull requests gjennomgått av Copilot code review i repoene under, så lenge vi har data."
        />
      </HGrid>
      <RepositoryUsageTable repositories={repositories} />
    </>
  );
}

export default async function BrukPage() {
  await getUser();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;

  return (
    <InsightPage
      title="Bruk og kostnad"
      description="Hvor mange som bruker Copilot i Nav, hva det koster, og hvor det brukes."
      intro="Her ser du hvor mange i Nav som bruker Copilot, hvor mye de bruker agenten, hva det koster, og hvordan Copilot er med i pull requests. Nederst finner du repoene med mest Copilot-aktivitet."
      updated={async () => (await dailySummaryFor(token)).summary?.date}
      hourly
      source={
        <>
          Tallene kommer fra copilot-api, som leser GitHubs bruks- og fakturadata fra BigQuery. Nøkkeltall bruker siste
          hele måned fra <code>/usage/trends</code>. Kostnad bruker <code>/billing/model-daily</code> og{" "}
          <code>/billing/monthly-trend</code>, netto USD etter rabatt. Adopsjonsfasene kommer fra{" "}
          <code>/adoption/cohorts</code>. Pull requests og daglig aktivitet bruker de siste 28 dagene fra{" "}
          <code>/usage/metrics</code>, og tid til første review kommer fra <code>/usage/daily-summary</code> (
          <code>v_daily_summary</code>). Repoene gjelder hele perioden med data, uten private repoer og uten repoer med
          færre enn fem PR-er. «Sist oppdatert» er siste dag med data i <code>v_daily_summary</code>.
        </>
      }
    >
      <InsightSection id="nokkeltall" title="Nøkkeltall">
        <Suspense fallback={fallback}>
          <KeyFigures token={token} />
        </Suspense>
      </InsightSection>

      <InsightSection id="kostnad" title="Kostnad">
        {/* Keeps old #kostnad-over-tid links landing here after the two cost sections were merged. */}
        <span id="kostnad-over-tid" aria-hidden />
        <Suspense fallback={fallback}>
          <Cost token={token} />
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

      <InsightSection id="repositorier" title="Repoer">
        <Suspense fallback={fallback}>
          <Repositories token={token} />
        </Suspense>
      </InsightSection>
    </InsightPage>
  );
}
