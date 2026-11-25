import type { Metadata } from "next";
import { isName } from "@/lib/fleet-names";
import { notFound } from "next/navigation";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { FleetFrame, fleetRobots } from "../../../frame";
import { FleetProblemAlert, ServerDetail } from "../../../fleet-ui";

export const metadata: Metadata = { title: "MCP-server", robots: fleetRobots };

type Props = { params: Promise<{ namespace: string; name: string }> };

export default async function ServerPage({ params }: Props) {
  requireFleet();
  const { namespace, name } = await params;
  if (!isName(namespace) || !isName(name)) notFound();
  // Both are for the owner team only: anyone else gets 404, which is «Fant ikke siden».
  const result = await fleetLoad(async () => {
    const [server, history, me] = await Promise.all([
      fleetApi.server(namespace, name),
      fleetApi.history("mcpservers", namespace, name),
      fleetApi.whoami(),
    ]);
    return { server, events: history.items, me: me.name };
  });

  return (
    <FleetFrame title={name} description={`MCP-serveren til ${namespace}`}>
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : (
        <ServerDetail server={result.data.server} events={result.data.events} me={result.data.me} />
      )}
    </FleetFrame>
  );
}
