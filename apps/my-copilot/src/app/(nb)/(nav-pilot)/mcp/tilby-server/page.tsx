import { BodyLong, BodyShort, Tag, VStack } from "@navikt/ds-react";
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
  { id: "autorisasjon", label: "Hvem får bruke serveren" },
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
      claims:
        groups:
          - id: <gruppe-ID for teamet>
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

const AZP_LOOKUP = `// fleet-gateway i samme cluster som appen: dev-gcp eller prod-gcp
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

const GROUP_CHECK = `// Etter at Texas har godkjent tokenet. groups har gruppe-ID-ene (GUID) fra
// appens azure.application.claims.groups som brukeren er direkte medlem av.
if !slices.Contains(claims.Groups, teamGroupID) {
    return errForbidden // 403
}`;

const OUTBOUND_YAML = `outbound_rules:
  - application: <app>
    namespace: <namespace>`;

const ROUTE_YAML = `- route: /mcp/<namespace>/<navn>
  team: <team>
  target: <cluster>:<namespace>:<app>
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
  "azure.application har enabled, tenant: nav.no og allowAllUsers: true, med teamets grupper i claims.groups",
  "fleet-gateway står i accessPolicy.inbound",
  "Appen sjekker aud, idtyp, NAVident og azp med Texas",
  "Appen sjekker groups i tokenet og avviser brukere utenfor teamets gruppe",
  "Appen logger aldri tokenet",
  "Ruten er testet i dev før prod",
];

