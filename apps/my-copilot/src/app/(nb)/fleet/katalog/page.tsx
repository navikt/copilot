import type { Metadata } from "next";
import { fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { FleetFrame, fleetRobots } from "../frame";
import { FleetProblemAlert } from "../fleet-ui";
import { Catalog, NoTeam } from "../catalog";

export const metadata: Metadata = {
  title: "Katalog – Fleet",
  description: "Publiserte MCP-servere fra alle team.",
  robots: fleetRobots,
};

export default async function CatalogPage() {
  requireFleet();
  const result = await fleetLoad(async () => {
    const [me, catalog] = await Promise.all([fleetApi.whoami(), fleetApi.catalog()]);
    return { member: me.teams.length > 0, entries: catalog.items };
  });
  return (
    <FleetFrame title="Katalog" description="Publiserte MCP-servere fra alle team.">
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
