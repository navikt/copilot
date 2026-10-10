import { Suspense, type ReactNode } from "react";
import type { Metadata } from "next";
import { Alert, BodyShort, Button, HStack, Select, Skeleton } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { InsightPage, InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { CopilotPRChart, CreditsPerUserChart, FamilyShareChart } from "@/components/charts/MonthlyTrendCharts";
import {
  getBillingModelBreakdown,
  getCohortRetention,
  getCopilotPRsMonthly,
  getCreditsPerUserMonthly,
  getMyTeams,
  getTeamActiveUsersMonthly,
} from "@/lib/cached-bigquery";
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
  // 36 months is the most the endpoint returns, so «Alt» means at most 36 months here (see «Kilde og metode»).
  const { breakdown, error } = await getBillingModelBreakdown(token, 36);
  if (error) return <BodyShort>{`Kunne ikke hente kostnad per modell: ${error}`}</BodyShort>;
  const { months, clamped, first } = visibleMonths(
    breakdown.map((r) => r.year_month.slice(0, 7)),
    start
  );
  const data = familyShares(breakdown, months);
  if (!data.series.length) return <BodyShort>Ingen kostnadsdata i perioden.</BodyShort>;
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
      <CreditsPerUserChart months={months} data={data} annotations={annotationsFor(CHART_ANNOTATIONS, months)} />
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
      <CopilotPRChart months={months} data={data} annotations={annotationsFor(CHART_ANNOTATIONS, months)} />
    </>
  );
}

/** user_metrics starts 2025-10-10, so the October 2025 cohort holds everyone already active then, not new users. */
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
        at måneden ikke er over ennå. Kohorter med færre enn fem personer vises ikke.
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
                <TableDataCell align="right">{c.cohort_size}</TableDataCell>
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
          * Dataene starter 10. oktober 2025. Denne raden er alle som allerede brukte Copilot da, ikke nye brukere.
        </BodyShort>
      )}
    </>
  );
}

