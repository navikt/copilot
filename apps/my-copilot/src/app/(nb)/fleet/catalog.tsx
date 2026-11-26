"use client";

import { useState, type ReactNode } from "react";
import NextLink from "next/link";
import {
  Alert,
  BodyLong,
  Box,
  BodyShort,
  Detail,
  Heading,
  HGrid,
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
import { FleetTable } from "./fleet-ui";

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
      (!q ||
        [e.metadata.namespace, e.metadata.name, ...(e.spec.allowedTools ?? [])].join(" ").toLowerCase().includes(q))
  );
  return (
    <section aria-label="Katalog">
      <VStack gap="space-16">
        {entries.length === 0 ? (
          <BodyShort>Ingen MCP-servere er publisert ennå.</BodyShort>
        ) : (
          <>
            <HGrid columns={{ xs: 1, md: "1fr 14rem" }} gap="space-16" align="end">
              <TextField
                label="Søk"
                placeholder="Navn, verktøy eller team"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
              <Select label="Datakategori" value={category} onChange={(e) => setCategory(e.target.value)}>
                <option value="">Alle</option>
                {Object.entries(CATEGORY).map(([value, c]) => (
                  <option key={value} value={value}>
                    {c.label}
                  </option>
                ))}
              </Select>
            </HGrid>
            <BodyShort size="small" aria-live="polite">
              Viser {shown.length} av {entries.length}
            </BodyShort>
            {shown.length > 0 && (
              <FleetTable aria-label="Publiserte MCP-servere">
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
              </FleetTable>
            )}
          </>
        )}
      </VStack>
    </section>
  );
}

export type TeamSummary = { team: string; servers: number; pending: number };

