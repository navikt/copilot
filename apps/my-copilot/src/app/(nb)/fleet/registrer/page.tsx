import type { Metadata } from "next";
import { BodyLong } from "@navikt/ds-react";
import { isLocalDev } from "@/lib/backend-api";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { isName } from "@/lib/fleet-names";
import { FleetFrame, fleetRobots } from "../frame";
import { FleetProblemAlert, KortFortalt } from "../fleet-ui";
import { RegisterWizard } from "./wizard";

export const metadata: Metadata = { title: "Registrer MCP-server", robots: fleetRobots };

type Props = { searchParams: Promise<{ team?: string }> };

export default async function RegisterPage({ searchParams }: Props) {
  requireFleet();
  const result = await fleetLoad(() => fleetApi.whoami());
  const { team: asked } = await searchParams;
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
    <FleetFrame title="Registrer MCP-server" description="Send inn en MCP-server til godkjenning.">
      <KortFortalt>
        <BodyLong>
          Seks steg, rundt fem minutter. Ingenting publiseres før en annen på teamet har godkjent. Du kan gå tilbake og
          endre underveis.
        </BodyLong>
      </KortFortalt>
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
