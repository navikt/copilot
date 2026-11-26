"use client";

// Client component because Aksel's compound components (Table.Row, ExpansionCard.Header)
// are undefined when read off a client reference in a server component.
import NextLink from "next/link";
import { useRouter } from "next/navigation";
import { useRef, useState, type FormEvent, type ReactNode } from "react";
import {
  Alert,
  BodyLong,
  BodyShort,
  Box,
  Button,
  Checkbox,
  CopyButton,
  ExpansionCard,
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
  Table,
  Tag,
  Textarea,
  TextField,
  VStack,
} from "@navikt/ds-react";
import { ExternalLinkIcon } from "@navikt/aksel-icons";
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
  if (hasCondition(p, "Published")) return { label: "Godkjent", variant: "success" };
  return { label: "Venter på tilgang", variant: "warning" };
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
  <Tag size="small" variant={s.variant}>
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
    <div className="overflow-x-auto" role="region" tabIndex={0} aria-label={label} data-fleet-table="">
      <Table size="small" aria-label={label}>
        {children}
      </Table>
    </div>
  );
}

export function KortFortalt({ children }: { children: ReactNode }) {
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

export type TeamData = { team: string; servers: MCPServer[]; agents: AgentProfile[] };

export function TeamSections({ teams }: { teams: TeamData[] }) {
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
                <FleetTable aria-label={`MCP-serverne til ${t.team}`}>
                  <Table.Header>
                    <Table.Row>
                      <Table.HeaderCell scope="col">Navn</Table.HeaderCell>
                      <Table.HeaderCell scope="col">Status</Table.HeaderCell>
                      <Table.HeaderCell scope="col">Publisert versjon</Table.HeaderCell>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {t.servers.map((s) => (
                      <Table.Row key={s.metadata.name}>
                        <Table.DataCell>
                          <NextLink className={linkClass} href={`/fleet/mcp-servere/${refKey(s.metadata)}`}>
                            {s.metadata.name}
                          </NextLink>
                        </Table.DataCell>
                        <Table.DataCell>
                          <StatusTag s={serverStatus(s)} />
                        </Table.DataCell>
                        <Table.DataCell>{s.status.publishedGeneration ?? "–"}</Table.DataCell>
                      </Table.Row>
                    ))}
                  </Table.Body>
                </FleetTable>
              )}
            </VStack>
            {/* A team with MCP servers and no agent profiles needs no empty section. */}
            {(t.agents.length > 0 || t.servers.length === 0) && (
              <VStack gap="space-8">
                <Heading id="agentprofiler" level="2" size="medium">
                  Agentprofiler
                </Heading>
                {t.agents.length === 0 ? (
                  <BodyShort>{t.team} har ingen agentprofiler ennå.</BodyShort>
                ) : (
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
                          <Table.DataCell>
                            {(a.spec.grants ?? []).map((g) => refKey(g.serverRef)).join(", ")}
                          </Table.DataCell>
                        </Table.Row>
                      ))}
                    </Table.Body>
                  </FleetTable>
                )}
              </VStack>
            )}
          </VStack>
        </section>
      ))}
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
      <Heading id="til-godkjenning" level="2" size="medium">
        Til godkjenning
      </Heading>
      {agents.length === 0 ? (
        <BodyShort>Ingen agentprofiler venter på teamene dine.</BodyShort>
      ) : (
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
  owner = false,
  agents = [],
  agentsFailed = false,
  grafanaUrl,
  submitted = false,
}: {
  server: MCPServer;
  events: AuditEvent[];
  submitted?: boolean;
  me?: string;
  // In the owner team: only then the edit form shows. fleet-control checks it again.
  owner?: boolean;
  agents?: AgentProfile[];
  agentsFailed?: boolean;
  grafanaUrl?: string;
}) {
  return (
    <VStack gap="space-16">
      {submitted && server.status.pending && <SubmittedAlert server={server} />}
      <KortFortalt>
        <div>
          <StatusTag s={serverStatus(server)} />
        </div>
        <BodyLong>Neste steg: {serverNextStep(server)}</BodyLong>
      </KortFortalt>
      <Detaljer
        rows={[
          ["Team", server.metadata.namespace],
          ["Navn", server.metadata.name],
          ["Rute", server.status.route?.path ?? "–"],
          ["Publisert versjon", server.status.publishedGeneration ?? "–"],
          ["Versjon som venter på godkjenning", server.status.pending?.generation ?? "–"],
        ]}
      />
      {grafanaUrl && (
        <BodyShort>
          <Link href={grafanaUrl} target="_blank" rel="noopener noreferrer">
            Se bruk i Grafana <ExternalLinkIcon fontSize="1rem" title="åpnes i ny fane" />
          </Link>
        </BodyShort>
      )}
      {agentsFailed ? (
        <Alert variant="warning" size="small">
          Vi fikk ikke hentet agentprofilene som venter på tilgang. Last inn siden på nytt.
        </Alert>
      ) : (
        <WaitingForAccess server={server} agents={agents} />
      )}
      <AccessPolicy app={(server.status.pending?.spec ?? server.spec)?.upstream?.app} />
      <ServerDocs spec={server.status.pending?.spec ?? server.spec} />
      {server.status.pending?.spec && <Versjoner server={server} next={server.status.pending.spec} />}
      <ServerActions server={server} me={me} owner={owner} />
      <Historikk events={events} />
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
    <VStack gap="space-8">
      <Heading level="2" size="small">
        Om MCP-serveren
      </Heading>
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
  );
}

