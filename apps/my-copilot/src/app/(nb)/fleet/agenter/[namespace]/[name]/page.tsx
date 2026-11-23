import type { Metadata } from "next";
import { FleetControlError, fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { FleetFrame, fleetRobots } from "../../../frame";
import { AgentDetailView, FleetProblemAlert } from "../../../fleet-ui";

export const metadata: Metadata = { title: "Agentprofil", robots: fleetRobots };

type Props = { params: Promise<{ namespace: string; name: string }> };

export default async function AgentPage({ params }: Props) {
  requireFleet();
  const { namespace, name } = await params;
  const result = await fleetLoad(async () => {
    const detail = await fleetApi.agent(namespace, name);
    // The audit trail is for the owner team only: a route owner sees the profile without it.
    const events = await fleetApi.audit("agents", namespace, name).then(
      (a) => a.events,
      (err) => {
        if (err instanceof FleetControlError && err.status === 404) return null;
        throw err;
      }
    );
    return { detail, events };
  });

  return (
    <FleetFrame title={name} description={`Agentprofil fra ${namespace}`}>
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : (
        <AgentDetailView detail={result.data.detail} events={result.data.events} />
      )}
    </FleetFrame>
  );
}
