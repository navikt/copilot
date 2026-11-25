import type { Metadata } from "next";
import { isName } from "@/lib/fleet-names";
import { notFound } from "next/navigation";
import { FleetControlError, fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { FleetFrame, fleetRobots } from "../../../frame";
import { FleetProblemAlert, ServerDetail } from "../../../fleet-ui";
import { ServerReadOnly } from "../../../catalog";

export const metadata: Metadata = { title: "MCP-server", robots: fleetRobots };

type Props = { params: Promise<{ namespace: string; name: string }> };

export default async function ServerPage({ params }: Props) {
  requireFleet();
  const { namespace, name } = await params;
  if (!isName(namespace) || !isName(name)) notFound();
  // The server and its history are for the owner team only. Anyone else gets 404
  // and sees the catalogue entry; an unpublished server stays «Fant ikke siden».
  const result = await fleetLoad(async () => {
    // Only a 404 on the server itself means another team's server; whoami and history must not decide.
    const server = await fleetApi.server(namespace, name).catch((err: unknown) => {
      if (err instanceof FleetControlError && err.status === 404) return null;
      throw err;
    });
    if (!server) {
      const [entry, me] = await Promise.all([fleetApi.catalogEntry(namespace, name), fleetApi.whoami()]);
      return { owner: false as const, entry, myTeams: me.teams };
    }
    const [history, me, pending] = await Promise.all([
      fleetApi.history("mcpservers", namespace, name),
      fleetApi.whoami(),
      // The agent profiles waiting for «gi tilgang»; a failure shows a warning, not an empty list.
      fleetApi.pending().catch(() => null),
    ]);
    return {
      owner: true as const,
      server,
      events: history.items,
      me: me.name,
      // The platform team also sees the server; only the owner team gets the edit form.
      inTeam: me.teams.includes(namespace),
      agents: pending?.items ?? [],
      agentsFailed: pending === null,
    };
  });

  return (
    <FleetFrame title={name} description={`MCP-serveren til ${namespace}`}>
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : !result.data.owner ? (
        <ServerReadOnly entry={result.data.entry} myTeams={result.data.myTeams} />
      ) : (
        <ServerDetail
          server={result.data.server}
          events={result.data.events}
          me={result.data.me}
          owner={result.data.inTeam}
          agents={result.data.agents}
          agentsFailed={result.data.agentsFailed}
        />
      )}
    </FleetFrame>
  );
}
