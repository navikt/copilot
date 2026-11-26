import type { Metadata } from "next";
import { BodyLong } from "@navikt/ds-react";
import { isLocalDev } from "@/lib/backend-api";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { isName } from "@/lib/fleet-names";
import { FleetFrame, fleetRobots, teamNavLoader } from "../frame";
import { FleetProblemAlert } from "../fleet-ui";
import { RegisterWizard } from "./wizard";

export const metadata: Metadata = { title: "Registrer MCP-server", robots: fleetRobots };

type Props = { searchParams: Promise<{ team?: string }> };

export default async function RegisterPage({ searchParams }: Props) {
  requireFleet();
  const { team: asked } = await searchParams;
  const navFor = teamNavLoader(asked && isName(asked) ? asked : undefined);
  const result = await fleetLoad(() => fleetApi.whoami());
  const teams = "data" in result ? result.data.teams : [];
  const team = asked && isName(asked) && teams.includes(asked) ? asked : teams[0];
  // A failed app list is not a stop: the wizard asks for the app name instead.
  const apps = team
    ? await fleetApi.apps(team).then(
        (l) => l.items,
        () => null
      )
    : null;
  // Same rule as fleet-control: outside dev-*, write tools need a ROS. Local dev counts as dev.
  const requireRos = !isLocalDev() && !(process.env.NAIS_CLUSTER_NAME ?? "").startsWith("dev-");
  return (
    <FleetFrame
      nav={await navFor(teams, {}, team)}
      crumbs={team ? [{ label: team, href: `/fleet?team=${team}` }] : []}
      title="Registrer MCP-server"
      description={`${team ? `For ${team}. ` : ""}Tar rundt fem minutter. Ingenting publiseres før team copilot har godkjent.`}
    >
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : !team ? (
        <BodyLong>Du må være med i et team i Nais for å registrere en MCP-server.</BodyLong>
      ) : (
        <RegisterWizard
          key={team}
          teams={teams}
          team={team}
          apps={apps}
          requireRos={requireRos}
          discoveryEnabled={!!process.env.FLEET_GATEWAY_URL}
        />
      )}
    </FleetFrame>
  );
}
