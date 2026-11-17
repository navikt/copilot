import { PersonGroupIcon, PersonIcon } from "@navikt/aksel-icons";
import { Alert, BodyLong, Box, HGrid, HStack, Heading, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { FleetGatewayAccessDiagram, FleetGatewayCallDiagram } from "@/components/fleet-gateway-diagrams";
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
  { id: "hvem", label: "Hvem og hvorfor" },
  { id: "hva-det-er", label: "Hva det er" },
  { id: "koble-til", label: "Slik kobler du til" },
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
        <LinkableHeading id="hvem" size="medium" level="2">
          Hvem er det for, og hvorfor?
        </LinkableHeading>
        <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
          <Box padding="space-16" borderRadius="12" borderWidth="1" borderColor="neutral-subtle">
            <VStack gap="space-8">
              <HStack gap="space-8" align="center">
                <PersonIcon aria-hidden fontSize="1.5rem" />
                <Heading size="small" level="3">
                  Utviklere
                </Heading>
              </HStack>
              <Bullets>
                <li>For deg som bruker Copilot CLI eller OpenCode. I dev fungerer bare OpenCode startet direkte.</li>
                <li>Hent data fra Navs egne MCP-servere.</li>
                <li>Logg inn med Nav-kontoen. Du trenger ingen egne nøkler per server.</li>
              </Bullets>
            </VStack>
          </Box>
          <Box padding="space-16" borderRadius="12" borderWidth="1" borderColor="neutral-subtle">
            <VStack gap="space-8">
              <HStack gap="space-8" align="center">
                <PersonGroupIcon aria-hidden fontSize="1.5rem" />
                <Heading size="small" level="3">
                  Team med en MCP-server
                </Heading>
              </HStack>
              <Bullets>
                <li>Tilby serveren til resten av Nav uten å bygge egen OAuth. Dere sjekker bare tokenet.</li>
                <li>Gatewayen sender et token laget for akkurat appen deres (OBO). Klienten ser det aldri.</li>
                <li>
                  <NextLink href="/verktoy/fleet-gateway/for-team" className={linkClass}>
                    Slik kobler dere til serveren
                  </NextLink>
                </li>
              </Bullets>
            </VStack>
          </Box>
        </HGrid>
        <BodyLong>
          Vi styrer hvem som får bruke hvilke servere, og vi kan stenge en server raskt. Derfor starter vi med en pilot
          før alle får tilgang.
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hva-det-er" size="medium" level="2">
          Hva det er
        </LinkableHeading>
        <BodyLong>
          Team skal kunne tilby MCP-servere uten å bygge egen OAuth. fleet-gateway er ett sted på Nais som tar seg av
          innloggingen med Nav-brukeren via Entra ID. Den kaller teamets MCP-server med et on-behalf-of-token (OBO) fra
          Texas, så serveren ser den faktiske brukeren. Teamene søker om å bli med via et skjema, og serveren
          registreres i fleet-control. Klienten (Copilot CLI eller OpenCode) logger inn og holder tokenene selv. Veien
          via cplt-brokeren er valgfri og er ikke tilgjengelig ennå.
        </BodyLong>
        <FleetGatewayCallDiagram />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="koble-til" size="medium" level="2">
          Slik kobler du til
        </LinkableHeading>
        <Bullets>
          <li>
            I dev fungerer bare OpenCode som du starter direkte, ikke via nav-pilot. Legg til adressen som ekstern
            MCP-server i OpenCode.
          </li>
          <li>
            Copilot CLI kan ikke bruke gatewayen ennå. Nav tillater bare MCP-servere som står i MCP-registeret på{" "}
            <code className={code}>mcp-registry.nav.no</code>, og nav-pilot følger samme regel når den starter OpenCode.
            Rutene legges i registeret når gatewayen er i prod. Dev-adresser publiseres ikke.
          </li>
          <li>
            Adressen er <code className={code}>https://fleet-gateway.intern.dev.nav.no/mcp/&lt;navn&gt;</code>. Første
            rute er <code className={code}>/mcp/onboarding</code> (mcp-onboarding).{" "}
            <code className={code}>/mcp/test</code> er en testserver.
          </li>
          <li>Du må være på naisdevice. Ellers når du ikke adressen.</li>
          <li>
            Første gang åpnes nettleseren. Du logger inn med Nav-kontoen og godkjenner klienten for ruten. Valget huskes
            i 30 dager per rute og klient. Tilbakekaller du tokenet, må du godkjenne på nytt.
          </li>
          <li>
            Klienter som registrerer seg selv, merkes som «Unverified client» på godkjenningssiden. Fortsett bare hvis
            du nettopp startet innloggingen selv.
          </li>
        </Bullets>
        <Alert variant="info" size="small">
          Bare i dev. Innloggingen er dekket av enhetstester. Hele løpet fra OpenCode er ikke kjørt ende til ende ennå.
        </Alert>
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
            Registeret (mcp-registry, del av fleet-control): team copilot godkjenner hver server før den blir synlig.
          </li>
          <li>
            Nettverket: fleet-gateway har en liste over hvilke servere den kan kalle, og teamets server slipper bare inn
            fleet-gateway.
          </li>
          <li>Pilotlisten: fleet-gateway åpner bare ruter som står på en eksplisitt liste.</li>
          <li>
            Klienten: cplt, Copilot CLI og OpenCode er forhåndsgodkjent i dev. Andre klienter kan registrere seg selv og
            merkes som «Unverified client». Alle må være på naisdevice.
          </li>
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
        <FleetGatewayAccessDiagram />
        <BodyLong>Texas utsteder tokenene. fleet-control gir tilgang, og fleet-gateway håndhever den.</BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hva-teamet-gjor" size="medium" level="2">
          Hva teamet gjør
        </LinkableHeading>
        <Bullets>
          <li>
            Følg{" "}
            <NextLink href="/verktoy/fleet-gateway/for-team" className={linkClass}>
              veiledningen for team
            </NextLink>
            .
          </li>
          <li>Søk med skjemaet under. Vi registrerer serveren til selvbetjening finnes.</li>
          <li>
            Legg til fleet-gateway i <code className={code}>accessPolicy.inbound</code> i serverens Nais-manifest.
            Mangler regelen, feiler kallet med <code className={code}>AADSTS65001</code>.
          </li>
          <li>
            Valider tokenet med Texas-introspeksjon. Sjekk at <code className={code}>aud</code> er serveren din, avvis{" "}
            <code className={code}>idtyp</code> lik <code className={code}>app</code>, sjekk at{" "}
            <code className={code}>azp</code> er fleet-gateway, og krev NAVident.
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
