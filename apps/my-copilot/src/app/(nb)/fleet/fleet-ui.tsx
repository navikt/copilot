"use client";

// Client component because Aksel's compound components (Table.Row, ExpansionCard.Header)
// are undefined when read off a client reference in a server component.
import NextLink from "next/link";
import { useRouter } from "next/navigation";
import { useId, useRef, useState, type FormEvent, type ReactNode } from "react";
import {
  Alert,
  BodyLong,
  BodyShort,
  Box,
  Button,
  Checkbox,
  CopyButton,
  Detail,
  ExpansionCard,
  HGrid,
  Hide,
  HStack,
  Modal,
  Radio,
  RadioGroup,
  Link,
  ReadMore,
  Heading,
  Select,
  Show,
  Stack,
  Table,
  Tag,
  Textarea,
  TextField,
  VStack,
} from "@navikt/ds-react";
import { ExternalLinkIcon, PersonIcon } from "@navikt/aksel-icons";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { hasCondition, refKey } from "@/lib/fleet-names";
import type { AgentProfile, AuditEvent, FleetProblem, MCPServer, MCPServerSpec, ServerRef } from "@/lib/fleet-control";

const linkClass = "text-blue-600 hover:underline";

type TagVariant = "success" | "warning" | "info" | "neutral" | "error";

// Status comes from the conditions; a missing or unknown one reads as false.
export function serverStatus(s: MCPServer): { label: string; variant: TagVariant } {
  const pending = s.status.pending?.generation;
  // A revoked MCP server can get a new draft, and approving it publishes the server again.
  if (hasCondition(s, "Revoked"))
    return pending
      ? { label: "Trukket tilbake, ny versjon venter", variant: "warning" }
      : { label: "Trukket tilbake", variant: "neutral" };
  if (!hasCondition(s, "Published")) return { label: "Venter på godkjenning", variant: "warning" };
  if (pending) return { label: "Ny versjon venter", variant: "info" };
  return { label: "Publisert", variant: "success" };
}

export function agentStatus(p: AgentProfile): { label: string; variant: TagVariant } {
  if (hasCondition(p, "Revoked")) return { label: "Trukket tilbake", variant: "neutral" };
  if (hasCondition(p, "Expired")) return { label: "Utløpt", variant: "neutral" };
  if (hasCondition(p, "Published")) return { label: "I bruk", variant: "success" };
  return { label: "Venter på eierteamene", variant: "warning" };
}

const ACTIONS: Record<string, string> = {
  draft_submitted: "Utkast sendt inn",
  approved_published: "Godkjent og publisert",
  revoked: "Trukket tilbake",
  rolled_back: "Rullet tilbake",
  agent_profile_submitted: "Utkast sendt inn",
  agent_profile_access_granted: "Tilgang gitt",
  agent_profile_published: "Profilen godkjent",
  agent_profile_revoked: "Trukket tilbake",
  agent_profile_access_revoked: "Tilgang trukket tilbake",
  metadata_updated: "Etiketter endret",
  // From before v1alpha1, still in the history of older profiles.
  agent_profile_drafted: "Utkast opprettet",
  agent_profile_route_approved: "Tilgang gitt",
  agent_profile_approved: "Profilen godkjent",
  agent_profile_expired: "Utløpt",
};

const date = (iso: string) =>
  new Date(iso).toLocaleString("nb-NO", { dateStyle: "medium", timeStyle: "short", timeZone: "Europe/Oslo" });

const n = (v: number) => v.toLocaleString("nb-NO");

const StatusTag = ({ s }: { s: { label: string; variant: TagVariant } }) => (
  <Tag size="small" variant={s.variant} className="whitespace-nowrap">
    {s.label}
  </Tag>
);

/** 403, 409 and 503 (and timeouts). 401 and 404 never get here: the page logs in again or shows «Fant ikke siden». */
export function FleetProblemAlert({ problem }: { problem: FleetProblem }) {
  const router = useRouter();
  if (problem === "forbidden") {
    return (
      <Alert variant="warning">
        Du har ikke tilgang. Bare eierteamet til MCP-serveren eller agentprofilen kan se den. Er du på teamet, be en
        eier legge deg til i <Link href="https://console.nav.cloud.nais.io/">Nais Console</Link> og last inn siden på
        nytt.
      </Alert>
    );
  }
  if (problem === "conflict") {
    return <Alert variant="warning">Noen andre har endret dette. Last inn siden på nytt.</Alert>;
  }
  return (
    <Alert variant="error">
      <VStack gap="space-8" align="start">
        <Heading level="2" size="small">
          Vi får ikke kontakt med Fleet akkurat nå
        </Heading>
        <BodyShort>MCP-serverne virker som før. Bare denne siden er nede. Prøv igjen om litt.</BodyShort>
        <Button variant="secondary" onClick={() => router.refresh()}>
          Prøv igjen
        </Button>
      </VStack>
    </Alert>
  );
}

/** Every /fleet table: scrolls inside its own box so a narrow screen never scrolls the page sideways. */
export function FleetTable({ "aria-label": label, children }: { "aria-label": string; children: ReactNode }) {
  return (
    <Box
      className="overflow-x-auto [&_thead]:bg-[var(--ax-bg-neutral-soft)]"
      background="default"
      borderWidth="1"
      borderColor="neutral-subtle"
      borderRadius="12"
      role="region"
      tabIndex={0}
      aria-label={label}
      data-fleet-table=""
    >
      <Table size="small" aria-label={label}>
        {children}
      </Table>
    </Box>
  );
}

export function KortFortalt({ children, collapsed = false }: { children: ReactNode; collapsed?: boolean }) {
  if (collapsed)
    return (
      <ExpansionCard aria-label="Kort fortalt: hva er Fleet?" size="small">
        <ExpansionCard.Header>
          <ExpansionCard.Title as="h2" size="small">
            Kort fortalt: hva er Fleet?
          </ExpansionCard.Title>
        </ExpansionCard.Header>
        <ExpansionCard.Content>
          <VStack gap="space-8">{children}</VStack>
        </ExpansionCard.Content>
      </ExpansionCard>
    );
  return (
    <Box background="info-soft" padding="space-20" borderRadius="12">
      <VStack gap="space-8">
        <Heading level="2" size="small">
          Kort fortalt
        </Heading>
        {children}
      </VStack>
    </Box>
  );
}

/** Client side, since a server component cannot pass NextLink to Button. */
export const RegisterButton = ({ team }: { team: string }) => (
  <Button as={NextLink} href={`/fleet/registrer?team=${team}`} className="w-full sm:w-auto">
    Registrer MCP-server
  </Button>
);

/** A team without MCP servers: what to do first. */
export function EmptyTeam({ team }: { team: string }) {
  return (
    <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12" padding="space-24">
      <VStack gap="space-12" align="start">
        <Heading level="3" size="medium">
          {team} har ingen MCP-servere ennå
        </Heading>
        <BodyLong>
          Registrer en MCP-server som teamet allerede kjører i Nais. En annen i teamet godkjenner før den blir
          publisert.
        </BodyLong>
        <HStack gap="space-16" align="center">
          <Button as={NextLink} href={`/fleet/registrer?team=${team}`}>
            Registrer MCP-server
          </Button>
          <Link as={NextLink} href="/fleet/katalog">
            Se katalogen
          </Link>
        </HStack>
      </VStack>
    </Box>
  );
}

