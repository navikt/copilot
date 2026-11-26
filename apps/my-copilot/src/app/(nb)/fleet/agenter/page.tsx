import type { Metadata } from "next";
import { cookies } from "next/headers";
import { BodyLong } from "@navikt/ds-react";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { TEAM_COOKIE, isName } from "@/lib/fleet-names";
import { FleetFrame, fleetRobots, teamNavLoader } from "../frame";
import { AgentTable } from "../agent";
import { FleetProblemAlert } from "../fleet-ui";
import { PendingInbox } from "../inbox";

export const metadata: Metadata = { title: "Agentprofiler – Fleet", robots: fleetRobots };

type Props = { searchParams: Promise<{ team?: string }> };

/** The team's agent profiles: the side navigation's «Agentprofiler». */
export default async function AgentsPage({ searchParams }: Props) {
  requireFleet();
  const asked = (await searchParams).team ?? (await cookies()).get(TEAM_COOKIE)?.value;
  const navFor = teamNavLoader(asked && isName(asked) ? asked : undefined);
  const result = await fleetLoad(async () => {
    const me = await fleetApi.whoami();
    const team = asked && isName(asked) && me.teams.includes(asked) ? asked : me.teams[0];
    const [agents, pending] = await Promise.all([
      team ? fleetApi.agents(team).then((l) => l.items) : [],
      fleetApi.pending().then((l) => l.items),
    ]);
    return { team, teams: me.teams, agents, pending };
  });
  if ("problem" in result)
    return (
      <FleetFrame title="Agentprofiler" description="Agentprofilene til teamet.">
        <FleetProblemAlert problem={result.problem} />
      </FleetFrame>
    );
  const { team, teams, agents, pending } = result.data;
  return (
    <FleetFrame
      title="Agentprofiler"
      description="Hvilke MCP-servere KI-agentene i teamet får bruke."
      crumbs={team ? [{ label: team, href: `/fleet?team=${team}` }] : []}
      nav={team ? await navFor(teams, {}, team) : undefined}
    >
      {team ? (
        <>
          {pending.length > 0 && <PendingInbox agents={pending} />}
          <AgentTable t={{ team, servers: [], agents }} />
        </>
      ) : (
        <BodyLong>Du må være med i et team i Nais for å ha agentprofiler.</BodyLong>
      )}
    </FleetFrame>
  );
}
