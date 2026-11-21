import { Alert, BodyLong, BodyShort, Box, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Retningslinjer for MCP-servere",
  description:
    "Hva teamet må gjøre når dere skriver en MCP-server bak fleet-gateway: tokensjekk, grenser, hoder, logging, verktøybeskrivelser, målinger og ROS.",
};

const SPEC = "https://modelcontextprotocol.io/specification/2026-07-28";
const OWASP_URL = "https://owasp.org/www-project-mcp-top-10/";
const SPEC_BEST_PRACTICES = "https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices";
const KODEVERK_URL = "https://github.com/navikt/copilot/tree/main/apps/mcp-kodeverk";
const ISSUE_URL = "https://github.com/navikt/copilot/issues/2490";
const SKILL_URL = "https://github.com/navikt/copilot/tree/main/skills/mcp-server-authoring";

type Ref = { label: string; href: string };

type Section = {
  id: string;
  title: string;
  points: string[];
  owasp: Ref[];
  spec: Ref[];
  note?: string;
};

const owasp = (id: string): Ref => ({ label: id, href: OWASP_URL });

const SECTIONS: Section[] = [
  {
    id: "token",
    title: "Sjekk tokenet med Texas",
    points: [
      "Send tokenet til Texas for introspeksjon. Ikke les JWT-en selv.",
      "Sjekk at `azp` er fleet-gateway. `azp` er klient-ID-en, ikke navnet: slå opp `<klynge>:copilot:fleet-gateway` i `AZURE_APP_PRE_AUTHORIZED_APPS` og sammenlign med `clientId`.",
      "Krev at tokenet har `NAVident`. Avvis apptokens, altså tokens uten bruker.",
      "Svar 401 på manglende, ugyldig eller utløpt token, og 403 når brukeren mangler rettighet.",
      "Hvis oppslaget eller konfigurasjonen feiler, avvis forespørselen.",
      "Send aldri det innkommende tokenet videre til et annet API. Bruk Texas OBO for å få et eget token.",
      "Bruk aldri sesjons-ID som innlogging. Sjekk hvert kall.",
    ],
    owasp: [owasp("MCP01"), owasp("MCP07")],
    spec: [
      { label: "Authorization", href: `${SPEC}/basic/authorization` },
      { label: "Security best practices", href: SPEC_BEST_PRACTICES },
    ],
  },
  {
    id: "lesing-forst",
    title: "Start med lesing",
    points: [
      "Første versjon har bare verktøy som leser. Skriveverktøy kommer etter at ROS er godkjent.",
      "Skriveverktøy trenger tak på antall og alder, kvote per døgn, eierskapssjekk og en revisjonslinje. En beskrivelse som ber modellen bekrefte med brukeren er ikke nok.",
      "Gjør skriveverktøy trygge å prøve på nytt, for eksempel med en idempotensnøkkel eller et tydelig «allerede opprettet»-svar.",
    ],
    owasp: [owasp("MCP02")],
    spec: [
      { label: "Tools", href: `${SPEC}/server/tools` },
      { label: "Multi round-trip requests", href: `${SPEC}/basic/patterns/mrtr` },
    ],
    note: "For bekreftelse kan serveren bruke input_required der klienten støtter det. Referanseserveren med skriveverktøy er apps/mcp-dolly.",
  },
  {
    id: "verktoylister",
    title: "Bruk eksplisitte verktøylister",
    points: [
      "Ruten i fleet-gateway og oppføringen i registeret lister hvert verktøy ved navn. De to listene må være like.",
      "Bare operatøren av gatewayen bruker `passAllTools`. Teamets rute bruker det ikke.",
      "Gi hvert verktøy et navn med tjeneste og ressurs, for eksempel `kodeverk_search`.",
      "Ha ikke servere i gatewayen som mangler i registeret.",
    ],
    owasp: [owasp("MCP02"), owasp("MCP09")],
    spec: [{ label: "Tools: Tool Names", href: `${SPEC}/server/tools` }],
  },
  {
    id: "grenser",
    title: "Sett grenser på input og svar",
    points: [
      "Valider input mot JSON Schema: `minimum`, `maximum`, `maxLength`, `enum` og `additionalProperties: false`.",
      "Sett tak på størrelsen på forespørselen (`MaxBytesReader` i Go).",
      "Sett tidsavbrudd på alle kall til andre systemer. Serverens skrivetidsavbrudd må være lengre enn summen av dem.",
      "Sett tak på sidestørrelse og antall sider, og sorter likt hver gang.",
      "Sett tak på svarstørrelsen. mcp-kodeverk bruker 32 KiB. Svar med `truncated: true`, `nextOffset` og én setning om hvordan brukeren kan snevre inn søket.",
      "Behandle tekst fra andre systemer og fra brukere som data. Ikke legg instruksjoner til modellen i svaret.",
      "Ikke hent URL-er som brukeren oppgir. Er det uunngåelig: bare https, blokker private adresser, sjekk omdirigeringer og begrens utgående trafikk med `accessPolicy`.",
    ],
    owasp: [owasp("MCP05"), owasp("MCP06"), owasp("MCP10")],
    spec: [
      { label: "Tools: Security Considerations", href: `${SPEC}/server/tools` },
      { label: "Security best practices", href: SPEC_BEST_PRACTICES },
    ],
    note: "Ingen av referanseserverne begrenser hvor ofte et verktøy kan kalles. Spesifikasjonen sier at serveren skal begrense kall. Om gatewayen dekker det, er ikke undersøkt. Sett en grense per bruker i serveren selv.",
  },
  {
    id: "hoder",
    title: "Sjekk Mcp-hodene mot innholdet",
    points: [
      "Sjekk `MCP-Protocol-Version`, `Mcp-Method` og, for `tools/call`, `Mcp-Name` mot innholdet i forespørselen. Avvis forespørsler der et påkrevd hode mangler.",
      "Sjekk også `Mcp-Param-*` for parametere verktøyet har merket for det.",
      "Ved avvik svarer dere HTTP 400 med JSON-RPC-feilen `-32020`.",
      "Svar 405 på GET hvis serveren ikke tilbyr en strøm.",
    ],
    owasp: [owasp("MCP05")],
    spec: [{ label: "Spesifikasjonen 2026-07-28", href: SPEC }],
    note: "Testen mcp2026_compliance_test.go i mcp-kodeverk viser hvordan dere tester dette.",
  },
  {
    id: "logging",
    title: "Logg uten persondata",
    points: [
      "Logg hvem som kalte, hvilket verktøy, utfallet og en korrelasjons-ID.",
      "Ikke logg tokens, fritekst eller input som kan inneholde persondata.",
      "Logger dere NAV-ident med vilje som revisjonsspor, skriv det i ROS.",
      "Hemmeligheter kommer bare fra Nais-secrets. De skal aldri stå i kode, logger, feilmeldinger eller `x-mcp-header`, og aldri i query string.",
    ],
    owasp: [owasp("MCP01"), owasp("MCP08")],
    spec: [{ label: "Tools: Security Considerations", href: `${SPEC}/server/tools` }],
    note: "Regelen om persondata er vår egen. Den står ikke i spesifikasjonen.",
  },
  {
    id: "beskrivelser",
    title: "Skriv beskrivelser og annotasjoner",
    points: [
      "Skriv beskrivelsen til en ny kollega: når verktøyet brukes og ikke brukes, format, forkortelser og ett eksempel.",
      "Bruk tydelige parameternavn, for eksempel `user_id` og ikke `user`.",
      "Hold beskrivelsene faste og i kildekontroll. Ikke bygg dem av tekst fra andre systemer. Test dem i CI for skjulte instruksjoner og usynlig Unicode.",
      "Sett alle fire annotasjonene på hvert verktøy: `readOnlyHint`, `destructiveHint`, `idempotentHint` og `openWorldHint`.",
      "Annotasjoner er hint, ikke en sikkerhetsgrense. Klienten kan ikke stole på dem. Håndhev reglene i serveren.",
      "Svar på feil med `isError: true`, hva som er galt, gyldig format og et eksempel, så modellen kan rette seg. Bruk JSON-RPC-feil bare for ugyldige forespørsler.",
    ],
    owasp: [owasp("MCP03")],
    spec: [{ label: "Tools: Tool og Error Handling", href: `${SPEC}/server/tools` }],
  },
  {
    id: "metrikker",
    title: "Mål hvert verktøy",
    points: [
      "Tell forespørsler, feil og latens per verktøy.",
      "Hold antall unike verdier lavt. Bruk verktøynavn og utfall (ok, feil, avvist) som etiketter. Bruk aldri bruker, input, URL eller feilmelding som etikett.",
    ],
    owasp: [owasp("MCP08")],
    spec: [],
    note: "Ikke bekreftet i en kilde. Dette er vårt eget råd. mcp-kodeverk og mcp-dolly har ingen egne målinger i dag. Planen er å gjøre det til standard i et felles bibliotek (#2490).",
  },
  {
    id: "ros",
    title: "Lag en ROS per server",
    points: [
      "Hver server har sin egen risiko- og sårbarhetsanalyse (ROS) før den får en rute i prod.",
      "Bruk malen for MCP-servere i copilot-intern. Malen ligger i et privat repo.",
      "Skriv inn det som er spesielt for serveren: skriveverktøy, logging av NAV-ident, egen OAuth-proxy og hvilke data den eksponerer.",
    ],
    owasp: [owasp("MCP02"), owasp("MCP08")],
    spec: [],
  },
];

