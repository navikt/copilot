import type { Metadata } from "next";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { FleetFrame, fleetRobots, teamNavLoader } from "../frame";
import { FleetProblemAlert } from "../fleet-ui";
import { Catalog, NoTeam } from "../catalog";

export const metadata: Metadata = {
  title: "Katalog – Fleet",
  description: "Publiserte MCP-servere fra alle team.",
  robots: fleetRobots,
};

export default async function CatalogPage() {
  requireFleet();
  const navFor = teamNavLoader();
  const result = await fleetLoad(async () => {
    const [me, catalog] = await Promise.all([fleetApi.whoami(), fleetApi.catalog()]);
    return { member: me.teams.length > 0, teams: me.teams, entries: catalog.items };
  });
  return (
    <FleetFrame
      nav={"data" in result ? await navFor(result.data.teams) : undefined}
      title="Katalog"
      description="Publiserte MCP-servere fra alle team. Åpne en MCP-server for å se verktøyene og be eierteamet om tilgang."
    >
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : (
        <>
          {!result.data.member && <NoTeam />}
          <Catalog entries={result.data.entries} />
        </>
      )}
    </FleetFrame>
  );
}
