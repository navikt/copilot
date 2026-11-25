import type { Metadata } from "next";
import { BodyLong } from "@navikt/ds-react";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { FleetFrame, fleetRobots } from "../frame";
import { FleetProblemAlert, KortFortalt, ServerForm } from "../fleet-ui";

export const metadata: Metadata = { title: "Registrer MCP-server", robots: fleetRobots };

export default async function RegisterPage() {
  requireFleet();
  const result = await fleetLoad(() => fleetApi.whoami());
  return (
    <FleetFrame title="Registrer MCP-server" description="Send inn en MCP-server til godkjenning.">
      <KortFortalt>
        <BodyLong>
          Du sender inn et utkast. En annen person på teamet må godkjenne det før MCP-serveren blir publisert på ruten{" "}
          <code>/mcp/team/navn</code>.
        </BodyLong>
      </KortFortalt>
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : result.data.teams.length === 0 ? (
        <BodyLong>Du må være med i et team i Nais for å registrere en MCP-server.</BodyLong>
      ) : (
        <ServerForm teams={result.data.teams} />
      )}
    </FleetFrame>
  );
}
