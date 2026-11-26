"use client";

import NextLink from "next/link";
import { useRouter } from "next/navigation";
import { useState, type ReactNode } from "react";
import { Alert, BodyShort, Box, Button, Heading, HStack, List, Tag, TextField, VStack } from "@navikt/ds-react";
import { ListItem } from "@navikt/ds-react/List";
import type { MCPServer, MCPServerSpec, TeamApp } from "@/lib/fleet-control";
import { fromSpec, specChanges, toSpec, validateStep, type WizardState } from "@/lib/fleet-wizard";
import { APPROVER } from "@/lib/fleet-approval";
import { AddressField, AppField, DataFields, ToolsFields } from "./server-fields";

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Box background="default" borderColor="neutral-subtle" borderWidth="1" borderRadius="12" padding="space-24">
      <VStack gap="space-16">
        <Heading level="2" size="medium">
          {title}
        </Heading>
        {children}
      </VStack>
    </Box>
  );
}

/**
 * The edit page (#3118): the wizard's fields on one page, then what changes against the published version and
 * whether team copilot must approve it. published undefined: nothing is published, so everything waits.
 */
export function EditServer({
  server,
  published,
  apps,
  requireRos,
  discoveryEnabled,
}: {
  server: MCPServer;
  published?: MCPServerSpec;
  apps: TeamApp[] | null;
  requireRos: boolean;
  discoveryEnabled: boolean;
}) {
  const router = useRouter();
  const { namespace, name } = server.metadata;
  const href = `/fleet/mcp-servere/${namespace}/${name}`;
  const [s, setS] = useState<WizardState>(() => fromSpec(namespace, name, server.status.pending?.spec ?? server.spec));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submit, setSubmit] = useState<{ busy?: boolean; error?: string; detail?: string }>({});
  const set = (patch: Partial<WizardState>) => {
    setS((p) => ({ ...p, ...patch }));
    setErrors((e) => Object.fromEntries(Object.entries(e).filter(([k]) => !(k in patch))));
  };
  const fields = { s, set, errors };
  const changes = specChanges(published, toSpec(s));
  const approval = changes.some((c) => c.approval);

  async function send() {
    const e = Object.assign({}, ...[2, 3, 4, 5].map((n) => validateStep(n, s, { requireRos })));
    setErrors(e);
    if (Object.keys(e).length) return setSubmit({ error: "Rett feltene som er merket, og prøv igjen." });
    setSubmit({ busy: true });
    try {
      const res = await fetch("/api/fleet", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "edit",
          namespace,
          name,
          resourceVersion: server.metadata.resourceVersion ?? "",
          spec: toSpec(s),
        }),
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
      setErrors(out.fields ?? {});
      setSubmit({ error: out.error ?? "Noe gikk galt. Prøv igjen om litt.", detail: out.detail });
    } catch {
      setSubmit({ error: "Vi får ikke kontakt med Fleet akkurat nå. Prøv igjen om litt." });
    }
  }

  return (
    <VStack gap="space-24" className="max-w-3xl">
      <Section title="Team og navn">
        <BodyShort>
          Team og navn kan ikke endres, fordi de er en del av adressen{" "}
          <code>
            /mcp/{namespace}/{name}
          </code>
          .
        </BodyShort>
        <HStack gap="space-16">
          <TextField label="Team" value={namespace} readOnly />
          <TextField label="Navn" value={name} readOnly />
        </HStack>
      </Section>
      <Section title="App og adresse">
        <AppField {...fields} apps={apps} />
        <AddressField
          {...fields}
          discoveryEnabled={discoveryEnabled}
          onManual={() => document.getElementById("verktoy")?.scrollIntoView()}
        />
      </Section>
      <div id="verktoy">
        <Section title="Verktøy og risiko">
          <ToolsFields {...fields} requireRos={requireRos} />
        </Section>
      </div>
      <Section title="Data og beskrivelse">
        <DataFields {...fields} />
      </Section>
      <Section title="Hva endres">
        {changes.length === 0 ? (
          <BodyShort>Ingen endringer mot den publiserte versjonen ennå.</BodyShort>
        ) : (
          <>
            <List size="small" aria-label="Endringer mot den publiserte versjonen">
              {changes.map((c) => (
                <ListItem key={c.text}>
                  <HStack gap="space-8" align="center">
                    <span>{c.text}</span>
                    <Tag size="xsmall" variant={c.approval ? "warning" : "neutral"}>
                      {c.approval ? `Må godkjennes av ${APPROVER}` : "Publiseres med en gang"}
                    </Tag>
                  </HStack>
                </ListItem>
              ))}
            </List>
            <Alert variant="info" size="small">
              {approval
                ? `Endringen må godkjennes av ${APPROVER}. Til da gjelder versjonen som er publisert nå.`
                : "Endringen gir ikke mer tilgang og publiseres med en gang."}
            </Alert>
          </>
        )}
        {submit.error && (
          <Alert variant="error" size="small">
            <VStack gap="space-8">
              <BodyShort>{submit.error}</BodyShort>
              {submit.detail && (
                <BodyShort size="small" lang="en">
                  {submit.detail}
                </BodyShort>
              )}
            </VStack>
          </Alert>
        )}
        <HStack gap="space-16" justify="space-between">
          <Button type="button" onClick={send} loading={submit.busy} disabled={changes.length === 0}>
            {approval ? "Send til godkjenning" : "Publiser endringen"}
          </Button>
          <Button as={NextLink} href={href} variant="tertiary">
            Avbryt
          </Button>
        </HStack>
      </Section>
    </VStack>
  );
}
