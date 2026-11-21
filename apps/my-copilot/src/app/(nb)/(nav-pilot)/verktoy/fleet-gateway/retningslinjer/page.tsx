import {
  BarChartIcon,
  ClipboardCheckmarkIcon,
  EyeIcon,
  FileCodeIcon,
  PadlockLockedIcon,
  PencilIcon,
  RulerIcon,
  ShieldLockIcon,
  TasklistIcon,
} from "@navikt/aksel-icons";
import { BodyLong, BodyShort, Heading, HGrid, HStack, ReadMore, Tag, VStack } from "@navikt/ds-react";
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
const OWASP_BASE =
  "https://github.com/OWASP/www-project-mcp-top-10/blob/165fe0f78ef104459237b4a8e0f6e78db9b02391/2025/";
const SPEC_BEST_PRACTICES = "https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices";
const KODEVERK_URL = "https://github.com/navikt/copilot/tree/main/apps/mcp-kodeverk";
const ISSUE_URL = "https://github.com/navikt/copilot/issues/2490";
const SKILL_URL = "https://github.com/navikt/copilot/tree/main/skills/mcp-server-authoring";

type Ref = { label: string; href: string };

type Section = {
  id: string;
  icon: typeof EyeIcon;
  // The line in "Kort fortalt". Without it the section is left out there.
  short?: string;
  why: string;
  todo: string[];
  points: string[];
  owasp: Ref[];
  spec: Ref[];
  note?: string;
};

const OWASP_FILES: Record<string, string> = {
  MCP01: "MCP01-2025-Token-Mismanagement-and-Secret-Exposure.md",
  MCP02: "MCP02-2025%E2%80%93Privilege-Escalation-via-Scope-Creep.md",
  MCP03: "MCP03-2025%E2%80%93Tool-Poisoning.md",
  MCP05: "MCP05-2025%E2%80%93Command-Injection%26Execution.md",
  MCP06: "MCP06-2025%E2%80%93Intent-Flow-Subversion.md",
  MCP07: "MCP07-2025%E2%80%93Insufficient-Authentication%26Authorization.md",
  MCP08: "MCP08-2025%E2%80%93Lack-of-Audit-and-Telemetry.md",
  MCP09: "MCP09-2025%E2%80%93Shadow-MCP-Servers.md",
  MCP10: "MCP10-2025%E2%80%93ContextInjection%26OverSharing.md",
};

const owasp = (id: string): Ref => ({ label: id, href: `${OWASP_BASE}${OWASP_FILES[id]}` });