async function TeamActiveUsers({
  token,
  start,
  period,
  team,
}: {
  token: string;
  start: string | null;
  period: string;
  team?: string;
}) {
  const { rows, error } = await getTeamActiveUsersMonthly(token);
  if (error) return <BodyShort>{`Kunne ikke hente aktive brukere per team: ${error}`}</BodyShort>;
  const teams = [...new Set(rows.map((r) => r.team_slug))].sort();
  if (!teams.length) return <BodyShort>Ingen team har minst fem aktive brukere.</BodyShort>;
  let chosen = team && teams.includes(team) ? team : undefined;
  if (!chosen) {
    const mine = await getMyTeams(token).catch(() => [] as string[]);
    chosen = mine.find((t) => teams.includes(t));
  }
  if (!chosen) {
    // The team with the most active users in the latest month, as a starting point only.
    const latest = rows.reduce((m, r) => (r.month > m ? r.month : m), "");
    chosen = rows.filter((r) => r.month === latest).sort((a, b) => b.active_users - a.active_users)[0]?.team_slug;
    chosen ??= teams[0];
  }
  const data = rows.filter((r) => r.team_slug === chosen && (!start || r.month >= start));
  return (
    <>
      <form action="/innsikt/trender#team" method="get" aria-label="Velg team">
        <input type="hidden" name="periode" value={period} />
        <HStack gap="space-8" align="end">
          <Select key={chosen} label="Team" name="team" defaultValue={chosen} size="small">
            {teams.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </Select>
          <Button type="submit" size="small" variant="secondary-neutral">
            Vis
          </Button>
        </HStack>
      </form>
      <BodyShort size="small" textColor="subtle">
        Måneder der teamet hadde færre enn fem aktive brukere, vises ikke.
      </BodyShort>
      {data.length ? (
        <div className="min-w-0 overflow-x-auto">
          <Table size="small">
            <TableHeader>
              <TableRow>
                <TableHeaderCell>Måned</TableHeaderCell>
                <TableHeaderCell align="right">Aktive brukere</TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.map((r) => (
                <TableRow key={r.month}>
                  <TableDataCell className="whitespace-nowrap">{monthLabel(r.month)}</TableDataCell>
                  <TableDataCell align="right">{r.active_users}</TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      ) : (
        <BodyShort>Ingen måneder med minst fem aktive brukere i perioden.</BodyShort>
      )}
    </>
  );
}

function PeriodSelect({ value, team }: { value: string; team?: string }) {
  return (
    <form action="/innsikt/trender" method="get" aria-label="Velg periode">
      {team && <input type="hidden" name="team" value={team} />}
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

export default async function TrenderPage({
  searchParams,
}: {
  searchParams: Promise<{ periode?: string; team?: string }>;
}) {
  await getUser();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;
  const params = await searchParams;
  const period = parsePeriod(params.periode);
  const start = periodStart(period, currentMonthUTC());

  return (
    <InsightPage
      title="Trender"
      description="Copilot i Nav måned for måned."
      intro="Her ser du hvordan kostnaden fordeler seg på modellfamilier, hvor mange AI Credits en typisk bruker bruker, hvor mye Copilot er med i pull requests, hvor mange nye brukere som fortsetter, og hvor mange som er aktive i et team, måned for måned. Hver graf starter der dataene starter."
      updated="tallene hentes på nytt hver time."
      source={
        <>
          Modellfamiliene bygger på netto kostnad per modell fra <code>v_billing_model_breakdown</code> (
          <code>/billing/model-breakdown</code>), som gir høyst de siste 36 månedene, også med «Alt». AI Credits per
          bruker er median og snitt per måned av hver aktiv brukers <code>ai_credits_used</code> i{" "}
          <code>user_metrics</code> (<code>/usage/credits-per-user</code>). En bruker er aktiv når hen har brukt AI
          Credits, chat eller kodeforslag i måneden, og måneder med færre enn fem brukere vises ikke. Pull requests er
          summen over alle repositorier unntatt private i <code>repository_metrics</code> (
          <code>/usage/copilot-prs</code>). Den inneværende måneden er ikke ferdig. De loddrette strekene markerer når
          noe skjedde, ikke hva som var årsaken. Modellvalgene gjelder bare våre egne agenter, mens faktureringen
          gjelder hele Nav. Fra 1. juni 2026 ble premium requests erstattet av AI Credits, så kostnadene før og etter er
          ikke direkte sammenlignbare. Kohortene (<code>/usage/cohort-retention</code>) grupperer brukerne etter måneden
          de første gang var aktive i <code>user_metrics</code>, og viser hvor stor andel som var aktive igjen én, tre
          og seks måneder senere, avrundet til hele prosent. Oktober 2025 er ikke en ekte kohort, fordi dataene starter
          10. oktober og vi ikke vet hvem som var nye da. Tallene regnes ut per person, men bare summene vises, og
          kohorter med færre enn fem personer er utelatt. Perioden du velger, styrer hvilke kohorter som vises. Aktive
          brukere per team (<code>/usage/team-active-users</code>) bruker teammedlemskapet slik det er i dag i{" "}
          <code>user_teams</code>, også for tidligere måneder. Et team som har vokst, ser derfor ut til å ha hatt flere
          brukere før enn det hadde. Måneder der et team har færre enn fem aktive brukere, vises ikke.
        </>
      }
    >
      <PeriodSelect value={period.value} team={params.team} />
      <Section id="modellfamilier" title="Kostnad per modellfamilie">
        <ModelFamilies token={token} start={start} />
      </Section>
      <Section id="ai-credits-per-bruker" title="AI Credits per bruker">
        <CreditsPerUser token={token} start={start} />
      </Section>
      <Section id="copilot-i-pull-requests" title="Copilot i pull requests">
        <CopilotPRs token={token} start={start} />
      </Section>
      <Section id="kohorter" title="Blir brukerne værende?">
        <Cohorts token={token} start={start} />
      </Section>
      <Section id="team" title="Aktive brukere i et team">
        <TeamActiveUsers token={token} start={start} period={period.value} team={params.team} />
      </Section>
    </InsightPage>
  );
}
