"use client";

import NextLink from "next/link";
import { useRef } from "react";
import {
  BodyShort,
  Box,
  Button,
  Detail,
  Hide,
  HStack,
  Link,
  Heading,
  Select,
  Show,
  Stack,
  Table,
  Tag,
  VStack,
} from "@navikt/ds-react";
import { PersonIcon } from "@navikt/aksel-icons";
import { refKey } from "@/lib/fleet-names";
import type { AgentProfile, MCPServer, ServerRef } from "@/lib/fleet-control";
import { APPROVER } from "@/lib/fleet-approval";
import { ActionError, CardList, FleetTable, linkClass, useFleetPost } from "./fleet-ui";
import { awaiting } from "./fleet-status";

/** MCP server versions that wait for someone on the team: approve, or wait for the others. */
const DAY = 24 * 60 * 60 * 1000;

const days = (n: number) => (n === 1 ? "1 dag" : `${n} dager`);

/** «Sendt inn i dag» or «for N dager siden», from when the PendingApproval condition turned true. */
function submittedAgo(s: MCPServer): string | undefined {
  const at = s.status.conditions?.find((c) => c.type === "PendingApproval" && c.status === "True")?.lastTransitionTime;
  if (!at) return undefined;
  const n = Math.floor((Date.now() - Date.parse(at)) / DAY);
  return Number.isNaN(n) ? undefined : n < 1 ? "i dag" : `for ${days(n)} siden`;
}

/** Days left before the pending version can no longer be approved (#2731). */
function expiryText(expiresAt?: string): string | undefined {
  if (!expiresAt) return undefined;
  const n = Math.ceil((Date.parse(expiresAt) - Date.now()) / DAY);
  if (Number.isNaN(n)) return undefined;
  return n <= 0 ? "Utløpt, må sendes inn på nytt" : `Utløper om ${days(n)}`;
}

/** «Godkjenn» right in the list: the same POST as on the server page, then the list reloads. */
function InboxApprove({ server }: { server: MCPServer }) {
  const approve = useFleetPost("approve", true);
  const p = server.status.pending!;
  return (
    <form onSubmit={approve.onSubmit} className="w-full md:w-auto">
      <input type="hidden" name="namespace" value={server.metadata.namespace} />
      <input type="hidden" name="name" value={server.metadata.name} />
      <input type="hidden" name="generation" value={p.generation} />
      <input type="hidden" name="contentHash" value={p.contentHash} />
      <VStack gap="space-8">
        <ActionError error={approve.error} />
        <Button loading={approve.busy} className="w-full md:w-auto">
          Godkjenn
        </Button>
      </VStack>
    </form>
  );
}

