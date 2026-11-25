"use client";

import { useState } from "react";
import NextLink from "next/link";
import {
  Alert,
  BodyLong,
  BodyShort,
  Detail,
  Heading,
  HStack,
  Link,
  List,
  Select,
  Table,
  Tag,
  TextField,
  VStack,
} from "@navikt/ds-react";
import { ExternalLinkIcon } from "@navikt/aksel-icons";
import type { CatalogEntry } from "@/lib/fleet-control";
import { isName, refKey } from "@/lib/fleet-names";
import { KortFortalt } from "./fleet-ui";

const NAIS_CONSOLE = "https://console.nav.cloud.nais.io/";

// Datakategori is the risk level. An unknown value is shown as it is.
const CATEGORY: Record<string, { label: string; variant: "success" | "info" | "warning" }> = {
  Aapen: { label: "Åpne data", variant: "success" },
  Intern: { label: "Interne data", variant: "info" },
  Fortrolig: { label: "Fortrolige data", variant: "warning" },
};

export function CategoryTag({ category }: { category?: string }) {
  if (!category) return <>–</>;
  const c = CATEGORY[category] ?? { label: category, variant: "info" as const };
  return (
    <Tag size="small" variant={c.variant}>
      {c.label}
    </Tag>
  );
}

/** Published MCP servers from every team, with search and a filter by data category. */
export function Catalog({ entries }: { entries: CatalogEntry[] }) {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("");
  const q = query.trim().toLowerCase();
  const shown = entries.filter(
    (e) =>
      (!category || e.spec.dataCategory === category) &&
      (!q || `${e.metadata.namespace}/${e.metadata.name}`.toLowerCase().includes(q))
  );
  return (
    <section aria-label="Katalog">
      <VStack gap="space-16">
        {entries.length === 0 ? (
          <BodyShort>Ingen MCP-servere er publisert ennå.</BodyShort>
        ) : (
          <>
            <HStack gap="space-16" align="end">
              <TextField
                label="Søk"
                description="Navn på MCP-server eller eierteam"
                size="small"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
              <Select label="Datakategori" size="small" value={category} onChange={(e) => setCategory(e.target.value)}>
                <option value="">Alle</option>
                {Object.entries(CATEGORY).map(([value, c]) => (
                  <option key={value} value={value}>
                    {c.label}
                  </option>
                ))}
              </Select>
            </HStack>
            <BodyShort size="small" aria-live="polite">
              Viser {shown.length} av {entries.length}
            </BodyShort>
            {shown.length > 0 && (
              <Table size="small" aria-label="Publiserte MCP-servere">
                <Table.Header>
                  <Table.Row>
                    <Table.HeaderCell scope="col">MCP-server</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Eierteam</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Datakategori</Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {shown.map((e) => (
                    <Table.Row key={refKey(e.metadata)}>
                      <Table.DataCell>
                        <VStack>
                          <Link as={NextLink} href={`/fleet/mcp-servere/${refKey(e.metadata)}`}>
                            {e.metadata.name}
                          </Link>
                          {e.status.route && <Detail as="code">{e.status.route.path}</Detail>}
                        </VStack>
                      </Table.DataCell>
                      <Table.DataCell>{e.metadata.namespace}</Table.DataCell>
                      <Table.DataCell>
                        <CategoryTag category={e.spec.dataCategory} />
                      </Table.DataCell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table>
            )}
          </>
        )}
      </VStack>
    </section>
  );
}

export type TeamSummary = { team: string; servers: number; pending: number };

/** Shown when a person in several teams has not picked one yet. */
export function TeamChooser({ teams }: { teams: TeamSummary[] }) {
  return (
    <VStack gap="space-16">
      <Heading level="2" size="medium">
        Velg team
      </Heading>
      <BodyLong>
        Du er med i flere team. Fleet viser ett team om gangen, og vi husker valget ditt til neste gang.
      </BodyLong>
      <VStack gap="space-8" as="ul">
        {teams.map((t) => (
          <li key={t.team}>
            <Link as={NextLink} href={`/fleet?team=${t.team}`}>
              {t.team}
            </Link>
            <BodyShort size="small">
              {t.servers === 1 ? "1 MCP-server" : `${t.servers} MCP-servere`}
              {t.pending > 0 && ` · ${t.pending} venter på godkjenning`}
            </BodyShort>
          </li>
        ))}
      </VStack>
    </VStack>
  );
}

/** A ?team= that is not one of the person's teams. Only a valid team name is echoed. */
export function UnknownTeam({ team }: { team: string }) {
  return (
    <Alert variant="info">
      <BodyLong>
        {isName(team) ? `Du er ikke med i ${team}.` : "Du er ikke med i det teamet."}{" "}
        <Link as={NextLink} href="/fleet?team=">
          Velg et av teamene dine
        </Link>
      </BodyLong>
    </Alert>
  );
}

export function NoTeam() {
  return (
    <Alert variant="info">
      <VStack gap="space-8">
        <Heading level="2" size="small">
          Kom i gang
        </Heading>
        <BodyLong>
          Du er ikke med i et team i Nais ennå. Du kan se katalogen, men bare team kan registrere MCP-servere og få
          tilgang til dem.
        </BodyLong>
        <Heading level="3" size="xsmall">
          Gjør dette
        </Heading>
        <List as="ol" size="small">
          <List.Item>
            Bli med i teamet ditt i <Link href={NAIS_CONSOLE}>Nais Console</Link>.
          </List.Item>
          <List.Item>Kom tilbake hit. Du kommer rett til teamet ditt.</List.Item>
        </List>
      </VStack>
    </Alert>
  );
}

/** What another team sees of an MCP server: the catalogue entry, no drafts, history or actions. */
export function ServerReadOnly({
  entry,
  myTeams = [],
  grafanaUrl,
}: {
  entry: CatalogEntry;
  myTeams?: string[];
  grafanaUrl?: string;
}) {
  const owner = entry.metadata.namespace;
  const docs = new Map((entry.spec.tools ?? []).map((t) => [t.name, t.description]));
  const tools = entry.spec.allowedTools ?? [];
  const consumers = entry.status.consumers ?? [];
  const mine = consumers.filter((c) => myTeams.includes(c.namespace));
  return (
    <VStack gap="space-24">
      <KortFortalt>
        <BodyLong>
          Denne MCP-serveren eies av <strong>{owner}</strong>. Eierteamet bestemmer hvem som får tilgang. Bare
          eierteamet ser utkast, historikk og handlinger.
        </BodyLong>
      </KortFortalt>
      <dl>
        <VStack gap="space-8">
          <HStack gap="space-8">
            <dt>Status:</dt>
            <dd>
              <Tag size="small" variant="success">
                Publisert
              </Tag>
            </dd>
          </HStack>
          <HStack gap="space-8">
            <dt>Eierteam:</dt>
            <dd>{owner}</dd>
          </HStack>
          <HStack gap="space-8">
            <dt>Datakategori:</dt>
            <dd>
              <CategoryTag category={entry.spec.dataCategory} />
            </dd>
          </HStack>
          {entry.status.route && (
            <HStack gap="space-8">
              <dt>Rute:</dt>
              <dd>
                <code>{entry.status.route.path}</code>
              </dd>
            </HStack>
          )}
        </VStack>
      </dl>
      {grafanaUrl && (
        <BodyShort>
          <Link href={grafanaUrl} target="_blank" rel="noopener noreferrer">
            Se bruk i Grafana <ExternalLinkIcon fontSize="1rem" title="åpnes i ny fane" />
          </Link>
        </BodyShort>
      )}
      <section aria-labelledby="be-om-tilgang">
        <VStack gap="space-8">
          <Heading id="be-om-tilgang" level="2" size="small">
            Be eierteamet om tilgang
          </Heading>
          {myTeams.length === 0 ? (
            <BodyLong>
              Bare team kan få tilgang. Bli med i teamet ditt i <Link href={NAIS_CONSOLE}>Nais Console</Link> først.
            </BodyLong>
          ) : (
            <List as="ol" size="small">
              <List.Item>
                Legg til MCP-serveren og verktøyene dere trenger i en agentprofil i teamet ditt, og send den inn.
              </List.Item>
              <List.Item>{owner} får forespørselen under «Til godkjenning» og gir tilgang, eller sier nei.</List.Item>
              <List.Item>
                Har dere spørsmål først, finner dere kontaktinformasjon og Slack-kanalen til eierteamet i{" "}
                <Link href={`${NAIS_CONSOLE}team/${owner}`}>Nais Console</Link>.
              </List.Item>
            </List>
          )}
        </VStack>
      </section>
      <VStack gap="space-8">
        <Heading level="2" size="small">
          Verktøy
        </Heading>
        {tools.length === 0 ? (
          <BodyShort>Eierteamet har ikke oppgitt verktøy.</BodyShort>
        ) : (
          <List size="small">
            {tools.map((t) => (
              <List.Item key={t}>
                <code>{t}</code>
                {docs.get(t) && <>: {docs.get(t)}</>}
              </List.Item>
            ))}
          </List>
        )}
      </VStack>
      <VStack gap="space-8">
        <Heading level="2" size="small">
          Team med tilgang
        </Heading>
        {consumers.length === 0 ? (
          <BodyShort>Ingen agentprofiler har tilgang ennå.</BodyShort>
        ) : (
          <>
            {mine.length > 0 && (
              <BodyShort>Teamet ditt har tilgang gjennom {mine.map((c) => c.name).join(", ")}.</BodyShort>
            )}
            <Table size="small" aria-label="Agentprofiler med tilgang">
              <Table.Header>
                <Table.Row>
                  <Table.HeaderCell scope="col">Team</Table.HeaderCell>
                  <Table.HeaderCell scope="col">Agentprofil</Table.HeaderCell>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {consumers.map((c) => (
                  <Table.Row key={refKey(c)}>
                    <Table.DataCell>{c.namespace}</Table.DataCell>
                    <Table.DataCell>{c.name}</Table.DataCell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table>
          </>
        )}
      </VStack>
    </VStack>
  );
}
