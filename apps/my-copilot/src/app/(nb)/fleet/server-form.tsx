"use client";

import NextLink from "next/link";
import { ListItem } from "@navikt/ds-react/List";
import { useState } from "react";
import {
  Alert,
  BodyLong,
  BodyShort,
  Box,
  Button,
  CopyButton,
  Detail,
  HStack,
  List,
  Heading,
  VStack,
} from "@navikt/ds-react";
import type { DiscoveredTool } from "@/lib/fleet-tools";
import { linkClass } from "./fleet-ui";

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

type Discovery =
  { state: "idle" | "busy" } | { state: "ok"; count: number } | { state: "error"; kind: string; message: string; other?: { cluster: string; href: string } };

const RETRY = "Fleet kunne ikke hente verktøyene akkurat nå. Prøv igjen om litt.";

/**
 * «Hent verktøy» through fleet-gateway's probe, for the wizard and the edit
 * page. The answer only pre-fills; the team decides. «Skriv inn verktøyene
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
  onTools: (tools: DiscoveredTool[]) => void;
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
      const out = (await res.json().catch(() => ({}))) as {
        tools?: DiscoveredTool[];
        error?: string;
        kind?: string;
        other?: { cluster: string; href: string };
      };
      if (!res.ok || !out.tools) {
        setD({ state: "error", kind: out.kind ?? "unavailable", message: out.error ?? RETRY, other: out.other });
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
          Fant {d.count} verktøy. Nye verktøy som kan endre noe, er ikke krysset av.
        </Alert>
      )}
      {d.state === "error" && (
        <Alert variant={d.kind === "access" ? "error" : "warning"} size="small">
          <VStack gap="space-12">
            <BodyLong>{d.message}</BodyLong>
            {d.kind === "notincluster" && d.other && (
              <BodyLong>
                <a className={linkClass} href={d.other.href}>
                  Gå til fleet i {d.other.cluster}
                </a>
              </BodyLong>
            )}
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
