import { BodyLong, BodyShort, Box, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import type { ReactNode } from "react";
import { Table, TableBody, TableRow, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, HeaderRow, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";
import { OPENCODE_INSTALL } from "@/lib/install-commands";

export const metadata: Metadata = {
  title: "Klienter",
  description:
    "Copilot CLI, opencode og pi: hva hver klient kan, hvordan nav-pilot håndhever Navs MCP-register i opencode, og hva som mangler.",
};

const TOC: TocItem[] = [
  { id: "stotte-klienter", label: "Klientene" },
  { id: "paritet", label: "Hva hver klient kan" },
  { id: "mcp-register", label: "Navs MCP-register" },
  { id: "uten-cplt", label: "Uten cplt" },
  { id: "opencode", label: "opencode" },
  { id: "copilot-cli", label: "Copilot CLI" },
  { id: "pi", label: "pi" },
  { id: "kjente-hull", label: "Kjente hull i opencode" },
];

const GH = "https://github.com/navikt/copilot";
const HOOKS_DOC = `${GH}/blob/main/cli/nav-pilot/docs/opencode-hooks.md`;
// OpenCode1TestedRange and OpenCode2TestedRange in cli/nav-pilot/internal/provider/opencode_policy.go
const OPENCODE_RANGE = ">=1.18.20,<1.19";
const OPENCODE2_RANGE = ">=2.0.24,<2.1";
const c = (s: string) => <code className={code}>{s}</code>;

const OPENCODE_INSTALL_BLOCK = `# macOS
${OPENCODE_INSTALL.mac}
# Linux og WSL
${OPENCODE_INSTALL.linux}`;

const CLIENTS = [
  {
    name: "opencode",
    tag: { text: "Standard", variant: "info" as const },
    desc: "opencode med Copilot-abonnementet ditt. Den eneste klienten der hovedagenten kan kjøre i skyen og sende jobber til en lokal modell.",
  },
  {
    name: "copilot",
    tag: { text: "Støttet", variant: "success" as const },
    desc: "GitHub Copilot CLI, i sandkassen cplt. Agentpakka virker også i VS Code, JetBrains og på github.com.",
  },
  {
    name: "pi",
    tag: { text: "Eksperimentell", variant: "warning" as const },
    desc: "pi i cplt, med skills, persona og AGENTS.md lagt inn ved oppstart. Mangler blant annet hooks.",
  },
];

// Parity with navikt/copilot#1022 as of 2026-09-28, with pi from pi_launch.go.
// "Ja" renders ✓, "Nei" –, "Ikke aktuelt" i.a.; anything else is shown as text.
const PARITY: { what: ReactNode; copilot: string; opencode: string; pi: string; note?: ReactNode }[] = [
  {
    what: "Maskering av hemmeligheter og fnr.",
    copilot: "Ja",
    opencode: "Ja",
    pi: "Nei",
    note: "Feiler maskeringen i opencode, holder nav-pilot verktøyresultatet tilbake.",
  },
  { what: "Merknad om prompt-injeksjon", copilot: "Ja", opencode: "Ja", pi: "Nei" },
  { what: "Løkkevakt i skyøkter", copilot: "Ja", opencode: "Ja", pi: "Nei" },
  {
    what: "Gates og agentpakke-hooks",
    copilot: "Ja",
    opencode: "Ja",
    pi: "Nei",
    note: "Gates fra repoet kjører i opencode uten at du har godtatt mappa først, slik Copilot CLI krever.",
  },
  {
    what: <>{c("tools:")} i agentene</>,
    copilot: "Ja",
    opencode: "Ja",
    pi: "Nei",
    note: <>opencode får dem som permission per agent. MCP-verktøy og {c("task")} oversettes ikke.</>,
  },
  {
    what: "Navs MCP-register",
    copilot: "Ja",
    opencode: "Ja",
    pi: "Nei",
    note: "GitHub håndhever det i Copilot CLI, nav-pilot i opencode.",
  },
  {
    what: "Testet klientversjon",
    copilot: "Ikke aktuelt",
    opencode: "Ja",
    pi: "Nei",
    note: (
      <>
        opencode {c(OPENCODE_RANGE)} og {c(OPENCODE2_RANGE)}. Utenfor dette advarer nav-pilot ved oppstart og i{" "}
        {c("nav-pilot doctor")}.
      </>
    ),
  },
  { what: "Deling slått av, oppdatering som varsel", copilot: "Ikke aktuelt", opencode: "Ja", pi: "Ikke aktuelt" },
  { what: "WSL2 på Windows", copilot: "Ja", opencode: "Ja", pi: "Nei" },
  {
    what: <>Sky + {c("local-worker")}</>,
    copilot: "Nei",
    opencode: "Ja",
    pi: "Nei",
    note: "Copilot CLI kjører en økt helt lokalt eller helt i skyen, aldri blandet.",
  },
  {
    what: "Utsendingsstopp",
    copilot: "Nei",
    opencode: "Ja",
    pi: "Nei",
    note: (
      <>
        Med {c("local_dispatch")} balanced eller aggressive stopper nav-pilot hovedagenten når den redigerer for mye
        selv.
      </>
    ),
  },
  {
    what: <>Egen server ({c("local_endpoint")})</>,
    copilot: "Ja",
    opencode: "Ja",
    pi: "Nei",
    note: "I Copilot CLI tar serveren hele økten.",
  },
  {
    what: (
      <>
        {c("autopilot")}, {c("context_tier")}, {c("ask_user")}
      </>
    ),
    copilot: "Ja",
    opencode: "Nei",
    pi: "Nei",
    note: "nav-pilot advarer ved oppstart i opencode.",
  },
  { what: "Utvidelser (extensions)", copilot: "Ja", opencode: "Nei", pi: "Nei" },
];

function Mark({ v }: { v: string }) {
  const sym = v === "Ja" ? "✓" : v === "Nei" ? "–" : v === "Ikke aktuelt" ? "i.a." : null;
  if (!sym) return <>{v}</>;
  return (
    <>
      <span aria-hidden="true">{sym}</span>
      <span className="sr-only">{v}</span>
    </>
  );
}

const MCP_WARNING = `⚠ MCP servers turned off for this session (not in Nav's MCP registry): <navn>. See https://ki-utvikling.nav.no/verktoy (approved servers); to add one: https://github.com/navikt/copilot/blob/main/apps/mcp-registry/README.md#adding-servers`;

function Section({ children }: { children: ReactNode }) {
  return (
    <section>
      <VStack gap="space-16">{children}</VStack>
    </section>
  );
}

const notes = PARITY.filter((r) => r.note);
const cols = ["Funksjon", "Copilot CLI", "opencode", "pi"];

export default function Klienter() {
  return (
    <DocPage
      label="Referanse"
      title="Klienter"
      description="Hva Copilot CLI, opencode og pi kan når nav-pilot starter dem, og hva som mangler."
      toc={TOC}
      wide
    >
      <Section>
        <LinkableHeading id="stotte-klienter" size="medium" level="2">
          Klientene
        </LinkableHeading>
        <div className="overflow-x-auto">
          <Table size="small" className="table-stack" role="table">
            <HeaderRow stack cells={["Klient", "Status", "Hva du får"]} />
            <TableBody role="rowgroup">
              {CLIENTS.map((k) => (
                <TableRow role="row" key={k.name}>
                  <TableDataCell role="cell" className="whitespace-nowrap">
                    {c(k.name)}
                  </TableDataCell>
                  <TableDataCell role="cell" data-label="Status">
                    <Tag size="small" variant={k.tag.variant} className="whitespace-nowrap">
                      {k.tag.text}
                    </Tag>
                  </TableDataCell>
                  <TableDataCell role="cell" data-label="Hva du får">
                    {k.desc}
                  </TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        <Bullets>
          <li>
            Nye installasjoner får opencode: første gang spør nav-pilot hvilken klient du vil ha, med opencode
            forhåndsvalgt. Har du brukt nav-pilot før, beholder du klienten din.
          </li>
          <li>
            Bytt for én økt med {c("--client copilot")}, for godt med {c("nav-pilot config set client copilot")}.
          </li>
          <li>
            I CI og uten terminal starter nav-pilot Copilot CLI når {c("config.toml")} mangler. Bruk{" "}
            {c("--client opencode")} for opencode.
          </li>
          <li>
            Bruker du lokal modell i Copilot CLI og har opencode installert, får du ett tips om opencode. Slå av med{" "}
            {c("nav-pilot config set surveys false")}.
          </li>
        </Bullets>
        <BodyLong>
          Mangler opencode, tilbyr nav-pilot å installere en testet versjon med en av kommandoene under. Sier du nei,
          får du Copilot CLI.
        </BodyLong>
        <div className="[&_pre]:[overflow-wrap:anywhere]">
          <CodeBlock compact>{OPENCODE_INSTALL_BLOCK}</CodeBlock>
        </div>
      </Section>

      <Section>
        <LinkableHeading id="paritet" size="medium" level="2">
          Hva hver klient kan
        </LinkableHeading>
        <BodyLong>✓ betyr ja, – nei og i.a. ikke aktuelt.</BodyLong>
        <div className="overflow-x-auto">
          <Table size="small">
            <HeaderRow cells={cols} />
            <TableBody>
              {PARITY.map((r, i) => (
                <TableRow key={i}>
                  <TableDataCell>
                    <strong>{r.what}</strong>
                    {r.note && (
                      <sup>
                        {" "}
                        <a
                          href={`#merknad-${notes.indexOf(r) + 1}`}
                          aria-label={`merknad ${notes.indexOf(r) + 1}`}
                          className={linkClass}
                        >
                          {notes.indexOf(r) + 1}
                        </a>
                      </sup>
                    )}
                  </TableDataCell>
                  {[r.copilot, r.opencode, r.pi].map((v, j) => (
                    <TableDataCell key={j} className="text-center">
                      <Mark v={v} />
                    </TableDataCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        <ol className="list-decimal space-y-1 pl-6 text-sm">
          {notes.map((r, i) => (
            <li key={i} id={`merknad-${i + 1}`}>
              <strong>{r.what}:</strong> {r.note}
            </li>
          ))}
        </ol>
        <Box background="warning-soft" borderRadius="8" padding="space-16">
          <BodyShort>
            Uten nav-pilots hooks er det verken løkkevakt eller maskering. Det gjelder alltid pi, og opencode med{" "}
            {c("--pure")} eller uten nav-pilot. Se{" "}
            <a href="#kjente-hull" className={linkClass}>
              Kjente hull i opencode
            </a>
            .
          </BodyShort>
        </Box>
      </Section>

      <Section>
        <LinkableHeading id="mcp-register" size="medium" level="2">
          Navs MCP-register
        </LinkableHeading>
        <BodyLong>
          Nav tillater bare MCP-servere som står i Navs MCP-register (se{" "}
          <NextLink href="/verktoy?type=mcp" className={linkClass}>
            verktøykatalogen
          </NextLink>
          ). Copilot CLI spør GitHub og kjører bare de godkjente. opencode har ingen slik sjekk, så nav-pilot gjør den
          ved oppstart:
        </BodyLong>
        <Bullets>
          <li>
            Servere i opencode-konfigen din som ikke står i registeret, slås av for økten. En ekstern server må ha samme
            URL som i registeret, en lokal må starte en pakke registeret lister.
          </li>
          <li>
            Kobler du til en avslått server med {c("/mcp")} i økten, avviser hooks-pluginen verktøyene dens. Med{" "}
            {c("--pure")} kjører ingen hooks, så da virker ikke dette.
          </li>
          <li>Konfigen din endres ikke, og opencode du starter uten nav-pilot, berøres ikke.</li>
          <li>Mangler {c("gh")}, eller svarer ikke GitHub, slår nav-pilot ingenting av. Du får en advarsel.</li>
        </Bullets>
        <BodyLong>
          Ved oppstart ser du hva som ble slått av. {c("nav-pilot doctor")} viser det samme på forhånd.
        </BodyLong>
        {/* The URLs have no break points; let them wrap on a phone. */}
        <div className="[&_pre]:[overflow-wrap:anywhere]">
          <CodeBlock compact>{MCP_WARNING}</CodeBlock>
        </div>
        <BodyLong>
          Slå på en server fra registeret med {c("nav-pilot mcp enable <navn>")}, av med{" "}
          {c("nav-pilot mcp disable <navn>")}. {c("nav-pilot mcp list")} viser hva som hindrer en server, og kommandoen
          som retter det. I cplt spør nav-pilot om sandkassen skal slippe gjennom hostene serverne trenger. Enter betyr
          nei, og {c("nav-pilot config set mcp_hosts off")} slår spørsmålet av. Se{" "}
          <NextLink href="/nav-pilot/guider/feilsoking#mcp" className={linkClass}>
            Når en MCP-server ikke virker
          </NextLink>
          . Hvilke konfigfiler nav-pilot leser, står i{" "}
          <a href={`${HOOKS_DOC}#mcp-servers-outside-navs-registry-1027`} className={linkClass}>
            opencode-hooks.md
          </a>
          .
        </BodyLong>
        <LinkableHeading id="legg-til-server" size="small" level="3">
          Få en server inn i registeret
        </LinkableHeading>
        <BodyLong>
          Legg den til i {c("apps/mcp-registry/allowlist.json")} i navikt/copilot og lag en pull request. Den må gjennom
          en sikkerhetsgjennomgang. Hvordan du skriver oppføringen, står i{" "}
          <a href={`${GH}/blob/main/apps/mcp-registry/README.md#adding-servers`} className={linkClass}>
            README for MCP-registeret
          </a>
          .
        </BodyLong>
      </Section>

      <Section>
        <LinkableHeading id="uten-cplt" size="medium" level="2">
          Uten cplt
        </LinkableHeading>
        <BodyLong>
          cplt er sandkassen som holder agenten unna SSH-nøkler, skytilganger og andre hemmeligheter (se{" "}
          <NextLink href="/nav-pilot/forklaring/sandkassen" className={linkClass}>
            Sandkassen
          </NextLink>
          ). Uten cplt starter ingen klient. {c("--no-sandbox")} finnes ikke lenger. nav-pilot avslutter med en
          feilmelding som viser hvordan du installerer cplt.
        </BodyLong>
      </Section>

      <Section>
        <LinkableHeading id="opencode" size="medium" level="2">
          opencode
        </LinkableHeading>
        <Bullets>
          <li>
            nav-pilot legger AGENTS.md, skills, kommandoer og agenter i {c("~/.config/opencode/")} og oppdaterer dem ved
            hver oppstart. Filer du har endret selv, lar den være.
          </li>
          <li>
            Oppstarten venter ikke på GitHub. nav-pilot bruker kopien av agentpakka i {c("~/.nav-pilot/sources/")} og
            henter ny i bakgrunnen, høyst én gang i timen. Bare første oppstart venter på nedlastingen.
          </li>
          <li>
            Velger du ikke modell selv, bruker nav-pilot standarden agentpakka oppgir. For agentpakka nav-pilot er det
            GPT-6 Sol.
          </li>
          <li>
            {c("nav-pilot export opencode")} skriver agentpakka til {c(".opencode/")} i repoet. Du trenger det ikke for
            å bruke opencode.
          </li>
          <li>nav-pilot setter opp OpenTelemetry for opencode, med mindre du har slått av telemetri.</li>
        </Bullets>
        <LinkableHeading id="testet-versjon" size="small" level="3">
          Testet versjon
        </LinkableHeading>
        <BodyLong>
          nav-pilot er testet mot opencode {c(OPENCODE_RANGE)} og {c(OPENCODE2_RANGE)}. Utenfor dette advarer nav-pilot
          og starter likevel, men hooks og utsendingsstopp virker kanskje ikke. {c("nav-pilot doctor")} viser versjonen
          din. opencode 3 starter ikke før vi har testet den.
        </BodyLong>
        <BodyLong>opencode 2 krever:</BodyLong>
        <Bullets>
          <li>macOS med cplt fra 8. oktober 2026 eller nyere. {c("cplt --version")} viser datoen først i versjonen.</li>
          <li>På Linux og WSL kjører cplt ikke opencode 2 ennå. Der må du bruke opencode 1, og nav-pilot sier fra.</li>
          <li>
            Går du fra opencode 1 til 2, må du kjøre {c("opencode auth import")} én gang. Innloggingen følger ikke med
            automatisk.
          </li>
        </Bullets>
        <LinkableHeading id="deling-og-oppdatering" size="small" level="3">
          Deling og oppdatering
        </LinkableHeading>
        <BodyLong>
          Hver økt nav-pilot starter, får {c('"share": "disabled"')} og {c('"autoupdate": "notify"')}, uansett hva{" "}
          {c("opencode.json")} sier. Deling ville lastet opp økten til opencode.ai, og en oppdatering midt i en økt
          kunne gitt deg en versjon utenfor det testede området. nav-pilot setter også {c('"share": "disabled"')} i{" "}
          {c("~/.config/opencode/opencode.json")} når fila ikke sier noe om deling. Står det {c('"auto"')} der, får du
          en advarsel.
        </BodyLong>
        <BodyLong>
          Økter nav-pilot starter, får lese filene i {c("~/.config/opencode/")} uten å spørre om{" "}
          {c("external_directory")}, men ikke endre dem. Har du satt {c("external_directory")} til {c('"deny"')}, legger
          nav-pilot ikke til noe. Detaljene står i{" "}
          <a href={`${HOOKS_DOC}#what-else-a-launch-sets`} className={linkClass}>
            opencode-hooks.md
          </a>
          .
        </BodyLong>
        <LinkableHeading id="utsending" size="small" level="3">
          Lokal utsending
        </LinkableHeading>
        <BodyLong>
          Bare i opencode kan hovedagenten kjøre i skyen og sende avgrensede jobber til {c("local-worker")}, en
          underagent på den lokale modellen. {c("local_dispatch")} styrer hvor mye som sendes. Se{" "}
          <NextLink href="/nav-pilot/guider/lokal#utsending" className={linkClass}>
            Styr utsendingen
          </NextLink>
          .
        </BodyLong>
      </Section>

      <Section>
        <LinkableHeading id="copilot-cli" size="medium" level="2">
          Copilot CLI
        </LinkableHeading>
        <BodyLong>
          {c("mode = autopilot")}, {c("context_tier")}, {c("ask_user")}, {c("autonomy")} og utvidelser (extensions)
          finnes bare i Copilot CLI. I cplt kjører Copilot CLI kommandoer på egen hånd og spør når den trenger det, med
          mindre du har satt {c("nav-pilot config set autonomy conservative")}. En lokal modell tar hele økten, uten
          utsending, se{" "}
          <NextLink href="/nav-pilot/lokal" className={linkClass}>
            Lokal modell på Mac
          </NextLink>
          .
        </BodyLong>
        <Box background="info-soft" borderRadius="8" padding="space-16">
          <BodyShort size="small">
            nav-pilot henter ikke GitHub-tokenet selv. Er gh-vakta i cplt på, henter cplt det fra {c("GH_TOKEN")},{" "}
            {c("GITHUB_TOKEN")}, {c("COPILOT_GITHUB_TOKEN")} eller {c("gh auth token")}. {c("copilot_auth_mode")}{" "}
            begrenser kildene: {c("env_only")} krever token i miljøet, {c("gh_only")} fjerner token-variablene.
          </BodyShort>
        </Box>
      </Section>

      <Section>
        <LinkableHeading id="pi" size="medium" level="2">
          pi{" "}
          <Tag variant="warning" size="small">
            eksperimentell
          </Tag>
        </LinkableHeading>
        <BodyLong>
          Installer pi med {c("npm i -g @earendil-works/pi-coding-agent")}. nav-pilot legger agentpakka i{" "}
          {c("~/.nav-pilot/pi/")} og gir pi skills og AGENTS.md som flagg. Dette mangler:
        </BodyLong>
        <Bullets>
          <li>Hooks: ingen maskering, ingen løkkevakt og ingen gates.</li>
          <li>
            {c("--agent")}: pi kan ikke bytte agent. Personaen blir en del av systemprompten, og {c("tools:")} gjelder
            ikke.
          </li>
          <li>
            Lokal modell: ingen {c("local-worker")}, {c("local_dispatch")} eller {c("local_endpoint")}.
          </li>
          <li>Sjekk mot Navs MCP-register.</li>
          <li>
            Innstillingene {c("mode")}, {c("reasoning_effort")}, {c("context_tier")}, {c("allow_all_tools")},{" "}
            {c("ask_user")} og {c("log_level")}. nav-pilot advarer om dem.
          </li>
        </Bullets>
      </Section>

      <Section>
        <LinkableHeading id="kjente-hull" size="medium" level="2">
          Kjente hull i opencode
        </LinkableHeading>
        <BodyLong>Hookene kjører som en plugin nav-pilot legger inn ved oppstart. Dette dekker de ikke:</BodyLong>
        <Bullets>
          <li>{c("--pure")}: opencode laster ingen plugins, så ingen hooks kjører. nav-pilot sier fra ved oppstart.</li>
          <li>Vedlegg som bilder og PDF-er går til modellen uten maskering. Maskeringen virker på tekst.</li>
          <li>
            Sjekken mot MCP-registeret dekker ikke servere du legger til midt i økten, konfig fra{" "}
            {c(".well-known/opencode")} eller en organisasjon du er logget inn i, eller administrerte innstillinger på
            macOS.
          </li>
          <li>
            Plugins i repoet ({c(".opencode/plugin")} og {c("plugin")} i repoets {c("opencode.json")}) er kode som
            kjører med agentens rettigheter, i sandkassen. I opencode 2 kan en slik plugin også slå av nav-pilots hooks.
            Stoler du ikke på repoet, start med {c("OPENCODE_DISABLE_PROJECT_CONFIG=1")}. Da laster opencode verken
            plugins, MCP-servere eller konfig fra repoet.
          </li>
        </Bullets>
        <BodyShort size="small" textColor="subtle">
          Resten, blant annet kodemodus og hva terminalen viser, står i{" "}
          <a href={HOOKS_DOC} className={linkClass}>
            opencode-hooks.md
          </a>
          .
        </BodyShort>
      </Section>
    </DocPage>
  );
}
