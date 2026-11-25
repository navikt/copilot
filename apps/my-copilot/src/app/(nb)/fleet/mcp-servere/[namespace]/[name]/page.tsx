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
    if (!server) return { owner: false as const, entry: await fleetApi.catalogEntry(namespace, name) };
    const [history, me] = await Promise.all([fleetApi.history("mcpservers", namespace, name), fleetApi.whoami()]);
    return { owner: true as const, server, events: history.items, me: me.name };
  });

  return (
    <FleetFrame title={name} description={`MCP-serveren til ${namespace}`}>
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : !result.data.owner ? (
        <ServerReadOnly entry={result.data.entry} />
      ) : (
        <ServerDetail server={result.data.server} events={result.data.events} me={result.data.me} />
      )}
    </FleetFrame>
  );
}
