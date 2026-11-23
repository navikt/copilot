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
import type {
  AgentDetail,
  AgentProfile,
  AuditEvent,
  FleetProblem,
  PendingAgent,
  ServerSummary,
} from "@/lib/fleet-control";

const linkClass = "text-blue-600 hover:underline";

type TagVariant = "success" | "warning" | "info" | "neutral" | "error";

export function serverStatus(s: ServerSummary): { label: string; variant: TagVariant } {
  if (s.status === "draft") return { label: "Venter på godkjenning", variant: "warning" };
  // A revoked route can get a new draft, and approving it publishes the route again.
  if (s.status === "revoked")
    return s.pending_version
      ? { label: "Trukket tilbake, ny versjon venter", variant: "warning" }
      : { label: "Trukket tilbake", variant: "neutral" };
  if (s.pending_version) return { label: "Ny versjon venter", variant: "info" };
  return { label: "Publisert", variant: "success" };
}

export function agentStatus(status: AgentProfile["status"]): { label: string; variant: TagVariant } {
  switch (status) {
    case "approved":
      return { label: "Godkjent", variant: "success" };
    case "expired":
      return { label: "Utløpt", variant: "neutral" };
    case "revoked":
      return { label: "Trukket tilbake", variant: "neutral" };
    default:
      return { label: "Venter på godkjenning", variant: "warning" };
  }
}

const ACTIONS: Record<string, string> = {
  draft_submitted: "Utkast sendt inn",
  approved_published: "Godkjent og publisert",
  revoked: "Trukket tilbake",
  rolled_back: "Rullet tilbake",
  agent_profile_drafted: "Utkast opprettet",
  agent_profile_route_approved: "Rute godkjent",
  agent_profile_approved: "Profilen godkjent",
  agent_profile_revoked: "Trukket tilbake",
  agent_profile_expired: "Utløpt",
};

const date = (iso: string) =>
  new Date(iso).toLocaleString("nb-NO", { dateStyle: "medium", timeStyle: "short", timeZone: "Europe/Oslo" });

const StatusTag = ({ s }: { s: { label: string; variant: TagVariant } }) => (
  <Tag size="small" variant={s.variant}>
    {s.label}
  </Tag>
);

