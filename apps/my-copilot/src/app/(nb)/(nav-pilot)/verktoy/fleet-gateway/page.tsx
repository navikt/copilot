import { PersonGroupIcon, PersonIcon } from "@navikt/aksel-icons";
import { Alert, BodyLong, Box, HGrid, HStack, Heading, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { FleetGatewayAccessDiagram, FleetGatewayCallDiagram } from "@/components/fleet-gateway-diagrams";
import { LinkableHeading } from "@/components/linkable-heading";
import { CodeBlock } from "@/components/code-block";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "fleet-gateway (pilot)",
  description:
    "fleet-gateway er én gateway på Nais med Nav-innlogging foran teamenes MCP-servere. Slik blir teamets MCP-server med i piloten.",
};

const PLAN_URL = "https://github.com/navikt/copilot/blob/main/docs/mcp-gateway-plan.md";
const HELP_URL = "https://nav-it.slack.com/archives/C0ASJSZK8P8";
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

const OPENCODE_JSON = `{
  "mcp": {
    "onboarding": {
      "type": "remote",
      "url": "https://fleet-gateway.intern.nav.no/mcp/onboarding"
    }
  }
}`;

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
        fleet-gateway er i drift i prod som pilot. Detaljene kan endres. Spørsmål tar du i{" "}
        <a href={HELP_URL} className={linkClass}>
          #ki-produktutvikling-diskusjon
        </a>
        .
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
                <li>For deg som bruker OpenCode eller Copilot CLI.</li>
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
          Gatewayen sjekker hvem du er. Teamet som eier serveren, bestemmer hva du får gjøre. Vi starter med en pilot
          før alle team kan koble seg på.
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
          registreres i fleet-control. cplt holder tokenene på maskinen, utenfor agentens sandkasse.
        </BodyLong>
        <FleetGatewayCallDiagram />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="koble-til" size="medium" level="2">
          Slik kobler du til
        </LinkableHeading>
        <Bullets>
          <li>
            Adressen er <code className={code}>https://fleet-gateway.intern.nav.no/mcp/&lt;navn&gt;</code>. Første rute
            er <code className={code}>/mcp/onboarding</code> (mcp-onboarding). Til testing finnes samme rute i dev på{" "}
            <code className={code}>fleet-gateway.intern.dev.nav.no</code>.
          </li>
          <li>Du må være på naisdevice. Ellers når du ikke adressen.</li>
        </Bullets>
        <BodyLong>
          <strong>OpenCode</strong> (testet i prod 17. november 2026). Legg serveren til i{" "}
          <code className={code}>opencode.json</code>:
        </BodyLong>
        <CodeBlock filename="opencode.json">{OPENCODE_JSON}</CodeBlock>
        <BodyLong>Logg inn, og start OpenCode direkte, ikke via nav-pilot:</BodyLong>
        <CodeBlock filename="Terminal">{"opencode mcp auth onboarding\nopencode"}</CodeBlock>
        <BodyLong>
          Be agenten kalle verktøyet <code className={code}>whoami</code>. Svaret viser NAV-identen din. Logg ut med{" "}
          <code className={code}>opencode mcp logout onboarding</code>.
        </BodyLong>
        <BodyLong>
          <strong>Copilot CLI</strong> (ikke testet ennå). Nav tillater bare MCP-servere som står i MCP-registeret. Når
          ruten står der som <code className={code}>io.github.navikt/fleet-gateway-onboarding</code>, legger du den til
          med <code className={code}>/mcp add</code> som type <code className={code}>http</code> med adressen over.
          Innloggingen starter ved første kall.
        </BodyLong>
        <Bullets>
          <li>
            Første gang åpnes nettleseren. Du logger inn med Nav-kontoen og godkjenner klienten for ruten. Valget huskes
            i 30 dager per rute og klient.
          </li>
          <li>En innlogging varer i 12 timer. Etter det logger du inn på nytt.</li>
          <li>
            Klienter som registrerer seg selv, merkes som «Unverified client» på godkjenningssiden. Fortsett bare hvis
            du nettopp startet innloggingen selv.
          </li>
          <li>
            Er en rute stengt, svarer gatewayen <code className={code}>503</code> med{" "}
            <code className={code}>temporarily_unavailable</code>.
          </li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="status" size="medium" level="2">
          Status
        </LinkableHeading>
        <BodyLong>
          fleet-gateway kjører i prod. Hele løpet fra OpenCode ble testet i dev og prod 17. november 2026. Teamene
          bestemmer selv hvilke verktøy serveren har, og hvem som får bruke dem.
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hvem-slipper-inn" size="medium" level="2">
          Hvem slipper inn
        </LinkableHeading>
        <Bullets>
          <li>
            Innloggingen: gatewayen slipper bare inn brukere som har logget inn med Nav-kontoen. Hvert kall logges med
            NAV-ident, rute og MCP-metode.
          </li>
          <li>
            Teamets server: teamet bestemmer selv hvem som får gjøre hva. Gatewayen sender med hvem brukeren er, men
            sjekker ikke tilgang til data.
          </li>
          <li>
            Nettverket: fleet-gateway kan bare kalle servere som står i listen over utgående trafikk, og teamets server
            slipper bare inn fleet-gateway.
          </li>
          <li>Rutelisten: fleet-gateway åpner bare ruter som står i rutefilen for miljøet.</li>
          <li>Klienten: alle må være på naisdevice.</li>
        </Bullets>
        <BodyLong>Slik blir en server med:</BodyLong>
        <VStack as="ol" gap="space-4" className="list-decimal" style={{ paddingInlineStart: "var(--ax-space-20)" }}>
          <li>Teamet sender pilotsøknaden eller lager en PR med ruten.</li>
          <li>Team copilot godkjenner PR-en. Ruten legges først i dev, så i prod.</li>
          <li>
            Tilgangen åpnes begge veier i <code className={code}>accessPolicy</code>: utgående fra fleet-gateway,
            inngående hos teamets server.
          </li>
          <li>Ruten blir tilgjengelig når fleet-gateway er deployet.</li>
        </VStack>
        <FleetGatewayAccessDiagram />
        <BodyLong>
          Texas utsteder tokenene. Team copilot kan stenge en rute med en PR som setter{" "}
          <code className={code}>enabled: false</code>, og ruten er stengt når endringen er deployet.
        </BodyLong>
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
          <li>Søk med skjemaet under, eller lag en PR med ruten selv.</li>
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