/** Shown when a person in several teams has not picked one yet. */
export function TeamChooser({ teams, last }: { teams: TeamSummary[]; last?: string }) {
  return (
    <VStack gap="space-24">
      <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12">
        <VStack as="ul" aria-label="Teamene dine">
          {teams.map((t, i) => (
            <Box
              as="li"
              key={t.team}
              paddingBlock="space-16"
              paddingInline="space-24"
              borderWidth={i ? "1 0 0 0" : "0"}
              borderColor="neutral-subtle"
            >
              <HStack gap="space-16" justify="space-between" align="center">
                <VStack gap="space-4">
                  <Link as={NextLink} href={`/fleet?team=${t.team}`} underline={false}>
                    <strong>{t.team}</strong>
                  </Link>
                  <BodyShort size="small">
                    {t.servers === 1 ? "1 MCP-server" : `${t.servers} MCP-servere`}
                    {t.pending > 0 && ` · ${t.pending} venter på godkjenning`}
                  </BodyShort>
                </VStack>
                {t.team === last && <BodyShort size="small">Sist brukt</BodyShort>}
              </HStack>
            </Box>
          ))}
        </VStack>
      </Box>
      <BodyShort>
        Bare ett team? Da kommer du rett til teamet. Ingen team? Da ser du{" "}
        <Link as={NextLink} href="/fleet/katalog">
          katalogen og Kom i gang
        </Link>
        .
      </BodyShort>
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

const Chip = ({ children, accent = false }: { children: ReactNode; accent?: boolean }) => (
  <Box
    background={accent ? "info-soft" : "default"}
    borderWidth="1"
    borderColor={accent ? "info" : "neutral-subtle"}
    borderRadius="8"
    paddingBlock="space-8"
    paddingInline="space-16"
  >
    {children}
  </Box>
);

export function NoTeam() {
  return (
    <Box background="default" borderWidth="1" borderColor="info" borderRadius="12" padding="space-24">
      <VStack gap="space-16">
        <VStack gap="space-8">
          <Heading level="2" size="medium">
            Kom i gang
          </Heading>
          <BodyLong>
            Du er ikke med i et team i Nais ennå. Du kan se katalogen, men bare et team kan registrere MCP-servere og få
            tilgang.
          </BodyLong>
        </VStack>
        <Heading level="3" size="small">
          Slik henger det sammen
        </Heading>
        <HStack gap="space-8" align="center" wrap>
          <Chip>
            <strong>KI-agent (konsumentteam)</strong>
          </Chip>
          <span aria-hidden>→</span>
          <Chip accent>
            <VStack>
              <strong>fleet-gateway</strong>
              <Detail as="code">/mcp/&lt;team&gt;/&lt;navn&gt;</Detail>
            </VStack>
          </Chip>
          <span aria-hidden>→</span>
          <Chip>
            <strong>MCP-server (eierteam)</strong>
          </Chip>
        </HStack>
        <HStack gap="space-8" wrap>
          <Chip>Fleet: hvilke team får kalle hva</Chip>
          <Chip>Godkjenning: to personer i eierteamet</Chip>
        </HStack>
        <Heading level="3" size="small">
          Gjør dette
        </Heading>
        <List as="ol">
          <List.Item>
            Bli med i teamet ditt i <Link href={NAIS_CONSOLE}>Nais Console</Link>.
          </List.Item>
          <List.Item>Kom tilbake hit. Du blir sendt rett til teamet ditt.</List.Item>
          <List.Item>Registrer en MCP-server, eller be et eierteam om tilgang.</List.Item>
        </List>
      </VStack>
    </Box>
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
      <VStack gap="space-8">
        <HStack gap="space-8" align="center">
          <Tag size="small" variant="success">
            Publisert
          </Tag>
          {entry.status.route && (
            <Detail as="code" className="[overflow-wrap:anywhere]">
              {entry.status.route.path}
            </Detail>
          )}
        </HStack>
        <HStack gap="space-16" wrap align="center">
          <BodyShort size="small">Eierteam: {owner}</BodyShort>
          <BodyShort size="small">{tools.length === 1 ? "1 verktøy" : `${tools.length} verktøy`}</BodyShort>
          <CategoryTag category={entry.spec.dataCategory} />
        </HStack>
      </VStack>
      <Alert variant="info">
        {mine.length > 0
          ? `Teamet ditt har tilgang gjennom ${mine.map((c) => c.name).join(", ")}. `
          : myTeams.length > 0
            ? `Teamet ditt (${myTeams.join(", ")}) er ikke konsument av denne MCP-serveren. `
            : ""}
        Eierteamet bestemmer hvem som får tilgang. Utkast, historikk og handlinger ser bare eierteamet.
      </Alert>
      <VStack gap="space-8">
        <Heading level="2" size="medium">
          Verktøy
        </Heading>
        {tools.length === 0 ? (
          <BodyShort>Eierteamet har ikke oppgitt verktøy.</BodyShort>
        ) : (
          <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12">
            <VStack as="ul" aria-label="Verktøy">
              {tools.map((t, i) => (
                <Box
                  as="li"
                  key={t}
                  paddingBlock="space-12"
                  paddingInline="space-20"
                  borderWidth={i ? "1 0 0 0" : "0"}
                  borderColor="neutral-subtle"
                >
                  <code className="[overflow-wrap:anywhere]">{t}</code>
                  {docs.get(t) && <BodyShort size="small">{docs.get(t)}</BodyShort>}
                </Box>
              ))}
            </VStack>
          </Box>
        )}
      </VStack>
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
          Konsumenter
        </Heading>
        {consumers.length === 0 ? (
          <BodyShort>Ingen agentprofiler har tilgang ennå.</BodyShort>
        ) : (
          <>
            <BodyShort>
              {mine.length > 0
                ? `Teamet ditt har tilgang gjennom ${mine.map((c) => c.name).join(", ")}.`
                : "Teamet ditt har ikke tilgang."}{" "}
              {new Set(consumers.filter((c) => !myTeams.includes(c.namespace)).map((c) => c.namespace)).size} andre team
              har tilgang.
            </BodyShort>
            <FleetTable aria-label="Agentprofiler med tilgang">
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
            </FleetTable>
          </>
        )}
      </VStack>
    </VStack>
  );
}
