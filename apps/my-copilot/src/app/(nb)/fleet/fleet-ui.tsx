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
  Detail,
  ExpansionCard,
  HGrid,
  Hide,
  HStack,
  Modal,
  Link,
  ReadMore,
  Heading,
  Show,
  Table,
  Tag,
  VStack,
} from "@navikt/ds-react";
import { hasCondition, refKey } from "@/lib/fleet-names";
import type { AgentProfile, AuditEvent, FleetProblem, MCPServer } from "@/lib/fleet-control";
import { type TagVariant, agentStatus, categoryLabel, date, serverShortNext, serverStatus } from "./fleet-status";

export const linkClass = "text-blue-600 hover:underline";

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

export const StatusTag = ({ s }: { s: { label: string; variant: TagVariant } }) => (
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
          Registrer en MCP-server som teamet allerede kjører i Nais. Team copilot godkjenner før den blir publisert.
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

/** After register or edit: the version waits for team copilot unless it gives no more access. */
export function SubmittedAlert({ server }: { server: MCPServer }) {
  const path = server.status.route?.path ?? `/mcp/${server.metadata.namespace}/${server.metadata.name}`;
  if (!server.status.pending)
    return (
      <Alert variant="success">
        <Heading level="2" size="small">
          Endringen er publisert
        </Heading>
        <BodyLong>
          Versjon {server.status.publishedGeneration} gjelder nå på <code>{path}</code>. Endringen ga ikke mer tilgang,
          så team copilot trengte ikke å godkjenne den.
        </BodyLong>
      </Alert>
    );
  return (
    <Alert variant="success">
      <Heading level="2" size="small">
        {server.metadata.name} er sendt til godkjenning
      </Heading>
      <BodyLong>
        Versjon {server.status.pending.generation} blir publisert på <code>{path}</code> når team copilot har godkjent
        den.
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
        <BodyShort size="small" className="xl:whitespace-nowrap">
          {short ? (
            <>
              <Hide below="xl" asChild>
                <span>{label}</span>
              </Hide>
              <Show below="xl" asChild>
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

export type TeamData = { team: string; servers: MCPServer[]; agents: AgentProfile[] };

export function TeamSections({ teams, approver }: { teams: TeamData[]; approver?: boolean }) {
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
                              {serverShortNext(s, approver) && <Detail>{serverShortNext(s, approver)}</Detail>}
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
                              <VStack gap="space-4" align="start">
                                <StatusTag s={serverStatus(s)} />
                                {serverShortNext(s, approver) && <Detail>{serverShortNext(s, approver)}</Detail>}
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

export function Historikk({ events }: { events: AuditEvent[] }) {
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

export function Detaljer({ rows }: { rows: [string, ReactNode][] }) {
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
                <Table.HeaderCell scope="row" className="whitespace-nowrap">
                  {k}
                </Table.HeaderCell>
                <Table.DataCell>{v}</Table.DataCell>
              </Table.Row>
            ))}
          </Table.Body>
        </FleetTable>
      </ExpansionCard.Content>
    </ExpansionCard>
  );
}

/** Name line under the hero: status, route and the facts a reader looks for first. */
export const ServerStatusTag = ({ server }: { server: MCPServer }) => <StatusTag s={serverStatus(server)} />;

export const AgentStatusTag = ({ profile }: { profile: AgentProfile }) => <StatusTag s={agentStatus(profile)} />;

type FleetError = { message: string; detail?: string; fields?: Record<string, string> };

/** Posts a form to /api/fleet and opens the page it answers with (stay: reloads this one); the error stays on the form. */
export function useFleetPost(action: string, stay = false) {
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
      if (res.ok && stay) {
        router.refresh();
        return;
      }
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
export const ActionError = ({ error }: { error?: FleetError }) =>
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
export function ConfirmForm({
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