/** After register or edit: the version waits for someone else on the team. */
export function SubmittedAlert({ server }: { server: MCPServer }) {
  const path = server.status.route?.path ?? `/mcp/${server.metadata.namespace}/${server.metadata.name}`;
  return (
    <Alert variant="success">
      <Heading level="2" size="small">
        {server.metadata.name} er sendt til godkjenning
      </Heading>
      <BodyLong>
        Adressen <code>{path}</code> blir publisert når en annen på {server.metadata.namespace} godkjenner.
      </BodyLong>
    </Alert>
  );
}

/** A number with a label, as in the stat strip on top of the overview. */
export function Stat({
  label,
  short,
  value,
  help,
}: {
  label: string;
  short?: string;
  value: ReactNode;
  help?: string;
}) {
  return (
    <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12" padding="space-16">
      <VStack gap="space-4">
        <BodyShort size="small" className="md:whitespace-nowrap">
          {short ? (
            <>
              <Hide below="md" asChild>
                <span>{label}</span>
              </Hide>
              <Show below="md" asChild>
                <span>{short}</span>
              </Show>
            </>
          ) : (
            label
          )}
        </BodyShort>
        <Heading level="3" size="large" as="p">
          {value}
        </Heading>
        {help && <BodyShort size="small">{help}</BodyShort>}
      </VStack>
    </Box>
  );
}

/** The team's overview numbers. Calls and users per team are not in Fleet's API; Grafana has them per MCP server. */
export function TeamStats({ servers, me }: { servers: MCPServer[]; me?: string }) {
  const published = servers.filter((s) => hasCondition(s, "Published") && !hasCondition(s, "Revoked")).length;
  const pending = servers.filter((s) => s.status.pending);
  const forMe = pending.filter(
    (s) => s.status.pending?.contentHash && s.status.pending.submittedBy?.name !== me
  ).length;
  return (
    <HGrid columns={{ xs: 2, md: 4 }} gap="space-16">
      <Stat
        label="Publiserte MCP-servere"
        short="Publisert"
        value={published}
        help={`av ${servers.length} registrerte`}
      />
      <Stat label="Venter på godkjenning" short="Venter" value={pending.length} help={`${forMe} venter på deg`} />
      {/* shortcut: Fleet has no calls or users per team yet (#2753); the cards stay so the layout matches. */}
      <Stat label="Kall siste 7 dager" short="Kall, 7 dager" value="–" help="Kommer" />
      <Stat label="Unike brukere, 7 dager" short="Brukere" value="–" help="Kommer" />
    </HGrid>
  );
}

