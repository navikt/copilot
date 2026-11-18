import { Alert, BodyLong, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { FleetGatewayChecklist } from "@/components/fleet-gateway-checklist";
import { FleetGatewayTeamDiagram } from "@/components/fleet-gateway-diagrams";
import { FleetGatewaySteps } from "@/components/fleet-gateway-steps";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Koble teamets MCP-server til fleet-gateway",
  description:
    "Slik kobler et team en MCP-server på Nais til fleet-gateway: tilgang i accessPolicy, sjekk av tokenet og en rute i fleet-gateway.",
};

const APPLY_URL = "https://github.com/navikt/copilot/issues/new?template=fleet-gateway-pilot.yml";
const HELP_URL = "https://nav-it.slack.com/archives/C0ASJSZK8P8";
const ROUTES_URL = "https://github.com/navikt/copilot/tree/main/apps/fleet-gateway";
const ECHO_URL = "https://github.com/navikt/copilot/blob/main/apps/fleet-gateway/echo/main.go";
const ECHO_TREE_URL = "https://github.com/navikt/copilot/tree/main/apps/fleet-gateway/echo";
const SPEC_URL = "https://modelcontextprotocol.io/specification/2026-07-28";

const TOC: TocItem[] = [
  { id: "forutsetninger", label: "Forutsetninger" },
  { id: "krav-til-serveren", label: "Krav til serveren" },
  { id: "steg", label: "Steg for steg" },
  { id: "feil", label: "Vanlige feil" },
  { id: "ikke-gjor", label: "Dette må dere ikke gjøre" },
  { id: "drift", label: "Test, drift og avvikling" },
  { id: "planlagt", label: "Planlagt og kjente begrensninger" },
  { id: "sjekkliste", label: "Sjekkliste" },
];

const INBOUND_YAML = `spec:
  azure:
    application:
      enabled: true
      tenant: nav.no
      allowAllUsers: true
  accessPolicy:
    inbound:
      rules:
        - application: fleet-gateway
          namespace: copilot`;

const TOKEN_CHECK = `// Svar fra Texas: POST $NAIS_TOKEN_INTROSPECTION_ENDPOINT
// med {"identity_provider": "entra_id", "token": <Bearer-tokenet>}.
// Texas har allerede sjekket at aud er appen deres.
if !claims.Active {
    return errUnauthorized // 401
}
if strings.EqualFold(claims.Idtyp, "app") || slices.Contains(claims.Roles, "access_as_application") {
    return errForbidden // apptoken uten bruker
}
if strings.TrimSpace(claims.NAVident) == "" {
    return errForbidden
}
if subtle.ConstantTimeCompare([]byte(claims.Azp), []byte(gatewayClientID)) != 1 {
    return errForbidden
}`;

const AZP_LOOKUP = `// fleet-gateway i samme klynge som appen: dev-gcp eller prod-gcp
gatewayName := os.Getenv("NAIS_CLUSTER_NAME") + ":copilot:fleet-gateway"

var apps []struct {
    Name     string \`json:"name"\`
    ClientID string \`json:"clientId"\`
}
if err := json.Unmarshal([]byte(os.Getenv("AZURE_APP_PRE_AUTHORIZED_APPS")), &apps); err != nil {
    return err
}
var gatewayClientID string
for _, a := range apps {
    if a.Name == gatewayName {
        gatewayClientID = a.ClientID
    }
}
if gatewayClientID == "" {
    return errors.New("fleet-gateway mangler i AZURE_APP_PRE_AUTHORIZED_APPS")
}`;

const OUTBOUND_YAML = `outbound_rules:
  - application: <app>
    namespace: <namespace>`;

const ROUTE_YAML = `- route: /mcp/<namespace>/<navn>
  team: <team>
  target: <klynge>:<namespace>:<app>
  upstream_url: http://<app>.<namespace>/<sti-til-mcp>
  tools: [<verktøy>, <verktøy>]`;

const OPENCODE_JSON = `{
  "mcp": {
    "<navn>": {
      "type": "remote",
      "url": "https://fleet-gateway.intern.nav.no/mcp/<namespace>/<navn>",
      "protocol": "2026-07-28"
    }
  }
}`;

