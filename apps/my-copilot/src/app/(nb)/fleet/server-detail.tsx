"use client";

import { type ReactNode } from "react";
import NextLink from "next/link";
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
  Link,
  Heading,
  Show,
  Stack,
  Table,
  Tag,
  Textarea,
  VStack,
} from "@navikt/ds-react";
import { ExternalLinkIcon } from "@navikt/aksel-icons";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { hasCondition } from "@/lib/fleet-names";
import type { AgentProfile, AuditEvent, MCPServer, MCPServerSpec } from "@/lib/fleet-control";
import { APPROVAL_RULE, APPROVER, CHANGE_RULE, ownVersionText, waitingText } from "@/lib/fleet-approval";
import { ACTIONS, ActionError, ConfirmForm, FleetTable, SubmittedAlert, linkClass, useFleetPost } from "./fleet-ui";
import { categoryLabel, date, serverNextStep } from "./fleet-status";
import { Initials, WaitingForAccess } from "./inbox";
import { AccessPolicy } from "./server-form";

export function ServerDetail({
  server,
  events,
  me,
  myName,
  owner = false,
  approver = false,
  agents = [],
  agentsFailed = false,
  grafanaUrl,
  submitted = false,
}: {
  server: MCPServer;
  events: AuditEvent[];
  submitted?: boolean;
  me?: string;
  /** The signed-in person's name from the login token, for the initials. */
  myName?: string;
  // In the owner team: only then the edit form shows. fleet-control checks it again.
  owner?: boolean;
  // In the platform team: only then «Godkjenn» shows. fleet-control checks it again.
  approver?: boolean;
  agents?: AgentProfile[];
  agentsFailed?: boolean;
  grafanaUrl?: string;
}) {
  return (
    <VStack gap="space-24">
      {submitted && <SubmittedAlert server={server} />}
      <BodyLong>Neste steg: {serverNextStep(server)}</BodyLong>
      <ServerActions server={server} me={me} myName={myName} approver={approver} />
      {server.status.pending?.spec && (
        <VStack gap="space-8">
          <HStack justify="space-between" align="end" gap="space-8">
            <Heading level="2" size="medium" id="hva-endres">
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
          <TestCurl path={server.status.route!.path} />
          <AccessPolicy app={(server.status.pending?.spec ?? server.spec)?.upstream?.app} />
        </VStack>
      ) : null}
      <Versions server={server} events={events} owner={owner} />
      {agentsFailed ? (
        <Alert variant="warning" size="small">
          Vi fikk ikke hentet agentprofilene som venter på tilgang. Last inn siden på nytt.
        </Alert>
      ) : (
        <WaitingForAccess server={server} agents={agents} />
      )}
      <ServerDocs spec={server.status.pending?.spec ?? server.spec} />
      {!(server.status.route && hasCondition(server, "Published") && !hasCondition(server, "Revoked")) && (
        <AccessPolicy app={(server.status.pending?.spec ?? server.spec)?.upstream?.app} />
      )}
      <Unpublish server={server} owner={owner} />
    </VStack>
  );
}

export function ServerHeader({ server }: { server: MCPServer }) {
  const spec = server.status.pending?.spec ?? server.spec;
  const tools = spec?.allowedTools?.length ?? 0;
  const path = server.status.route?.path;
  return (
    <VStack gap="space-8">
      {path && (
        <HStack gap="space-4" align="center" wrap={false}>
          <Detail as="code" className="min-w-0 [overflow-wrap:anywhere]">
            {path}
          </Detail>
          <CopyButton size="xsmall" copyText={path} title="Kopier adressen i fleet-gateway" />
        </HStack>
      )}
      <HStack gap="space-16" wrap>
        <BodyShort size="small">Eierteam: {server.metadata.namespace}</BodyShort>
        <BodyShort size="small">{tools === 1 ? "1 verktøy" : `${tools} verktøy`}</BodyShort>
        {spec?.dataCategory && <BodyShort size="small">{categoryLabel(spec.dataCategory)}</BodyShort>}
      </HStack>
    </VStack>
  );
}

