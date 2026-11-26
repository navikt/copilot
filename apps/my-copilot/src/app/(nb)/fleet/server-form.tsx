"use client";

import NextLink from "next/link";
import { ListItem } from "@navikt/ds-react/List";
import { useRef, useState } from "react";
import {
  Alert,
  BodyLong,
  BodyShort,
  Box,
  Button,
  CopyButton,
  Detail,
  HStack,
  Radio,
  RadioGroup,
  List,
  Heading,
  Select,
  Textarea,
  TextField,
  VStack,
} from "@navikt/ds-react";
import { RISK_LABEL, type Risk } from "@/lib/fleet-wizard";
import type { MCPServer } from "@/lib/fleet-control";
import { ActionError, linkClass, useFleetPost } from "./fleet-ui";
import { DATA_CATEGORIES } from "./fleet-status";

const FLEET_GATEWAY_POLICY = `spec:
  accessPolicy:
    inbound:
      rules:
        - application: fleet-gateway
          namespace: copilot`;

/** Item 6 in #2920: what the app's nais.yaml needs before fleet-gateway can reach it. */
export function AccessPolicy({ app }: { app?: string }) {
  return (
    <VStack gap="space-8">
      <Heading level="2" size="small">
        Slipp inn fleet-gateway
      </Heading>
      <BodyLong>
        fleet-gateway kaller {app ? <code>{app}</code> : "appen"} på den interne adressen. Det virker bare hvis appen
        slipper inn fleet-gateway i <code>accessPolicy.inbound</code>. Fleet kan ikke sjekke dette ennå, så sjekk selv
        at <code>nais.yaml</code> har dette:
      </BodyLong>
      <Box background="neutral-soft" padding="space-12" borderRadius="8">
        <VStack gap="space-8">
          <HStack justify="space-between" align="center">
            <Detail as="code">nais.yaml</Detail>
            <CopyButton copyText={FLEET_GATEWAY_POLICY} text="Kopier" activeText="Kopiert" />
          </HStack>
          <pre className="overflow-x-auto" role="region" tabIndex={0} aria-label="nais.yaml">
            <code>{FLEET_GATEWAY_POLICY}</code>
          </pre>
        </VStack>
      </Box>
      <BodyShort size="small">Mangler regelen, svarer fleet-gateway med feil når agentene kaller verktøyene.</BodyShort>
    </VStack>
  );
}

/** The path of an address, or "" (the route then uses /mcp). Never the host. */
export const pathOf = (url?: string) => (url && URL.canParse(url) ? new URL(url).pathname : "");

export type FoundTool = { name: string; description?: string; suggestedRisk: Risk; destructive: boolean };

type Discovery =
  { state: "idle" | "busy" } | { state: "ok"; count: number } | { state: "error"; kind: string; message: string };

const RETRY = "Fleet kunne ikke hente verktøyene akkurat nå. Prøv igjen om litt.";

/**
 * «Hent verktøy» through fleet-gateway's probe, for the wizard and the edit
 * form. The answer only pre-fills; the team decides. «Skriv inn verktøyene
 * selv» is always there.
 */
export function ToolDiscovery({
  namespace,
  app,
  path,
  onTools,
  onManual,
}: {
  namespace: string;
  app: string;
  path: string;
  onTools: (tools: FoundTool[]) => void;
  onManual: () => void;
}) {
  const [d, setD] = useState<Discovery>({ state: "idle" });
  async function discover() {
    setD({ state: "busy" });
    try {
      const res = await fetch("/api/fleet/tools", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ namespace, app, path }),
      });
      const out = (await res.json().catch(() => ({}))) as { tools?: FoundTool[]; error?: string; kind?: string };
      if (!res.ok || !out.tools) {
        setD({ state: "error", kind: out.kind ?? "unavailable", message: out.error ?? RETRY });
        return;
      }
      onTools(out.tools);
      setD({ state: "ok", count: out.tools.length });
    } catch {
      setD({ state: "error", kind: "unavailable", message: RETRY });
    }
  }
  return (
    <VStack gap="space-12" align="start">
      <HStack gap="space-8">
        <Button variant="secondary" size="small" loading={d.state === "busy"} onClick={discover} type="button">
          Hent verktøy
        </Button>
        <Button variant="tertiary" size="small" onClick={onManual} type="button">
          Skriv inn verktøyene selv
        </Button>
      </HStack>
      <BodyShort size="small">
        fleet-gateway kaller <code>tools/list</code> på appen som deg. Svaret fyller bare ut listen. Du velger selv.
      </BodyShort>
      {d.state === "ok" && (
        <Alert variant="success" size="small" role="status">
          Fant {d.count} verktøy. Verktøy som kan endre data, er ikke krysset av.
        </Alert>
      )}
      {d.state === "error" && (
        <Alert variant={d.kind === "access" ? "error" : "warning"} size="small">
          <VStack gap="space-12">
            <BodyLong>{d.message}</BodyLong>
            {d.kind === "access" && (
              <>
                <BodyLong>Sjekk dette for {app ? <code>{app}</code> : "appen"}, deploy på nytt og prøv igjen:</BodyLong>
                <List size="small">
                  <ListItem>
                    Appen slipper inn fleet-gateway i <code>accessPolicy.inbound</code> i <code>nais.yaml</code>:
                    <Box background="neutral-soft" padding="space-12" borderRadius="8">
                      <HStack gap="space-8" justify="space-between" align="start" wrap={false}>
                        <pre className="overflow-x-auto">
                          <code>{FLEET_GATEWAY_POLICY}</code>
                        </pre>
                        <CopyButton copyText={FLEET_GATEWAY_POLICY} text="Kopier" activeText="Kopiert" />
                      </HStack>
                    </Box>
                  </ListItem>
                  <ListItem>Appnavnet er riktig, og appen kjører i teamets namespace.</ListItem>
                  <ListItem>
                    Appen har Entra ID (<code>azure.application.enabled</code>) og godtar OBO-tokener fra fleet-gateway.
                  </ListItem>
                  <ListItem>Appen svarer som en MCP-server på adressen.</ListItem>
                  <ListItem>Svaret er mindre enn 512 KiB.</ListItem>
                </List>
              </>
            )}
            {(d.kind === "access" || d.kind === "invalid") && (
              <BodyLong>
                Se{" "}
                <NextLink className={linkClass} href="/mcp/tilby-server/retningslinjer">
                  retningslinjene for MCP-servere
                </NextLink>
                , og test serveren med{" "}
                <NextLink className={linkClass} href="/mcp/referanse#mcp-compliance">
                  mcp-compliance
                </NextLink>
                .
              </BodyLong>
            )}
          </VStack>
        </Alert>
      )}
    </VStack>
  );
}

