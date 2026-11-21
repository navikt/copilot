import { BodyLong, BodyShort, Box, Heading, HStack, ReadMore, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CommandBlock } from "@/components/detail-drawer/shared";
import { FleetGatewayAccessDiagram, FleetGatewayCallDiagram } from "@/components/fleet-gateway-diagrams";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";
import { PILOT_SERVERS } from "./servers";

export const metadata: Metadata = {
  title: "fleet-gateway (pilot)",
  description:
    "For team som vil tilby en MCP-server til resten av Nav. fleet-gateway tar seg av innloggingen, så dere slipper å bygge OAuth.",
};

const PLAN_URL = "https://github.com/navikt/copilot/issues/2351";
const HELP_URL = "https://nav-it.slack.com/archives/C0ASJSZK8P8";
const SPEC_URL = "https://modelcontextprotocol.io/specification/2026-07-28";
const ECHO_TREE_URL = "https://github.com/navikt/copilot/tree/main/apps/fleet-gateway/echo";
const PROD_ROUTES_URL = "https://github.com/navikt/copilot/blob/main/apps/fleet-gateway/pilot-routes.prod.yaml";
const ALLOWLIST_URL = "https://github.com/navikt/copilot/blob/main/apps/mcp-registry/allowlist.json";

const TOC: TocItem[] = [
  { id: "hva-det-er", label: "Hva det er" },
  { id: "hvorfor", label: "Hvorfor" },
  { id: "hvem", label: "Hvem det er for" },
  { id: "hvem-slipper-inn", label: "Tilgang og kontroll" },
  { id: "status", label: "Status" },
  { id: "servere-i-piloten", label: "Servere i piloten" },
  { id: "planlagt", label: "Planlagt og kjente begrensninger" },
  { id: "sok", label: "Bli med" },
];

