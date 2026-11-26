import { BodyLong, Heading, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";
import { PILOT_SERVERS } from "@/components/mcp/servers";
import { APPROVAL_RULE } from "@/lib/fleet-approval";
import { DATA_CATEGORIES, DATA_LABEL } from "@/lib/fleet-wizard";
import { EFFECT, type Effect } from "@/lib/mcp-labels";

export const metadata: Metadata = {
  title: "MCP-referanse",
  description:
    "Adresser til serverne bak fleet-gateway, hva verktøynivåene og datakategoriene betyr, og hvor du finner protokollversjoner, feilkoder og verktøy.",
};

const GATEWAY = "https://fleet-gateway.intern.nav.no";

// Effect, where it applies, nav-pilot default, /fleet, agents (#3081).
const LEVELS: [Effect, string, string, string, string][] = [
  [
    "read",
    "Lokale og eksterne",
    "På",
    "Kan velges. Ny MCP-server godkjennes av team copilot.",
    "Ja. Agentprofiler får i dag bare dette nivået (maxRisk Read).",
  ],
  ["propose", "Eksterne i fleet", "Brukes ikke av registeret", "Kan velges.", "Nei, ikke ennå."],
  ["files", "Bare lokale", "På", "Finnes ikke. Servere bak fleet-gateway kjører eksternt.", "Nei."],
  ["acts", "Lokale og eksterne", "Av. Du velger verktøyene selv.", "Kan velges. Krever ROS i prod.", "Nei, ikke ennå."],
  ["unsandboxed", "Bare lokale", "Av. Krever i tillegg --allow-host-exec.", "Finnes ikke.", "Nei."],
];

const TOC: TocItem[] = [
  { id: "adresser", label: "Adresser" },
  { id: "verktoynivaer", label: "Verktøynivåer" },
  { id: "datakategorier", label: "Datakategorier" },
  { id: "oppslag", label: "Hvor står hva" },
  { id: "mcp-compliance", label: "mcp-compliance" },
];

export default function McpReferansePage() {
  return (
    <DocPage label="MCP" title="MCP-referanse" description={metadata.description as string} toc={TOC} wide>
      <VStack gap="space-12">
        <LinkableHeading id="adresser" size="medium" level="2">
          Adresser
        </LinkableHeading>
        <BodyLong>
          Serverne i piloten, med prod-adressen klienten kobler til. Registeret har den fulle listen på{" "}
          <code className={code}>https://mcp-registry.nav.no</code>.
        </BodyLong>
        {/* Long gateway URLs: scroll the table, not the page, on a phone. */}
        <div className="overflow-x-auto">
          <Table size="small">
            <TableHeader>
              <TableRow>
                <TableHeaderCell scope="col">Server</TableHeaderCell>
                <TableHeaderCell scope="col">Navn i registeret</TableHeaderCell>
                <TableHeaderCell scope="col">Adresse</TableHeaderCell>
                <TableHeaderCell scope="col">Tilgang</TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              {PILOT_SERVERS.map((s) => (
                <TableRow key={s.id}>
                  <TableDataCell>{s.title}</TableDataCell>
                  <TableDataCell>
                    <code className={code}>{s.id}</code>
                  </TableDataCell>
                  <TableDataCell>
                    <code className={code}>{GATEWAY + s.route}</code>
                  </TableDataCell>
                  <TableDataCell>{s.readOnly ? "Leser" : "Leser og utfører"}</TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="verktoynivaer" size="medium" level="2">
          Verktøynivåer
        </LinkableHeading>
        <BodyLong>
          Hvert verktøy på en MCP-server har ett nivå som sier hva det gjør. Om serveren kjører lokalt eller eksternt,
          står som vanlig tekst ved serveren. En server som kjører eksternt, kan aldri endre filene dine, så den viser
          bare Leser, Foreslår og Utfører. {APPROVAL_RULE}
        </BodyLong>
        <div className="overflow-x-auto">
          <Table size="small">
            <TableHeader>
              <TableRow>
                <TableHeaderCell scope="col">Nivå</TableHeaderCell>
                <TableHeaderCell scope="col">Betyr</TableHeaderCell>
                <TableHeaderCell scope="col">Gjelder</TableHeaderCell>
                <TableHeaderCell scope="col">I nav-pilot</TableHeaderCell>
                <TableHeaderCell scope="col">I /fleet</TableHeaderCell>
                <TableHeaderCell scope="col">Agenter</TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              {LEVELS.map(([e, where, navPilot, fleet, agents]) => (
                <TableRow key={e}>
                  <TableDataCell>
                    <Tag size="small" variant={EFFECT[e].tag}>
                      {EFFECT[e].label}
                    </Tag>
                  </TableDataCell>
                  <TableDataCell>{EFFECT[e].text}</TableDataCell>
                  <TableDataCell>{where}</TableDataCell>
                  <TableDataCell>{navPilot}</TableDataCell>
                  <TableDataCell>{fleet}</TableDataCell>
                  <TableDataCell>{agents}</TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        <BodyLong>
          Fri tilgang er et merke, ikke et trinn på skalaen: Agenten kan da gjøre alt du kan på maskinen din.
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="datakategorier" size="medium" level="2">
          Datakategorier
        </LinkableHeading>
        <BodyLong>Eierteamet velger hva slags data verktøyene gir ut når MCP-serveren registreres i /fleet.</BodyLong>
        <Bullets>
          {DATA_CATEGORIES.map((c) => (
            <li key={c}>
              <strong>{DATA_LABEL[c][0]}</strong> (<code className={code}>{c}</code>): {DATA_LABEL[c][1]}
            </li>
          ))}
        </Bullets>
        <Bullets>
          <li>
            Strengt fortrolige data (<code className={code}>StrengtFortrolig</code>) blir avvist. De krever egen
            risikovurdering og godkjenning først.
          </li>
          <li>Data om brukere eller saker hos Nav er aldri tillatt, uansett kategori.</li>
          <li>Verktøy som utfører, krever en ROS før MCP-serveren kan brukes i prod.</li>
          <li>Alle endringer av datakategori må godkjennes av team copilot.</li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="oppslag" size="medium" level="2">
          Hvor står hva
        </LinkableHeading>
        <Bullets>
          <li>
            Protokollversjon per klient:{" "}
            <NextLink href="/mcp/bruke#protokollversjon" className={linkClass}>
              Bruke, Protokollversjon
            </NextLink>
          </li>
          <li>
            Innlogging, varighet og godkjenning:{" "}
            <NextLink href="/mcp/bruke#innlogging" className={linkClass}>
              Bruke, Innlogging
            </NextLink>
          </li>
          <li>
            Hvem som slipper inn:{" "}
            <NextLink href="/mcp/fleet-gateway#hvem-slipper-inn" className={linkClass}>
              fleet-gateway, Hvem slipper inn
            </NextLink>
          </li>
          <li>
            Krav til serveren og autorisasjon:{" "}
            <NextLink href="/mcp/tilby-server#krav-til-serveren" className={linkClass}>
              Tilby en server, Krav til serveren
            </NextLink>
          </li>
          <li>
            Feilkoder og årsaker:{" "}
            <NextLink href="/mcp/tilby-server#feil" className={linkClass}>
              Tilby en server, Vanlige feil
            </NextLink>
          </li>
          <li>
            Maler, samsvarstesten og GitHub Action-en:{" "}
            <NextLink href="/mcp/tilby-server/retningslinjer#eksempler" className={linkClass}>
              Retningslinjer, Eksempler
            </NextLink>
          </li>
        </Bullets>
      </VStack>
      <VStack gap="space-12">
        <LinkableHeading id="mcp-compliance" size="medium" level="2">
          mcp-compliance
        </LinkableHeading>
        <BodyLong>
          Samsvarstesten sjekker en MCP-server bak <code className={code}>fleet-gateway</code> mot MCP 2026-07-28 og
          autentiseringsreglene Navs MCP-servere deler. Den snakker bare HTTP, så serveren kan være skrevet i alle
          språk. Kilden er{" "}
          <a href="https://github.com/navikt/copilot/tree/main/cli/mcp-compliance" className={linkClass}>
            README for mcp-compliance
          </a>
          .
        </BodyLong>
        <Heading size="small" level="3">
          Flagg og miljøvariabler
        </Heading>
        <BodyLong>Tokenene leses fra miljøvariabler, aldri flagg, og testen skriver dem aldri ut.</BodyLong>
        <div className="overflow-x-auto">
          <Table size="small">
            <TableHeader>
              <TableRow>
                <TableHeaderCell scope="col">Flagg eller variabel</TableHeaderCell>
                <TableHeaderCell scope="col">Betydning</TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow>
                <TableDataCell className="whitespace-nowrap">
                  <code className={code}>-url string</code>
                </TableDataCell>
                <TableDataCell>
                  MCP-endpointet, for eksempel <code className={code}>http://localhost:8080/mcp</code>. Påkrevd.
                </TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell className="whitespace-nowrap">
                  <code className={code}>-tool string</code>
                </TableDataCell>
                <TableDataCell>
                  Et verktøy som kan kalles med tomme argumenter. Standard: første verktøy fra{" "}
                  <code className={code}>tools/list</code>. Med <code className={code}>-tool</code> sjekkes bare det
                  verktøyet.
                </TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell className="whitespace-nowrap">
                  <code className={code}>-legacy</code>
                </TableDataCell>
                <TableDataCell>
                  Serveren er i overgangsvinduet (<code className={code}>MCP_LEGACY_PROTOCOL_UNTIL</code>): hopper over
                  405-sjekkene for <code className={code}>GET</code> og <code className={code}>DELETE</code>.
                </TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell className="whitespace-nowrap">
                  <code className={code}>-max-bytes int</code>
                </TableDataCell>
                <TableDataCell>
                  Størrelsesgrensen serveren må håndheve. Standard <code className={code}>1048576</code> (1 MiB), må
                  være større enn 0.
                </TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell className="whitespace-nowrap">
                  <code className={code}>-version</code>
                </TableDataCell>
                <TableDataCell>Skriver ut versjonen og avslutter.</TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell className="whitespace-nowrap">
                  <code className={code}>MCP_COMPLIANCE_TOKEN</code>
                </TableDataCell>
                <TableDataCell>
                  Et gyldig brukertoken for serveren. Uten det feiler autentiseringssjekkene, og det er riktig for alt
                  som skal kjøre bak <code className={code}>fleet-gateway</code>.
                </TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell className="whitespace-nowrap">
                  <code className={code}>MCP_COMPLIANCE_APP_TOKEN</code>
                </TableDataCell>
                <TableDataCell>
                  Et app-token (<code className={code}>idtyp=app</code>) serveren må avvise med 403. Ikke satt betyr at
                  sjekken hoppes over.
                </TableDataCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
        <Heading size="small" level="3">
          Sjekker
        </Heading>
        <div className="overflow-x-auto">
          <Table size="small">
            <TableHeader>
              <TableRow>
                <TableHeaderCell scope="col">Sjekk</TableHeaderCell>
                <TableHeaderCell scope="col">Forventet</TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow>
                <TableDataCell>
                  <code className={code}>server/discover</code> kjører først
                </TableDataCell>
                <TableDataCell>Svarer den ikke 200, stopper testen.</TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell>Uten token, ugyldig token</TableDataCell>
                <TableDataCell>
                  401. Uten token: WARN hvis <code className={code}>WWW-Authenticate: Bearer</code> mangler eller har en{" "}
                  <code className={code}>error</code>-kode (RFC 6750 3.1). Ugyldig token: WARN uten{" "}
                  <code className={code}>{'error="invalid_token"'}</code>. Feil eller manglende challenge-header gir
                  bare WARN; annen status enn 401 er FAIL.
                </TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell>
                  App-token (<code className={code}>idtyp=app</code>), hvis{" "}
                  <code className={code}>MCP_COMPLIANCE_APP_TOKEN</code> er satt
                </TableDataCell>
                <TableDataCell>403</TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell>
                  <code className={code}>server/discover</code>
                </TableDataCell>
                <TableDataCell>
                  200, <code className={code}>supportedVersions</code> har <code className={code}>2026-07-28</code>,{" "}
                  <code className={code}>capabilities</code>, <code className={code}>ttlMs</code>,{" "}
                  <code className={code}>cacheScope</code>, <code className={code}>resultType: complete</code>,{" "}
                  <code className={code}>_meta</code> med serverinfo
                </TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell>
                  Manglende <code className={code}>MCP-Protocol-Version</code>, eller{" "}
                  <code className={code}>Mcp-Method</code>, <code className={code}>Mcp-Name</code> eller{" "}
                  <code className={code}>_meta</code>-protokollversjon som ikke stemmer med innholdet
                </TableDataCell>
                <TableDataCell>
                  400, <code className={code}>-32020</code>
                </TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell>
                  Protokollversjon som ikke støttes (<code className={code}>MCP-Protocol-Version</code>)
                </TableDataCell>
                <TableDataCell>
                  400, <code className={code}>-32022</code>
                </TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell>
                  Manglende <code className={code}>method</code>, <code className={code}>id: null</code>
                </TableDataCell>
                <TableDataCell>
                  400, <code className={code}>-32600</code>
                </TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell>
                  <code className={code}>GET</code>, <code className={code}>DELETE</code> (hoppes over med{" "}
                  <code className={code}>-legacy</code>)
                </TableDataCell>
                <TableDataCell>
                  405, <code className={code}>Allow: POST</code>
                </TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell>
                  Innhold over <code className={code}>-max-bytes</code>
                </TableDataCell>
                <TableDataCell>413</TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell>
                  <code className={code}>tools/list</code>: skjulte instruksjoner i en beskrivelse
                </TableDataCell>
                <TableDataCell>
                  FAIL for injeksjonsfraser, WARN for <code className={code}>you must</code>,{" "}
                  <code className={code}>always</code>, <code className={code}>never</code>; én rad per verktøy og
                  fragment
                </TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell>
                  <code className={code}>tools/list</code>: nullbredde-, bidi- (U+202A-U+202E, U+2066-U+2069) eller
                  tag-tegn (U+E0000-U+E007F)
                </TableDataCell>
                <TableDataCell>FAIL</TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell>
                  <code className={code}>tools/list</code>: <code className={code}>readOnlyHint</code>,{" "}
                  <code className={code}>destructiveHint</code>, <code className={code}>idempotentHint</code> eller{" "}
                  <code className={code}>openWorldHint</code> mangler i <code className={code}>annotations</code>
                </TableDataCell>
                <TableDataCell>FAIL</TableDataCell>
              </TableRow>
              <TableRow>
                <TableDataCell>
                  <code className={code}>tools/list</code>: beskrivelse over 1024 tegn
                </TableDataCell>
                <TableDataCell>WARN</TableDataCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
        <BodyLong>
          Beskrivelsessjekkene dekker <code className={code}>description</code> for hvert verktøy, hver{" "}
          <code className={code}>description</code> i <code className={code}>inputSchema</code>, og{" "}
          <code className={code}>title</code> og <code className={code}>annotations.title</code>. FAIL-mønstre (uten
          skille på store og små bokstaver):{" "}
          <code className={code}>ignore (all|any|the) previous|prior (instructions)</code>,{" "}
          <code className={code}>system prompt</code>, <code className={code}>do not tell the user</code> (også{" "}
          <code className={code}>{"don't"}</code>) og taggene <code className={code}>{"<system>"}</code> og{" "}
          <code className={code}>{"<instructions>"}</code>. U+061C gir også FAIL; myk bindestrek, U+034F, U+180E og
          variasjonsvelgere gir WARN. Testen skriver bare ut fragmentet som traff, sladdet og kuttet ved 200 tegn.
          Usynlige tegn rapporteres som kodepunkt og byteposisjon. Sjekken normaliserer ikke tekst, så den finner ikke
          homoglyfer, tekst i full bredde eller synonymer som disregard og override.
        </BodyLong>
        <BodyLong>
          Forespørslene sender <code className={code}>Accept: application/json, text/event-stream</code>, og testen
          leser svar i JSON eller SSE.
        </BodyLong>
        <Heading size="small" level="3">
          Resultat og avslutningskode
        </Heading>
        <BodyLong>
          Testen skriver én rad per sjekk med PASS, FAIL, WARN eller SKIP, og en oppsummeringslinje. WARN og SKIP feiler
          ikke kjøringen.
        </BodyLong>
        <Bullets>
          <li>Kode 0: ingen sjekk ga FAIL.</li>
          <li>
            Kode 1: minst én sjekk ga FAIL, eller <code className={code}>server/discover</code> svarte ikke 200.
          </li>
          <li>
            Kode 2: feil bruk. <code className={code}>-url</code> mangler, flagget er ukjent, eller{" "}
            <code className={code}>-max-bytes</code> er ikke større enn 0. <code className={code}>-h</code> skriver
            hjelpeteksten og gir også kode 2.
          </li>
        </Bullets>
        <Heading size="small" level="3">
          Testen dekker ikke dette
        </Heading>
        <BodyLong>Behold disse sjekkene i egne tester:</BodyLong>
        <Bullets>
          <li>
            <code className={code}>-32602</code> for manglende eller ugyldig <code className={code}>_meta</code>
          </li>
          <li>
            404 for ukjente metoder og for <code className={code}>ping</code>
          </li>
          <li>
            base64-kodet <code className={code}>Mcp-Name</code> (<code className={code}>=?base64?...?=</code>)
          </li>
          <li>
            at <code className={code}>Mcp-Session-Id</code> ignoreres
          </li>
        </Bullets>
        <Heading size="small" level="3">
          Installere, låse versjon og verifisere
        </Heading>
        <CodeBlock filename="Terminal">
          {"brew install navikt/tap/mcp-compliance\ngo install github.com/navikt/copilot/cli/mcp-compliance@latest"}
        </CodeBlock>
        <BodyLong>
          <code className={code}>go install</code> krever Go 1.26 eller nyere. Hver endring i{" "}
          <code className={code}>cli/mcp-compliance</code> som merges til <code className={code}>main</code>, blir
          automatisk en release med taggen <code className={code}>cli/mcp-compliance/vX.Y.Z</code>.{" "}
          <code className={code}>@latest</code> tar nyeste tagg, og Homebrew-formelen følger etter innen en time. Bruk{" "}
          <code className={code}>@vX.Y.Z</code> for å låse en versjon. Binærfiler for macOS og Linux (amd64, arm64) og
          en <code className={code}>SHA256SUMS</code>-fil ligger på hver{" "}
          <a href="https://github.com/navikt/copilot/releases?q=mcp-compliance" className={linkClass}>
            release
          </a>
          .
        </BodyLong>
        <CodeBlock filename="Terminal">
          {
            "gh attestation verify mcp-compliance-linux-amd64 --repo navikt/copilot\n\ngh release download cli/mcp-compliance/v0.2.2 --repo navikt/copilot \\\n  --pattern mcp-compliance-linux-amd64 --pattern SHA256SUMS\nsha256sum --check --ignore-missing SHA256SUMS"
          }
        </CodeBlock>
        <BodyLong>
          GitHub Action-en (
          <code className={code}>uses: navikt/copilot/cli/mcp-compliance@cli/mcp-compliance/v0.2.2</code>) tar inputene{" "}
          <code className={code}>url</code> (påkrevd), <code className={code}>version</code>,{" "}
          <code className={code}>token</code>, <code className={code}>app-token</code>,{" "}
          <code className={code}>tool</code>, <code className={code}>args</code> og{" "}
          <code className={code}>github-token</code> (standard <code className={code}>github.token</code>). Tom{" "}
          <code className={code}>version</code> bruker releasen Action-en er låst til;{" "}
          <code className={code}>latest</code> tar nyeste release, og brukes også, med en melding, når Action-referansen
          er en gren, en SHA eller en lokal sti.
        </BodyLong>
      </VStack>
    </DocPage>
  );
}
