import { BodyLong, BodyShort, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { FleetGatewayAccessDiagram, FleetGatewayCallDiagram } from "@/components/fleet-gateway-diagrams";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";
import { PilotServerCards, WhyGateway } from "@/components/mcp/blocks";

export const metadata: Metadata = {
  title: "fleet-gateway (pilot)",
  description:
    "For team som vil tilby en MCP-server til resten av Nav. fleet-gateway tar seg av innloggingen, så dere slipper å bygge OAuth.",
};

const PLAN_URL = "https://github.com/navikt/copilot/issues/2351";
const HELP_URL = "https://nav-it.slack.com/archives/C0ASJSZK8P8";
const SPEC_URL = "https://modelcontextprotocol.io/specification/2026-07-28";
const ECHO_TREE_URL = "https://github.com/navikt/copilot/tree/main/apps/fleet-gateway/echo";

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
      label="MCP"
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
        <NextLink href="/mcp/bruke" className={linkClass}>
          Bruke Navs MCP-servere
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
          <li>En rute kan stenges raskt hvis noe går galt.</li>
        </Bullets>
        <WhyGateway />
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
          <NextLink href="/mcp/tilby-server#krav-til-serveren" className={linkClass}>
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
            Gatewayen når apper i alle teamenes navnerom, men serveren må selv slippe den inn i{" "}
            <code className={code}>accessPolicy.inbound</code>. Gatewayen sender bare kall til ruter som er godkjent i
            /fleet.
          </li>
          <li>
            Eierteamet eller team copilot kan avpublisere en rute i /fleet. Ved nødstopp kan team copilot stenge én rute
            straks med <code className={code}>GATEWAY_DISABLED_ROUTES</code>.
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
        <PilotServerCards />
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
            Teamene registrerer ruter selv i /fleet, og team copilot godkjenner dem. Fleet-gateway henter rutene fra
            fleet-control i både dev og prod (
            <a href="https://github.com/navikt/copilot/issues/2342" className={linkClass}>
              #2342
            </a>
            ).
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
            En SSE-strøm kan vare i 10 minutter, være stille i 60 sekunder og sende 16 MiB. Serveren har 60 sekunder på
            seg til å svare, og gatewayen har tak på samtidige kall (
            <a href="https://github.com/navikt/copilot/pull/2359" className={linkClass}>
              #2359
            </a>
            ). Brukere over sitt tak får <code className={code}>429</code>.
          </li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="sok" size="medium" level="2">
          Bli med
        </LinkableHeading>
        <BodyLong>
          Følg{" "}
          <NextLink href="/mcp/tilby-server" className={linkClass}>
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
