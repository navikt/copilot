import { Suspense, type ReactNode } from "react";
import type { Metadata } from "next";
import { Alert, BodyShort, Button, HStack, Select, Skeleton } from "@navikt/ds-react";
import { InsightPage, InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { CopilotPRChart, CreditsPerUserChart, FamilyShareChart } from "@/components/charts/MonthlyTrendCharts";
import { getBillingModelBreakdown, getCopilotPRsMonthly, getCreditsPerUserMonthly } from "@/lib/cached-bigquery";
import { getUser, getUserToken } from "@/lib/auth";
import { currentMonthUTC } from "@/lib/month-utils";
import {
  PERIODS,
  annotationsFor,
  familyShares,
  monthLabel,
  parsePeriod,
  periodStart,
  visibleMonths,
} from "@/lib/trends";
import { CHART_ANNOTATIONS } from "../../reisen/milestones";

export const metadata: Metadata = {
  title: "Trender",
  description: "Hvordan modellvalg, forbruk per bruker og Copilot i pull requests har endret seg måned for måned.",
};

const fallback = <Skeleton variant="rectangle" height={300} />;

/** Says where the chart's data starts, and when the chosen period reaches further back than that. */
function DataStart({ clamped, first }: { clamped: boolean; first?: string }) {
  if (!first) return null;
  return (
    <BodyShort size="small" textColor="subtle">
      Dataene starter i {monthLabel(first)}.{clamped && " Perioden du har valgt, går lenger tilbake enn det."}
    </BodyShort>
  );
}

async function ModelFamilies({ token, start }: { token: string; start: string | null }) {
  // 36 months is the most the endpoint returns.
  const { breakdown, error } = await getBillingModelBreakdown(token, 36);
  if (error) return <BodyShort>{`Kunne ikke hente kostnad per modell: ${error}`}</BodyShort>;
  const { months, clamped, first } = visibleMonths(
    breakdown.map((r) => r.year_month.slice(0, 7)),
    start
  );
  const data = familyShares(breakdown, months);
  if (!data.months.length) return <BodyShort>Ingen kostnadsdata i perioden.</BodyShort>;
  return (
    <>
      <DataStart clamped={clamped} first={first} />
      <FamilyShareChart data={data} annotations={annotationsFor(CHART_ANNOTATIONS, data.months)} />
    </>
  );
}

async function CreditsPerUser({ token, start }: { token: string; start: string | null }) {
  const { months: rows, error } = await getCreditsPerUserMonthly(token);
  if (error) return <BodyShort>{`Kunne ikke hente AI Credits per bruker: ${error}`}</BodyShort>;
  const { months, clamped, first } = visibleMonths(
    rows.map((r) => r.month),
    start
  );
  const data = rows.filter((r) => months.includes(r.month));
  if (!data.length) return <BodyShort>Ingen data om AI Credits i perioden.</BodyShort>;
  return (
    <>
      <DataStart clamped={clamped} first={first} />
      <CreditsPerUserChart data={data} annotations={annotationsFor(CHART_ANNOTATIONS, months)} />
    </>
  );
}

async function CopilotPRs({ token, start }: { token: string; start: string | null }) {
  const { months: rows, error } = await getCopilotPRsMonthly(token);
  if (error) return <BodyShort>{`Kunne ikke hente pull requests: ${error}`}</BodyShort>;
  const { months, clamped, first } = visibleMonths(
    rows.map((r) => r.month),
    start
  );
  const data = rows.filter((r) => months.includes(r.month));
  if (!data.length) return <BodyShort>Ingen PR-data i perioden.</BodyShort>;
  return (
    <>
      <Alert variant="info" size="small">
        Historikken er kort. GitHub har bare levert disse tallene siden 17. juli 2026, så det er for tidlig å lese en
        trend ut av dem.
      </Alert>
      <DataStart clamped={clamped} first={first} />
      <CopilotPRChart data={data} annotations={annotationsFor(CHART_ANNOTATIONS, months)} />
    </>
  );
}

function PeriodSelect({ value }: { value: string }) {
  return (
    <form action="/innsikt/trender" method="get" aria-label="Velg periode">
      <HStack gap="space-8" align="end">
        <Select key={value} label="Periode" name="periode" defaultValue={value} size="small">
          {PERIODS.map((p) => (
            <option key={p.value} value={p.value}>
              {p.label}
            </option>
          ))}
        </Select>
        <Button type="submit" size="small" variant="secondary-neutral">
          Vis
        </Button>
      </HStack>
    </form>
  );
}

function Section({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <InsightSection id={id} title={title}>
      <Suspense fallback={fallback}>{children}</Suspense>
    </InsightSection>
  );
}

export default async function TrenderPage({ searchParams }: { searchParams: Promise<{ periode?: string }> }) {
  await getUser();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;
  const period = parsePeriod((await searchParams).periode);
  const start = periodStart(period, currentMonthUTC());

  return (
    <InsightPage
      title="Trender"
      description="Copilot i Nav måned for måned."
      intro="Her ser du hvordan kostnaden fordeler seg på modellfamilier, hvor mange AI Credits en typisk bruker bruker, og hvor mye Copilot er med i pull requests, måned for måned. Hver graf starter der dataene starter."
      updated="tallene hentes på nytt hver time."
      source={
        <>
          Modellfamiliene bygger på netto kostnad per modell fra <code>v_billing_model_breakdown</code> (
          <code>/billing/model-breakdown</code>). AI Credits per bruker er median og snitt per måned av hver aktiv
          brukers <code>ai_credits_used</code> i <code>user_metrics</code> (<code>/usage/credits-per-user</code>). En
          bruker er aktiv når hen har brukt AI Credits, chat eller kodeforslag i måneden, og måneder med færre enn fem
          brukere vises ikke. Pull requests er summen over alle repositorier unntatt private i{" "}
          <code>repository_metrics</code> (<code>/usage/copilot-prs</code>). Den inneværende måneden er ikke ferdig. De
          loddrette strekene markerer når noe skjedde, ikke hva som var årsaken. Modellvalgene gjelder bare våre egne
          agenter, mens faktureringen gjelder hele Nav. Fra 1. juni 2026 ble premium requests erstattet av AI Credits,
          så kostnadene før og etter er ikke direkte sammenlignbare.
        </>
      }
    >
      <PeriodSelect value={period.value} />
      <Section id="modellfamilier" title="Kostnad per modellfamilie">
        <ModelFamilies token={token} start={start} />
      </Section>
      <Section id="ai-credits-per-bruker" title="AI Credits per bruker">
        <CreditsPerUser token={token} start={start} />
      </Section>
      <Section id="copilot-i-pull-requests" title="Copilot i pull requests">
        <CopilotPRs token={token} start={start} />
      </Section>
    </InsightPage>
  );
}