/** The platform team's filter by owner team: a GET form, so the choice is in the URL. */
export function OwnerFilter({ team, owners, owner }: { team: string; owners: string[]; owner?: string }) {
  const form = useRef<HTMLFormElement>(null);
  return (
    <form method="get" ref={form}>
      <input type="hidden" name="team" value={team} />
      <Select
        label="Vis team"
        name="eier"
        defaultValue={owner ?? ""}
        onChange={() => form.current?.requestSubmit()}
        className="max-w-xs"
      >
        <option value="">Alle team ({owners.length})</option>
        {owners.map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </Select>
      <noscript>
        <Button size="small" variant="secondary">
          Vis
        </Button>
      </noscript>
    </form>
  );
}

export function ServerInbox({
  servers,
  me,
  myName,
  approver,
  showTeam,
}: {
  servers: MCPServer[];
  me?: string;
  myName?: string;
  approver?: boolean;
  /** The platform team's list across teams: name the owner team on each row. */
  showTeam?: boolean;
}) {
  const waiting = servers.filter((s) => s.status.pending);
  if (!waiting.length) return null;
  return (
    <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12">
      <VStack as="ul" aria-label="MCP-servere til godkjenning">
        {waiting.map((s, i) => {
          const p = s.status.pending!;
          const mine = !!me && p.submittedBy?.name === me;
          const href = `/fleet/mcp-servere/${refKey(s.metadata)}`;
          const ago = submittedAgo(s);
          const expiry = expiryText(p.expiresAt);
          // The UI only hides the button; fleet-control still decides. Team copilot may approve its own version.
          const canApprove = !!approver && !!p.contentHash;
          return (
            <Box
              as="li"
              key={refKey(s.metadata)}
              padding="space-20"
              borderWidth={i ? "1 0 0 0" : "0"}
              borderColor="neutral-subtle"
            >
              <Stack
                direction={{ xs: "column", md: "row" }}
                gap="space-16"
                justify="space-between"
                align={{ md: "center" }}
                wrap={false}
              >
                <VStack gap="space-8" className="min-w-0">
                  <HStack gap="space-8" align="center">
                    <Tag size="small" variant="warning" className="whitespace-nowrap">
                      Venter på godkjenning
                    </Tag>
                    {showTeam && (
                      <Tag size="small" variant="neutral" className="whitespace-nowrap">
                        {s.metadata.namespace}
                      </Tag>
                    )}
                    <Link as={NextLink} href={href} className="[overflow-wrap:anywhere]">
                      <strong>{s.metadata.name}</strong>
                    </Link>
                    <BodyShort size="small">versjon {p.generation}</BodyShort>
                  </HStack>
                  <HStack gap="space-8" align="center">
                    <Initials name={mine ? myName : undefined} />
                    <BodyShort size="small">
                      {(mine
                        ? `Sendt inn av ${myName ? `${myName} (deg)` : "deg"}`
                        : p.submittedBy
                          ? `Sendt inn av ${p.submittedBy.name}`
                          : "Ingen person sendte inn") + (ago ? ` ${ago}` : "")}
                    </BodyShort>
                  </HStack>
                  <BodyShort size="small">
                    <strong>{mine && !approver ? "Venter på:" : "Hvem kan godkjenne:"}</strong> {APPROVER}
                    {expiry && <> · {expiry}</>}
                  </BodyShort>
                </VStack>
                <Stack
                  direction={{ xs: "column", md: "row" }}
                  gap="space-8"
                  className="w-full shrink-0 md:w-auto"
                  wrap={false}
                >
                  <Button
                    as={NextLink}
                    href={`${href}#hva-endres`}
                    variant={canApprove || !approver ? "secondary" : "primary"}
                    className="w-full md:w-auto"
                  >
                    Se endringen
                  </Button>
                  {canApprove && <InboxApprove server={s} />}
                </Stack>
              </Stack>
            </Box>
          );
        })}
      </VStack>
    </Box>
  );
}

/**
 * Initials in a circle, as in the sketches. Decorative: the name is always written next to it.
 * name is a person's real name, never a NAVident (fleet-control only has the NAVident): without one, a person icon.
 */
export function Initials({ name }: { name?: string }) {
  const text = (name ?? "")
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 3)
    .map((w) => w[0]?.toUpperCase())
    .join("");
  return (
    <Box
      aria-hidden
      background="accent-soft"
      borderRadius="full"
      className="inline-flex h-7 w-7 shrink-0 items-center justify-center text-xs font-semibold"
    >
      {text || <PersonIcon fontSize="1.25rem" />}
    </Box>
  );
}

/** Grants on the pending generation that the owner team has not given yet; only those can be given. */
export const waitingGrants = (p: AgentProfile) =>
  p.status.pending ? (p.status.grants ?? []).filter((g) => !g.granted && !g.grantedAt && g.contentHash) : [];

/** «Gi tilgang»: the owner team of the MCP server grants the agent profile access. fleet-control checks the team. */
export function GrantAccess({
  profile,
  serverRef,
  withServerName,
}: {
  profile: AgentProfile;
  serverRef: ServerRef;
  withServerName?: boolean;
}) {
  const grant = useFleetPost("grant");
  const hash = profile.status.grants?.find((g) => refKey(g.serverRef) === refKey(serverRef))?.contentHash;
  if (!profile.status.pending || !hash) return null;
  return (
    <form onSubmit={grant.onSubmit}>
      <VStack gap="space-8" align="start">
        <input type="hidden" name="namespace" value={profile.metadata.namespace} />
        <input type="hidden" name="name" value={profile.metadata.name} />
        <input type="hidden" name="serverNamespace" value={serverRef.namespace} />
        <input type="hidden" name="serverName" value={serverRef.name} />
        <input type="hidden" name="generation" value={profile.status.pending.generation} />
        <input type="hidden" name="contentHash" value={hash} />
        <ActionError error={grant.error} />
        <Button variant="secondary" loading={grant.busy}>
          {withServerName ? `Gi tilgang til ${serverRef.name}` : "Gi tilgang"}
        </Button>
      </VStack>
    </form>
  );
}

