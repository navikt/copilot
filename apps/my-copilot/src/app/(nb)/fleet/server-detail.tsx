"use client";

import { type ReactNode } from "react";
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
  Textarea,
  VStack,
} from "@navikt/ds-react";
import { ExternalLinkIcon } from "@navikt/aksel-icons";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { hasCondition } from "@/lib/fleet-names";
import type { AgentProfile, AuditEvent, MCPServer, MCPServerSpec } from "@/lib/fleet-control";
import { APPROVAL_RULE, APPROVER, CHANGE_RULE, ownVersionText, waitingText } from "@/lib/fleet-approval";
import {
  ActionError,
  ConfirmForm,
  Detaljer,
  FleetTable,
  Historikk,
  SubmittedAlert,
  linkClass,
  useFleetPost,
} from "./fleet-ui";
import { DATA_CATEGORIES, categoryLabel, serverNextStep } from "./fleet-status";
import { Initials, WaitingForAccess } from "./inbox";
import { AccessPolicy, ServerForm } from "./server-form";

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
  discoveryEnabled = false,
  submitted = false,
}: {
  /** fleet-gateway's probe is configured: the edit form offers «Hent verktøy». */
  discoveryEnabled?: boolean;
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
      {submitted && server.status.pending && <SubmittedAlert server={server} />}
      <BodyLong>Neste steg: {serverNextStep(server)}</BodyLong>
      <ServerActions server={server} me={me} myName={myName} owner={owner} approver={approver} part="approve" />
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
      <ServerActions server={server} me={me} owner={owner} part="edit" discoveryEnabled={discoveryEnabled} />
      <ServerDocs spec={server.status.pending?.spec ?? server.spec} />
      {!(server.status.route && hasCondition(server, "Published") && !hasCondition(server, "Revoked")) && (
        <AccessPolicy app={(server.status.pending?.spec ?? server.spec)?.upstream?.app} />
      )}
    </VStack>
  );
}

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
  owner,
  approver = false,
  part,
  discoveryEnabled,
}: {
  server: MCPServer;
  me?: string;
  myName?: string;
  owner: boolean;
  approver?: boolean;
  part: "approve" | "rest" | "edit";
  discoveryEnabled?: boolean;
}) {
  const approve = useFleetPost("approve");
  const revoke = useFleetPost("revoke");
  const live = hasCondition(server, "Published") && !hasCondition(server, "Revoked");
  const { namespace, name } = server.metadata;
  const pending = server.status.pending;
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
                Eierteamet kan rulle tilbake til en tidligere publisert versjon. Kommer i Fleet.
                {owner &&
                  " Til da: Er feil i oppsettet, endre MCP-serveren og send inn en ny versjon. Er feilen i koden til appen, ruller dere tilbake appen i Nais. Haster det for alle, kan plattformteamet bruke nødstopp i fleet-gateway."}
              </BodyShort>
            </VStack>
            <ConfirmForm
              onSubmit={revoke.onSubmit}
              heading={`Trekk tilbake ${name}?`}
              question="Ingen kan bruke MCP-serveren etterpå, og agentene mister tilgangen. For å publisere den igjen må dere sende inn en ny versjon som team copilot godkjenner."
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
            <ExpansionCard.Description>Endringen blir en ny versjon. {CHANGE_RULE}</ExpansionCard.Description>
          </ExpansionCard.Header>
          <ExpansionCard.Content>
            <ServerForm server={server} discoveryEnabled={discoveryEnabled} />
          </ExpansionCard.Content>
        </ExpansionCard>
      )}
    </VStack>
  );
}
