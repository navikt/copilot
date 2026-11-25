import type { Metadata } from "next";
import NextLink from "next/link";
import { BodyLong } from "@navikt/ds-react";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { FleetFrame, fleetRobots } from "./frame";
import { FleetProblemAlert, KortFortalt, PendingInbox, TeamSections } from "./fleet-ui";
import { Catalog, NoTeam, TeamPicker } from "./catalog";

export const metadata: Metadata = {
  title: "Fleet",
  description: "MCP-serverne og agentprofilene til teamene dine.",
  robots: fleetRobots,
};

type Props = { searchParams: Promise<{ team?: string }> };

export default async function FleetPage({ searchParams }: Props) {
  requireFleet();
  const { team: picked } = await searchParams;
  const result = await fleetLoad(async () => {
    const me = await fleetApi.whoami();
    const [teams, pending, catalog] = await Promise.all([
      Promise.all(
        me.teams.map(async (team) => {
          const [s, a] = await Promise.all([fleetApi.servers(team), fleetApi.agents(team)]);
          return { team, servers: s.items, agents: a.items };
        })
      ),
      fleetApi.pending(),
      fleetApi.catalog(),
    ]);
    // A team with nothing in Fleet gets no section; registering starts at /fleet/registrer.
    const used = teams.filter((t) => t.servers.length > 0 || t.agents.length > 0);
    return { member: me.teams.length > 0, used, pending: pending.items, catalog: catalog.items };
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
      ) : (
        <>
          {result.data.member ? (
            <>
              <BodyLong>
                <NextLink className="text-blue-600 hover:underline" href="/fleet/registrer">
                  Registrer en MCP-server
                </NextLink>
              </BodyLong>
              <PendingInbox agents={result.data.pending} />
              <TeamPicker teams={result.data.used.map((t) => t.team)} current={picked} />
              <TeamSections teams={result.data.used.filter((t) => !picked || t.team === picked)} />
            </>
          ) : (
            <NoTeam />
          )}
          <Catalog entries={result.data.catalog} />
        </>
      )}
    </FleetFrame>
  );
}