function Versjoner({ server, next }: { server: MCPServer; next: MCPServerSpec }) {
  // Before the first approval fleet-control copies the draft into spec, so only a live server has a version in use.
  const none = hasCondition(server, "Revoked")
    ? "Trukket tilbake"
    : hasCondition(server, "Published")
      ? null
      : "Ikke publisert ennå";
  const now = specRows(none ? undefined : server.spec).map(([k, v]): [string, string] => [k, none ?? v]);
  return (
    <FleetTable aria-label="Versjonen som gjelder, og versjonen som venter">
      <Table.Header>
        <Table.Row>
          <Table.HeaderCell scope="col">Felt</Table.HeaderCell>
          <Table.HeaderCell scope="col">Gjelder nå</Table.HeaderCell>
          <Table.HeaderCell scope="col">Venter på godkjenning</Table.HeaderCell>
        </Table.Row>
      </Table.Header>
      <Table.Body>
        {specRows(next).map(([k, v], i) => (
          <Table.Row key={k}>
            <Table.HeaderCell scope="row">{k}</Table.HeaderCell>
            <Table.DataCell>{now[i][1]}</Table.DataCell>
            <Table.DataCell>{v === now[i][1] ? v : <strong>{v}</strong>}</Table.DataCell>
          </Table.Row>
        ))}
      </Table.Body>
    </FleetTable>
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
}: {
  onSubmit: (e: FormEvent<HTMLFormElement>) => void;
  heading: string;
  question: ReactNode;
  confirm: string;
  busy: boolean;
  children: ReactNode;
}) {
  const form = useRef<HTMLFormElement>(null);
  const [open, setOpen] = useState(false);
  return (
    <form ref={form} onSubmit={onSubmit}>
      <VStack gap="space-8" align="start">
        {children}
        <Button
          type="button"
          variant="danger"
          loading={busy}
          onClick={() => form.current?.reportValidity() && setOpen(true)}
        >
          {confirm}
        </Button>
      </VStack>
      <Modal open={open} onClose={() => setOpen(false)} header={{ heading }} width="small">
        <Modal.Body>
          <BodyLong>{question}</BodyLong>
        </Modal.Body>
        <Modal.Footer>
          <Button
            type="button"
            variant="danger"
            onClick={() => {
              setOpen(false);
              form.current?.requestSubmit();
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

function ServerActions({ server, me, owner }: { server: MCPServer; me?: string; owner: boolean }) {
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
  return (
    <VStack gap="space-24">
      {pending?.contentHash && ownVersion && (
        <BodyShort>
          Du sendte inn versjon {pending.generation} selv. En annen på {namespace} må godkjenne den.
        </BodyShort>
      )}
      {pending?.contentHash && !ownVersion && (
        <form onSubmit={approve.onSubmit}>
          <VStack gap="space-8" align="start">
            <Heading level="2" size="small">
              Godkjenn versjon {pending.generation}
            </Heading>
            <BodyShort>Du kan ikke godkjenne en versjon du sendte inn selv.</BodyShort>
            {ids}
            <input type="hidden" name="generation" value={pending.generation} />
            <input type="hidden" name="contentHash" value={pending.contentHash} />
            <ActionError error={approve.error} />
            <Button loading={approve.busy}>Godkjenn og publiser</Button>
          </VStack>
        </form>
      )}
      {live && (
        <ConfirmForm
          onSubmit={revoke.onSubmit}
          heading={`Trekk tilbake ${name}?`}
          question="Ingen kan bruke MCP-serveren etterpå, og agentene mister tilgangen. For å publisere den igjen må dere sende inn en ny versjon som en annen på teamet godkjenner."
          confirm="Trekk tilbake"
          busy={revoke.busy}
        >
          <Heading level="2" size="small">
            Trekk tilbake
          </Heading>
          <BodyShort>Ingen kan bruke MCP-serveren etterpå. Du trenger ikke godkjenning fra andre.</BodyShort>
          {ids}
          <Textarea label="Begrunnelse" name="reason" maxLength={500} required />
          <Checkbox name="block">
            Sperr denne versjonen for alltid. Den kan aldri sendes inn eller publiseres igjen, heller ikke ved
            tilbakerulling.
          </Checkbox>
          <ActionError error={revoke.error} />
        </ConfirmForm>
      )}
      {owner && live && (
        <VStack gap="space-8">
          <Heading level="2" size="small">
            Har MCP-serveren et problem?
          </Heading>
          <BodyLong>
            Trekk den tilbake ovenfor hvis ingen skal bruke den. Er feil i oppsettet, endre MCP-serveren og send inn en
            ny versjon. Er feilen i koden til appen, ruller dere tilbake appen i Nais. Haster det for alle, kan
            plattformteamet bruke nødstopp i fleet-gateway.
          </BodyLong>
        </VStack>
      )}
      {owner && (
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
        <HStack gap="space-8" justify="space-between" align="start" wrap={false}>
          <pre className="overflow-x-auto">
            <code>{FLEET_GATEWAY_POLICY}</code>
          </pre>
          <CopyButton copyText={FLEET_GATEWAY_POLICY} text="Kopier" activeText="Kopiert" />
        </HStack>
      </Box>
      <BodyShort size="small">Mangler regelen, svarer fleet-gateway med feil når agentene kaller verktøyene.</BodyShort>
    </VStack>
  );
}

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

export function AgentDetailView({ profile: p, events }: { profile: AgentProfile; events: AuditEvent[] | null }) {
  const grants = p.status.grants ?? [];
  // Show what waits for approval, since status.grants describe that generation.
  const spec = p.status.pending?.spec ?? p.spec;
  return (
    <VStack gap="space-16">
      <KortFortalt>
        <div>
          <StatusTag s={agentStatus(p)} />
        </div>
        <BodyLong>Neste steg: {agentNextStep(p)}</BodyLong>
      </KortFortalt>
      {p.status.needsResubmit && (
        <Alert variant="warning" size="small">
          Utkastet ble ikke sendt inn av en person, så ingen kan godkjenne det. En person på teamet må sende det inn på
          nytt.
        </Alert>
      )}
      <Detaljer
        rows={[
          ["Team", p.metadata.namespace],
          ["App", `${spec.identity.cluster}/${p.metadata.namespace}/${spec.identity.app}`],
          ["Entra-klient-ID", spec.identity.entraClientID],
          ...(spec.grants ?? []).map((g): [string, ReactNode] => {
            const st = grants.find((s) => refKey(s.serverRef) === refKey(g.serverRef));
            // Another team's view of the profile leaves out who granted access.
            const given = ["AccessRevoked", "ServerRevoked"].includes(st?.reason ?? "")
              ? "tilgang trukket tilbake"
              : st?.granted
                ? ["tilgang gitt", st.grantedBy && `av ${st.grantedBy.name}`, st.grantedAt && date(st.grantedAt)]
                    .filter(Boolean)
                    .join(" ")
                : "ikke gitt tilgang";
            return [`MCP-server ${refKey(g.serverRef)}`, `${g.allowedTools.join(", ")} (${given})`];
          }),
          [
            "Kvote",
            `${n(spec.quota.requestsPerMinute)} kall i minuttet, ${n(spec.quota.dailyCalls)} kall i døgnet, ${n(spec.quota.dailyCallsPerUser)} per bruker i døgnet, ${n(spec.quota.maxInFlight)} samtidige kall`,
          ],
          ...(spec.dpia ? [["DPIA", `${spec.dpia.reference} (eier: ${spec.dpia.owner})`] as [string, ReactNode]] : []),
        ]}
      />
      {events && <Historikk events={events} />}
    </VStack>
  );
}
