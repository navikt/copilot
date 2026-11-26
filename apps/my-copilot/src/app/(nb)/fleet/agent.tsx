"use client";

import NextLink from "next/link";
import { type ReactNode } from "react";
import { Alert, BodyLong, BodyShort, Detail, Hide, Link, Heading, Show, Table, Tag, VStack } from "@navikt/ds-react";
import { refKey } from "@/lib/fleet-names";
import type { AgentProfile, AuditEvent, ServerRef } from "@/lib/fleet-control";
import { CardList, Detaljer, FleetTable, Historikk, StatusTag, type TeamData, linkClass } from "./fleet-ui";
import { agentNextStep, agentStatus, date, n } from "./fleet-status";

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
                  <Table.HeaderCell scope="col" className="whitespace-nowrap">
                    MCP-servere
                  </Table.HeaderCell>
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