/** tools/list through fleet-gateway, for the owner team to try the published route. */
function TestCurl({ path }: { path: string }) {
  const curl = `curl -X POST https://<fleet-gateway>${path} \\
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
  ["Datakategori", s?.dataCategory ? categoryLabel(s.dataCategory) : "–"],
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
    ? "Avpublisert"
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
                <Table.HeaderCell scope="row" className="whitespace-nowrap">
                  {k}
                </Table.HeaderCell>
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

function ServerActions({
  server,
  me,
  myName,
  approver = false,
}: {
  server: MCPServer;
  me?: string;
  myName?: string;
  approver?: boolean;
}) {
  const approve = useFleetPost("approve");
  const { namespace, name } = server.metadata;
  const pending = server.status.pending;
  const ownVersion = !!me && pending?.submittedBy?.name === me;
  const ids = (
    <>
      <input type="hidden" name="namespace" value={namespace} />
      <input type="hidden" name="name" value={name} />
    </>
  );
  return pending?.contentHash ? (
    <Box background="default" borderWidth="1" borderColor="warning" borderRadius="12" padding="space-20">
      <VStack gap="space-12" align={{ xs: "stretch", md: "start" }}>
        <Heading level="2" size="medium">
          Versjon {pending.generation} venter
        </Heading>
        <BodyShort size="small">{APPROVAL_RULE}</BodyShort>
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
            <BodyShort weight="semibold">{APPROVER}</BodyShort>
          </VStack>
        </Stack>
        {/* The UI only hides the button; fleet-control still decides, and a 403 is shown via route.ts. */}
        {!approver ? (
          <BodyShort>{ownVersion ? ownVersionText(pending.generation) : waitingText(pending.generation)}</BodyShort>
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
}

const PUBLISHED = ["approved_published", "auto_published"];

/** One row per published version, newest first, from the history. */
export function versionRows(server: MCPServer, events: AuditEvent[]) {
  const live = hasCondition(server, "Published") && !hasCondition(server, "Revoked");
  const current = live ? server.status.publishedGeneration : undefined;
  const revoked = new Set(events.filter((e) => e.action === "revoked").map((e) => e.generation));
  const seen = new Set<number>();
  return events.flatMap((e) => {
    const gen = e.generation;
    if (gen === undefined || !PUBLISHED.includes(e.action) || seen.has(gen)) return [];
    seen.add(gen);
    return [
      {
        event: e,
        generation: gen,
        current: gen === current,
        revoked: revoked.has(gen),
        // Approved by team copilot, or published as a change that added no access: fleet-control counts both as
        // approved (approved_via). Only older versions, never a revoked one. fleet-control checks all of it again.
        canUse: current !== undefined && gen < current && !revoked.has(gen),
      },
    ];
  });
}

function Versions({ server, events, owner }: { server: MCPServer; events: AuditEvent[]; owner: boolean }) {
  const rows = versionRows(server, events);
  const rolledBack = events.find((e) => e.action === "rolled_back");
  return (
    <VStack gap="space-8">
      <Heading level="2" size="medium">
        Versjoner
      </Heading>
      {rows.length === 0 ? (
        <BodyShort>Ingen versjon er publisert ennå.</BodyShort>
      ) : (
        <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12">
          <VStack as="ul" aria-label="Versjoner">
            {rows.map((r, i) => (
              <Box
                as="li"
                key={r.generation}
                padding="space-16"
                borderWidth={i ? "1 0 0 0" : "0"}
                borderColor="neutral-subtle"
              >
                <Stack direction={{ xs: "column", md: "row" }} justify="space-between" gap="space-8 space-16">
                  <VStack gap="space-4" className="min-w-0 [overflow-wrap:anywhere]">
                    <HStack gap="space-8" align="center">
                      <BodyShort weight="semibold">Versjon {r.generation}</BodyShort>
                      {r.current && (
                        <Tag size="xsmall" variant="success">
                          I bruk
                        </Tag>
                      )}
                      {r.revoked && (
                        <Tag size="xsmall" variant="neutral">
                          Avpublisert
                        </Tag>
                      )}
                    </HStack>
                    <BodyShort size="small">
                      {ACTIONS[r.event.action]} {date(r.event.at)} av {r.event.actor.name}
                    </BodyShort>
                    {r.current && rolledBack?.generation === r.generation && (
                      <BodyShort size="small">
                        Tatt i bruk igjen {date(rolledBack.at)} av {rolledBack.actor.name}
                        {rolledBack.detail?.reason ? `: ${rolledBack.detail.reason}` : ""}
                      </BodyShort>
                    )}
                  </VStack>
                  {owner && r.canUse && <UseVersion server={server} generation={r.generation} />}
                </Stack>
              </Box>
            ))}
          </VStack>
        </Box>
      )}
    </VStack>
  );
}

function UseVersion({ server, generation }: { server: MCPServer; generation: number }) {
  const rollback = useFleetPost("rollback");
  return (
    <ConfirmForm
      onSubmit={rollback.onSubmit}
      heading={`Bruke versjon ${generation} igjen?`}
      question="Versjonen blir publisert med en gang, uten ny godkjenning. Agentene får verktøyene og tilgangen i den versjonen."
      confirm="Bruk denne versjonen"
      variant="secondary"
      busy={rollback.busy}
      fields={(formId) => (
        <Textarea
          label="Begrunnelse"
          description="Vises i historikken."
          name="reason"
          form={formId}
          maxLength={500}
          required
        />
      )}
    >
      <input type="hidden" name="namespace" value={server.metadata.namespace} />
      <input type="hidden" name="name" value={server.metadata.name} />
      <input type="hidden" name="fromGeneration" value={server.status.publishedGeneration} />
      <input type="hidden" name="generation" value={generation} />
      <ActionError error={rollback.error} />
    </ConfirmForm>
  );
}

function Unpublish({ server, owner }: { server: MCPServer; owner: boolean }) {
  const revoke = useFleetPost("revoke");
  const { namespace, name } = server.metadata;
  if (hasCondition(server, "Revoked"))
    return owner ? (
      <VStack gap="space-8" align="start">
        <Heading level="2" size="medium">
          Publiser igjen
        </Heading>
        <BodyLong>
          MCP-serveren er avpublisert. Send inn en ny versjon for å publisere den igjen. {CHANGE_RULE}
        </BodyLong>
        <Button as={NextLink} href={`/fleet/mcp-servere/${namespace}/${name}/endre`} variant="secondary">
          Publiser igjen
        </Button>
      </VStack>
    ) : null;
  if (!hasCondition(server, "Published")) return null;
  return (
    <VStack gap="space-8">
      <Heading level="2" size="medium">
        Avpubliser MCP-serveren
      </Heading>
      <ConfirmForm
        onSubmit={revoke.onSubmit}
        heading={`Avpublisere ${name}?`}
        question="Ingen kan bruke MCP-serveren etterpå, og agentene mister tilgangen. For å publisere den igjen må dere sende inn en ny versjon som team copilot godkjenner."
        confirm="Avpubliser…"
        confirmModal="Avpubliser"
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
        <BodyShort>Adressen slutter å svare. Agentene som bruker MCP-serveren, får feil.</BodyShort>
        <BodyShort size="small">Eierteamet og team copilot kan avpublisere. Det virker med en gang.</BodyShort>
        <input type="hidden" name="namespace" value={namespace} />
        <input type="hidden" name="name" value={name} />
        <ActionError error={revoke.error} />
      </ConfirmForm>
    </VStack>
  );
}