/** Register a new MCP server, or with server: edit it, which submits a new version for approval. */
export function ServerForm({
  teams = [],
  server,
  discoveryEnabled = false,
}: {
  teams?: string[];
  server?: MCPServer;
  discoveryEnabled?: boolean;
}) {
  const post = useFleetPost(server ? "edit" : "register");
  const toolsRef = useRef<HTMLTextAreaElement>(null);
  const [found, setFound] = useState<FoundTool[]>([]);
  const spec = server?.status.pending?.spec ?? server?.spec;
  const err = post.error?.fields;
  return (
    <form onSubmit={post.onSubmit}>
      <VStack gap="space-16" align="start">
        {server ? (
          <>
            <input type="hidden" name="namespace" value={server.metadata.namespace} />
            <input type="hidden" name="name" value={server.metadata.name} />
            <input type="hidden" name="resourceVersion" value={server.metadata.resourceVersion ?? ""} />
          </>
        ) : (
          <>
            <Select label="Team" name="namespace" required>
              {teams.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </Select>
            <TextField
              label="Navn"
              name="name"
              error={err?.name}
              description="Små bokstaver, tall og bindestrek."
              required
            />
          </>
        )}
        <TextField
          label="App"
          name="app"
          defaultValue={spec?.upstream?.app}
          error={err?.app}
          description="Nais-appen som kjører MCP-serveren."
          required
        />
        <TextField
          label="Intern adresse"
          name="url"
          defaultValue={spec?.upstream?.url}
          error={err?.url}
          description="For eksempel http://min-app.mitt-team/mcp eller http://min-app.mitt-team.svc.cluster.local/mcp"
          required
        />
        {server && discoveryEnabled && (
          <ToolDiscovery
            namespace={server.metadata.namespace}
            app={spec?.upstream?.app ?? ""}
            path={pathOf(spec?.upstream?.url)}
            onTools={(tools) => {
              // Destructive tools stay out of the field; the team adds them by hand if they belong.
              if (toolsRef.current)
                toolsRef.current.value = tools
                  .filter((t) => !t.destructive)
                  .map((t) => t.name)
                  .join("\n");
              setFound(tools);
            }}
            onManual={() => toolsRef.current?.focus()}
          />
        )}
        {found.length > 0 && (
          <VStack gap="space-4" as="ul" aria-label="Verktøyene MCP-serveren oppgir">
            {found.map((t) => (
              <li key={t.name}>
                <BodyShort size="small">
                  <code>{t.name}</code>: MCP-serveren oppgir {RISK_LABEL[t.suggestedRisk].toLowerCase()}
                  {t.destructive && ". Kan endre data, ikke lagt inn i feltet"}
                </BodyShort>
              </li>
            ))}
          </VStack>
        )}
        <Textarea
          label="Verktøy"
          name="tools"
          ref={toolsRef}
          defaultValue={spec?.allowedTools?.join("\n")}
          error={err?.tools}
          description="Verktøyene som skal være tilgjengelige, ett per linje."
          required
        />
        <Textarea
          label="Beskrivelse (valgfritt)"
          name="readme"
          defaultValue={spec?.readme}
          error={err?.readme}
          description="Fortell hva MCP-serveren gjør og hvordan den brukes. Du kan bruke Markdown. Høyst 64 KiB."
        />
        <RadioGroup
          legend="Hva slags data gir verktøyene ut?"
          name="dataCategory"
          defaultValue={spec?.dataCategory}
          error={err?.dataCategory}
          description="Velg den strengeste kategorien som passer. Strengt fortrolig krever egen risikovurdering og godkjenning først."
        >
          {DATA_CATEGORIES.map(([value, label, help]) => (
            <Radio key={value} value={value} description={help}>
              {label}
            </Radio>
          ))}
        </RadioGroup>
        {!server && <AccessPolicy />}
        <ActionError error={post.error} />
        <Button loading={post.busy}>
          {server ? "Send inn ny versjon til godkjenning" : "Send inn til godkjenning"}
        </Button>
      </VStack>
    </form>
  );
}
