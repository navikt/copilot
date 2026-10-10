import { Suspense, cache } from "react";
import type { Metadata } from "next";
import { BodyShort } from "@navikt/ds-react";
import { InsightPage, InsightSection } from "@/components/insight-page";
import TeamGrossUsage from "@/components/team-gross-usage";
import ErrorState from "@/components/error-state";
import { getMyTeams, getTeamGrossOverview, getTeamNetOverview } from "@/lib/cached-bigquery";
import { getUser, getUserToken } from "@/lib/auth";
import { teamInsightMonth } from "@/lib/month-utils";
import TeamControls from "@/components/team-controls";
import TeamSpendSkeleton from "./team-spend-skeleton";

// Shared by the section and «Sist oppdatert» so the no-store request runs once per render.
const grossOverview = cache(getTeamGrossOverview);

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
    myTeams = await getMyTeams(token);
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

export default async function TeamPage({ searchParams }: { searchParams: Promise<{ month?: string }> }) {
  await getUser();
  const token = await getUserToken();
  const { month: requestedMonth } = await searchParams;
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
          av dem.
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
    </InsightPage>
  );
}