export default function FleetGatewayPage() {
  return (
    <DocPage
      label="Verktøy"
      title="fleet-gateway"
      description={metadata.description as string}
      badge={
        <Tag variant="warning" size="small">
          Pilot
        </Tag>
      }
      toc={TOC}
    >
      <BodyShort>
        Denne siden er for team som eier en server. Vil du bare bruke en, les{" "}
        <NextLink href="/nav-pilot/guider/nav-mcp-servere" className={linkClass}>
          Koble til Navs MCP-servere
        </NextLink>
        .
      </BodyShort>

      <VStack gap="space-12">
        <LinkableHeading id="hva-det-er" size="medium" level="2">
          Hva det er
        </LinkableHeading>
        <BodyLong>
          fleet-gateway står foran teamenes MCP-servere på Nais. Brukeren logger inn med Nav-kontoen hos gatewayen.
          Gatewayen kaller serveren deres med et OBO-token laget for akkurat deres app, så dere ser hvem brukeren er.
        </BodyLong>
        <FleetGatewayCallDiagram />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hvorfor" size="medium" level="2">
          Hvorfor
        </LinkableHeading>
        <Bullets>
          <li>Ingen OAuth å bygge. Dere sjekker bare tokenet med Texas.</li>
          <li>Én innlogging for brukerne, den samme for alle Navs MCP-servere.</li>
          <li>Hvert kall logges med NAV-ident, rute og MCP-metode.</li>
          <li>En rute kan stenges raskt hvis noe går galt.</li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hvem" size="medium" level="2">
          Hvem det er for
        </LinkableHeading>
        <BodyLong>
          Team som har, eller vil lage, en MCP-server på Nais og vil gjøre den tilgjengelig for utviklere i Nav.
          Serveren må bruke Entra ID og støtte MCP-revisjon{" "}
          <a href={SPEC_URL} className={linkClass}>
            2026-07-28
          </a>{" "}
          over streamable HTTP fra start. Se{" "}
          <NextLink href="/verktoy/fleet-gateway/for-team#krav-til-serveren" className={linkClass}>
            kravene til serveren
          </NextLink>{" "}
          og referansen{" "}
          <a href={ECHO_TREE_URL} className={linkClass}>
            fleet-echo
          </a>
          . Gatewayen håndhever ikke kravet ennå (
          <a href="https://github.com/navikt/copilot/issues/2367" className={linkClass}>
            #2367
          </a>
          ).
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hvem-slipper-inn" size="medium" level="2">
          Tilgang og kontroll
        </LinkableHeading>
        <Bullets>
          <li>
            Foreløpig bare internt: gatewayen nås via naisdevice på{" "}
            <code className={code}>fleet-gateway.intern.nav.no</code>. En ekstern gateway med offentlig ingress er{" "}
            <a href="https://github.com/navikt/copilot/issues/2297" className={linkClass}>
              planlagt (#2297)
            </a>
            . Den vil kreve en Nav-administrert enhet, og DCR blir slått av eksternt.
          </li>
          <li>Gatewayen slipper bare inn brukere som har logget inn med Nav-kontoen, og logger hvert kall.</li>
          <li>Teamet bestemmer selv hvem som får gjøre hva på serveren. Gatewayen sjekker ikke tilgang til data.</li>
          <li>
            Nettverket er åpnet begge veier i <code className={code}>accessPolicy</code>, og bare for ruter som står i
            rutefilen.
          </li>
          <li>
            Team copilot kan stenge en rute med en PR som setter <code className={code}>enabled: false</code>. Ruten er
            stengt når endringen er deployet.
          </li>
        </Bullets>
        <FleetGatewayAccessDiagram />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="status" size="medium" level="2">
          Status
        </LinkableHeading>
        <BodyLong>
          Pilot i dev-gcp og prod-gcp. Hele løpet ble testet i dev og prod 17. november 2026. Detaljene kan endres.
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="servere-i-piloten" size="medium" level="2">
          Servere i piloten
        </LinkableHeading>
        <div className="fleet-gateway-servers" role="region" aria-label="Servere i piloten" tabIndex={0}>
          {PILOT_SERVERS.map((server) => {
            const Icon = server.icon;
            const name = server.id.slice(server.id.lastIndexOf("/") + 1);
            return (
              <Box
                as="article"
                key={server.id}
                borderWidth="1"
                borderColor="neutral-subtle"
                borderRadius="12"
                padding="space-16"
              >
                <VStack gap="space-12" className="fleet-gateway-card">
                  <HStack gap="space-8" align="center" wrap={false}>
                    <Icon aria-hidden fontSize="1.75rem" />
                    <Heading size="small" level="3">
                      {server.title}
                    </Heading>
                  </HStack>
                  <BodyShort className="fleet-gateway-clamp">{server.summary}</BodyShort>
                  <HStack gap="space-4" wrap>
                    {server.readOnly && (
                      <Tag variant="neutral" size="small">
                        Bare lesing
                      </Tag>
                    )}
                  </HStack>
                  <Box
                    as="blockquote"
                    className="fleet-gateway-quote"
                    borderWidth="0 0 0 4"
                    borderColor="neutral-subtle"
                    paddingInline="space-12"
                  >
                    <BodyShort size="small">«{server.example}»</BodyShort>
                  </Box>
                  <CommandBlock command={`nav-pilot mcp enable ${name}`} />
                  <ReadMore header={`Oppsett for ${server.title}`} size="small">
                    <Bullets>
                      <li>
                        Registernavn: <code className={code}>{server.id}</code>
                      </li>
                      <li>
                        MCP-serveren i{" "}
                        <a
                          href={`https://github.com/navikt/copilot/tree/main/${server.codeLabel}`}
                          className={linkClass}
                        >
                          {server.codeLabel}
                        </a>
                      </li>
                      <li>
                        Ruten <code className={code}>{server.route}</code> i{" "}
                        <a href={PROD_ROUTES_URL} className={linkClass}>
                          apps/fleet-gateway/pilot-routes.prod.yaml
                        </a>
                      </li>
                      <li>
                        Oppføringen i MCP-registeret i{" "}
                        <a href={ALLOWLIST_URL} className={linkClass}>
                          apps/mcp-registry/allowlist.json
                        </a>
                      </li>
                      {"inboundPrUrl" in server && (
                        <li>
                          Eksempel på PR til eierteamet for å be om tilgang i{" "}
                          <code className={code}>accessPolicy.inbound</code>:{" "}
                          <a href={server.inboundPrUrl} className={linkClass}>
                            {server.inboundPrLabel}
                          </a>
                        </li>
                      )}
                    </Bullets>
                  </ReadMore>
                </VStack>
              </Box>
            );
          })}
        </div>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="planlagt" size="medium" level="2">
          Planlagt og kjente begrensninger
        </LinkableHeading>
        <Bullets>
          <li>
            Hver rute må liste verktøyene den tillater, i <code className={code}>tools</code>. Gatewayen avviser andre
            verktøykall. Ruten heter <code className={code}>/mcp/&lt;namespace&gt;/&lt;navn&gt;</code>.
          </li>
          <li>
            Gatewayen logger hvert kall med <code className={code}>NAVident</code>, rute, metode og verktøy. Personvern
            og lagringstid avklares (
            <a href="https://github.com/navikt/copilot/issues/2316" className={linkClass}>
              #2316
            </a>{" "}
            og{" "}
            <a href="https://github.com/navikt/copilot/issues/2324" className={linkClass}>
              #2324
            </a>
            ).
          </li>
          <li>
            Ruter legges inn med PR i dag. Deklarativ eller API-styrt konfigurasjon er{" "}
            <a href="https://github.com/navikt/copilot/issues/2342" className={linkClass}>
              planlagt (#2342)
            </a>
            .
          </li>
          <li>
            Synlighet per team er{" "}
            <a href="https://github.com/navikt/copilot/issues/2209" className={linkClass}>
              planlagt (#2209)
            </a>
            .
          </li>
          <li>
            Delegert tilgang for agenter er{" "}
            <a href="https://github.com/navikt/copilot/issues/2304" className={linkClass}>
              planlagt (#2304)
            </a>
            .
          </li>
          <li>
            En SSE-strøm kan vare i 10 minutter, være stille i 60 sekunder og sende 16 MiB. Tidsavbrudd på 60 sekunder
            mot serveren og tak på samtidige kall kommer (
            <a href="https://github.com/navikt/copilot/pull/2359" className={linkClass}>
              #2359
            </a>
            ).
          </li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="sok" size="medium" level="2">
          Bli med
        </LinkableHeading>
        <BodyLong>
          Følg{" "}
          <NextLink href="/verktoy/fleet-gateway/for-team" className={linkClass}>
            veiledningen for team
          </NextLink>
          . Arkitektur og tokenflyt står i{" "}
          <a href={PLAN_URL} className={linkClass}>
            planen for fleet-gateway
          </a>
          . Spørsmål tar du i{" "}
          <a href={HELP_URL} className={linkClass}>
            #ki-produktutvikling-diskusjon
          </a>
          .
        </BodyLong>
      </VStack>
    </DocPage>
  );
}