export default function FleetGatewayForTeamPage() {
  return (
    <DocPage
      label="MCP"
      title="Koble teamets MCP-server til fleet-gateway"
      description={metadata.description as string}
      badge={
        <Tag variant="warning" size="small">
          Pilot
        </Tag>
      }
      toc={TOC}
    >
      <BodyShort>
        Les{" "}
        <NextLink href="/mcp/fleet-gateway" className={linkClass}>
          oversikten over fleet-gateway
        </NextLink>{" "}
        først. Der står også{" "}
        <NextLink href="/mcp/fleet-gateway#status" className={linkClass}>
          status for piloten
        </NextLink>
        . Stegene kan endres.
      </BodyShort>

      <FleetGatewayTeamDiagram />

      <VStack gap="space-12">
        <LinkableHeading id="forutsetninger" size="medium" level="2">
          Forutsetninger
        </LinkableHeading>
        <BodyLong>
          Skriver dere serveren selv? Les{" "}
          <NextLink href="/mcp/tilby-server/retningslinjer" className={linkClass}>
            retningslinjene for MCP-servere
          </NextLink>
          . De gir kravene til tokensjekk, grenser, logging, verktøybeskrivelser og ROS.
        </BodyLong>
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
          <li>
            Teamet bestemmer selv hvilke verktøy serveren har, og hvem som får bruke dem. Gatewayen slipper inn alle
            Nav-ansatte som logger inn, og begrenser bare hvilke verktøy ruten tillater. Serveren deres må selv sjekke
            hvem brukeren er, se{" "}
            <a href="#autorisasjon" className={linkClass}>
              Hvem får bruke serveren
            </a>
            .
          </li>
          <li>
            Samme manifest fungerer i begge miljøer. Inbound-regelen har ingen <code className={code}>cluster</code>, så
            den gjelder fleet-gateway i samme cluster som appen.
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
            .{" "}
            <a href="https://github.com/navikt/copilot/tree/main/apps/mcp-kodeverk" className={linkClass}>
              mcp-kodeverk
            </a>{" "}
            og{" "}
            <a href="https://github.com/navikt/copilot/tree/main/apps/mcp-dolly" className={linkClass}>
              mcp-dolly
            </a>{" "}
            er referanseimplementasjoner for servere i piloten, se{" "}
            <NextLink href="/mcp/fleet-gateway#servere-i-piloten" className={linkClass}>
              Servere i piloten
            </NextLink>
            .
          </li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="autorisasjon" size="medium" level="2">
          Hvem får bruke serveren
        </LinkableHeading>
        <BodyLong>
          Gatewayen autentiserer brukeren og sjekker at verktøyet står i <code className={code}>tools</code>. Den
          sjekker ikke hvem som får bruke verktøyet. Med <code className={code}>allowAllUsers: true</code> kommer alle
          Nav-ansatte som logger inn gjennom til serveren, med mindre serverens egen kode avviser dem. Teamet
          autoriserer selv.
        </BodyLong>
        <BodyLong>
          Tokenet serveren får (OBO-tokenet), har bare de gruppene som står i serverens egen{" "}
          <code className={code}>azure.application.claims.groups</code>, og som brukeren er direkte medlem av. Gatewayen
          sender ikke med gruppene den selv kjenner til, og serveren kan ikke lese andre gruppemedlemskap fra tokenet.
          Det er samme modell som med Wonderwall: tjenesten fører opp gruppene, Entra filtrerer, og koden sjekker.
        </BodyLong>
        <Bullets>
          <li>
            Før opp teamets gruppe i <code className={code}>claims.groups</code> (se manifestet i steg 1).
          </li>
          <li>
            Sjekk <code className={code}>groups</code> i tokenet, etter sjekkene i steg 2, og svar 403 når teamets
            gruppe-ID mangler.
          </li>
          <li>
            Uten egne grupper i <code className={code}>claims.groups</code> og med{" "}
            <code className={code}>allowAllUsers: true</code> har tokenet bare gruppen Nais legger til for alle brukere
            (én gruppe målt i dev 18. november 2026). Da avviser en sjekk mot teamets gruppe alle.
          </li>
        </Bullets>
        <CodeBlock filename="Go-utdrag: gruppesjekk">{GROUP_CHECK}</CodeBlock>
        <BodyLong>
          Alternativet er <code className={code}>allowAllUsers: false</code> med gruppene i{" "}
          <code className={code}>claims.groups</code>. Etter Nais-dokumentasjonen skal Entra da avvise brukere utenfor
          gruppene når gatewayen veksler tokenet (<code className={code}>AADSTS50105</code>). Dette er ikke testet
          gjennom gatewayen (
          <a href="https://github.com/navikt/copilot/issues/2321" className={linkClass}>
            #2321
          </a>
          ). Slik koden er nå, vil brukeren få feilen som <code className={code}>upstream_token_error</code>, men det er
          heller ikke testet. Bruk gruppesjekken i koden til alternativet er testet.
        </BodyLong>
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
                    fleet-gateway sender et OBO-token laget for appen deres. Valider det via Texas (introspect), og svar
                    401 eller 403 hvis en sjekk feiler:
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
                      <code className={code}>azp</code> er klient-ID-en til fleet-gateway i samme cluster, fra{" "}
                      <code className={code}>AZURE_APP_PRE_AUTHORIZED_APPS</code>. Slå opp navnet{" "}
                      <code className={code}>&lt;cluster&gt;:copilot:fleet-gateway</code> med clusteret fra{" "}
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
                    <code className={code}>upstream_url</code> er den interne adressen med full sti til MCP-endpointet,
                    i samme namespace som <code className={code}>target</code>. <code className={code}>tools</code> er
                    listen over verktøy ruten slipper gjennom; andre <code className={code}>tools/call</code> avvises.
                  </BodyLong>
                  <CodeBlock filename="apps/fleet-gateway/pilot-routes.<miljø>.yaml">{ROUTE_YAML}</CodeBlock>
                  <CodeBlock filename="apps/fleet-gateway/.nais/<cluster>.yaml">{OUTBOUND_YAML}</CodeBlock>
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
                    <NextLink href="/mcp/koble-til" className={linkClass}>
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
            Grenser per kall: forespørselen kan være opptil 1 MiB og svaret opptil 10 MiB. Serveren må sende
            svarheaderne innen 60 sekunder, og et vanlig svar må være ferdig innen 60 nye sekunder. Svarer dere med SSE
            (<code className={code}>text/event-stream</code>), gjelder i stedet 16 MiB, maks 10 minutter og at det ikke
            går mer enn 60 sekunder mellom hendelsene.
          </li>
          <li>
            Hver gateway-pod tar 16 vanlige MCP-kall og 64 åpne SSE-strømmer samtidig. Hver bruker kan ha høyst 4
            samtidige kall per pod. Kall over taket får <code className={code}>503</code> med{" "}
            <code className={code}>Retry-After</code>.
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
          <NextLink href="/mcp/fleet-gateway#planlagt" className={linkClass}>
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
