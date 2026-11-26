import type { Metadata } from "next";
import { cookies } from "next/headers";
import { BodyLong } from "@navikt/ds-react";
import { fullName, getUser } from "@/lib/auth";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { TEAM_COOKIE, isName } from "@/lib/fleet-names";
import { FleetFrame, fleetRobots, teamNavLoader } from "../frame";
import { APPROVAL_RULE, PLATFORM_TEAM } from "@/lib/fleet-approval";
import { LinkText } from "../nav";
import { FleetProblemAlert } from "../fleet-ui";
import { OwnerFilter, ServerInbox } from "../inbox";

export const metadata: Metadata = { title: "Til godkjenning – Fleet", robots: fleetRobots };

type Props = { searchParams: Promise<{ team?: string; eier?: string }> };

/**
 * What waits on the team: the side navigation's «Til godkjenning».
 * The platform team sees every team's pending versions, with a filter by owner team (#3095).
 */
export default async function ApprovalPage({ searchParams }: Props) {
  requireFleet();
  const params = await searchParams;
  const asked = params.team ?? (await cookies()).get(TEAM_COOKIE)?.value;
  const navFor = teamNavLoader(asked && isName(asked) ? asked : undefined);
  const result = await fleetLoad(async () => {
    const me = await fleetApi.whoami();
    const team = asked && isName(asked) && me.teams.includes(asked) ? asked : me.teams[0];
    const approver = me.teams.includes(PLATFORM_TEAM);
    // Others never ask for other teams' servers; fleet-control refuses them anyway.
    const [servers, pending] = await Promise.all([
      approver ? fleetApi.myServers().then((l) => l.items) : team ? fleetApi.servers(team).then((l) => l.items) : [],
      fleetApi.pending().then((l) => l.items),
    ]);
    return { team, teams: me.teams, me: me.name, approver, servers, pending };
  });
  const description = APPROVAL_RULE;
  if ("problem" in result)
    return (
      <FleetFrame title="Til godkjenning" description={description}>
        <FleetProblemAlert problem={result.problem} />
      </FleetFrame>
    );
  const { team, teams, me, approver, servers, pending } = result.data;
  const all = servers.filter((s) => s.status.pending);
  const owners = [...new Set(all.map((s) => s.metadata.namespace))].sort();
  const owner = approver && params.eier && owners.includes(params.eier) ? params.eier : undefined;
  const shown = owner ? all.filter((s) => s.metadata.namespace === owner) : all;
  const waiting = shown.length > 0;
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
          {approver && owners.length > 1 && <OwnerFilter team={team} owners={owners} owner={owner} />}
          <ServerInbox
            servers={shown}
            me={me}
            myName={fullName(await getUser(false))}
            approver={approver}
            showTeam={approver}
          />
          {pending.length > 0 && (
            <BodyLong>
              {pending.length === 1 ? "1 agentprofil venter" : `${pending.length} agentprofiler venter`} på at dere gir
              tilgang. <LinkText href={`/fleet/agenter?team=${team}#venter-pa-tilgang`}>Se agentprofilene</LinkText>
            </BodyLong>
          )}
          {!waiting && pending.length === 0 && (
            <BodyLong>{approver ? "Ingenting venter på godkjenning nå." : `Ingenting venter på ${team} nå.`}</BodyLong>
          )}
        </>
      )}
    </FleetFrame>
  );
}