/** 403 and 503 (and timeouts). 401 and 404 never get here: the page logs in again or shows «Fant ikke siden». */
export function FleetProblemAlert({ problem }: { problem: FleetProblem }) {
  const router = useRouter();
  if (problem === "forbidden") {
    return (
      <Alert variant="warning">Du har ikke tilgang. Bare teamet som eier ruten eller agentprofilen, kan se den.</Alert>
    );
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

export type TeamData = { team: string; servers: ServerSummary[]; agents: AgentProfile[] };

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
                Ruter
              </Heading>
              {t.servers.length === 0 ? (
                <BodyShort>{t.team} har ingen ruter ennå.</BodyShort>
              ) : (
                <Table size="small" aria-label={`Rutene til ${t.team}`}>
                  <Table.Header>
                    <Table.Row>
                      <Table.HeaderCell scope="col">Navn</Table.HeaderCell>
                      <Table.HeaderCell scope="col">Status</Table.HeaderCell>
                      <Table.HeaderCell scope="col">Publisert versjon</Table.HeaderCell>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {t.servers.map((s) => (
                      <Table.Row key={s.name}>
                        <Table.DataCell>
                          <NextLink className={linkClass} href={`/fleet/ruter/${s.namespace}/${s.name}`}>
                            {s.name}
                          </NextLink>
                        </Table.DataCell>
                        <Table.DataCell>
                          <StatusTag s={serverStatus(s)} />
                        </Table.DataCell>
                        <Table.DataCell>{s.published_version ?? "–"}</Table.DataCell>
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
                      <Table.HeaderCell scope="col">Ruter</Table.HeaderCell>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {t.agents.map((a) => (
                      <Table.Row key={a.name}>
                        <Table.DataCell>
                          <NextLink className={linkClass} href={`/fleet/agenter/${a.namespace}/${a.name}`}>
                            {a.name}
                          </NextLink>
                        </Table.DataCell>
                        <Table.DataCell>
                          <StatusTag s={agentStatus(a.status)} />
                        </Table.DataCell>
                        <Table.DataCell>{a.grants.map((g) => g.route).join(", ")}</Table.DataCell>
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

export function PendingInbox({ agents }: { agents: PendingAgent[] }) {
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
              <Table.HeaderCell scope="col">Ruter som venter på godkjenning</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {agents.map((a) => (
              <Table.Row key={a.id}>
                <Table.DataCell>
                  <NextLink className={linkClass} href={`/fleet/agenter/${a.profile.namespace}/${a.profile.name}`}>
                    {a.profile.name}
                  </NextLink>
                </Table.DataCell>
                <Table.DataCell>{a.profile.namespace}</Table.DataCell>
                <Table.DataCell>{a.awaiting.join(", ")}</Table.DataCell>
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
                  <Table.DataCell>{e.actor}</Table.DataCell>
                  <Table.DataCell>{e.version ?? "–"}</Table.DataCell>
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

export function routeNextStep(s: ServerSummary): string {
  if (s.status === "draft") return "En annen person på teamet må godkjenne utkastet før ruten blir publisert.";
  if (s.status === "revoked")
    return s.pending_version
      ? `En annen person på teamet må godkjenne versjon ${s.pending_version} før ruten blir publisert igjen.`
      : "Ingen. Ruten er trukket tilbake.";
  if (s.pending_version)
    return `En annen person på teamet må godkjenne versjon ${s.pending_version}. Til da gjelder versjon ${s.published_version}.`;
  return `Ingen. Versjon ${s.published_version} er publisert.`;
}

export function RouteDetail({ server, events }: { server: ServerSummary; events: AuditEvent[] }) {
  return (
    <VStack gap="space-16">
      <KortFortalt>
        <div>
          <StatusTag s={serverStatus(server)} />
        </div>
        <BodyLong>Neste steg: {routeNextStep(server)}</BodyLong>
      </KortFortalt>
      <Detaljer
        rows={[
          ["Team", server.namespace],
          ["Navn", server.name],
          ["Publisert versjon", server.published_version ?? "–"],
          ["Versjon som venter på godkjenning", server.pending_version ?? "–"],
        ]}
      />
      <Historikk events={events} />
    </VStack>
  );
}

export function agentNextStep(d: AgentDetail): string {
  if (d.status === "approved")
    return d.profile.expires_at
      ? `Ingen. Profilen er godkjent til ${date(d.profile.expires_at)}.`
      : "Ingen. Profilen er godkjent.";
  if (d.status === "expired" || d.status === "revoked") return "Ingen.";
  const left = Object.entries(d.approvals)
    .filter(([, a]) => !a)
    .map(([route]) => route);
  return `Teamet som eier hver rute, må godkjenne den. Venter på: ${left.join(", ")}.`;
}

export function AgentDetailView({ detail, events }: { detail: AgentDetail; events: AuditEvent[] | null }) {
  const p = detail.profile;
  return (
    <VStack gap="space-16">
      <KortFortalt>
        <div>
          <StatusTag s={agentStatus(detail.status)} />
        </div>
        <BodyLong>Neste steg: {agentNextStep(detail)}</BodyLong>
      </KortFortalt>
      <Detaljer
        rows={[
          ["Team", p.namespace],
          ["App", `${p.identity.cluster}/${p.identity.namespace}/${p.identity.app}`],
          ["Entra-klient-ID", p.identity.entra_client_id],
          ...p.grants.map((g): [string, ReactNode] => {
            const a = detail.approvals[g.route];
            return [
              `Rute ${g.route}`,
              `${g.tools.join(", ")} (${a ? `godkjent av ${a.approved_by} ${date(a.approved_at)}` : "ikke godkjent"})`,
            ];
          }),
          [
            "Kvote",
            `${p.quota.rpm} kall i minuttet, ${p.quota.daily_calls} kall i døgnet, ${p.quota.daily_calls_per_user} per bruker i døgnet, ${p.quota.max_in_flight} samtidige kall`,
          ],
          ["DPIA", `${p.dpia.reference} (eier: ${p.dpia.owner})`],
        ]}
      />
      {events && <Historikk events={events} />}
    </VStack>
  );
}
