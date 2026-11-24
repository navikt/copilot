"use client";

// Client component because Aksel's compound components (Table.Row, Tabs.List)
// are undefined when read off a client reference in a server component.
import NextLink from "next/link";
import { useRouter } from "next/navigation";
import type { ReactNode } from "react";
import {
  Alert,
  BodyLong,
  BodyShort,
  Box,
  Button,
  ExpansionCard,
  Heading,
  Table,
  Tabs,
  Tag,
  VStack,
} from "@navikt/ds-react";
import { hasCondition, refKey } from "@/lib/fleet-control";
import type { AgentProfile, AuditEvent, FleetProblem, MCPServer } from "@/lib/fleet-control";

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
        Du har ikke tilgang. Bare eierteamet til MCP-serveren eller agentprofilen kan se den.
      </Alert>
    );
  }
  if (problem === "conflict") {
    return <Alert variant="warning">Noen andre har endret dette. Last inn siden på nytt.</Alert>;
  }
  return (
    <Alert variant="error">
      <VStack gap="space-8" align="start">
        <BodyShort>Vi får ikke kontakt med fleet-control akkurat nå. Prøv igjen om litt.</BodyShort>
        <Button size="small" variant="secondary" onClick={() => router.refresh()}>
          Prøv igjen
        </Button>
      </VStack>
    </Alert>
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

export type TeamData = { team: string; servers: MCPServer[]; agents: AgentProfile[] };

export function TeamTabs({ teams }: { teams: TeamData[] }) {
  return (
    <Tabs defaultValue={teams[0].team}>
      <Tabs.List>
        {teams.map((t) => (
          <Tabs.Tab key={t.team} value={t.team} label={t.team} />
        ))}
      </Tabs.List>
      {teams.map((t) => (
        <Tabs.Panel key={t.team} value={t.team}>
          <VStack gap="space-24" paddingBlock="space-16">
            <VStack gap="space-8">
              <Heading level="3" size="small">
                MCP-servere
              </Heading>
              {t.servers.length === 0 ? (
                <BodyShort>{t.team} har ingen MCP-servere ennå.</BodyShort>
              ) : (
                <Table size="small" aria-label={`MCP-serverne til ${t.team}`}>
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
                </Table>
              )}
            </VStack>
            <VStack gap="space-8">
              <Heading level="3" size="small">
                Agentprofiler
              </Heading>
              {t.agents.length === 0 ? (
                <BodyShort>{t.team} har ingen agentprofiler ennå.</BodyShort>
              ) : (
                <Table size="small" aria-label={`Agentprofilene til ${t.team}`}>
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
                </Table>
              )}
            </VStack>
          </VStack>
        </Tabs.Panel>
      ))}
    </Tabs>
  );
}

/** The MCP servers whose owner team has not granted access yet. A revoked grant keeps grantedAt and is not waiting. */
const awaiting = (p: AgentProfile) =>
  (p.status.grants ?? []).filter((g) => !g.granted && !g.grantedAt).map((g) => `MCP-server ${refKey(g.serverRef)}`);

export function PendingInbox({ agents }: { agents: AgentProfile[] }) {
  return (
    <VStack gap="space-8">
      <Heading level="2" size="medium">
        Til godkjenning
      </Heading>
      {agents.length === 0 ? (
        <BodyShort>Ingen agentprofiler venter på teamene dine.</BodyShort>
      ) : (
        <Table size="small" aria-label="Agentprofiler til godkjenning">
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell scope="col">Agentprofil</Table.HeaderCell>
              <Table.HeaderCell scope="col">Fra team</Table.HeaderCell>
              <Table.HeaderCell scope="col">Venter på tilgang</Table.HeaderCell>
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
              </Table.Row>
            ))}
          </Table.Body>
        </Table>
      )}
    </VStack>
  );
}

function Historikk({ events }: { events: AuditEvent[] }) {
  return (
    <ExpansionCard aria-label="Historikk" size="small">
      <ExpansionCard.Header>
        <ExpansionCard.Title as="h2" size="small">
          Historikk
        </ExpansionCard.Title>
      </ExpansionCard.Header>
      <ExpansionCard.Content>
        {events.length === 0 ? (
          <BodyShort>Ingen hendelser ennå.</BodyShort>
        ) : (
          <Table size="small" aria-label="Historikk">
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
          </Table>
        )}
      </ExpansionCard.Content>
    </ExpansionCard>
  );
}

function Detaljer({ rows }: { rows: [string, ReactNode][] }) {
  return (
    <ExpansionCard aria-label="Detaljer" size="small">
      <ExpansionCard.Header>
        <ExpansionCard.Title as="h2" size="small">
          Detaljer
        </ExpansionCard.Title>
      </ExpansionCard.Header>
      <ExpansionCard.Content>
        <Table size="small" aria-label="Detaljer">
          <Table.Body>
            {rows.map(([k, v]) => (
              <Table.Row key={k}>
                <Table.HeaderCell scope="row">{k}</Table.HeaderCell>
                <Table.DataCell>{v}</Table.DataCell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table>
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
      : "Ingen. MCP-serveren er trukket tilbake.";
  if (!hasCondition(s, "Published"))
    return "En annen person på teamet må godkjenne utkastet før MCP-serveren blir publisert.";
  if (pending) return `En annen person på teamet må godkjenne versjon ${pending}. Til da gjelder versjon ${published}.`;
  return `Ingen. Versjon ${published} er publisert.`;
}

export function ServerDetail({ server, events }: { server: MCPServer; events: AuditEvent[] }) {
  return (
    <VStack gap="space-16">
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
      <Historikk events={events} />
    </VStack>
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
