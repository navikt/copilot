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
        Forhåndsvisning. Piloten er bare åpen for utvalgte team, og detaljene kan endre seg. Navnene følger utkastet til
        nav-fleet og kan endres.
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
          Spiken med kjedet OBO gjennom Texas ble godkjent 16. november 2026. Piloten er ikke åpen ennå, bortsett fra
          for utvalgte team. Den starter med bare lesing.
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hvem-slipper-inn" size="medium" level="2">
          Hvem slipper inn
        </LinkableHeading>
        <Bullets>
          <li>
            Registeret (mcp-registry, del av fleet-control): en adminsgruppe godkjenner hver server før den blir synlig.
          </li>
          <li>
            Nettverket: fleet-gateway har en liste over hvilke servere den kan kalle, og teamets server slipper bare inn
            fleet-gateway.
          </li>
          <li>Piloten: hver rute har en liste over hvem som er med.</li>
          <li>Klienten: i piloten er bare cplt på naisdevice med.</li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hva-teamet-gjor" size="medium" level="2">
          Hva teamet gjør
        </LinkableHeading>
        <Bullets>
          <li>Registrer MCP-serveren i registeret.</li>
          <li>
            Legg til fleet-gateway i <code className={code}>accessPolicy.inbound</code> i serverens Nais-manifest.
            Mangler regelen, får brukeren feilen <code className={code}>AADSTS65001</code>.
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