const SECTIONS: Section[] = [
  {
    id: "token",
    icon: ShieldLockIcon,
    short: "Sjekk hvert token med Texas.",
    why: "Gatewayen slipper inn alle med Nav-konto, så serveren må selv sjekke at kallet kommer fra fleet-gateway og har en bruker. Svarer serveren 401 når Texas er nede, ber klienten brukeren logge inn på nytt.",
    todo: [
      "La Texas sjekke tokenet. Ikke les JWT-en selv.",
      "Godta bare tokens fra fleet-gateway som har en bruker.",
      "Bruk Texas OBO når dere kaller et annet API.",
      "Sjekk hvert kall, ikke sesjonen.",
    ],
    points: [
      "Send tokenet til Texas for introspeksjon. Ikke les JWT-en selv.",
      "Sjekk at `azp` er fleet-gateway. `azp` er klient-ID-en, ikke navnet: slå opp `<klynge>:copilot:fleet-gateway` i `AZURE_APP_PRE_AUTHORIZED_APPS` og sammenlign med `clientId`.",
      "Krev at tokenet har `NAVident`. Avvis apptokens, altså tokens uten bruker.",
      "Svar 401 når Texas melder at tokenet er inaktivt (manglende, ugyldig eller utløpt). Svar 403 på apptoken, manglende `NAVident`, feil `azp` og manglende rettighet.",
      "Feiler Texas, svar 502 (`introspection_failed`). Mangler serveren auth-konfigurasjon, svar 503. Svar aldri 401 når Texas er nede, for da tror klienten at brukeren må logge inn på nytt.",
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
    icon: EyeIcon,
    short: "Start med verktøy som bare leser.",
    why: "Et verktøy som endrer data kan gjøre skade hvis modellen tar feil. Derfor kommer skriveverktøy først etter godkjent ROS.",
    todo: [
      "Første versjon har bare verktøy som leser.",
      "Vent med skriveverktøy til ROS er godkjent.",
      "La serveren selv kreve bekreftelse før den skriver.",
      "Gjør skriveverktøy trygge å prøve på nytt.",
    ],
    points: [
      "Første versjon har bare verktøy som leser. Skriveverktøy kommer først etter at ROS er godkjent.",
      "Skriveverktøy trenger tak på antall og alder, kvote per døgn, eierskapssjekk og en revisjonslinje. En beskrivelse som ber modellen bekrefte med brukeren er ikke nok.",
      "Serveren må selv håndheve bekreftelsen, for eksempel med to steg: første kall gir en forhåndsvisning og et kortlevd bekreftelsestoken, og andre kall må ha tokenet. mcp-dolly gjør dette i `create_order`. En melding til brukeren fra klienten eller `input_required` alene er ikke nok, for serveren kan ikke vite at klienten viste den.",
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
    icon: TasklistIcon,
    short: "List hvert verktøy ved navn.",
    why: "Gatewayen slipper bare gjennom verktøy som står på listen. Da vet alle hva serveren kan gjøre.",
    todo: [
      "List verktøyene ved navn i ruten og i registeret, og hold listene like.",
      "Gi verktøyene navn med tjeneste og ressurs.",
      "Registrer serveren i MCP-registeret.",
    ],
    points: [
      "Ruten i fleet-gateway og oppføringen i registeret lister hvert verktøy ved navn. De to listene må være like.",
      "Bare operatøren av gatewayen bruker `passAllTools`. Teamets rute bruker det ikke.",
      "Gi hvert verktøy et navn med tjeneste og ressurs, for eksempel `kodeverk_search`.",
      "Alle servere i gatewayen må stå i registeret.",
    ],
    owasp: [owasp("MCP02"), owasp("MCP09")],
    spec: [{ label: "Tools: Tool Names", href: `${SPEC}/server/tools` }],
  },
  {
    id: "grenser",
    icon: RulerIcon,
    short: "Sett grenser for input og svar.",
    why: "Spesifikasjonen krever at serveren begrenser kallene, og gatewayen gjør bare en del av jobben. Tekst fra andre systemer og fra brukere er data, ikke instruksjoner.",
    todo: [
      "Valider input mot et stramt JSON Schema.",
      "Sett tak for størrelse, sider og tidsbruk.",
      "Sett en grense for kall per bruker i serveren.",
      "Behandle tekst fra andre systemer som data.",
    ],
    points: [
      "Valider input mot JSON Schema: `minimum`, `maximum`, `maxLength`, `enum` og `additionalProperties: false`.",
      "Sett tak for størrelsen på forespørselen (`MaxBytesReader` i Go).",
      "Sett tidsavbrudd på alle kall til andre systemer. Serverens skrivetidsavbrudd må være lengre enn summen av dem.",
      "Sett tak for sidestørrelse og antall sider, og sorter likt hver gang.",
      "Sett tak for svarstørrelsen. mcp-kodeverk bruker 32 KiB. Svar med `truncated: true`, `nextOffset` og én setning om hvordan brukeren kan snevre inn søket.",
      "Behandle tekst fra andre systemer og fra brukere som data. Ikke legg instruksjoner til modellen i svaret.",
      "Ikke hent URL-er som brukeren oppgir. Er det uunngåelig: bare https, blokker private adresser, sjekk omdirigeringer og begrens utgående trafikk med `accessPolicy`.",
    ],
    owasp: [owasp("MCP05"), owasp("MCP06"), owasp("MCP10")],
    spec: [
      { label: "Tools: Security Considerations", href: `${SPEC}/server/tools` },
      { label: "Security best practices", href: SPEC_BEST_PRACTICES },
    ],
    note: "Spesifikasjonen krever at serveren begrenser kallene. Gatewayen begrenser bare samtidige kall per bruker (`user_cap` og `user_sse_cap`), ikke hvor mange kall brukeren sender per minutt. Dere må selv sette en grense for kall per bruker i serveren. Ingen av referanseserverne gjør det i dag.",
  },
  {
    id: "hoder",
    icon: FileCodeIcon,
    short: "Sjekk Mcp-hodene mot innholdet.",
    why: "MCP-revisjonen 2026-07-28 sender metode og verktøynavn i HTTP-hoder. Serveren må sjekke at hodene stemmer med innholdet i forespørselen.",
    todo: ["Sjekk påkrevde Mcp-hoder mot innholdet.", "Avvis forespørsler med manglende eller feil hode."],
    points: [
      "Sjekk `MCP-Protocol-Version`, `Mcp-Method` og, for `tools/call`, `Mcp-Name` mot innholdet i forespørselen. Avvis forespørsler der et påkrevd hode mangler.",
      "Sjekk også `Mcp-Param-*` for parametere verktøyet har merket for det. mcp-kodeverk har ingen slike parametere, så det finnes ingen referanseimplementasjon for dette.",
      "Ved avvik: svar HTTP 400 med JSON-RPC-feilen `-32020`.",
      "Svar 405 på GET hvis serveren ikke tilbyr en strøm.",
    ],
    owasp: [owasp("MCP05")],
    spec: [{ label: "Spesifikasjonen 2026-07-28", href: SPEC }],
    note: "Testen mcp2026_compliance_test.go i mcp-kodeverk viser hvordan dere tester `Mcp-Method` og `Mcp-Name`. En samsvarstest dere kan kjøre mot egen server kommer (#2548).",
  },
  {
    id: "logging",
    icon: PadlockLockedIcon,
    short: "Logg uten persondata.",
    why: "Gatewayen logger allerede hvem som kalte, så serveren trenger ikke gjenta persondata. Tokens og hemmeligheter skal aldri havne i loggene.",
    todo: [
      "Logg hvem som kalte, verktøy, utfall og korrelasjons-ID.",
      "Ikke logg tokens, fritekst eller persondata.",
      "Hent hemmeligheter bare fra Nais-secrets.",
    ],
    points: [
      "Logg hvem som kalte, hvilket verktøy, utfallet og en korrelasjons-ID. Gatewayens revisjonslogg lagrer allerede NAVident for hvert kall, så serveren trenger ikke gjenta persondata.",
      "Ikke logg tokens, fritekst eller input som kan inneholde persondata.",
      "Logger dere NAVident med vilje som revisjonsspor, skriv det i ROS.",
      "Hent hemmeligheter bare fra Nais-secrets. Ha dem aldri i kode, logger, feilmeldinger eller `x-mcp-header`, og aldri i query string.",
    ],
    owasp: [owasp("MCP01"), owasp("MCP08")],
    spec: [{ label: "Tools: Security Considerations", href: `${SPEC}/server/tools` }],
    note: "Regelen om persondata er vår egen og står ikke i spesifikasjonen.",
  },
  {
    id: "beskrivelser",
    icon: PencilIcon,
    short: "Skriv klare beskrivelser og sett annotasjoner.",
    why: "Modellen velger verktøy ut fra beskrivelsen. Den må være klar, og den må ikke kunne endres av tekst fra andre systemer.",
    todo: [
      "Skriv beskrivelsen som til en ny kollega.",
      "Ha beskrivelsene i kildekontroll. Ikke bygg dem av andre tekster.",
      "Sett alle fire annotasjonene på hvert verktøy.",
      "Skriv feilsvar som lar modellen rette seg.",
    ],
    points: [
      "Skriv beskrivelsen som til en ny kollega: når verktøyet skal og ikke skal brukes, format, forkortelser og ett eksempel.",
      "Bruk tydelige parameternavn, for eksempel `user_id` og ikke `user`.",
      "Hold beskrivelsene faste og lagre dem i kildekontroll. Ikke bygg dem av tekst fra andre systemer. Sjekk dem i CI for skjulte instruksjoner og usynlige Unicode-tegn.",
      "Sett alle fire annotasjonene på hvert verktøy: `readOnlyHint`, `destructiveHint`, `idempotentHint` og `openWorldHint`.",
      "Annotasjoner er hint, ikke en sikkerhetsgrense. Klienten kan ikke stole på dem. Håndhev reglene i serveren.",
      "Svar på feil med `isError: true`, og si hva som er galt, hva som er gyldig format og gi et eksempel, så modellen kan rette seg. Bruk JSON-RPC-feil bare for ugyldige forespørsler.",
    ],
    owasp: [owasp("MCP03")],
    spec: [{ label: "Tools: Tool, Error Handling", href: `${SPEC}/server/tools` }],
  },
  {
    id: "metrikker",
    icon: BarChartIcon,
    why: "Gatewayen ser bare kallene utenfra. Serveren må selv måle tid og feil per verktøy.",
    todo: ["Tell forespørsler og feil, og mål svartid, per verktøy.", "Bruk bare verktøynavn og utfall som etiketter."],
    points: [
      "Gatewayen sender allerede `fleet_gateway_mcp_requests_total` med `route` og `tool` (`other` for verktøy som ikke står på listen). Serveren må måle det gatewayen ikke ser: tid og feil inne i serveren, per verktøy.",
      "Tell forespørsler og feil, og mål svartid, per verktøy.",
      "Hold antallet mulige etikettverdier lavt. Bruk verktøynavn og utfall (ok, feil, avvist) som etiketter. Bruk aldri brukeren, input, URL eller feilmelding som etikett.",
    ],
    owasp: [owasp("MCP08")],
    spec: [],
    note: "Dette er vårt eget råd, ikke hentet fra en kilde. mcp-kodeverk og mcp-dolly har ingen egne målinger i dag. Vi planlegger å gjøre det til standard i et felles bibliotek (#2490).",
  },
  {
    id: "ros",
    icon: ClipboardCheckmarkIcon,
    short: "Lag en ROS per server.",
    why: "Hver server har sin egen risiko. ROS må være klar før serveren får en rute i prod.",
    todo: [
      "Lag ROS før serveren går i prod.",
      "Bruk malen for MCP-servere i copilot-intern.",
      "Skriv inn det som er spesielt for serveren.",
    ],
    points: [
      "Hver server har sin egen risiko- og sårbarhetsanalyse (ROS) før den får en rute i prod.",
      "Bruk malen for MCP-servere i copilot-intern. Malen ligger i et privat repo.",
      "Skriv inn det som er spesielt for serveren: skriveverktøy, logging av NAVident, egen OAuth-proxy og hvilke data den eksponerer.",
    ],
    owasp: [owasp("MCP02"), owasp("MCP08")],
    spec: [],
  },
];

const TOC: TocItem[] = [
  { id: "token", label: "Sjekk tokenet med Texas" },
  { id: "lesing-forst", label: "Start med lesing" },
  { id: "verktoylister", label: "Bruk eksplisitte verktøylister" },
  { id: "grenser", label: "Sett grenser for input og svar" },
  { id: "hoder", label: "Sjekk Mcp-hodene mot innholdet" },
  { id: "logging", label: "Logg uten persondata" },
  { id: "beskrivelser", label: "Skriv beskrivelser og annotasjoner" },
  { id: "metrikker", label: "Mål hvert verktøy" },
  { id: "ros", label: "Lag en ROS per server" },
  { id: "eksempler", label: "Eksempler" },
  { id: "kilder", label: "Kilder" },
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

function RefLinks({ refs }: { refs: Ref[] }) {
  return refs.map((r, i) => (
    <span key={r.label}>
      {i > 0 && ", "}
      <a href={r.href} className={linkClass}>
        {r.label}
      </a>
    </span>
  ));
}

function Rules({ id }: { id: string }) {
  const s = SECTIONS.find((x) => x.id === id)!;
  return (
    <>
      <BodyLong>{s.why}</BodyLong>
      <VStack gap="space-4">
        <Heading size="xsmall" level="3">
          Gjør dette
        </Heading>
        <Bullets>
          {s.todo.map((t) => (
            <li key={t}>{t}</li>
          ))}
        </Bullets>
      </VStack>
      <ReadMore header={`Detaljer: ${TOC.find((t) => t.id === s.id)!.label}`} size="small">
        <Bullets>
          {s.points.map((p) => (
            <li key={p}>
              <Inline text={p} />
            </li>
          ))}
        </Bullets>
        {s.note && (
          <BodyShort size="small">
            <Inline text={s.note} />
          </BodyShort>
        )}
      </ReadMore>
    </>
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
      <BodyShort>
        Gjelder servere bak fleet-gateway. Stegene for å koble til står i{" "}
        <NextLink href="/verktoy/fleet-gateway/for-team" className={linkClass}>
          Koble teamets MCP-server til fleet-gateway
        </NextLink>
        .
      </BodyShort>

      <VStack gap="space-12">
        <LinkableHeading id="kort-fortalt" size="medium" level="2">
          Kort fortalt
        </LinkableHeading>
        <HGrid as="ul" columns={{ xs: 1, md: 2 }} gap="space-8">
          {SECTIONS.filter((x) => x.short).map((x) => {
            const Icon = x.icon;
            return (
              <li key={x.id}>
                <HStack gap="space-8" align="center" wrap={false}>
                  <Icon aria-hidden fontSize="1.5rem" />
                  <a href={`#${x.id}`} className={linkClass}>
                    {x.short}
                  </a>
                </HStack>
              </li>
            );
          })}
        </HGrid>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="token" size="medium" level="2">
          Sjekk tokenet med Texas
        </LinkableHeading>
        <Rules id="token" />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="lesing-forst" size="medium" level="2">
          Start med lesing
        </LinkableHeading>
        <Rules id="lesing-forst" />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="verktoylister" size="medium" level="2">
          Bruk eksplisitte verktøylister
        </LinkableHeading>
        <Rules id="verktoylister" />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="grenser" size="medium" level="2">
          Sett grenser for input og svar
        </LinkableHeading>
        <Rules id="grenser" />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hoder" size="medium" level="2">
          Sjekk Mcp-hodene mot innholdet
        </LinkableHeading>
        <Rules id="hoder" />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="logging" size="medium" level="2">
          Logg uten persondata
        </LinkableHeading>
        <Rules id="logging" />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="beskrivelser" size="medium" level="2">
          Skriv beskrivelser og annotasjoner
        </LinkableHeading>
        <Rules id="beskrivelser" />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="metrikker" size="medium" level="2">
          Mål hvert verktøy
        </LinkableHeading>
        <Rules id="metrikker" />
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="ros" size="medium" level="2">
          Lag en ROS per server
        </LinkableHeading>
        <Rules id="ros" />
      </VStack>

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
          <li>
            Kotlin:{" "}
            <a href="https://github.com/navikt/mcp-server-kotlin-template" className={linkClass}>
              navikt/mcp-server-kotlin-template
            </a>
            .
          </li>
          <li>
            TypeScript:{" "}
            <a href="https://github.com/navikt/mcp-server-typescript-template" className={linkClass}>
              navikt/mcp-server-typescript-template
            </a>
            .
          </li>
          <li>Samsvarstest dere kan kjøre mot egen server: kommer (#2548).</li>
        </Bullets>
        <BodyLong>
          Bruker dere en KI-agent til å skrive serveren, finnes en skill med samme krav som en kort sjekkliste:{" "}
          <a href={SKILL_URL} className={linkClass}>
            mcp-server-authoring
          </a>
          .
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="kilder" size="medium" level="2">
          Kilder
        </LinkableHeading>
        <BodyLong>
          Hver del bygger på risikoene i{" "}
          <a href={OWASP_URL} className={linkClass}>
            OWASP MCP Top 10
          </a>{" "}
          og på{" "}
          <a href={SPEC} className={linkClass}>
            MCP-spesifikasjonen 2026-07-28
          </a>
          . Punkter uten kilde er vårt eget råd. Kilder og status for planen står i{" "}
          <a href={ISSUE_URL} className={linkClass}>
            #2490
          </a>
          .
        </BodyLong>
        <Bullets>
          {SECTIONS.map((x) => (
            <li key={x.id}>
              <a href={`#${x.id}`} className={linkClass}>
                {TOC.find((t) => t.id === x.id)!.label}
              </a>
              : OWASP <RefLinks refs={x.owasp} />
              {x.spec.length > 0 && (
                <>
                  . Spesifikasjon: <RefLinks refs={x.spec} />
                </>
              )}
              {x.spec.length === 0 && ". Ingen kilde i spesifikasjonen"}.
            </li>
          ))}
        </Bullets>
      </VStack>
    </DocPage>
  );
}
