import { Suspense, cache } from "react";
import type { Metadata } from "next";
import { BodyShort, VStack } from "@navikt/ds-react";
import { InsightPage, InsightSection } from "@/components/insight-page";
import TeamGrossUsage from "@/components/team-gross-usage";
import ErrorState from "@/components/error-state";
import { getMyTeams, getTeamGrossOverview, getTeamNetOverview, getTeamYearOverview } from "@/lib/cached-bigquery";
import TeamYearCost, { TeamYearPicker } from "@/components/team-year-cost";
import { getUser, getUserToken } from "@/lib/auth";
import { teamInsightMonth } from "@/lib/month-utils";
import TeamControls from "@/components/team-controls";
import TeamSpendSkeleton from "./team-spend-skeleton";

// Shared by the section and «Sist oppdatert» so the no-store request runs once per render.
const grossOverview = cache(getTeamGrossOverview);
const myTeamsOf = cache(getMyTeams);

export const metadata: Metadata = {
  title: "Teaminnsikt",
  description: "Copilot-bruk og kostnad per team i Nav.",
};

async function TeamSpend({ month, token }: { month: string; token: string }) {
  let gross;
  try {
    gross = await grossOverview(month, token);
  } catch (error) {
    console.error("[team] Gross usage failed:", error);
    return <ErrorState message="Kunne ikke hente teamenes brutto KI-bruk." />;
  }
  if (!gross.last_usage_day) return <BodyShort>Ingen teamdata for denne måneden.</BodyShort>;

  let net = null;
  try {
    net = await getTeamNetOverview(month, token);
  } catch (error) {
    console.error("[team] Net usage failed:", error);
  }
  let myTeams: string[] | null = null;
  try {
    myTeams = await myTeamsOf(token);
  } catch (error) {
    console.error("[team] Caller teams unavailable:", error);
  }
  const comparisonReason = {
    ok: "Endring viser forskjellen fra forrige måned. En strek betyr at teamet ikke kan sammenlignes.",
    current_month: "Endring vises når måneden er ferdig.",
    previous_net_missing: "Endring mangler fordi fakturert forbruk for forrige måned ikke er tilgjengelig.",
    incomplete: "Endring mangler fordi en av månedene har ufullstendige data.",
  }[(net ?? gross).comparison];
  return <TeamGrossUsage data={gross} net={net} myTeams={myTeams} comparisonReason={comparisonReason} />;
}

async function TeamYear({ month, team, token }: { month: string; team: string; token: string }) {
  const year = Number(month.slice(0, 4));
  let teams;
  try {
    teams = (await grossOverview(month, token)).teams;
  } catch {
    return <ErrorState message="Kunne ikke hente teamlisten." />;
  }
  const mine = new Set(((await myTeamsOf(token).catch(() => null)) ?? []).map((slug) => slug.toLowerCase()));
  const own = (slug: string) => (mine.has(slug.toLowerCase()) ? 0 : 1);
  const sorted = [...teams].sort(
    (a, b) => own(a.team_slug) - own(b.team_slug) || a.team_slug.localeCompare(b.team_slug, "nb")
  );
  let data = null;
  if (team) {
    try {
      data = await getTeamYearOverview(team, year, token);
    } catch (error) {
      console.error("[team] Year usage failed:", error);
      return <ErrorState message="Kunne ikke hente forbruket for teamet." />;
    }
  }
  return (
    <VStack gap="space-24">
      <BodyShort>
        Forbruk per måned i {year} for ett team om gangen, regnet på samme måte som i månedsoversikten. Listen viser
        dine team først, og bare team der minst fem medlemmer hadde forbruk i måneden du har valgt.
      </BodyShort>
      <TeamYearPicker teams={sorted} team={team} month={month} />
      {data ? <TeamYearCost data={data} /> : <BodyShort>Velg et team for å se forbruket per måned.</BodyShort>}
    </VStack>
  );
}

export default async function TeamPage({ searchParams }: { searchParams: Promise<{ month?: string; team?: string }> }) {
  await getUser();
  const token = await getUserToken();
  const { month: requestedMonth, team: requestedTeam } = await searchParams;
  const team = requestedTeam && /^[1-9][0-9]{0,19}$/.test(requestedTeam) ? requestedTeam : "";
  const month = teamInsightMonth(requestedMonth);

  if (!token) return <ErrorState message="Mangler innloggingstoken" />;

  return (
    <InsightPage
      title="Teaminnsikt"
      description="Copilot-bruk og kostnad per team i Nav."
      intro="Se hva hvert team bruker på Copilot i en måned, og hvordan det endrer seg fra forrige måned. Velg måned for å se tidligere tall."
      updated={async () => (await grossOverview(month, token)).last_usage_day}
      source={
        <>
          Brutto bruk kommer fra <code>/usage/team-gross</code>, og fakturert forbruk fra <code>/usage/team-net</code>.
          Begge viser én kalendermåned, tidligst mai 2026. Brutto bruk oppdateres daglig, og «Sist oppdatert» er siste
          dag med brutto bruk i måneden du har valgt. Fakturert forbruk finnes først når måneden er avsluttet og
          fakturaen er lest inn, og fordelingen på dager er et anslag. En person som er med i flere team, telles i hvert
          av dem. «Hittil i år» kommer fra <code>/usage/team-year</code> og regner hver måned på samme måte.
        </>
      }
    >
      <InsightSection id="teamkostnad" title="Kostnad og bruk">
        <TeamControls month={month}>
          <Suspense key={month} fallback={<TeamSpendSkeleton />}>
            <TeamSpend month={month} token={token} />
          </Suspense>
        </TeamControls>
      </InsightSection>
      <InsightSection id="hittil-i-ar" title="Hittil i år">
        <Suspense key={`${month}-${team}`} fallback={<TeamSpendSkeleton />}>
          <TeamYear month={month} team={team} token={token} />
        </Suspense>
      </InsightSection>
    </InsightPage>
  );
}
