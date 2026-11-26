import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocalDev } from "@/lib/backend-api";
import { isName, hasCondition } from "@/lib/fleet-names";
import { FleetControlError, fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { FleetFrame, fleetRobots, teamNavLoader } from "../../../../frame";
import { FleetProblemAlert } from "../../../../fleet-ui";
import { EditServer } from "../../../../server-edit";
import { CHANGE_RULE } from "@/lib/fleet-approval";

export const metadata: Metadata = { title: "Endre MCP-server", robots: fleetRobots };

type Props = { params: Promise<{ namespace: string; name: string }> };

export default async function EditServerPage({ params }: Props) {
  requireFleet();
  const { namespace, name } = await params;
  if (!isName(namespace) || !isName(name)) notFound();
  const navFor = teamNavLoader(namespace);
  const result = await fleetLoad(async () => {
    const server = await fleetApi.server(namespace, name).catch((err: unknown) => {
      if (err instanceof FleetControlError && err.status === 404) return null;
      throw err;
    });
    const me = await fleetApi.whoami();
    // Only the owner team edits; fleet-control checks it again.
    if (!server || !me.teams.includes(namespace)) return null;
    const apps = await fleetApi.apps(namespace).then(
      (l) => l.items,
      () => null
    );
    return { server, teams: me.teams, apps };
  });
  if ("data" in result && !result.data) notFound();
  // Same rule as fleet-control: outside dev-*, write tools need a ROS. Local dev counts as dev.
  const requireRos = !isLocalDev() && !(process.env.NAIS_CLUSTER_NAME ?? "").startsWith("dev-");
  const data = "data" in result ? result.data : null;
  // A revoked or never published server has nothing to compare with: everything waits for team copilot.
  const live = data && data.server.status.publishedGeneration && !hasCondition(data.server, "Revoked");
  return (
    <FleetFrame
      title={`Endre ${name}`}
      crumbs={[
        { label: namespace, href: `/fleet?team=${namespace}` },
        { label: name, href: `/fleet/mcp-servere/${namespace}/${name}` },
      ]}
      description={`Endringen blir en ny versjon. ${CHANGE_RULE}`}
      nav={data ? await navFor(data.teams) : undefined}
    >
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : (
        data && (
          <EditServer
            server={data.server}
            published={live ? data.server.spec : undefined}
            apps={data.apps}
            requireRos={requireRos}
            discoveryEnabled={!!process.env.FLEET_GATEWAY_URL}
          />
        )
      )}
    </FleetFrame>
  );
}
