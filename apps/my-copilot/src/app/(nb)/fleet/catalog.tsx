"use client";

import NextLink from "next/link";
import {
  Alert,
  BodyLong,
  BodyShort,
  Button,
  Heading,
  HStack,
  Link,
  List,
  Select,
  Table,
  Tag,
  VStack,
} from "@navikt/ds-react";
import type { CatalogEntry } from "@/lib/fleet-control";
import { refKey } from "@/lib/fleet-names";
import { KortFortalt } from "./fleet-ui";

const linkClass = "text-blue-600 hover:underline";
const NAIS_CONSOLE = "https://console.nav.cloud.nais.io/";

// Datakategori is the risk level. An unknown value is shown as it is.
const CATEGORY: Record<string, { label: string; variant: "success" | "info" | "warning" }> = {
  Aapen: { label: "Åpen", variant: "success" },
  Intern: { label: "Intern", variant: "info" },
  Fortrolig: { label: "Fortrolig", variant: "warning" },
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

/** Published MCP servers from every team. The catalogue only holds published servers. */
export function Catalog({ entries }: { entries: CatalogEntry[] }) {
  return (
    <section aria-labelledby="katalog">
      <VStack gap="space-8">
        <Heading id="katalog" level="2" size="medium">
          Katalog
        </Heading>
        <BodyShort>Publiserte MCP-servere fra alle team. Åpne en for å se hva den gjør og hvem som eier den.</BodyShort>
        {entries.length === 0 ? (
          <BodyShort>Ingen MCP-servere er publisert ennå.</BodyShort>
        ) : (
          <Table size="small" aria-label="Publiserte MCP-servere">
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell scope="col">MCP-server</Table.HeaderCell>
                <Table.HeaderCell scope="col">Eierteam</Table.HeaderCell>
                <Table.HeaderCell scope="col">Datakategori</Table.HeaderCell>
                <Table.HeaderCell scope="col">Status</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {entries.map((e) => (
                <Table.Row key={refKey(e.metadata)}>
                  <Table.DataCell>
                    <NextLink className={linkClass} href={`/fleet/mcp-servere/${refKey(e.metadata)}`}>
                      {e.metadata.name}
                    </NextLink>
                  </Table.DataCell>
                  <Table.DataCell>{e.metadata.namespace}</Table.DataCell>
                  <Table.DataCell>
                    <CategoryTag category={e.spec.dataCategory} />
                  </Table.DataCell>
                  <Table.DataCell>
                    <Tag size="small" variant="success">
                      Publisert
                    </Tag>
                  </Table.DataCell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table>
        )}
      </VStack>
    </section>
  );
}

/** A plain GET form: no client code, and the choice lives in the address. */
export function TeamPicker({ teams, current }: { teams: string[]; current?: string }) {
  if (teams.length < 2) return null;
  return (
    <form method="get" action="/fleet">
      <HStack gap="space-8" align="end">
        <Select label="Team" name="team" defaultValue={current ?? ""} size="small">
          <option value="">Alle teamene mine</option>
          {teams.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </Select>
        <Button size="small" variant="secondary">
          Vis
        </Button>
      </HStack>
    </form>
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
          Du er ikke med i et team i Nais ennå. Du kan se katalogen, men bare et team kan registrere MCP-servere og få
          tilgang til dem.
        </BodyLong>
        <Heading level="3" size="xsmall">
          Gjør dette
        </Heading>
        <List as="ol" size="small">
          <List.Item>
            Bli med i teamet ditt i <Link href={NAIS_CONSOLE}>Nais Console</Link>.
          </List.Item>
          <List.Item>Kom tilbake hit og last inn siden på nytt.</List.Item>
        </List>
      </VStack>
    </Alert>
  );
}

/** What another team sees of an MCP server: the catalogue entry, no drafts, history or actions. */
export function ServerReadOnly({ entry }: { entry: CatalogEntry }) {
  const tools = entry.spec.allowedTools ?? [];
  return (
    <VStack gap="space-24">
      <KortFortalt>
        <BodyLong>
          Denne MCP-serveren eies av <strong>{entry.metadata.namespace}</strong>. Eierteamet bestemmer hvem som får
          tilgang. Utkast, historikk og handlinger ser bare eierteamet.
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
            <dd>{entry.metadata.namespace}</dd>
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
              </List.Item>
            ))}
          </List>
        )}
      </VStack>
      <BodyShort>
        <NextLink className={linkClass} href="/fleet">
          Tilbake til Fleet
        </NextLink>
      </BodyShort>
    </VStack>
  );
}
