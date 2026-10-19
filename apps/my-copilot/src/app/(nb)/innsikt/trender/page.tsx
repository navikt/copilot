import { Suspense } from "react";
import type { Metadata } from "next";
import { Alert, BodyShort, Link, Skeleton } from "@navikt/ds-react";
import NextLink from "next/link";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { InsightPage, InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { CopilotPRChart } from "@/components/charts/MonthlyTrendCharts";
import { getCohortRetention, getCopilotPRsMonthly, getDailySummary } from "@/lib/cached-bigquery";
import { getUser, getUserToken } from "@/lib/auth";
import { currentMonthUTC } from "@/lib/month-utils";
import { formatNumber } from "@/lib/format";
import { annotationsFor, monthLabel, parsePeriod, periodStart, showAllEvents, visibleMonths } from "@/lib/trends";
import { CHART_ANNOTATIONS } from "../../reisen/milestones";
import { PeriodSelect } from "./controls";
import { Events } from "@/components/chart-events";
import { Intensity, Movement, SegmentChanges, WayOfWorking } from "./segments";

export const metadata: Metadata = {
  title: "Trender",
  description:
    "Hvordan bruken av Copilot i Nav endrer seg måned for måned: intensitet, arbeidsmåte, nye brukere og Copilot i pull requests.",
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

type ChartProps = { token: string; start: string | null; all: boolean };

async function CopilotPRs({ token, start, all }: ChartProps) {
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
      <CopilotPRChart months={months} data={data} annotations={annotationsFor(CHART_ANNOTATIONS, months, all)} />
    </>
  );
}

/** user_metrics starts 2025-10-10, so the October 2025 cohort mixes earlier users with new ones and cannot tell them apart. */
const CENSORED_COHORT = "2025-10";

function pct(v: number | null) {
  return v === null ? "–" : `${v}\u00a0%`;
}

async function Cohorts({ token, start }: { token: string; start: string | null }) {
  const { cohorts, error } = await getCohortRetention(token);
  if (error) return <BodyShort>{`Kunne ikke hente kohorter: ${error}`}</BodyShort>;
  const rows = cohorts.filter((c) => !start || c.cohort_month >= start);
  if (!rows.length) return <BodyShort>Ingen kohorter i perioden.</BodyShort>;
  return (
    <>
      <BodyShort size="small" textColor="subtle">
        Andelen av hver kohort som brukte Copilot igjen én, tre og seks måneder etter den første måneden. En strek betyr
        at måneden ikke er over ennå.
      </BodyShort>
      <div className="min-w-0 overflow-x-auto">
        <Table size="small">
          <TableHeader>
            <TableRow>
              <TableHeaderCell>Første måned</TableHeaderCell>
              <TableHeaderCell align="right">Brukere</TableHeaderCell>
              <TableHeaderCell align="right">+1</TableHeaderCell>
              <TableHeaderCell align="right">+3</TableHeaderCell>
              <TableHeaderCell align="right">+6</TableHeaderCell>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((c) => (
              <TableRow key={c.cohort_month}>
                <TableDataCell className="whitespace-nowrap">
                  {monthLabel(c.cohort_month)}
                  {c.cohort_month === CENSORED_COHORT && " *"}
                </TableDataCell>
                <TableDataCell align="right">{formatNumber(c.cohort_size)}</TableDataCell>
                <TableDataCell align="right">{pct(c.m1)}</TableDataCell>
                <TableDataCell align="right">{pct(c.m3)}</TableDataCell>
                <TableDataCell align="right">{pct(c.m6)}</TableDataCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {rows.some((c) => c.cohort_month === CENSORED_COHORT) && (
        <BodyShort size="small" textColor="subtle">
          * Dataene starter 10. oktober 2025. Raden blander dem som brukte Copilot fra før, med nye brukere, og vi kan
          ikke skille dem.
        </BodyShort>
      )}
    </>
  );
}

export default async function TrenderPage({
  searchParams,
}: {
  searchParams: Promise<{ periode?: string; hendelser?: string | string[] }>;
}) {
  await getUser();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;
  const params = await searchParams;
  const period = parsePeriod(params.periode);
  const start = periodStart(period, currentMonthUTC());
  const all = showAllEvents(params.hendelser);
  const props = { token, start, all };

  return (
    <InsightPage
      title="Trender"
      description="Copilot i Nav måned for måned."
      intro="Her ser du hvordan bruken av Copilot i Nav endrer seg over tid: hvor mye folk bruker den, på hvilken måte, hvor mange som går opp eller ned, og hvor mange nye brukere som fortsetter. Nederst ser du Copilot i pull requests. Ingen personer eller team navngis. Hver graf starter der dataene starter."
      updated={async () => (await getDailySummary(token)).summary?.date}
      hourly
      source={
        <>
          Pull requests er summen over alle repoer unntatt private i <code>repository_metrics</code> (
          <code>/usage/copilot-prs</code>). Den inneværende måneden er ikke ferdig. Hendelsene og bruddene i dataene
          står i «Hendelser» øverst på siden. Kohortene (<code>/usage/cohort-retention</code>) grupperer brukerne etter
          måneden de første gang var aktive i <code>user_metrics</code>, og viser hvor stor andel som var aktive igjen
          én, tre og seks måneder senere, avrundet til hele prosent. Oktober 2025 er ikke en ekte kohort: dataene
          starter 10. oktober, så vi kan ikke skille dem som brukte Copilot fra før, fra dem som var nye. Tallene regnes
          ut per person, men bare summene vises. Små grupper vises også, fordi ingen person eller team navngis. Perioden
          du velger, styrer hvilke kohorter som vises. Segmentene (<code>/usage/segments</code>) har hver sin «Kilde og
          metode» under grafen. «Hva har endret seg» regnes ut fra de samme tallene som grafene. «Sist oppdatert» er
          siste dag med bruksdata i <code>v_daily_summary</code>.
        </>
      }
    >
      <PeriodSelect value={period.value} all={all} />
      <BodyShort>
        Kostnad per modellfamilie og AI Credits per bruker ligger nå på{" "}
        <Link as={NextLink} href="/innsikt/bruk#kostnad">
          Bruk og kostnad
        </Link>
        .
      </BodyShort>
      <InsightSection id="hva-har-endret-seg" title="Hva har endret seg">
        <Suspense fallback={fallback}>
          <SegmentChanges {...props} />
        </Suspense>
      </InsightSection>
      <Events />
      <InsightSection id="bevegelse" title="Bevegelse mellom intensitetsgrupper">
        <Suspense fallback={fallback}>
          <Movement {...props} />
        </Suspense>
      </InsightSection>
      <InsightSection id="intensitet" title="Brukere etter intensitet">
        <Suspense fallback={fallback}>
          <Intensity {...props} />
        </Suspense>
      </InsightSection>
      <InsightSection id="arbeidsmate" title="Brukere etter arbeidsmåte">
        <Suspense fallback={fallback}>
          <WayOfWorking {...props} />
        </Suspense>
      </InsightSection>
      <InsightSection id="kohorter" title="Blir brukerne værende?">
        <Suspense fallback={fallback}>
          <Cohorts token={token} start={start} />
        </Suspense>
      </InsightSection>
      <InsightSection id="copilot-i-pull-requests" title="Copilot i pull requests">
        <Suspense fallback={fallback}>
          <CopilotPRs {...props} />
        </Suspense>
      </InsightSection>
    </InsightPage>
  );
}
