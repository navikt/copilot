import type { Metadata } from "next";
import { BodyLong } from "@navikt/ds-react";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { FleetFrame, fleetRobots } from "./frame";
import { FleetProblemAlert, KortFortalt, PendingInbox, TeamTabs } from "./fleet-ui";

export const metadata: Metadata = {
  title: "Fleet",
  description: "MCP-serverne og agentprofilene til teamene dine.",
  robots: fleetRobots,
};

export default async function FleetPage() {
  requireFleet();
  const result = await fleetLoad(async () => {
    const me = await fleetApi.whoami();
    const [teams, pending] = await Promise.all([
      Promise.all(
        me.teams.map(async (team) => {
          const [s, a] = await Promise.all([fleetApi.servers(team), fleetApi.agents(team)]);
          return { team, servers: s.items, agents: a.items };
        })
      ),
      fleetApi.pending(),
    ]);
    return { teams, pending: pending.items };
  });

  return (
    <FleetFrame title="Fleet" description="MCP-serverne og agentprofilene til teamene dine.">
      <KortFortalt>
        <BodyLong>
          En <strong>MCP-server</strong> blir tilgjengelig gjennom fleet-gateway på en <strong>rute</strong>, adressen{" "}
          <code>/mcp/team/navn</code>. En <strong>agentprofil</strong> bestemmer hvilken app som får bruke hvilke
          verktøy på hvilke MCP-servere, på vegne av en innlogget bruker.
        </BodyLong>
        <BodyLong>
          En annen person enn den som skrev endringen, må godkjenne den. Da har alltid to personer sett endringen før
          den gjelder. Eierteamet til en MCP-server gir tilgang til agentprofiler som vil bruke den.
        </BodyLong>
      </KortFortalt>
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : result.data.teams.length === 0 ? (
        <BodyLong>
          Du er ikke med i noe team i Nais ennå. Når du blir med i et team, ser du MCP-serverne og agentprofilene her.
        </BodyLong>
      ) : (
        <>
          <PendingInbox agents={result.data.pending} />
          <TeamTabs teams={result.data.teams} />
        </>
      )}
    </FleetFrame>
  );
}
