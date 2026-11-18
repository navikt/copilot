import { Alert, BodyLong, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { FleetGatewayAccessDiagram, FleetGatewayCallDiagram } from "@/components/fleet-gateway-diagrams";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "fleet-gateway (pilot)",
  description:
    "For team som vil tilby en MCP-server til resten av Nav. fleet-gateway tar seg av innloggingen, så dere slipper å bygge OAuth.",
};

const PLAN_URL = "https://github.com/navikt/copilot/issues/2351";
const HELP_URL = "https://nav-it.slack.com/archives/C0ASJSZK8P8";

const TOC: TocItem[] = [
  { id: "hva-det-er", label: "Hva det er" },
  { id: "hvorfor", label: "Hvorfor" },
  { id: "hvem", label: "Hvem det er for" },
  { id: "hvem-slipper-inn", label: "Tilgang og kontroll" },
  { id: "status", label: "Status" },
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
      <Alert variant="info">
        Vil du bruke en MCP-server? Les{" "}
        <NextLink href="/nav-pilot/guider/nav-mcp-servere" className={linkClass}>
          Koble til Navs MCP-servere
        </NextLink>
        . Denne siden er for team som eier en server.
      </Alert>

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
          Serveren må snakke MCP over streamable HTTP og bruke Entra ID.
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hvem-slipper-inn" size="medium" level="2">
          Tilgang og kontroll
        </LinkableHeading>
        <Bullets>
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
