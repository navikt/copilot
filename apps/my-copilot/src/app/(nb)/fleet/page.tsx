import type { Metadata } from "next";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { BodyLong, Heading, HStack, VStack } from "@navikt/ds-react";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { TEAM_COOKIE } from "@/lib/fleet-names";
import { FleetFrame, fleetRobots } from "./frame";
import {
  FleetProblemAlert,
  KortFortalt,
  PendingInbox,
  RegisterButton,
  ServerInbox,
  TeamSections,
  TeamStats,
} from "./fleet-ui";
import { TeamChooser, UnknownTeam } from "./catalog";

const DNS_LABEL = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/;

export const metadata: Metadata = {
  title: "Fleet",
  description: "MCP-serverne og agentprofilene til teamet ditt.",
  robots: fleetRobots,
};

type Props = { searchParams: Promise<{ team?: string }> };

export default async function FleetPage({ searchParams }: Props) {
  requireFleet();
  const { team } = await searchParams;
  // ?team= (empty) asks for the chooser; no ?team falls back to the remembered team.
  const last = (await cookies()).get(TEAM_COOKIE)?.value;
  const asked = team ?? last;
  const wanted = asked && DNS_LABEL.test(asked) ? asked : undefined;
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
    return { teams, pending: pending.items, me: me.name };
  });
  if ("problem" in result) {
    return (
      <FleetFrame title="Fleet" description="MCP-serverne og agentprofilene til teamet ditt.">
        <FleetProblemAlert problem={result.problem} />
      </FleetFrame>
    );
  }
  const { teams, pending, me } = result.data;
  // No team: the catalogue explains how to start.
  if (teams.length === 0) redirect("/fleet/katalog");
  const remembered = team === undefined ? teams.find((t) => t.team === wanted) : undefined;
  const picked = team
    ? teams.find((t) => t.team === wanted)
    : (remembered ?? (team === undefined && teams.length === 1 ? teams[0] : undefined));
  const unknown = team && !picked ? team : undefined;

  return (
    <FleetFrame
      title={picked ? picked.team : "Velg team"}
      description={
        picked
          ? "MCP-servere teamet eier, og det som venter på dere."
          : "Du er med i flere team. Fleet viser alltid ett team om gangen. Vi husker valget ditt til neste gang."
      }
      crumbs={picked ? [] : false}
      action={picked ? <RegisterButton team={picked.team} /> : undefined}
      nav={
        picked
          ? {
              team: picked.team,
              teams: teams.map((t) => t.team),
              pending: pending.length + picked.servers.filter((s) => s.status.pending).length,
              hasAgents: picked.agents.length > 0,
            }
          : undefined
      }
    >
      {picked ? (
        <>
          <TeamStats servers={picked.servers} me={me} />
          {(pending.length > 0 || picked.servers.some((s) => s.status.pending)) && (
            <VStack gap="space-12">
              <HStack justify="space-between" align="end" gap="space-8">
                <Heading id="til-godkjenning" level="2" size="medium">
                  Til godkjenning
                </Heading>
                <BodyLong size="small">
                  Endringer må godkjennes av to personer i eierteamet: én sender inn, en annen godkjenner.
                </BodyLong>
              </HStack>
              <ServerInbox servers={picked.servers} me={me} />
              {pending.length > 0 && <PendingInbox agents={pending} />}
            </VStack>
          )}
          <TeamSections teams={[picked]} me={me} />
          <KortFortalt collapsed>
            <BodyLong>
              Teamet registrerer en <strong>MCP-server</strong>. En annen i teamet godkjenner. Da får MCP-serveren en{" "}
              <strong>rute</strong> i fleet-gateway, adressen <code>/mcp/team/navn</code>, og team dere gir tilgang, kan
              bruke den.
            </BodyLong>
            <BodyLong>
              En <strong>agentprofil</strong> bestemmer hvilken app som får bruke hvilke verktøy på hvilke MCP-servere,
              på vegne av en innlogget bruker. Eierteamet til MCP-serveren gir tilgang.
            </BodyLong>
          </KortFortalt>
        </>
      ) : (
        <>
          {unknown && <UnknownTeam team={unknown} />}
          <TeamChooser
            last={last}
            teams={teams.map((t) => ({
              team: t.team,
              servers: t.servers.length,
              pending: t.servers.filter((s) => s.status.pending).length,
            }))}
          />
        </>
      )}
    </FleetFrame>
  );
}
