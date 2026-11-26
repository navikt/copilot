import type { Metadata } from "next";
import { cookies } from "next/headers";
import { BodyLong } from "@navikt/ds-react";
import { fullName, getUser } from "@/lib/auth";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { TEAM_COOKIE, isName } from "@/lib/fleet-names";
import { FleetFrame, fleetRobots, teamNavLoader } from "../frame";
import { APPROVAL_RULE, PLATFORM_TEAM } from "@/lib/fleet-approval";
import { LinkText } from "../nav";
import { FleetProblemAlert, ServerInbox } from "../fleet-ui";

export const metadata: Metadata = { title: "Til godkjenning – Fleet", robots: fleetRobots };

type Props = { searchParams: Promise<{ team?: string }> };

/** What waits on the team: the side navigation's «Til godkjenning». */
export default async function ApprovalPage({ searchParams }: Props) {
  requireFleet();
  const asked = (await searchParams).team ?? (await cookies()).get(TEAM_COOKIE)?.value;
  const navFor = teamNavLoader(asked && isName(asked) ? asked : undefined);
  const result = await fleetLoad(async () => {
    const me = await fleetApi.whoami();
    const team = asked && isName(asked) && me.teams.includes(asked) ? asked : me.teams[0];
    const [servers, pending] = await Promise.all([
      team ? fleetApi.servers(team).then((l) => l.items) : [],
      fleetApi.pending().then((l) => l.items),
    ]);
    return { team, teams: me.teams, me: me.name, approver: me.teams.includes(PLATFORM_TEAM), servers, pending };
  });
  const description = APPROVAL_RULE;
  if ("problem" in result)
    return (
      <FleetFrame title="Til godkjenning" description={description}>
        <FleetProblemAlert problem={result.problem} />
      </FleetFrame>
    );
  const { team, teams, me, approver, servers, pending } = result.data;
  const waiting = servers.some((s) => s.status.pending);
  return (
    <FleetFrame
      title="Til godkjenning"
      description={description}
      crumbs={team ? [{ label: team, href: `/fleet?team=${team}` }] : []}
      nav={team ? await navFor(teams, {}, team) : undefined}
    >
      {!team ? (
        <BodyLong>Du må være med i et team i Nais for å godkjenne endringer.</BodyLong>
      ) : (
        <>
          <ServerInbox servers={servers} me={me} myName={fullName(await getUser(false))} approver={approver} />
          {pending.length > 0 && (
            <BodyLong>
              {pending.length === 1 ? "1 agentprofil venter" : `${pending.length} agentprofiler venter`} på at dere gir
              tilgang. <LinkText href={`/fleet/agenter?team=${team}#venter-pa-tilgang`}>Se agentprofilene</LinkText>
            </BodyLong>
          )}
          {!waiting && pending.length === 0 && <BodyLong>Ingenting venter på {team} nå.</BodyLong>}
        </>
      )}
    </FleetFrame>
  );
}
