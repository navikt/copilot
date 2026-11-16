import { Alert, BodyLong, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "fleet-gateway (pilot)",
  description:
    "fleet-gateway er én gateway på Nais med Nav-innlogging foran teamenes MCP-servere. Slik blir teamets MCP-server med i piloten.",
};

const PLAN_URL = "https://github.com/navikt/copilot/blob/main/docs/mcp-gateway-plan.md";
const APPLY_URL = "https://github.com/navikt/copilot/issues/new?template=fleet-gateway-pilot.yml";

const TOC: TocItem[] = [
  { id: "hva-det-er", label: "Hva det er" },
  { id: "status", label: "Status" },
  { id: "hvem-slipper-inn", label: "Hvem slipper inn" },
  { id: "hva-teamet-gjor", label: "Hva teamet gjør" },
  { id: "sok", label: "Søk om å bli med" },
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
      <Alert variant="warning">
        Forhåndsvisning. Piloten har ikke startet, og detaljene kan endres. Navnene følger utkastet til nav-fleet.
      </Alert>

      <VStack gap="space-12">
        <LinkableHeading id="hva-det-er" size="medium" level="2">
          Hva det er
        </LinkableHeading>
        <BodyLong>
          Team skal kunne tilby MCP-servere uten å bygge egen OAuth. fleet-gateway er ett sted på Nais som tar seg av
          innloggingen med Nav-brukeren. Den kaller teamets MCP-server med et on-behalf-of-token (OBO) fra Texas, så
          serveren ser den faktiske brukeren. Teamene registrerer serverne sine på ki-utvikling, og cplt holder tokenene
          på maskinen, utenfor agentens sandkasse.
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="status" size="medium" level="2">
          Status
        </LinkableHeading>
        <BodyLong>
          Testen av kjedet OBO gjennom Texas ble bestått 16. november 2026. Piloten har ikke startet. Den starter med
          bare lesing.
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hvem-slipper-inn" size="medium" level="2">
          Hvem slipper inn
        </LinkableHeading>
        <Bullets>
          <li>
            Registeret (mcp-registry, del av fleet-control): en admingruppe godkjenner hver server før den blir synlig.
          </li>
          <li>
            Nettverket: fleet-gateway har en liste over hvilke servere den kan kalle, og teamets server slipper bare inn
            fleet-gateway.
          </li>
          <li>Pilotlisten: fleet-gateway åpner bare ruter som står på en eksplisitt liste.</li>
          <li>Klienten: piloten godtar bare cplt, på naisdevice.</li>
        </Bullets>
        <BodyLong>Slik blir en server med:</BodyLong>
        <VStack as="ol" gap="space-4" className="list-decimal" style={{ paddingInlineStart: "var(--ax-space-20)" }}>
          <li>Teamet sender inn serveren til fleet-control (registeret).</li>
          <li>Team copilot godkjenner serveren i fleet-control.</li>
          <li>
            Tilgangen åpnes begge veier i <code className={code}>accessPolicy</code>: utgående fra fleet-gateway,
            inngående hos teamets server.
          </li>
          <li>Serveren settes på pilotlisten.</li>
        </VStack>
        <BodyLong>Texas utsteder tokenene. fleet-control gir tilgang, og fleet-gateway håndhever den.</BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hva-teamet-gjor" size="medium" level="2">
          Hva teamet gjør
        </LinkableHeading>
        <Bullets>
          <li>Søk med skjemaet under. Vi registrerer serveren til selvbetjening finnes.</li>
          <li>
            Legg til fleet-gateway i <code className={code}>accessPolicy.inbound</code> i serverens Nais-manifest.
            Mangler regelen, feiler kallet med <code className={code}>AADSTS65001</code>.
          </li>
          <li>
            Valider tokenet med Texas-introspeksjon. Avvis <code className={code}>idtyp</code> lik{" "}
            <code className={code}>app</code>, sjekk at <code className={code}>azp</code> er fleet-gateway, og krev
            NAVident.
          </li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="sok" size="medium" level="2">
          Søk om å bli med
        </LinkableHeading>
        <BodyLong>
          Fyll ut{" "}
          <a href={APPLY_URL} className={linkClass}>
            pilotsøknaden på GitHub
          </a>
          . Detaljene om arkitektur, tokenflyt og faser står i{" "}
          <a href={PLAN_URL} className={linkClass}>
            planen for fleet-gateway
          </a>
          .
        </BodyLong>
      </VStack>
    </DocPage>
  );
}
