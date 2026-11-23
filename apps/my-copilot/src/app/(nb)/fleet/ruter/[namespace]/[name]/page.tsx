import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { FleetFrame, fleetRobots } from "../../../frame";
import { FleetProblemAlert, RouteDetail } from "../../../fleet-ui";

export const metadata: Metadata = { title: "Rute", robots: fleetRobots };

type Props = { params: Promise<{ namespace: string; name: string }> };

export default async function RoutePage({ params }: Props) {
  requireFleet();
  const { namespace, name } = await params;
  // fleet-control has no single-route read, so the page takes it from the team's list.
  const result = await fleetLoad(async () => {
    const [list, audit] = await Promise.all([fleetApi.servers(namespace), fleetApi.audit("servers", namespace, name)]);
    return { server: list.servers.find((s) => s.name === name), events: audit.events };
  });
  if ("data" in result && !result.data.server) notFound();

  return (
    <FleetFrame title={name} description={`Rute fra ${namespace}`}>
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : (
        <RouteDetail server={result.data.server!} events={result.data.events} />
      )}
    </FleetFrame>
  );
}
