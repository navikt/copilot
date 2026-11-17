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
    "Slik kobler et team en MCP-server på Nais til fleet-gateway: tilgang i accessPolicy, sjekk av tokenet og registrering av ruten.",
};

const APPLY_URL = "https://github.com/navikt/copilot/issues/new?template=fleet-gateway-pilot.yml";
const HELP_URL = "https://nav-it.slack.com/archives/C0ASJSZK8P8";
const ROUTES_URL = "https://github.com/navikt/copilot/tree/main/apps/fleet-gateway";
const ECHO_URL = "https://github.com/navikt/copilot/blob/main/apps/fleet-gateway/echo/main.go";

const TOC: TocItem[] = [
  { id: "forutsetninger", label: "Forutsetninger" },
  { id: "steg", label: "Steg for steg" },
  { id: "feil", label: "Vanlige feil" },
  { id: "ikke-gjor", label: "Dette må dere ikke gjøre" },
  { id: "drift", label: "Test, drift og avvikling" },
  { id: "sjekkliste", label: "Sjekkliste" },
];

const INBOUND_YAML = `spec:
  azure:
    application:
      enabled: true
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

const ROUTE_YAML = `- route: /mcp/<navn>
  team: <team>
  target: <klynge>:<namespace>:<app>
  upstream_url: http://<app>.<namespace>/<sti-til-mcp>`;

const OPENCODE_JSON = `{
  "mcp": {
    "<navn>": {
      "type": "remote",
      "url": "https://fleet-gateway.intern.nav.no/mcp/<navn>"
    }
  }
}`;

const CHECKLIST = [
  "MCP-serveren støtter streamable HTTP og kjører på Nais",
  "Entra ID er slått på i azure.application",
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
          <li>Serveren snakker MCP over streamable HTTP.</li>
          <li>
            Appen kjører på Nais i dev-gcp eller prod-gcp med Entra ID slått på i{" "}
            <code className={code}>azure.application</code>.
          </li>
          <li>Teamet bestemmer selv hvilke verktøy serveren har, og hvem som får bruke dem.</li>
          <li>
            Samme manifest fungerer i begge miljøer. Inbound-regelen har ingen <code className={code}>cluster</code>, så
            den gjelder fleet-gateway i samme klynge som appen.
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
              title: "Få ruten registrert",
              content: (
                <VStack gap="space-8">
                  <BodyLong>
                    Send{" "}
                    <a href={APPLY_URL} className={linkClass}>
                      pilotsøknaden
                    </a>
                    , eller lag en PR selv. Ruten står i <code className={code}>pilot-routes.dev.yaml</code> og{" "}
                    <code className={code}>pilot-routes.prod.yaml</code> i{" "}
                    <a href={ROUTES_URL} className={linkClass}>
                      apps/fleet-gateway
                    </a>
                    . Appen må også inn i <code className={code}>outbound_rules</code> i{" "}
                    <code className={code}>.nais/dev-gcp.yaml</code> eller{" "}
                    <code className={code}>.nais/prod-gcp.yaml</code>. <code className={code}>upstream_url</code> er den
                    interne adressen med full sti til MCP-endepunktet, i samme namespace som{" "}
                    <code className={code}>target</code>.
                  </BodyLong>
                  <CodeBlock filename="apps/fleet-gateway/pilot-routes.<miljø>.yaml">{ROUTE_YAML}</CodeBlock>
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
                    Adressen blir <code className={code}>https://fleet-gateway.intern.nav.no/mcp/&lt;navn&gt;</code> i
                    prod og <code className={code}>https://fleet-gateway.intern.dev.nav.no/mcp/&lt;navn&gt;</code> i
                    dev. Dere må være på naisdevice.
                  </BodyLong>
                  <CodeBlock filename="opencode.json">{OPENCODE_JSON}</CodeBlock>
                  <CodeBlock filename="Terminal">{"opencode mcp auth <navn>\nopencode"}</CodeBlock>
                  <BodyLong>
                    Start OpenCode direkte, ikke via nav-pilot, og be agenten kalle et verktøy. Første innlogging åpner
                    nettleseren. Copilot CLI kan brukes når ruten står i MCP-registeret. Bruk via nav-pilot er ikke
                    testet ennå.
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
        <LinkableHeading id="sjekkliste" size="medium" level="2">
          Sjekkliste
        </LinkableHeading>
        <FleetGatewayChecklist items={CHECKLIST} />
      </VStack>
    </DocPage>
  );
}