const TOC: TocItem[] = [
  ...SECTIONS.map((s) => ({ id: s.id, label: s.title })),
  { id: "eksempler", label: "Eksempler" },
];

// Splits on backticks so `x` renders as inline code.
function Inline({ text }: { text: string }) {
  return (
    <>
      {text.split("`").map((part, i) =>
        i % 2 === 1 ? (
          <code key={i} className={code}>
            {part}
          </code>
        ) : (
          part
        )
      )}
    </>
  );
}

function Refs({ label, refs }: { label: string; refs: Ref[] }) {
  if (refs.length === 0) return null;
  return (
    <BodyShort size="small">
      {label}:{" "}
      {refs.map((r, i) => (
        <span key={r.label}>
          {i > 0 && ", "}
          <a href={r.href} className={linkClass}>
            {r.label}
          </a>
        </span>
      ))}
    </BodyShort>
  );
}

export default function McpRetningslinjerPage() {
  return (
    <DocPage
      label="Verktøy"
      title="Retningslinjer for MCP-servere"
      description={metadata.description as string}
      badge={
        <Tag variant="warning" size="small">
          Pilot
        </Tag>
      }
      toc={TOC}
    >
      <Alert variant="info">
        Retningslinjene gjelder servere bak fleet-gateway. Se{" "}
        <NextLink href="/verktoy/fleet-gateway/for-team" className={linkClass}>
          Koble teamets MCP-server til fleet-gateway
        </NextLink>{" "}
        for stegene for å registrere serveren.
      </Alert>

      <BodyLong>
        Hvert punkt under viser hva dere må gjøre, hvilken risiko i{" "}
        <a href={OWASP_URL} className={linkClass}>
          OWASP MCP Top 10
        </a>{" "}
        det dekker og hvilken del av{" "}
        <a href={SPEC} className={linkClass}>
          MCP-spesifikasjonen 2026-07-28
        </a>{" "}
        det bygger på. Er det ikke en lenke til en kilde, er punktet vårt eget råd. Kilder og status for planen står i{" "}
        <a href={ISSUE_URL} className={linkClass}>
          #2490
        </a>
        .
      </BodyLong>

      {SECTIONS.map((s) => (
        <VStack key={s.id} gap="space-12">
          <LinkableHeading id={s.id} size="medium" level="2">
            {s.title}
          </LinkableHeading>
          <Bullets>
            {s.points.map((p) => (
              <li key={p}>
                <Inline text={p} />
              </li>
            ))}
          </Bullets>
          <Box background="neutral-soft" padding="space-12" borderRadius="8">
            <VStack gap="space-4">
              <Refs label="OWASP" refs={s.owasp} />
              <Refs label="Spesifikasjon" refs={s.spec} />
              {s.note && (
                <BodyShort size="small">
                  <Inline text={s.note} />
                </BodyShort>
              )}
            </VStack>
          </Box>
        </VStack>
      ))}

      <VStack gap="space-12">
        <LinkableHeading id="eksempler" size="medium" level="2">
          Eksempler
        </LinkableHeading>
        <Bullets>
          <li>
            Go:{" "}
            <a href={KODEVERK_URL} className={linkClass}>
              apps/mcp-kodeverk
            </a>
            . Referanse for tokensjekk, tak, sidedeling, svarskjema og samsvarstest.
          </li>
          <li>Kotlin med Ktor: kommer, legges i examples/ i en egen PR.</li>
          <li>TypeScript: kommer, legges i examples/ i en egen PR.</li>
        </Bullets>
        <BodyLong>
          Bruker dere en KI-agent til å skrive serveren, finnes en skill med samme krav som en kort sjekkliste:{" "}
          <a href={SKILL_URL} className={linkClass}>
            mcp-server-authoring
          </a>
          .
        </BodyLong>
      </VStack>
    </DocPage>
  );
}
