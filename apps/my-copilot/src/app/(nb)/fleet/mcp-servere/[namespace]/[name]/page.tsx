import type { Metadata } from "next";
import { isName } from "@/lib/fleet-names";
import { notFound } from "next/navigation";
import { FleetControlError, fleetApi, fleetLoad, requireFleet } from "@/lib/fleet-control";
import { Tag } from "@navikt/ds-react";
import { FleetFrame, fleetRobots, teamNavLoader } from "../../../frame";
import { LinkButton } from "../../../nav";
import { FleetProblemAlert, ServerDetail, ServerHeader, ServerStatusTag } from "../../../fleet-ui";
import { EntryHeader, ServerReadOnly } from "../../../catalog";
import { fleetGrafanaUrl } from "@/lib/fleet-grafana";

export const metadata: Metadata = { title: "MCP-server", robots: fleetRobots };

type Props = {
  params: Promise<{ namespace: string; name: string }>;
  searchParams?: Promise<{ sendt?: string }>;
};

export default async function ServerPage({ params, searchParams }: Props) {
  requireFleet();
  const { namespace, name } = await params;
  const submitted = (await searchParams)?.sendt === "1";
  if (!isName(namespace) || !isName(name)) notFound();
  const navFor = teamNavLoader(namespace);
  // The server and its history are for the owner team only. Anyone else gets 404
  // and sees the catalogue entry; an unpublished server stays «Fant ikke siden».
  const result = await fleetLoad(async () => {
    // Only a 404 on the server itself means another team's server; whoami and history must not decide.
    const server = await fleetApi.server(namespace, name).catch((err: unknown) => {
      if (err instanceof FleetControlError && err.status === 404) return null;
      throw err;
    });
    if (!server) {
      const [entry, me] = await Promise.all([fleetApi.catalogEntry(namespace, name), fleetApi.whoami()]);
      return { owner: false as const, entry, myTeams: me.teams };
    }
    const [history, me, pending] = await Promise.all([
      fleetApi.history("mcpservers", namespace, name),
      fleetApi.whoami(),
      // The agent profiles waiting for «gi tilgang»; a failure shows a warning, not an empty list.
      fleetApi.pending().catch(() => null),
    ]);
    return {
      owner: true as const,
      server,
      events: history.items,
      me: me.name,
      teams: me.teams,
      // The platform team also sees the server; only the owner team gets the edit form.
      inTeam: me.teams.includes(namespace),
      agents: pending?.items ?? [],
      agentsFailed: pending === null,
    };
  });

  return (
    <FleetFrame
      title={name}
      crumbs={
        "data" in result && !result.data.owner
          ? [{ label: "Katalog", href: "/fleet/katalog" }]
          : [{ label: namespace, href: `/fleet?team=${namespace}` }]
      }
      titleTag={
        "data" in result ? (
          result.data.owner ? (
            <ServerStatusTag server={result.data.server} />
          ) : (
            <Tag size="small" variant="success">
              Publisert
            </Tag>
          )
        ) : undefined
      }
      action={
        "data" in result && !result.data.owner ? (
          <LinkButton href="#be-om-tilgang">Be eierteamet om tilgang</LinkButton>
        ) : undefined
      }
      description={
        "data" in result ? (
          result.data.owner ? (
            <ServerHeader server={result.data.server} />
          ) : (
            <EntryHeader entry={result.data.entry} />
          )
        ) : (
          `MCP-serveren til ${namespace}`
        )
      }
      nav={
        "data" in result
          ? await navFor(result.data.owner ? result.data.teams : result.data.myTeams, {
              inCatalog: !result.data.owner,
            })
          : undefined
      }
    >
      {"problem" in result ? (
        <FleetProblemAlert problem={result.problem} />
      ) : !result.data.owner ? (
        <ServerReadOnly
          entry={result.data.entry}
          myTeams={result.data.myTeams}
          grafanaUrl={result.data.entry.status.route && fleetGrafanaUrl(result.data.entry.status.route.path)}
        />
      ) : (
        <ServerDetail
          submitted={submitted}
          gatewayUrl={process.env.FLEET_GATEWAY_URL}
          server={result.data.server}
          events={result.data.events}
          me={result.data.me}
          owner={result.data.inTeam}
          agents={result.data.agents}
          agentsFailed={result.data.agentsFailed}
          grafanaUrl={
            result.data.server.status.publishedGeneration && result.data.server.status.route
              ? fleetGrafanaUrl(result.data.server.status.route.path)
              : undefined
          }
        />
      )}
    </FleetFrame>
  );
}
