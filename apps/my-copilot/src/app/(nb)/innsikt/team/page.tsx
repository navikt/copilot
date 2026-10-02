import { Suspense } from "react";
import type { Metadata } from "next";
import { BodyShort, Box, Heading, Skeleton } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import TeamGrossUsage from "@/components/team-gross-usage";
import ErrorState from "@/components/error-state";
import { getMyTeams, getTeamGrossOverview, getTeamNetOverview } from "@/lib/cached-bigquery";
import { getUser, getUserToken } from "@/lib/auth";
import { currentMonthUTC, daysInCalendarMonth, previousMonth } from "@/lib/month-utils";
import TeamMonthPicker from "@/components/team-month-picker";

export const metadata: Metadata = {
  title: "Teaminnsikt",
  description: "Copilot-bruk og kostnad per team i Nav.",
};

async function TeamSpend({ month, token }: { month: string; token: string }) {
  let gross;
  try {
    gross = await getTeamGrossOverview(month, token);
  } catch (error) {
    console.error("[team] Gross usage failed:", error);
    return <ErrorState message="Kunne ikke hente teamenes brutto AI-bruk." />;
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
  const previous = previousMonth(month);
  let previousGross = null;
  let previousNet = null;
  if (previous >= "2026-05") {
    try {
      previousGross = await getTeamGrossOverview(previous, token);
      if (net) previousNet = await getTeamNetOverview(previous, token);
    } catch (error) {
      console.error("[team] Previous month unavailable:", error);
    }
  }
  const fullMonths =
    month < currentMonthUTC() &&
    gross.days_with_usage === daysInCalendarMonth(month) &&
    previousGross?.days_with_usage === daysInCalendarMonth(previous);
  const comparison = fullMonths && (net ? previousNet : previousGross) ? (net ? previousNet : previousGross) : null;
  const comparisonReason = comparison
    ? "Endring viser forskjellen fra forrige måned. En strek betyr at teamet ikke kan sammenlignes."
    : net && previousGross?.last_usage_day && !previousNet
      ? "Endring mangler fordi fakturert forbruk for forrige måned ikke er tilgjengelig."
      : "Endring vises når begge måneder har komplette og sammenlignbare data.";
  return (
    <TeamGrossUsage
      data={gross}
      net={net}
      myTeams={myTeams}
      previous={comparison}
      comparisonReason={comparisonReason}
    />
  );
}

export default async function TeamPage({ searchParams }: { searchParams: Promise<{ month?: string }> }) {
  await getUser();
  const token = await getUserToken();
  const { month: requestedMonth } = await searchParams;
  const month =
    requestedMonth &&
    /^20\d\d-(0[1-9]|1[0-2])$/.test(requestedMonth) &&
    requestedMonth >= "2026-05" &&
    requestedMonth <= currentMonthUTC()
      ? requestedMonth
      : previousMonth(currentMonthUTC());

  if (!token) return <ErrorState message="Mangler innloggingstoken" />;

  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero title="Teaminnsikt" description="Copilot-bruk og kostnad per team i Nav." />
      <Box
        paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <section aria-labelledby="teamkostnad">
          <Heading id="teamkostnad" level="2" size="medium" spacing>
            Kostnad og bruk
          </Heading>
          <TeamMonthPicker month={month} />
          <Suspense fallback={<Skeleton variant="rectangle" height={200} />}>
            <TeamSpend month={month} token={token} />
          </Suspense>
        </section>
      </Box>
    </main>
  );
}