/** MCP server versions that wait for someone on the team: approve, or wait for the others. */
export function ServerInbox({ servers, me, myName }: { servers: MCPServer[]; me?: string; myName?: string }) {
  const waiting = servers.filter((s) => s.status.pending);
  if (!waiting.length) return null;
  return (
    <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12">
      <VStack as="ul" aria-label="MCP-servere til godkjenning">
        {waiting.map((s, i) => {
          const p = s.status.pending!;
          const mine = !!me && p.submittedBy?.name === me;
          const href = `/fleet/mcp-servere/${refKey(s.metadata)}`;
          return (
            <Box
              as="li"
              key={s.metadata.name}
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
                    <Link as={NextLink} href={href} className="[overflow-wrap:anywhere]">
                      <strong>{s.metadata.name}</strong>
                    </Link>
                    <BodyShort size="small">versjon {p.generation}</BodyShort>
                  </HStack>
                  <HStack gap="space-8" align="center">
                    <Initials name={mine ? myName : undefined} />
                    <BodyShort size="small">
                      {mine
                        ? `Sendt inn av ${myName ? `${myName} (deg)` : "deg"}`
                        : p.submittedBy
                          ? `Sendt inn av ${p.submittedBy.name}`
                          : "Ingen person sendte inn"}
                    </BodyShort>
                  </HStack>
                  <BodyShort size="small">
                    <strong>{mine ? "Venter på en annen i teamet:" : "Hvem kan godkjenne:"}</strong>{" "}
                    {mine ? `alle andre i ${s.metadata.namespace}` : `du og de andre i ${s.metadata.namespace}`}
                  </BodyShort>
                </VStack>
                <div className="w-full shrink-0 md:w-auto">
                  <Button
                    as={NextLink}
                    href={href}
                    variant={mine ? "secondary" : "primary"}
                    className="w-full md:w-auto"
                  >
                    Se endringen
                  </Button>
                </div>
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
function Initials({ name }: { name?: string }) {
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

/** One short next step per MCP server for the overview table. */
export function serverShortNext(s: MCPServer, me?: string): string {
  const p = s.status.pending;
  if (p)
    return !!me && p.submittedBy?.name === me ? "En annen i teamet godkjenner" : "Godkjenn versjon " + p.generation;
  if (hasCondition(s, "Revoked")) return "Send inn en ny versjon";
  return "Ingen";
}

export type TeamData = { team: string; servers: MCPServer[]; agents: AgentProfile[] };

export function TeamSections({ teams, me }: { teams: TeamData[]; me?: string }) {
  return (
    <VStack gap="space-32">
      {teams.map((t) => (
        <section key={t.team} aria-label={t.team}>
          <VStack gap="space-24">
            <VStack gap="space-8">
              <Heading id="mcp-servere" level="2" size="medium">
                MCP-servere
              </Heading>
              {t.servers.length === 0 ? (
                <EmptyTeam team={t.team} />
              ) : (
                <>
                  <Show below="md">
                    <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12">
                      <VStack as="ul" aria-label={`MCP-serverne til ${t.team}`}>
                        {t.servers.map((s, i) => (
                          <Box
                            as="li"
                            key={s.metadata.name}
                            padding="space-16"
                            borderWidth={i ? "1 0 0 0" : "0"}
                            borderColor="neutral-subtle"
                          >
                            <VStack gap="space-4" align="start">
                              <StatusTag s={serverStatus(s)} />
                              <NextLink
                                className={`${linkClass} [overflow-wrap:anywhere]`}
                                href={`/fleet/mcp-servere/${refKey(s.metadata)}`}
                              >
                                <strong>{s.metadata.name}</strong>
                              </NextLink>
                              {s.status.route && (
                                <Detail as="code" className="[overflow-wrap:anywhere]">
                                  {s.status.route.path}
                                </Detail>
                              )}
                              <Detail>{serverShortNext(s, me)}</Detail>
                            </VStack>
                          </Box>
                        ))}
                      </VStack>
                    </Box>
                  </Show>
                  <Hide below="md">
                    <FleetTable aria-label={`MCP-serverne til ${t.team}`}>
                      <Table.Header>
                        <Table.Row>
                          <Table.HeaderCell scope="col">MCP-server</Table.HeaderCell>
                          <Table.HeaderCell scope="col">Status</Table.HeaderCell>
                          <Table.HeaderCell scope="col">Neste steg</Table.HeaderCell>
                          <Table.HeaderCell scope="col">Publisert versjon</Table.HeaderCell>
                        </Table.Row>
                      </Table.Header>
                      <Table.Body>
                        {t.servers.map((s) => (
                          <Table.Row key={s.metadata.name}>
                            <Table.DataCell>
                              <VStack gap="space-4">
                                <NextLink className={linkClass} href={`/fleet/mcp-servere/${refKey(s.metadata)}`}>
                                  <strong>{s.metadata.name}</strong>
                                </NextLink>
                                {s.status.route && <Detail as="code">{s.status.route.path}</Detail>}
                                <Detail>
                                  {(s.spec?.allowedTools?.length ?? 0) === 1
                                    ? "1 verktøy"
                                    : `${s.spec?.allowedTools?.length ?? 0} verktøy`}
                                  {s.spec?.dataCategory && ` · ${categoryLabel(s.spec.dataCategory)}`}
                                </Detail>
                              </VStack>
                            </Table.DataCell>
                            <Table.DataCell>
                              <StatusTag s={serverStatus(s)} />
                            </Table.DataCell>
                            <Table.DataCell>{serverShortNext(s, me)}</Table.DataCell>
                            <Table.DataCell>{s.status.publishedGeneration ?? "–"}</Table.DataCell>
                          </Table.Row>
                        ))}
                      </Table.Body>
                    </FleetTable>
                  </Hide>
                </>
              )}
            </VStack>
          </VStack>
        </section>
      ))}
    </VStack>
  );
}

/** Rows as cards on a phone, where a table would push columns off-screen. */
export function CardList({
  label,
  items,
}: {
  label: string;
  items: { key: string; title: ReactNode; aside?: ReactNode; lines: ReactNode[] }[];
}) {
  return (
    <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12">
      <VStack as="ul" aria-label={label}>
        {items.map((it, i) => (
          <Box as="li" key={it.key} padding="space-16" borderWidth={i ? "1 0 0 0" : "0"} borderColor="neutral-subtle">
            <VStack gap="space-8">
              <HStack justify="space-between" align="start" gap="space-8" wrap={false}>
                <div className="min-w-0 [overflow-wrap:anywhere]">{it.title}</div>
                {it.aside}
              </HStack>
              {it.lines.map((l, j) => (
                <div key={j} className="min-w-0 [overflow-wrap:anywhere]">
                  {l}
                </div>
              ))}
            </VStack>
          </Box>
        ))}
      </VStack>
    </Box>
  );
}

/** No agent profiles yet: what they are, and that the agent runtime is not ready (#2686). */
function AgentEmptyState({ team }: { team: string }) {
  return (
    <VStack gap="space-8" className="max-w-prose">
      <Heading level="2" size="small">
        {team} har ingen agentprofiler ennå
      </Heading>
      <BodyLong>
        En agentprofil sier hvilken app som får bruke hvilke verktøy på hvilke MCP-servere, på vegne av en innlogget
        bruker. Eierteamet til hver MCP-server bestemmer om profilen får tilgang.
      </BodyLong>
      <BodyLong>
        Profilene skal brukes av KI-agenter som kjører i Fleet. Den delen er ikke i produksjon ennå, så foreløpig er det
        ingenting å sette opp her.
      </BodyLong>
      <BodyLong>
        Det dere kan gjøre nå, er å registrere MCP-serverne teamet har, slik at agentene kan bruke dem når de kommer.{" "}
        <Link as={NextLink} href={`/fleet/registrer?team=${team}`}>
          Registrer en MCP-server
        </Link>
      </BodyLong>
    </VStack>
  );
}

/** The team's agent profiles, on /fleet/agenter. */
export function AgentTable({ t }: { t: TeamData }) {
  return (
    <VStack gap="space-8">
      {t.agents.length === 0 ? (
        <AgentEmptyState team={t.team} />
      ) : (
        <>
          <Show below="md">
            <CardList
              label={`Agentprofilene til ${t.team}`}
              items={t.agents.map((a) => ({
                key: a.metadata.name,
                title: (
                  <NextLink className={linkClass} href={`/fleet/agenter/${refKey(a.metadata)}`}>
                    <strong>{a.metadata.name}</strong>
                  </NextLink>
                ),
                aside: <StatusTag s={agentStatus(a)} />,
                lines: [
                  <Detail key="g">
                    {(a.spec.grants ?? []).map((g) => refKey(g.serverRef)).join(", ") || "Ingen MCP-servere"}
                  </Detail>,
                ],
              }))}
            />
          </Show>
          <Hide below="md">
            <FleetTable aria-label={`Agentprofilene til ${t.team}`}>
              <Table.Header>
                <Table.Row>
                  <Table.HeaderCell scope="col">Navn</Table.HeaderCell>
                  <Table.HeaderCell scope="col">Status</Table.HeaderCell>
                  <Table.HeaderCell scope="col">MCP-servere</Table.HeaderCell>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {t.agents.map((a) => (
                  <Table.Row key={a.metadata.name}>
                    <Table.DataCell>
                      <NextLink className={linkClass} href={`/fleet/agenter/${refKey(a.metadata)}`}>
                        {a.metadata.name}
                      </NextLink>
                    </Table.DataCell>
                    <Table.DataCell>
                      <StatusTag s={agentStatus(a)} />
                    </Table.DataCell>
                    <Table.DataCell>{(a.spec.grants ?? []).map((g) => refKey(g.serverRef)).join(", ")}</Table.DataCell>
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

/** The MCP servers whose owner team has not granted access yet. A revoked grant keeps grantedAt and is not waiting. */
const awaiting = (p: AgentProfile) =>
  (p.status.grants ?? []).filter((g) => !g.granted && !g.grantedAt).map((g) => `MCP-server ${refKey(g.serverRef)}`);

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
function WaitingForAccess({ server, agents }: { server: MCPServer; agents: AgentProfile[] }) {
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
                  <Table.HeaderCell scope="col">Venter på tilgang</Table.HeaderCell>
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

function Historikk({ events }: { events: AuditEvent[] }) {
  return (
    <ExpansionCard aria-label="Historikk">
      <ExpansionCard.Header>
        <ExpansionCard.Title as="h2" size="small">
          Historikk
        </ExpansionCard.Title>
      </ExpansionCard.Header>
      <ExpansionCard.Content>
        {events.length === 0 ? (
          <BodyShort>Ingen hendelser ennå.</BodyShort>
        ) : (
          <FleetTable aria-label="Historikk">
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell scope="col">Tidspunkt</Table.HeaderCell>
                <Table.HeaderCell scope="col">Hendelse</Table.HeaderCell>
                <Table.HeaderCell scope="col">Utført av</Table.HeaderCell>
                <Table.HeaderCell scope="col">Versjon</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {events.map((e, i) => (
                <Table.Row key={i}>
                  <Table.DataCell>{date(e.at)}</Table.DataCell>
                  <Table.DataCell>{ACTIONS[e.action] ?? e.action}</Table.DataCell>
                  <Table.DataCell>{e.actor.name}</Table.DataCell>
                  <Table.DataCell>{e.generation ?? "–"}</Table.DataCell>
                </Table.Row>
              ))}
            </Table.Body>
          </FleetTable>
        )}
      </ExpansionCard.Content>
    </ExpansionCard>
  );
}

function Detaljer({ rows }: { rows: [string, ReactNode][] }) {
  return (
    <ExpansionCard aria-label="Detaljer">
      <ExpansionCard.Header>
        <ExpansionCard.Title as="h2" size="small">
          Detaljer
        </ExpansionCard.Title>
      </ExpansionCard.Header>
      <ExpansionCard.Content>
        <FleetTable aria-label="Detaljer">
          <Table.Body>
            {rows.map(([k, v]) => (
              <Table.Row key={k}>
                <Table.HeaderCell scope="row">{k}</Table.HeaderCell>
                <Table.DataCell>{v}</Table.DataCell>
              </Table.Row>
            ))}
          </Table.Body>
        </FleetTable>
      </ExpansionCard.Content>
    </ExpansionCard>
  );
}

export function serverNextStep(s: MCPServer): string {
  const pending = s.status.pending?.generation;
  const published = s.status.publishedGeneration;
  if (hasCondition(s, "Revoked"))
    return pending
      ? `En annen person på teamet må godkjenne versjon ${pending} før MCP-serveren blir publisert igjen.`
      : "Endre MCP-serveren og send inn en ny versjon. Når en annen på teamet har godkjent den, blir MCP-serveren publisert igjen.";
  if (!hasCondition(s, "Published"))
    return "En annen person på teamet må godkjenne utkastet før MCP-serveren blir publisert.";
  if (pending) return `En annen person på teamet må godkjenne versjon ${pending}. Til da gjelder versjon ${published}.`;
  return `Ingen. Versjon ${published} er publisert.`;
}

export function ServerDetail({
  server,
  events,
  me,
  myName,
  owner = false,
  agents = [],
  agentsFailed = false,
  grafanaUrl,
  gatewayUrl,
  submitted = false,
}: {
  gatewayUrl?: string;
  server: MCPServer;
  events: AuditEvent[];
  submitted?: boolean;
  me?: string;
  /** The signed-in person's name from the login token, for the initials. */
  myName?: string;
  // In the owner team: only then the edit form shows. fleet-control checks it again.
  owner?: boolean;
  agents?: AgentProfile[];
  agentsFailed?: boolean;
  grafanaUrl?: string;
}) {
  return (
    <VStack gap="space-24">
      {submitted && server.status.pending && <SubmittedAlert server={server} />}
      <BodyLong>Neste steg: {serverNextStep(server)}</BodyLong>
      <ServerActions server={server} me={me} myName={myName} owner={owner} part="approve" />
      {server.status.pending?.spec && (
        <VStack gap="space-8">
          <HStack justify="space-between" align="end" gap="space-8">
            <Heading level="2" size="medium">
              Hva endres
            </Heading>
            {server.status.publishedGeneration && (
              <BodyShort size="small">
                Publisert versjon {server.status.publishedGeneration} mot versjon {server.status.pending.generation}
              </BodyShort>
            )}
          </HStack>
          <Versjoner server={server} next={server.status.pending.spec} />
        </VStack>
      )}
      {hasCondition(server, "Published") && !hasCondition(server, "Revoked") && (
        <VStack gap="space-8">
          <Heading level="2" size="medium">
            Bruk siste 7 dager
          </Heading>
          <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12" padding="space-20">
            <VStack gap="space-12">
              {/* shortcut: Fleet has no usage numbers per MCP server yet (#2753); Grafana has them. */}
              <HGrid columns={{ xs: 1, sm: 3 }} gap="space-16">
                {["Kall", "Unike brukere", "Konsumenter (team)"].map((l) => (
                  <VStack key={l} gap="space-4">
                    <BodyShort size="small">{l}</BodyShort>
                    <Heading level="3" size="large" as="p">
                      –
                    </Heading>
                  </VStack>
                ))}
              </HGrid>
              {grafanaUrl && (
                <BodyShort size="small">
                  Tallene kommer hit. Til da:{" "}
                  <Link href={grafanaUrl} target="_blank" rel="noopener noreferrer">
                    se kall og brukere i Grafana <ExternalLinkIcon fontSize="1rem" title="åpnes i ny fane" />
                  </Link>
                </BodyShort>
              )}
            </VStack>
          </Box>
        </VStack>
      )}
      {server.status.route && hasCondition(server, "Published") && !hasCondition(server, "Revoked") ? (
        <VStack gap="space-16">
          <TestCurl path={server.status.route!.path} gateway={gatewayUrl} />
          <AccessPolicy app={(server.status.pending?.spec ?? server.spec)?.upstream?.app} />
        </VStack>
      ) : null}
      <ServerActions server={server} me={me} owner={owner} part="rest" />
      <Detaljer
        rows={[
          ["Team", server.metadata.namespace],
          ["Navn", server.metadata.name],
          ["Adresse i fleet-gateway", server.status.route?.path ?? "–"],
          ["Publisert versjon", server.status.publishedGeneration ?? "–"],
          ["Versjon som venter på godkjenning", server.status.pending?.generation ?? "–"],
        ]}
      />
      <Historikk events={events} />
      {agentsFailed ? (
        <Alert variant="warning" size="small">
          Vi fikk ikke hentet agentprofilene som venter på tilgang. Last inn siden på nytt.
        </Alert>
      ) : (
        <WaitingForAccess server={server} agents={agents} />
      )}
      <ServerActions server={server} me={me} owner={owner} part="edit" />
      <ServerDocs spec={server.status.pending?.spec ?? server.spec} />
      {!(server.status.route && hasCondition(server, "Published") && !hasCondition(server, "Revoked")) && (
        <AccessPolicy app={(server.status.pending?.spec ?? server.spec)?.upstream?.app} />
      )}
    </VStack>
  );
}

/** Name line under the hero: status, route and the facts a reader looks for first. */
export const ServerStatusTag = ({ server }: { server: MCPServer }) => <StatusTag s={serverStatus(server)} />;
export const AgentStatusTag = ({ profile }: { profile: AgentProfile }) => <StatusTag s={agentStatus(profile)} />;

export function ServerHeader({ server }: { server: MCPServer }) {
  const spec = server.status.pending?.spec ?? server.spec;
  const tools = spec?.allowedTools?.length ?? 0;
  return (
    <VStack gap="space-8">
      {server.status.route && (
        <Detail as="code" className="[overflow-wrap:anywhere]">
          {server.status.route.path}
        </Detail>
      )}
      <HStack gap="space-16" wrap>
        <BodyShort size="small">Eierteam: {server.metadata.namespace}</BodyShort>
        <BodyShort size="small">{tools === 1 ? "1 verktøy" : `${tools} verktøy`}</BodyShort>
        {spec?.dataCategory && <BodyShort size="small">{categoryLabel(spec.dataCategory)}</BodyShort>}
        {server.status.publishedGeneration && (
          <BodyShort size="small">Publisert versjon {server.status.publishedGeneration}</BodyShort>
        )}
      </HStack>
    </VStack>
  );
}

/** tools/list through fleet-gateway, for the owner team to try the published route. */
function TestCurl({ path, gateway }: { path: string; gateway?: string }) {
  const curl = `curl -X POST ${gateway ?? "https://<fleet-gateway>"}${path} \\
  -H "Authorization: Bearer $TOKEN" \\
  -H "Content-Type: application/json" \\
  -H "Accept: application/json, text/event-stream" \\
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'`;
  return (
    <VStack gap="space-8">
      <Heading level="2" size="medium">
        Test MCP-endpointet
      </Heading>
      <Box borderWidth="1" borderColor="neutral-subtle" borderRadius="8" className="overflow-hidden">
        <HStack
          justify="space-between"
          align="center"
          gap="space-8"
          paddingBlock="space-4"
          paddingInline="space-16"
          className="bg-[var(--ax-bg-neutral-soft)]"
        >
          <BodyShort size="small">Liste verktøyene (tools/list)</BodyShort>
          <CopyButton copyText={curl} text="Kopier" activeText="Kopiert" />
        </HStack>
        {/* The contrast text token on the strong background: the pair Aksel makes for WCAG AA. */}
        <Box
          padding="space-16"
          style={{ background: "var(--ax-bg-neutral-strong)", color: "var(--ax-text-neutral-contrast)" }}
        >
          <pre className="overflow-x-auto" role="region" tabIndex={0} aria-label="curl-kommando">
            <code>{curl}</code>
          </pre>
        </Box>
      </Box>
    </VStack>
  );
}

const specRows = (s?: MCPServerSpec): [string, string][] => [
  ["Datakategori", DATA_CATEGORIES.find((d) => d[0] === s?.dataCategory)?.[1] ?? s?.dataCategory ?? "–"],
  ["App", s?.upstream?.app ?? "–"],
  ["Adresse til appen", s?.upstream?.url ?? "–"],
  ["Verktøy", s?.allowedTools?.join(", ") || "–"],
  ["Versjon i registeret", s?.registry?.version ?? "–"],
  [
    "Adresser i registeret",
    s?.registry?.remotes?.map((r) => [r.type, r.url].filter(Boolean).join(" ")).join(", ") || "–",
  ],
];

const safeHref = (href?: string) => (href && /^(https?:|mailto:)/i.test(href) ? href : undefined);

// Readme headings sit under the h2 "Om MCP-serveren": h1 becomes h3, capped at h6.
const readmeHeadings = Object.fromEntries(
  [1, 2, 3, 4, 5, 6].map((n) => {
    const Tag = `h${Math.min(n + 2, 6)}` as "h3";
    return [`h${n}`, ({ children }: { children?: ReactNode }) => <Tag>{children}</Tag>];
  })
);

/** The readme is untrusted: raw HTML is skipped, links only for http/https/mailto, no images. */
export function ServerReadme({ markdown }: { markdown: string }) {
  return (
    <Markdown
      remarkPlugins={[remarkGfm]}
      skipHtml
      components={{
        a: ({ href, children }) => {
          const safe = safeHref(href);
          return safe ? (
            <a href={safe} className={linkClass} rel="noopener noreferrer">
              {children}
            </a>
          ) : (
            <span>{children}</span>
          );
        },
        img: ({ alt }) => <span>{alt}</span>,
        pre: ({ children }) => (
          <pre className="overflow-x-auto" role="region" tabIndex={0} aria-label="Kodeblokk">
            {children}
          </pre>
        ),
        ...readmeHeadings,
      }}
    >
      {markdown}
    </Markdown>
  );
}

function ServerDocs({ spec }: { spec?: MCPServerSpec }) {
  const tools = spec?.tools?.filter((t) => t.description) ?? [];
  if (!spec?.readme && !tools.length) return null;
  return (
    <ExpansionCard aria-label="Om MCP-serveren" size="small">
      <ExpansionCard.Header>
        <ExpansionCard.Title as="h2" size="small">
          Om MCP-serveren
        </ExpansionCard.Title>
      </ExpansionCard.Header>
      <ExpansionCard.Content>
        <VStack gap="space-8">
          {spec?.readme && (
            <div className="prose">
              <ServerReadme markdown={spec.readme} />
            </div>
          )}
          {tools.length > 0 && (
            <>
              <Show below="md">
                <VStack as="ul" gap="space-12" aria-label="Verktøy">
                  {tools.map((t) => (
                    <li key={t.name}>
                      <code className="[overflow-wrap:anywhere]">{t.name}</code>
                      {t.description && <BodyShort size="small">{t.description}</BodyShort>}
                    </li>
                  ))}
                </VStack>
              </Show>
              <Hide below="md">
                <FleetTable aria-label="Verktøy">
                  <Table.Body>
                    {tools.map((t) => (
                      <Table.Row key={t.name}>
                        <Table.HeaderCell scope="row">{t.name}</Table.HeaderCell>
                        <Table.DataCell>{t.description}</Table.DataCell>
                      </Table.Row>
                    ))}
                  </Table.Body>
                </FleetTable>
              </Hide>
            </>
          )}
        </VStack>
      </ExpansionCard.Content>
    </ExpansionCard>
  );
}

/** Tools in the pending version, with added ones marked «+» and removed ones struck through. */
function ToolDiff({ now, next }: { now?: string[]; next?: string[] }) {
  const before = new Set(now ?? []);
  const after = new Set(next ?? []);
  return (
    <HStack as="span" gap="space-4 space-8" wrap>
      {(next ?? []).map((t) =>
        before.has(t) ? (
          <span key={t}>{t}</span>
        ) : (
          <Box
            as="span"
            key={t}
            background="success-soft"
            paddingInline="space-4"
            borderRadius="4"
            className="[overflow-wrap:anywhere]"
          >
            <strong>+&nbsp;{t}</strong>
          </Box>
        )
      )}
      {(now ?? [])
        .filter((t) => !after.has(t))
        .map((t) => (
          <Box
            as="span"
            key={t}
            background="danger-soft"
            paddingInline="space-4"
            borderRadius="4"
            className="[overflow-wrap:anywhere]"
          >
            <s>−&nbsp;{t}</s>
          </Box>
        ))}
    </HStack>
  );
}

function Versjoner({ server, next }: { server: MCPServer; next: MCPServerSpec }) {
  // Before the first approval fleet-control copies the draft into spec, so only a live server has a version in use.
  const none = hasCondition(server, "Revoked")
    ? "Trukket tilbake"
    : hasCondition(server, "Published")
      ? null
      : "Ikke publisert ennå";
  const current = none ? undefined : server.spec;
  const now = specRows(current);
  const rows = specRows(next);
  const pub = server.status.publishedGeneration;
  const gen = server.status.pending?.generation;
  const changed = rows.filter(([, v], i) => v !== now[i][1]);
  const value = (k: string, v: string, i: number) =>
    k === "Verktøy" ? (
      <ToolDiff now={current?.allowedTools} next={next.allowedTools} />
    ) : v === now[i][1] ? (
      "Uendret"
    ) : (
      v
    );
  return (
    <VStack gap="space-8">
      {none && <BodyShort size="small">{none}: alt under er nytt.</BodyShort>}
      <Show below="md">
        <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12" padding="space-16">
          <VStack as="dl" gap="space-12">
            {changed.map(([k, v]) => (
              <div key={k}>
                <BodyShort as="dt" size="small">
                  {k}
                </BodyShort>
                <BodyShort as="dd" className="[overflow-wrap:anywhere]">
                  {k === "Verktøy" ? (
                    <ToolDiff now={current?.allowedTools} next={next.allowedTools} />
                  ) : (
                    <strong>{v}</strong>
                  )}
                </BodyShort>
              </div>
            ))}
            {changed.length < rows.length && (
              <BodyShort size="small">
                Uendret:{" "}
                {rows
                  .filter(([, v], i) => v === now[i][1])
                  .map(([k]) => k.toLowerCase())
                  .join(", ")}
                .
              </BodyShort>
            )}
          </VStack>
        </Box>
      </Show>
      <Hide below="md">
        <FleetTable aria-label="Versjonen som gjelder, og versjonen som venter">
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell scope="col" className="w-44">
                Felt
              </Table.HeaderCell>
              {!none && (
                <Table.HeaderCell scope="col" className="whitespace-nowrap">
                  Publisert{pub ? ` (v${pub})` : ""}
                </Table.HeaderCell>
              )}
              <Table.HeaderCell scope="col" className="whitespace-nowrap">
                Venter{gen ? ` (v${gen})` : ""}
              </Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {rows.map(([k, v], i) => (
              <Table.Row key={k}>
                <Table.HeaderCell scope="row">{k}</Table.HeaderCell>
                {!none && <Table.DataCell className="[overflow-wrap:anywhere]">{now[i][1]}</Table.DataCell>}
                <Table.DataCell className="[overflow-wrap:anywhere]">
                  {none ? k === "Verktøy" ? <ToolDiff next={next.allowedTools} /> : v : value(k, v, i)}
                </Table.DataCell>
              </Table.Row>
            ))}
          </Table.Body>
        </FleetTable>
      </Hide>
    </VStack>
  );
}

type FleetError = { message: string; detail?: string; fields?: Record<string, string> };

/** Posts a form to /api/fleet and opens the page it answers with; the error stays on the form. */
function useFleetPost(action: string) {
  const router = useRouter();
  const [error, setError] = useState<FleetError>();
  const [busy, setBusy] = useState(false);
  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError(undefined);
    const body: Record<string, unknown> = { action };
    new FormData(e.currentTarget).forEach((v, k) => (body[k] = v));
    if ("block" in body) body.block = true;
    try {
      const res = await fetch("/api/fleet", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const out = (await res.json().catch(() => ({}))) as {
        href?: string;
        error?: string;
        detail?: string;
        fields?: Record<string, string>;
      };
      if (res.ok && out.href) {
        router.push(out.href);
        router.refresh();
        return;
      }
      setError({ message: out.error ?? "Noe gikk galt. Prøv igjen om litt.", detail: out.detail, fields: out.fields });
    } catch {
      setError({ message: "Vi får ikke kontakt med Fleet akkurat nå. Prøv igjen om litt." });
    } finally {
      setBusy(false);
    }
  }
  return { onSubmit, error, busy };
}

// shortcut: the detail is fleet-control's English message; #2864 tracks translating it.
const ActionError = ({ error }: { error?: FleetError }) =>
  error ? (
    <Alert variant="error" size="small">
      <VStack gap="space-8">
        <BodyShort>{error.message}</BodyShort>
        {error.detail && (
          <ReadMore header="Detaljer" size="small">
            <span lang="en">{error.detail}</span>
          </ReadMore>
        )}
      </VStack>
    </Alert>
  ) : null;

/**
 * A form whose submit button asks first in a Modal. The Modal's button submits the form,
 * so the request and its error handling stay those of useFleetPost.
 */
function ConfirmForm({
  onSubmit,
  heading,
  question,
  confirm,
  busy,
  children,
  fields,
}: {
  /** Inputs asked for in the Modal, such as the reason. They belong to the form through form={id}. */
  fields?: (formId: string) => ReactNode;
  onSubmit: (e: FormEvent<HTMLFormElement>) => void;
  heading: string;
  question: ReactNode;
  confirm: string;
  busy: boolean;
  children: ReactNode;
}) {
  const form = useRef<HTMLFormElement>(null);
  const [open, setOpen] = useState(false);
  const id = useId();
  return (
    <form ref={form} id={id} onSubmit={onSubmit}>
      <VStack gap="space-8" align="start">
        {children}
        <Button
          type="button"
          variant="danger"
          loading={busy}
          className="w-full md:w-auto"
          onClick={() => setOpen(true)}
        >
          {confirm}
        </Button>
      </VStack>
      <Modal open={open} onClose={() => setOpen(false)} header={{ heading }} width="small">
        <Modal.Body>
          <VStack gap="space-16">
            <BodyLong>{question}</BodyLong>
            {fields?.(id)}
          </VStack>
        </Modal.Body>
        <Modal.Footer>
          <Button
            type="button"
            variant="danger"
            onClick={() => {
              if (!form.current?.reportValidity()) return;
              form.current.requestSubmit();
              setOpen(false);
            }}
          >
            {confirm}
          </Button>
          <Button type="button" variant="secondary" onClick={() => setOpen(false)}>
            Avbryt
          </Button>
        </Modal.Footer>
      </Modal>
    </form>
  );
}

function ServerActions({
  server,
  me,
  myName,
  owner,
  part,
}: {
  server: MCPServer;
  me?: string;
  myName?: string;
  owner: boolean;
  part: "approve" | "rest" | "edit";
}) {
  const approve = useFleetPost("approve");
  const revoke = useFleetPost("revoke");
  const live = hasCondition(server, "Published") && !hasCondition(server, "Revoked");
  const { namespace, name } = server.metadata;
  const pending = server.status.pending;
  // Hides the button only; fleet-control enforces the two-person rule on approval.
  const ownVersion = !!me && pending?.submittedBy?.name === me;
  const ids = (
    <>
      <input type="hidden" name="namespace" value={namespace} />
      <input type="hidden" name="name" value={name} />
    </>
  );
  if (part === "approve")
    return pending?.contentHash ? (
      <Box background="default" borderWidth="1" borderColor="warning" borderRadius="12" padding="space-20">
        <VStack gap="space-12" align={{ xs: "stretch", md: "start" }}>
          <Heading level="2" size="medium">
            Versjon {pending.generation} venter
          </Heading>
          <BodyShort size="small">To personer i eierteamet: én sender inn, en annen godkjenner.</BodyShort>
          <Stack direction={{ xs: "column", md: "row" }} gap="space-16 space-40">
            <HStack gap="space-8" align="center">
              <Initials name={ownVersion ? myName : undefined} />
              <VStack>
                <Detail>Sendt inn av</Detail>
                <BodyShort weight="semibold">
                  {ownVersion ? (myName ? `${myName} (deg)` : "deg") : (pending.submittedBy?.name ?? "ingen person")}
                </BodyShort>
              </VStack>
            </HStack>
            <VStack>
              <Detail>Krever godkjenning fra</Detail>
              <BodyShort weight="semibold">en annen i {namespace}</BodyShort>
            </VStack>
          </Stack>
          {ownVersion ? (
            <BodyShort>
              Du sendte inn versjon {pending.generation} selv. En annen på {namespace} må godkjenne den, så du får ikke
              knappen Godkjenn. Vil du endre noe før det, bruk «Endre MCP-serveren» nederst på siden. Det blir en ny
              versjon som erstatter denne.
            </BodyShort>
          ) : (
            <form onSubmit={approve.onSubmit}>
              <VStack gap="space-8" align={{ xs: "stretch", md: "start" }}>
                {ids}
                <input type="hidden" name="generation" value={pending.generation} />
                <input type="hidden" name="contentHash" value={pending.contentHash} />
                <ActionError error={approve.error} />
                <Button loading={approve.busy} className="w-full md:w-auto">
                  Godkjenn versjon {pending.generation}
                </Button>
              </VStack>
            </form>
          )}
        </VStack>
      </Box>
    ) : null;
  return (
    <VStack gap="space-24">
      {live && part === "rest" && (
        <VStack gap="space-4">
          <Heading level="2" size="medium">
            Nødtiltak
          </Heading>
          <BodyShort size="small">
            Kan brukes av eierteamet og plattformteamet. Virker med en gang, uten godkjenning.
          </BodyShort>
        </VStack>
      )}
      {live && part === "rest" && (
        <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12" padding="space-20">
          <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
            <VStack gap="space-8">
              <Heading level="3" size="small">
                Rull tilbake
              </Heading>
              {/* shortcut: fleet-control has no rollback yet; the column keeps the sketch's layout. */}
              <BodyShort>
                Kommer i Fleet.
                {owner &&
                  " Til da: Er feil i oppsettet, endre MCP-serveren og send inn en ny versjon. Er feilen i koden til appen, ruller dere tilbake appen i Nais. Haster det for alle, kan plattformteamet bruke nødstopp i fleet-gateway."}
              </BodyShort>
            </VStack>
            <ConfirmForm
              onSubmit={revoke.onSubmit}
              heading={`Trekk tilbake ${name}?`}
              question="Ingen kan bruke MCP-serveren etterpå, og agentene mister tilgangen. For å publisere den igjen må dere sende inn en ny versjon som en annen på teamet godkjenner."
              confirm="Trekk tilbake"
              busy={revoke.busy}
              fields={(formId) => (
                <>
                  <Textarea
                    label="Begrunnelse"
                    description="Vises i historikken."
                    name="reason"
                    form={formId}
                    maxLength={500}
                    required
                  />
                  <Checkbox name="block" form={formId}>
                    Sperr denne versjonen for alltid. Den kan aldri sendes inn eller publiseres igjen.
                  </Checkbox>
                </>
              )}
            >
              <Heading level="3" size="small">
                Trekk tilbake
              </Heading>
              <BodyShort>Adressen slutter å svare. Agentene som bruker MCP-serveren, får feil.</BodyShort>
              {ids}
              <ActionError error={revoke.error} />
            </ConfirmForm>
          </HGrid>
        </Box>
      )}
      {owner && part === "edit" && (
        <ExpansionCard aria-label="Endre MCP-serveren">
          <ExpansionCard.Header>
            <ExpansionCard.Title as="h2" size="small">
              Endre MCP-serveren
            </ExpansionCard.Title>
            <ExpansionCard.Description>
              Endringen blir en ny versjon. En annen på {server.metadata.namespace} må godkjenne den før den gjelder.
            </ExpansionCard.Description>
          </ExpansionCard.Header>
          <ExpansionCard.Content>
            <ServerForm server={server} />
          </ExpansionCard.Content>
        </ExpansionCard>
      )}
    </VStack>
  );
}

const FLEET_GATEWAY_POLICY = `spec:
  accessPolicy:
    inbound:
      rules:
        - application: fleet-gateway
          namespace: copilot`;

/** Item 6 in #2920: what the app's nais.yaml needs before fleet-gateway can reach it. */
export function AccessPolicy({ app }: { app?: string }) {
  return (
    <VStack gap="space-8">
      <Heading level="2" size="small">
        Slipp inn fleet-gateway
      </Heading>
      <BodyLong>
        fleet-gateway kaller {app ? <code>{app}</code> : "appen"} på den interne adressen. Det virker bare hvis appen
        slipper inn fleet-gateway i <code>accessPolicy.inbound</code>. Fleet kan ikke sjekke dette ennå, så sjekk selv
        at <code>nais.yaml</code> har dette:
      </BodyLong>
      <Box background="neutral-soft" padding="space-12" borderRadius="8">
        <VStack gap="space-8">
          <HStack justify="space-between" align="center">
            <Detail as="code">nais.yaml</Detail>
            <CopyButton copyText={FLEET_GATEWAY_POLICY} text="Kopier" activeText="Kopiert" />
          </HStack>
          <pre className="overflow-x-auto" role="region" tabIndex={0} aria-label="nais.yaml">
            <code>{FLEET_GATEWAY_POLICY}</code>
          </pre>
        </VStack>
      </Box>
      <BodyShort size="small">Mangler regelen, svarer fleet-gateway med feil når agentene kaller verktøyene.</BodyShort>
    </VStack>
  );
}

export const categoryLabel = (c: string) => DATA_CATEGORIES.find((d) => d[0] === c)?.[1] ?? c;

const DATA_CATEGORIES: [string, string, string][] = [
  ["Aapen", "Åpne data", "Kan deles med alle, også utenfor Nav. For eksempel offentlige kodeverk og regelverk."],
  [
    "Intern",
    "Interne data",
    "For ansatte i Nav. Ikke personopplysninger og ikke taushetsbelagt. For eksempel interne rutiner og systemdokumentasjon.",
  ],
  [
    "Fortrolig",
    "Fortrolige data",
    "Kan skade personer eller Nav hvis det kommer på avveie, for eksempel personopplysninger. Velg Fortrolige data hvis du er i tvil.",
  ],
];

/** Register a new MCP server, or with server: edit it, which submits a new version for approval. */
export function ServerForm({ teams = [], server }: { teams?: string[]; server?: MCPServer }) {
  const post = useFleetPost(server ? "edit" : "register");
  const spec = server?.status.pending?.spec ?? server?.spec;
  const err = post.error?.fields;
  return (
    <form onSubmit={post.onSubmit}>
      <VStack gap="space-16" align="start">
        {server ? (
          <>
            <input type="hidden" name="namespace" value={server.metadata.namespace} />
            <input type="hidden" name="name" value={server.metadata.name} />
            <input type="hidden" name="resourceVersion" value={server.metadata.resourceVersion ?? ""} />
          </>
        ) : (
          <>
            <Select label="Team" name="namespace" required>
              {teams.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </Select>
            <TextField
              label="Navn"
              name="name"
              error={err?.name}
              description="Små bokstaver, tall og bindestrek."
              required
            />
          </>
        )}
        <TextField
          label="App"
          name="app"
          defaultValue={spec?.upstream?.app}
          error={err?.app}
          description="Nais-appen som kjører MCP-serveren."
          required
        />
        <TextField
          label="Intern adresse"
          name="url"
          defaultValue={spec?.upstream?.url}
          error={err?.url}
          description="For eksempel http://min-app.mitt-team/mcp eller http://min-app.mitt-team.svc.cluster.local/mcp"
          required
        />
        <Textarea
          label="Verktøy"
          name="tools"
          defaultValue={spec?.allowedTools?.join("\n")}
          error={err?.tools}
          description="Verktøyene som skal være tilgjengelige, ett per linje."
          required
        />
        <Textarea
          label="Beskrivelse (valgfritt)"
          name="readme"
          defaultValue={spec?.readme}
          error={err?.readme}
          description="Fortell hva MCP-serveren gjør og hvordan den brukes. Du kan bruke Markdown. Høyst 64 KiB."
        />
        <RadioGroup
          legend="Hva slags data gir verktøyene ut?"
          name="dataCategory"
          defaultValue={spec?.dataCategory}
          error={err?.dataCategory}
          description="Velg den strengeste kategorien som passer. Strengt fortrolig krever egen risikovurdering og godkjenning først."
        >
          {DATA_CATEGORIES.map(([value, label, help]) => (
            <Radio key={value} value={value} description={help}>
              {label}
            </Radio>
          ))}
        </RadioGroup>
        {!server && <AccessPolicy />}
        <ActionError error={post.error} />
        <Button loading={post.busy}>
          {server ? "Send inn ny versjon til godkjenning" : "Send inn til godkjenning"}
        </Button>
      </VStack>
    </form>
  );
}

export function agentNextStep(p: AgentProfile): string {
  if (hasCondition(p, "Revoked") || hasCondition(p, "Expired")) return "Ingen.";
  const left = awaiting(p);
  if (left.length) return `Eierteamet til hver MCP-server må gi tilgang. Venter på: ${left.join(", ")}.`;
  if (hasCondition(p, "Published"))
    return p.status.expiresAt
      ? `Ingen. Profilen er godkjent til ${date(p.status.expiresAt)}.`
      : "Ingen. Profilen er godkjent.";
  return "Ingen. Profilen blir godkjent når alle har gitt tilgang.";
}

/** What the owner team has done with one grant. Another team's view of the profile leaves out who granted access. */
function grantGiven(grants: NonNullable<AgentProfile["status"]["grants"]>, ref: ServerRef): string {
  const st = grants.find((s) => refKey(s.serverRef) === refKey(ref));
  if (["AccessRevoked", "ServerRevoked"].includes(st?.reason ?? "")) return "tilgang trukket tilbake";
  if (st?.granted)
    return ["tilgang gitt", st.grantedBy && `av ${st.grantedBy.name}`, st.grantedAt && date(st.grantedAt)]
      .filter(Boolean)
      .join(" ");
  return "ikke gitt tilgang";
}

function GrantTag({ given }: { given: string }) {
  const ok = given.startsWith("tilgang gitt");
  const wait = given === "ikke gitt tilgang";
  return (
    <Tag size="small" className="whitespace-nowrap" variant={ok ? "success" : wait ? "warning" : "neutral"}>
      {ok ? "I bruk" : wait ? "Venter på eierteamet" : "Trukket tilbake"}
    </Tag>
  );
}

export function AgentDetailView({ profile: p, events }: { profile: AgentProfile; events: AuditEvent[] | null }) {
  const grants = p.status.grants ?? [];
  // Show what waits for approval, since status.grants describe that generation.
  const spec = p.status.pending?.spec ?? p.spec;
  return (
    <VStack gap="space-16">
      <VStack gap="space-8">
        <BodyLong>
          En agentprofil sier hvilke MCP-servere en KI-agent i teamet får bruke. Teamet er konsumenten, og eierteamet
          til hver MCP-server gir tilgang.
        </BodyLong>
        <BodyLong>Neste steg: {agentNextStep(p)}</BodyLong>
      </VStack>
      {p.status.needsResubmit && (
        <Alert variant="warning" size="small">
          Utkastet ble ikke sendt inn av en person, så ingen kan godkjenne det. En person på teamet må sende det inn på
          nytt.
        </Alert>
      )}
      <VStack gap="space-8">
        <Heading level="2" size="medium">
          Tilganger per MCP-server
        </Heading>
        {(spec.grants ?? []).length === 0 ? (
          <BodyShort>Profilen ber ikke om tilgang til noen MCP-server.</BodyShort>
        ) : (
          <>
            <Show below="md">
              <CardList
                label="Tilganger per MCP-server"
                items={(spec.grants ?? []).map((g) => {
                  const given = grantGiven(grants, g.serverRef);
                  return {
                    key: refKey(g.serverRef),
                    title: (
                      <NextLink className={linkClass} href={`/fleet/mcp-servere/${refKey(g.serverRef)}`}>
                        <strong>{g.serverRef.name}</strong>
                      </NextLink>
                    ),
                    aside: <GrantTag given={given} />,
                    lines: [
                      <Detail key="r" as="code">
                        /mcp/{g.serverRef.namespace}/{g.serverRef.name}
                      </Detail>,
                      <Detail key="t">{g.allowedTools.join(", ")}</Detail>,
                      <Detail key="g">
                        Eierteam {g.serverRef.namespace}: {given}
                      </Detail>,
                    ],
                  };
                })}
              />
            </Show>
            <Hide below="md">
              <FleetTable aria-label="Tilganger per MCP-server">
                <Table.Header>
                  <Table.Row>
                    <Table.HeaderCell scope="col">MCP-server og verktøy</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Eierteam</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Status</Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {(spec.grants ?? []).map((g) => {
                    const given = grantGiven(grants, g.serverRef);
                    return (
                      <Table.Row key={refKey(g.serverRef)}>
                        <Table.DataCell className="min-w-64">
                          <VStack gap="space-4">
                            <NextLink className={linkClass} href={`/fleet/mcp-servere/${refKey(g.serverRef)}`}>
                              <strong>{g.serverRef.name}</strong>
                            </NextLink>
                            <Detail as="code" className="[overflow-wrap:anywhere]">
                              /mcp/{g.serverRef.namespace}/{g.serverRef.name}
                            </Detail>
                            <Detail className="[overflow-wrap:anywhere]">{g.allowedTools.join(", ")}</Detail>
                          </VStack>
                        </Table.DataCell>
                        <Table.DataCell>{g.serverRef.namespace}</Table.DataCell>
                        <Table.DataCell>
                          <VStack gap="space-4" align="start">
                            <GrantTag given={given} />
                            <Detail>{given}</Detail>
                          </VStack>
                        </Table.DataCell>
                      </Table.Row>
                    );
                  })}
                </Table.Body>
              </FleetTable>
            </Hide>
          </>
        )}
      </VStack>
      <Detaljer
        rows={[
          ["Team", p.metadata.namespace],
          ["App", `${spec.identity.cluster}/${p.metadata.namespace}/${spec.identity.app}`],
          ["Entra-klient-ID", spec.identity.entraClientID],
          [
            "Kvote",
            `${n(spec.quota.requestsPerMinute)} kall i minuttet, ${n(spec.quota.dailyCalls)} kall i døgnet, ${n(spec.quota.dailyCallsPerUser)} per bruker i døgnet, ${n(spec.quota.maxInFlight)} samtidige kall. fleet-gateway avviser kall over kvoten.`,
          ],
          ...(spec.dpia
            ? [
                [
                  "DPIA",
                  `${spec.dpia.reference} (eier: ${spec.dpia.owner}). Vurderingen av personvernkonsekvenser for agenten.`,
                ] as [string, ReactNode],
              ]
            : []),
        ]}
      />
      {events && <Historikk events={events} />}
    </VStack>
  );
}
