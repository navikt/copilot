import type { Metadata } from "next";
import { isName } from "@/lib/fleet-names";
import { notFound } from "next/navigation";
import { FleetControlError, fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { FleetFrame, fleetRobots, teamNavLoader } from "../../../frame";
import { AgentDetailView, AgentStatusTag, FleetProblemAlert } from "../../../fleet-ui";

export const metadata: Metadata = { title: "Agentprofil", robots: fleetRobots };

type Props = { params: Promise<{ namespace: string; name: string }> };

export default async function AgentPage({ params }: Props) {
  requireFleet();
  const { namespace, name } = await params;
  if (!isName(namespace) || !isName(name)) notFound();
  const navFor = teamNavLoader(namespace);
  const result = await fleetLoad(async () => {
    const [profile, me] = await Promise.all([fleetApi.agent(namespace, name), fleetApi.whoami()]);
    // The history is for the owner team only: an MCP server's owner team sees the profile without it.
    const events = await fleetApi.history("agentprofiles", namespace, name).then(
      (h) => h.items,
      (err) => {
        if (err instanceof FleetControlError && err.status === 404) return null;
        throw err;
      }
    );
    return { profile, events, teams: me.teams };
  });

  return (
    <FleetFrame
      nav={"data" in result ? await navFor(result.data.teams) : undefined}
      title={name}
      description={`Agentprofil i ${namespace}`}
      crumbs={[
        { label: namespace, href: `/fleet?team=${namespace}` },
        { label: "Agentprofiler", href: `/fleet/agenter?team=${namespace}` },
      ]}
      titleTag={"data" in result ? <AgentStatusTag profile={result.data.profile} /> : undefined}
    >
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : (
        <AgentDetailView profile={result.data.profile} events={result.data.events} />
      )}
    </FleetFrame>
  );
}