/** The agent profiles that wait for this MCP server's owner team. */
export function WaitingForAccess({ server, agents }: { server: MCPServer; agents: AgentProfile[] }) {
  const key = refKey(server.metadata);
  const waiting = agents.filter((a) => waitingGrants(a).some((g) => refKey(g.serverRef) === key));
  if (!waiting.length) return null;
  return (
    <VStack gap="space-8">
      <Heading level="2" size="small">
        Venter på tilgang
      </Heading>
      <BodyShort>
        Disse agentprofilene vil bruke MCP-serveren. Når du gir tilgang, kan agentene bruke verktøyene.
      </BodyShort>
      <Show below="md">
        <VStack as="ul" gap="space-16" aria-label="Agentprofiler som venter på tilgang">
          {waiting.map((a) => (
            <VStack as="li" key={refKey(a.metadata)} gap="space-8" align="start">
              <NextLink
                className={`${linkClass} [overflow-wrap:anywhere]`}
                href={`/fleet/agenter/${refKey(a.metadata)}`}
              >
                {refKey(a.metadata)}
              </NextLink>
              <BodyShort size="small" className="[overflow-wrap:anywhere]">
                Verktøy: {a.spec.grants?.find((g) => refKey(g.serverRef) === key)?.allowedTools.join(", ") || "–"}
              </BodyShort>
              <GrantAccess profile={a} serverRef={{ group: "fleet.nav.no", kind: "MCPServer", ...server.metadata }} />
            </VStack>
          ))}
        </VStack>
      </Show>
      <Hide below="md">
        <FleetTable aria-label="Agentprofiler som venter på tilgang">
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell scope="col">Agentprofil</Table.HeaderCell>
              <Table.HeaderCell scope="col">Verktøy</Table.HeaderCell>
              <Table.HeaderCell scope="col">Handling</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {waiting.map((a) => (
              <Table.Row key={refKey(a.metadata)}>
                <Table.DataCell>
                  <NextLink className={linkClass} href={`/fleet/agenter/${refKey(a.metadata)}`}>
                    {refKey(a.metadata)}
                  </NextLink>
                </Table.DataCell>
                <Table.DataCell>
                  {a.spec.grants?.find((g) => refKey(g.serverRef) === key)?.allowedTools.join(", ") || "–"}
                </Table.DataCell>
                <Table.DataCell>
                  <GrantAccess
                    profile={a}
                    serverRef={{ group: "fleet.nav.no", kind: "MCPServer", ...server.metadata }}
                  />
                </Table.DataCell>
              </Table.Row>
            ))}
          </Table.Body>
        </FleetTable>
      </Hide>
    </VStack>
  );
}

export function PendingInbox({ agents }: { agents: AgentProfile[] }) {
  return (
    <VStack gap="space-8">
      <Heading id="venter-pa-tilgang" level="2" size="medium">
        Venter på at dere gir tilgang
      </Heading>
      {agents.length === 0 ? (
        <BodyShort>Ingen agentprofiler venter på teamene dine.</BodyShort>
      ) : (
        <>
          <Show below="md">
            <CardList
              label="Agentprofiler til godkjenning"
              items={agents.map((a) => ({
                key: refKey(a.metadata),
                title: (
                  <NextLink className={linkClass} href={`/fleet/agenter/${refKey(a.metadata)}`}>
                    <strong>{a.metadata.name}</strong>
                  </NextLink>
                ),
                lines: [
                  <Detail key="t">Fra {a.metadata.namespace}</Detail>,
                  <BodyShort key="w" size="small">
                    Venter på: {awaiting(a).join(", ")}
                  </BodyShort>,
                  ...waitingGrants(a).map((g) => (
                    <GrantAccess key={refKey(g.serverRef)} profile={a} serverRef={g.serverRef} withServerName />
                  )),
                ],
              }))}
            />
          </Show>
          <Hide below="md">
            <FleetTable aria-label="Agentprofiler til godkjenning">
              <Table.Header>
                <Table.Row>
                  <Table.HeaderCell scope="col">Agentprofil</Table.HeaderCell>
                  <Table.HeaderCell scope="col">Fra team</Table.HeaderCell>
                  <Table.HeaderCell scope="col" className="whitespace-nowrap">
                    Venter på tilgang
                  </Table.HeaderCell>
                  <Table.HeaderCell scope="col">Handling</Table.HeaderCell>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {agents.map((a) => (
                  <Table.Row key={refKey(a.metadata)}>
                    <Table.DataCell>
                      <NextLink className={linkClass} href={`/fleet/agenter/${refKey(a.metadata)}`}>
                        {a.metadata.name}
                      </NextLink>
                    </Table.DataCell>
                    <Table.DataCell>{a.metadata.namespace}</Table.DataCell>
                    <Table.DataCell>{awaiting(a).join(", ")}</Table.DataCell>
                    <Table.DataCell>
                      <VStack gap="space-8" align="start">
                        {waitingGrants(a).map((g) => (
                          <GrantAccess key={refKey(g.serverRef)} profile={a} serverRef={g.serverRef} withServerName />
                        ))}
                      </VStack>
                    </Table.DataCell>
                  </Table.Row>
                ))}
              </Table.Body>
            </FleetTable>
          </Hide>
        </>
      )}
    </VStack>
  );
}