const CHECKLIST = [
  "MCP-serveren støtter protokoll 2026-07-28 over streamable HTTP og kjører på Nais",
  "Serveren svarer på server/discover og bruker ikke sesjoner",
  "Serveren sjekker Mcp-Method og Mcp-Name (-32020 ved avvik) og svarer -32022 på protokollversjoner den ikke støtter",
  "azure.application har enabled, tenant: nav.no og allowAllUsers: true",
  "fleet-gateway står i accessPolicy.inbound",
  "Appen sjekker aud, idtyp, NAVident og azp med Texas",
  "Appen logger aldri tokenet",
  "Ruten er testet i dev før prod",
];

export default function FleetGatewayForTeamPage() {
  return (
    <DocPage
      label="Verktøy"
      title="Koble teamets MCP-server til fleet-gateway"
      description={metadata.description as string}
      badge={
        <Tag variant="warning" size="small">
          Pilot
        </Tag>
      }
      toc={TOC}
    >
      <Alert variant="warning">
        Pilot i dev-gcp og prod-gcp. Stegene kan endres. Les{" "}
        <NextLink href="/verktoy/fleet-gateway" className={linkClass}>
          oversikten over fleet-gateway
        </NextLink>{" "}
        først.
      </Alert>

      <FleetGatewayTeamDiagram />

      <VStack gap="space-12">
        <LinkableHeading id="forutsetninger" size="medium" level="2">
          Forutsetninger
        </LinkableHeading>
        <Bullets>
          <li>
            Serveren snakker MCP-protokoll{" "}
            <a href={SPEC_URL} className={linkClass}>
              2026-07-28
            </a>{" "}
            over streamable HTTP fra start.
          </li>
          <li>
            Appen kjører på Nais i dev-gcp eller prod-gcp med Entra ID slått på i{" "}
            <code className={code}>azure.application</code>, med <code className={code}>tenant: nav.no</code> og{" "}
            <code className={code}>allowAllUsers: true</code>.
          </li>
          <li>Teamet bestemmer selv hvilke verktøy serveren har, og hvem som får bruke dem.</li>
          <li>
            Samme manifest fungerer i begge miljøer. Inbound-regelen har ingen <code className={code}>cluster</code>, så
            den gjelder fleet-gateway i samme klynge som appen.
          </li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="krav-til-serveren" size="medium" level="2">
          Krav til serveren
        </LinkableHeading>
        <BodyLong>
          Serveren må støtte MCP-revisjon{" "}
          <a href={SPEC_URL} className={linkClass}>
            2026-07-28
          </a>{" "}
          fra start. Gatewayen håndhever ikke kravet ennå. Den gjør det når klientene er over på revisjonen (
          <a href="https://github.com/navikt/copilot/issues/2367" className={linkClass}>
            #2367
          </a>
          ).
        </BodyLong>
        <Bullets>
          <li>
            Svar på <code className={code}>server/discover</code>. Revisjonen har verken{" "}
            <code className={code}>initialize</code> eller sesjoner.
          </li>
          <li>
            Sjekk <code className={code}>Mcp-Method</code> og <code className={code}>Mcp-Name</code> mot innholdet i
            forespørselen. Svar 400 med <code className={code}>-32020</code> hvis en av dem mangler eller ikke stemmer.
          </li>
          <li>
            Svar 400 med <code className={code}>-32022</code> når klienten ber om en protokollversjon serveren ikke
            støtter.
          </li>
          <li>
            Eldre klienter, som OpenCode 1.19, trenger <code className={code}>initialize</code> i en overgangsperiode.
            fleet-echo støtter dem til en fast dato med <code className={code}>MCP_LEGACY_PROTOCOL_UNTIL</code>.
          </li>
          <li>
            Referanse:{" "}
            <a href={ECHO_TREE_URL} className={linkClass}>
              fleet-echo
            </a>{" "}
            (<code className={code}>apps/fleet-gateway/echo</code>), med samsvarstest.{" "}
            <code className={code}>mcp-onboarding</code> får samme støtte i{" "}
            <a href="https://github.com/navikt/copilot/pull/2365" className={linkClass}>
              #2365
            </a>
            .
          </li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="steg" size="medium" level="2">
          Steg for steg
        </LinkableHeading>
        <FleetGatewaySteps
          steps={[
            {
              title: "Slipp inn fleet-gateway",
              content: (
                <VStack gap="space-8">
                  <BodyLong>
                    Legg til fleet-gateway i <code className={code}>accessPolicy.inbound</code>:
                  </BodyLong>
                  <CodeBlock filename=".nais/app.yaml">{INBOUND_YAML}</CodeBlock>
                </VStack>
              ),
            },
            {
              title: "Sjekk tokenet",
              content: (
                <VStack gap="space-8">
                  <BodyLong>
                    fleet-gateway sender et OBO-token laget for appen deres. Valider det med Texas-introspeksjon, og
                    svar 401 eller 403 hvis en sjekk feiler:
                  </BodyLong>
                  <Bullets>
                    <li>
                      <code className={code}>aud</code> er appen deres. Texas sjekker dette.
                    </li>
                    <li>
                      <code className={code}>idtyp</code> er ikke <code className={code}>app</code>, og{" "}
                      <code className={code}>roles</code> inneholder ikke{" "}
                      <code className={code}>access_as_application</code>. Texas godtar apptokener, så dette sjekker
                      dere selv.
                    </li>
                    <li>
                      <code className={code}>NAVident</code> finnes, altså en innlogget bruker.
                    </li>
                    <li>
                      <code className={code}>azp</code> er klient-ID-en til fleet-gateway i samme klynge, fra{" "}
                      <code className={code}>AZURE_APP_PRE_AUTHORIZED_APPS</code>. Slå opp navnet{" "}
                      <code className={code}>&lt;klynge&gt;:copilot:fleet-gateway</code> med klyngen fra{" "}
                      <code className={code}>NAIS_CLUSTER_NAME</code>. Da virker samme kode i dev og prod.
                    </li>
                  </Bullets>
                  <CodeBlock filename="Go-utdrag: klient-ID for fleet-gateway">{AZP_LOOKUP}</CodeBlock>
                  <CodeBlock filename="Go-utdrag">{TOKEN_CHECK}</CodeBlock>
                  <BodyLong>
                    Hele eksempelet ligger i{" "}
                    <a href={ECHO_URL} className={linkClass}>
                      fleet-echo
                    </a>
                    .
                  </BodyLong>
                </VStack>
              ),
            },
            {
              title: "Be om en rute",
              content: (
                <VStack gap="space-8">
                  <BodyLong>
                    Lag en PR, eller fyll ut{" "}
                    <a href={APPLY_URL} className={linkClass}>
                      skjemaet
                    </a>{" "}
                    så lager team copilot den. Ruten står i <code className={code}>pilot-routes.dev.yaml</code> og{" "}
                    <code className={code}>pilot-routes.prod.yaml</code> i{" "}
                    <a href={ROUTES_URL} className={linkClass}>
                      apps/fleet-gateway
                    </a>
                    . Appen må også inn i <code className={code}>outbound_rules</code> i{" "}
                    <code className={code}>.nais/dev-gcp.yaml</code> eller{" "}
                    <code className={code}>.nais/prod-gcp.yaml</code>, med <code className={code}>namespace</code> når
                    appen ligger i et annet namespace enn <code className={code}>copilot</code>.{" "}
                    <code className={code}>upstream_url</code> er den interne adressen med full sti til MCP-endepunktet,
                    i samme namespace som <code className={code}>target</code>. <code className={code}>tools</code> er
                    listen over verktøy ruten slipper gjennom; andre <code className={code}>tools/call</code> avvises.
                  </BodyLong>
                  <CodeBlock filename="apps/fleet-gateway/pilot-routes.<miljø>.yaml">{ROUTE_YAML}</CodeBlock>
                  <CodeBlock filename="apps/fleet-gateway/.nais/<klynge>.yaml">{OUTBOUND_YAML}</CodeBlock>
                  <BodyLong>
                    Team copilot ser på PR-en innen to arbeidsdager. Ta dev først, og prod når dere har testet i dev.
                    Ruten virker når fleet-gateway er deployet.
                  </BodyLong>
                </VStack>
              ),
            },
            {
              title: "Test fra OpenCode",
              content: (
                <VStack gap="space-8">
                  <BodyLong>
                    Adressen blir{" "}
                    <code className={code}>https://fleet-gateway.intern.nav.no/mcp/&lt;namespace&gt;/&lt;navn&gt;</code>{" "}
                    i prod og{" "}
                    <code className={code}>
                      https://fleet-gateway.intern.dev.nav.no/mcp/&lt;namespace&gt;/&lt;navn&gt;
                    </code>{" "}
                    i dev. Dere må være på naisdevice.
                  </BodyLong>
                  <CodeBlock filename="opencode.json">{OPENCODE_JSON}</CodeBlock>
                  <CodeBlock filename="Terminal">{"opencode mcp auth <navn>\nopencode"}</CodeBlock>
                  <BodyLong>
                    Be agenten kalle et verktøy. Første innlogging åpner nettleseren. Andre klienter står i{" "}
                    <NextLink href="/nav-pilot/guider/nav-mcp-servere" className={linkClass}>
                      Koble til Navs MCP-servere
                    </NextLink>
                    .
                  </BodyLong>
                </VStack>
              ),
            },
          ]}
        />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="feil" size="medium" level="2">
          Vanlige feil
        </LinkableHeading>
        <Table size="small">
          <TableHeader>
            <TableRow>
              <TableHeaderCell scope="col">Feil</TableHeaderCell>
              <TableHeaderCell scope="col">Årsak</TableHeaderCell>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow>
              <TableDataCell>
                <code className={code}>upstream_access_missing</code> (502), <code className={code}>AADSTS65001</code>
              </TableDataCell>
              <TableDataCell>fleet-gateway mangler i accessPolicy.inbound (steg 1).</TableDataCell>
            </TableRow>
            <TableRow>
              <TableDataCell>401 fra appen deres</TableDataCell>
              <TableDataCell>
                Texas svarte <code className={code}>active: false</code>. Tokenet er utløpt eller laget for en annen app
                (feil aud).
              </TableDataCell>
            </TableRow>
            <TableRow>
              <TableDataCell>403 fra appen deres</TableDataCell>
              <TableDataCell>Apptoken, mangler NAVident eller feil azp (steg 2).</TableDataCell>
            </TableRow>
            <TableRow>
              <TableDataCell>
                <code className={code}>upstream_token_error</code> (502)
              </TableDataCell>
              <TableDataCell>fleet-gateway fikk ikke token for appen. Kontakt team copilot.</TableDataCell>
            </TableRow>
          </TableBody>
        </Table>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="ikke-gjor" size="medium" level="2">
          Dette må dere ikke gjøre
        </LinkableHeading>
        <Bullets>
          <li>Logge tokenet, helt eller delvis.</li>
          <li>
            Godta et token uten å sjekke <code className={code}>aud</code>, for eksempel ved å lese JWT-en uten Texas.
          </li>
          <li>Slippe inn andre enn fleet-gateway i inbound-regelen.</li>
          <li>Sende tokenet videre. Trenger appen et annet API, veksler dere tokenet med Texas OBO først.</li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="drift" size="medium" level="2">
          Test, drift og avvikling
        </LinkableHeading>
        <Bullets>
          <li>Test i prod på samme måte som i dev, med prod-adressen.</li>
          <li>En innlogging varer i 12 timer. Godkjenningen av klienten huskes i 30 dager.</li>
          <li>
            Grenser per kall: forespørselen kan være opptil 1 MiB og svaret opptil 10 MiB. Serveren må sende svarhodene
            innen 60 sekunder, og et vanlig svar må være ferdig innen 60 nye sekunder. Svarer dere med SSE (
            <code className={code}>text/event-stream</code>), gjelder i stedet 16 MiB, maks 10 minutter og at det ikke
            går mer enn 60 sekunder mellom hendelsene.
          </li>
          <li>
            Hver gateway-pod tar 16 vanlige MCP-kall og 64 åpne SSE-strømmer samtidig. Kall over taket får{" "}
            <code className={code}>503</code> med <code className={code}>Retry-After</code>.
          </li>
          <li>Gatewayen logger hvert kall med NAV-ident, rute og MCP-metode. Tilgang til data styrer dere i appen.</li>
          <li>
            Stenge en rute: PR som setter <code className={code}>enabled: false</code> på ruten. Ruten svarer{" "}
            <code className={code}>503</code> når endringen er deployet.
          </li>
          <li>
            Avslutte: PR som fjerner ruten og appen fra <code className={code}>outbound_rules</code>. Fjern deretter
            inbound-regelen hos dere.
          </li>
          <li>
            Spør i{" "}
            <a href={HELP_URL} className={linkClass}>
              #ki-produktutvikling-diskusjon
            </a>
            .
          </li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="planlagt" size="medium" level="2">
          Planlagt og kjente begrensninger
        </LinkableHeading>
        <BodyLong>
          Grenser, revisjonslogging og planlagte endringer står under{" "}
          <NextLink href="/verktoy/fleet-gateway#planlagt" className={linkClass}>
            Planlagt og kjente begrensninger på oversiktssiden
          </NextLink>
          .
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="sjekkliste" size="medium" level="2">
          Sjekkliste
        </LinkableHeading>
        <FleetGatewayChecklist items={CHECKLIST} />
      </VStack>
    </DocPage>
  );
}
