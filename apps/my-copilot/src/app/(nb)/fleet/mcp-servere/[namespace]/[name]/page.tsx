import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { fleetApi, fleetLoad, isName, requireFleet } from "@/lib/fleet-control";
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
    const [server, history] = await Promise.all([
      fleetApi.server(namespace, name),
      fleetApi.history("mcpservers", namespace, name),
    ]);
    return { server, events: history.items };
  });

  return (
    <FleetFrame title={name} description={`MCP-serveren til ${namespace}`}>
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : (
        <ServerDetail server={result.data.server} events={result.data.events} />
      )}
    </FleetFrame>
  );
}
