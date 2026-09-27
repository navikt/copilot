import { BodyLong, BodyShort, Box, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import type { ReactNode } from "react";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import { REFERENCE_PAGES } from "@/components/nav-pilot/doc-pages";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Klienter — nav-pilot",
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
const OPENCODE_RANGE = ">=1.18.20,<1.19"; // OpenCodeTestedRange in cli/nav-pilot/internal/provider/opencode_policy.go
const c = (s: string) => <code className={code}>{s}</code>;

const CLIENTS = [
  {
    name: "copilot",
    tag: { text: "Standard", variant: "info" as const },
    desc: "GitHub Copilot CLI, i cplt når den finnes. Agentpakka virker også i VS Code, JetBrains og på github.com.",
  },
  {
    name: "opencode",
    tag: { text: "Støttet", variant: "success" as const },
    desc: "opencode med Copilot-abonnementet ditt. Den eneste klienten der hovedagenten kan kjøre i skyen og sende jobber til en lokal modell.",
  },
  {
    name: "pi",
    tag: { text: "Eksperimentell", variant: "warning" as const },
    desc: "pi i cplt, med skills, agenter og AGENTS.md lagt inn ved oppstart. Mangler blant annet hooks.",
  },
];

// From the parity status in navikt/copilot#1022 (comment 5858353821), with pi
// added from pi_launch.go. Keep in step with #1022.
const PARITY: { what: ReactNode; copilot: string; opencode: string; pi: string; note?: ReactNode }[] = [
  {
    what: "Maskering av hemmeligheter og fødselsnumre",
    copilot: "Ja",
    opencode: "Ja",
    pi: "Nei",
    note: <>I opencode holdes verktøyresultatet tilbake hvis maskeringen feiler. Ikke med {c("--pure")}.</>,
  },
  { what: "Løkkevakt", copilot: "Ja", opencode: "Ja", pi: "Nei" },
  {
    what: "Gates du har installert, også agentpakke-hooks",
    copilot: "Ja",
    opencode: "Ja",
    pi: "Nei",
    note: "Gates fra repoet kjører i opencode uten at du først har godtatt mappen, slik Copilot krever.",
  },
  {
    what: <>{c("tools:")} i agentene</>,
    copilot: "Ja",
    opencode: "Ja, som permission per agent",
    pi: "Nei",
    note: <>MCP-verktøy og {c("task")} blir ikke oversatt.</>,
  },
  {
    what: "Navs MCP-register",
    copilot: "Ja, GitHub håndhever",
    opencode: "Ja, nav-pilot håndhever",
    pi: "Nei",
    note: "Servere utenfor registeret slås av, og verktøyene deres blir avvist.",
  },
  {
    what: "Deling av, oppdatering som varsel",
    copilot: "Ikke aktuelt",
    opencode: "Ja, i hver økt",
    pi: "Ikke aktuelt",
  },
  { what: "Testet versjon, med varsel og doctor", copilot: "–", opencode: OPENCODE_RANGE, pi: "–" },
  {
    what: <>Uten cplt ({c("--no-sandbox")}, CI)</>,
    copilot: "Ja",
    opencode: "Ja",
    pi: "Nei",
    note: "Tier 2 krever fortsatt cplt.",
  },
  {
    what: <>Skyorkestrator med {c("local-worker")}</>,
    copilot: "Nei",
    opencode: "Ja",
    pi: "Nei",
    note: "Copilot CLI kjører en økt helt lokalt eller helt i skyen, aldri blandet.",
  },
  { what: <>Stopp for {c("local_dispatch")} (balanced, aggressive)</>, copilot: "Nei", opencode: "Ja", pi: "Nei" },
  { what: <>Egen server ({c("local_endpoint")})</>, copilot: "Ja, hele økten", opencode: "Ja", pi: "Nei" },
  {
    what: (
      <>
        {c("autopilot")}, {c("context_tier")}, {c("ask_user")}
      </>
    ),
    copilot: "Ja",
    opencode: "Nei",
    pi: "Nei",
    note: "nav-pilot advarer ved oppstart.",
  },
  { what: "Utvidelser (extensions) for Copilot CLI", copilot: "Ja", opencode: "Nei", pi: "Nei" },
  { what: "Veiledning for WSL2", copilot: "Ja", opencode: "Ja", pi: "–" },
];

const MCP_WARNING = `⚠ MCP servers turned off for this session (not in Nav's MCP registry): <navn>. See ki-utvikling.nav.no/verktoy (approved servers); to add one: github.com/navikt/copilot/blob/main/apps/mcp-registry/README.md#adding-servers`;

function Section({ children }: { children: ReactNode }) {
  return (
    <section>
      <VStack gap="space-16">{children}</VStack>
    </section>
  );
}

const cols = ["Funksjon", "Copilot CLI", "opencode", "pi", "Merknad"];

export default function Klienter() {
  return (
    <DocPage
      label="Referanse"
      title="Klienter"
      description="Hva Copilot CLI, opencode og pi kan når nav-pilot starter dem, og hva som mangler. Søk på siden med Ctrl+F."
      toc={TOC}
      wide
      siblings={{ pages: REFERENCE_PAGES, current: "/nav-pilot/klienter" }}
    >
      <Section>
        <LinkableHeading id="stotte-klienter" size="medium" level="2">
          Klientene
        </LinkableHeading>
        <BodyLong>
          En klient er programmet nav-pilot starter. Standard er Copilot CLI. Velg klient for én økt med{" "}
          {c("--client opencode")}, eller for godt med {c("nav-pilot config set client opencode")}.
        </BodyLong>
        <div className="overflow-x-auto">
          <Table size="small" className="table-stack" role="table">
            <TableHeader role="rowgroup">
              <TableRow role="row">
                {["Klient", "Status", "Hva du får"].map((h) => (
                  <TableHeaderCell key={h} scope="col" role="columnheader">
                    {h}
                  </TableHeaderCell>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody role="rowgroup">
              {CLIENTS.map((k) => (
                <TableRow role="row" key={k.name}>
                  <TableDataCell role="cell">{c(k.name)}</TableDataCell>
                  <TableDataCell role="cell">
                    <Tag size="small" variant={k.tag.variant}>
                      {k.tag.text}
                    </Tag>
                  </TableDataCell>
                  <TableDataCell role="cell">{k.desc}</TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </Section>

      <Section>
        <LinkableHeading id="paritet" size="medium" level="2">
          Hva hver klient kan
        </LinkableHeading>
        <BodyLong>
          Tabellen viser status fra paritetsarbeidet i{" "}
          <a href={`${GH}/issues/1022`} className={linkClass}>
            #1022
          </a>
          , som skal gjøre opencode like trygg som Copilot CLI.
        </BodyLong>
        <div className="overflow-x-auto">
          <Table size="small" className="table-stack" role="table">
            <TableHeader role="rowgroup">
              <TableRow role="row">
                {cols.map((h) => (
                  <TableHeaderCell key={h} scope="col" role="columnheader">
                    {h}
                  </TableHeaderCell>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody role="rowgroup">
              {PARITY.map((r, i) => (
                <TableRow role="row" key={i}>
                  <TableDataCell role="cell">
                    <strong>{r.what}</strong>
                  </TableDataCell>
                  {[r.copilot, r.opencode, r.pi].map((v, j) => (
                    <TableDataCell role="cell" key={j} data-label={cols[j + 1]}>
                      {v}
                    </TableDataCell>
                  ))}
                  <TableDataCell role="cell" data-label={r.note ? "Merknad" : undefined}>
                    {r.note}
                  </TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        <Box background="warning-soft" borderRadius="8" padding="space-16">
          <BodyShort>
            En økt uten nav-pilots hooks har verken løkkevakt eller maskering av hemmeligheter og fødselsnumre. Det
            gjelder alltid pi, og opencode når du starter den med {c("--pure")} eller uten nav-pilot.
          </BodyShort>
        </Box>
      </Section>

      <Section>
        <LinkableHeading id="mcp-register" size="medium" level="2">
          Navs MCP-register
        </LinkableHeading>
        <BodyLong>
          Nav tillater bare MCP-servere som står i Navs MCP-register. Copilot CLI spør GitHub om policyen og kjører bare
          serverne registeret lister. opencode spør ingen, så nav-pilot gjør det samme når den starter opencode:
        </BodyLong>
        <Bullets>
          <li>
            nav-pilot leser MCP-serverne i opencode-konfigen din, i samme rekkefølge som opencode: den globale fila i{" "}
            {c("~/.config/opencode/")}, fila {c("OPENCODE_CONFIG")} peker på, {c("opencode.json")} og{" "}
            {c(".opencode/opencode.json")} fra roten av repoet ned til katalogen du står i, {c("~/.opencode/")},{" "}
            {c("OPENCODE_CONFIG_DIR")} og {c("OPENCODE_CONFIG_CONTENT")}.
          </li>
          <li>
            Den spør GitHub, som deg ({c("gh api /copilot/mcp_registry")}), hvilket register policyen peker på, og
            henter lista derfra.
          </li>
          <li>
            Hver server som er på og ikke står i registeret, slås av for økten med {c('"enabled": false')}. En ekstern
            server må ha samme URL som i registeret. En lokal server må starte en pakke registeret lister, for eksempel{" "}
            {c("npx @playwright/mcp")}.
          </li>
          <li>Kobler du til en avslått server med {c("/mcp")} i økten, blir verktøyene dens avvist likevel.</li>
        </Bullets>
        <BodyLong>
          nav-pilot endrer ikke {c("opencode.json")}, og opencode du starter uten nav-pilot, blir ikke berørt. Ved
          oppstart ser du hvilke servere som ble slått av:
        </BodyLong>
        <CodeBlock compact>{MCP_WARNING}</CodeBlock>
        <BodyLong>
          {c("nav-pilot doctor")} viser det samme før du starter. Har du ingen MCP-servere, spør nav-pilot ikke nettet.
          Mangler {c("gh")}, eller svarer ikke GitHub eller registeret, slår nav-pilot ingenting av. Da får du en
          advarsel, og serverne kjører som du har satt dem opp.
        </BodyLong>
        <LinkableHeading id="legg-til-server" size="small" level="3">
          Få en server inn i registeret
        </LinkableHeading>
        <BodyLong>
          Godkjente servere står i{" "}
          <NextLink href="/verktoy" className={linkClass}>
            verktøykatalogen
          </NextLink>
          . Mangler serveren du trenger, legger du den til i {c("apps/mcp-registry/allowlist.json")} og lager en pull
          request. Den må gjennom en sikkerhetsgjennomgang. Hvordan du skriver oppføringen, står i{" "}
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
          cplt er sandkassen som holder agenten unna SSH-nøkler, skytilganger og andre hemmeligheter. Hvorfor den er
          påkrevd på Nav-utstyr, står i{" "}
          <NextLink href="/nav-pilot/forklaring/sandkassen" className={linkClass}>
            Sandkassen
          </NextLink>
          . Mangler cplt:
        </BodyLong>
        <Bullets>
          <li>
            Copilot CLI og opencode: nav-pilot spør i terminalen om klienten skal starte uten sandkasse. Standardsvaret
            er nei. Med {c("--no-sandbox")} starter den uten å spørre, med én advarsel. Uten terminal, for eksempel i
            CI, starter den bare med {c("--no-sandbox")}.
          </li>
          <li>pi starter ikke.</li>
          <li>
            En agentpakke i Tier 2 starter ikke, uansett klient. Tier 2 vil si at pakka har ferdigbygde filer som
            nav-pilot sjekker mot en digest og låser per bruker, se{" "}
            <NextLink href="/nav-pilot/agentpakker#hvilken-tier" className={linkClass}>
              Hvilken tier
            </NextLink>
            .
          </li>
        </Bullets>
      </Section>

      <Section>
        <LinkableHeading id="opencode" size="medium" level="2">
          opencode
        </LinkableHeading>
        <Bullets>
          <li>
            nav-pilot legger AGENTS.md, skills, kommandoer og agenter i {c("~/.config/opencode/")} og oppdaterer dem ved
            hver oppstart. Endrer du en av filene selv, lar nav-pilot den være.{" "}
            {c("~/.config/opencode/.nav-pilot-state.json")} holder rede på hva som er installert.
          </li>
          <li>
            Velger du ikke modell selv, bruker nav-pilot standarden agentpakka oppgir. For agentpakka nav-pilot er det
            GPT-6 Sol. Oppgir pakka ingen, velger opencode. En Copilot-id uten prefiks får {c("github-copilot/")} foran.
          </li>
          <li>nav-pilot setter opp OpenTelemetry for opencode, med mindre du har slått av telemetri.</li>
          <li>
            {c("nav-pilot export opencode")} skriver agentpakka til {c(".opencode/")} i repoet. Du trenger det ikke for
            å bruke opencode.
          </li>
        </Bullets>
        <LinkableHeading id="testet-versjon" size="small" level="3">
          Testet versjon
        </LinkableHeading>
        <BodyLong>
          nav-pilot er testet mot opencode {c(OPENCODE_RANGE)}. Er versjonen din utenfor, skriver nav-pilot en advarsel
          og starter likevel. Hooks, stoppet for utsending og øktpolicyen virker da kanskje ikke som beskrevet her.{" "}
          {c("nav-pilot doctor")} viser versjonen og om den er testet.
        </BodyLong>
        <LinkableHeading id="deling-og-oppdatering" size="small" level="3">
          Deling og oppdatering
        </LinkableHeading>
        <BodyLong>
          Hver økt nav-pilot starter, får {c('"share": "disabled"')} og {c('"autoupdate": "notify"')}, uansett hva{" "}
          {c("opencode.json")} sier. Deling ville lastet opp økten til opencode.ai. Oppdateringer kommer som varsel, så
          du ikke får en versjon utenfor det testede området midt i en økt. I tillegg setter nav-pilot{" "}
          {c('"share": "disabled"')} i {c("~/.config/opencode/opencode.json")} når fila ikke sier noe om deling, så det
          gjelder også når du starter opencode selv. Står det {c('"auto"')} der, får du en advarsel.
        </BodyLong>
        <LinkableHeading id="utsending" size="small" level="3">
          Lokal utsending
        </LinkableHeading>
        <BodyLong>
          Bare i opencode kan hovedagenten kjøre i skyen og sende avgrensede jobber til {c("local-worker")}, en
          underagent på den lokale modellen. Hvor mye som sendes, styrer du med {c("local_dispatch")}. På{" "}
          {c("balanced")} og {c("aggressive")} stopper nav-pilot hovedagenten når den redigerer for mange filer selv. Se{" "}
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
          {c("mode = autopilot")}, {c("context_tier")}, {c("ask_user")} og utvidelser (extensions) finnes bare i Copilot
          CLI. Har du satt dem og bruker opencode, skriver nav-pilot én advarsel. En lokal modell tar hele økten, se{" "}
          <NextLink href="/nav-pilot/lokal" className={linkClass}>
            Lokal modell på Mac
          </NextLink>
          .
        </BodyLong>
        <Box background="info-soft" borderRadius="8" padding="space-16">
          <BodyShort size="small">
            For Copilot CLI henter nav-pilot ikke GitHub-tokenet selv. Er gh-vakta i cplt på, henter cplt det fra{" "}
            {c("GH_TOKEN")}, {c("GITHUB_TOKEN")}, {c("COPILOT_GITHUB_TOKEN")} eller {c("gh auth token")}.{" "}
            {c("copilot_auth_mode")} bestemmer hvilke kilder som slipper gjennom: {c("env_only")} stopper oppstarten
            uten token i miljøet, og {c("gh_only")} fjerner token-variablene.
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
          {c("~/.nav-pilot/pi/")} og gir skills og AGENTS.md til pi som flagg. Dette mangler:
        </BodyLong>
        <Bullets>
          <li>Hooks: ingen maskering, ingen løkkevakt og ingen gates.</li>
          <li>
            {c("--agent")}: pi har ikke agenter. Personaen blir en del av systemprompten, og {c("tools:")} gjelder ikke.
          </li>
          <li>
            Lokal modell: ingen utsending til {c("local-worker")}, ingen {c("local_dispatch")} og ingen{" "}
            {c("local_endpoint")}.
          </li>
          <li>Sjekk mot Navs MCP-register.</li>
          <li>Start uten cplt. pi krever både pi og cplt.</li>
          <li>
            Innstillinger som ikke sendes videre, med en advarsel: {c("mode")}, {c("reasoning_effort")},{" "}
            {c("context_tier")}, {c("allow_all_tools")}, {c("ask_user")} og {c("log_level")}.
          </li>
        </Bullets>
      </Section>

      <Section>
        <LinkableHeading id="kjente-hull" size="medium" level="2">
          Kjente hull i opencode
        </LinkableHeading>
        <BodyLong>
          Hookene kjører i opencode som en plugin nav-pilot legger inn ved oppstart. Dette dekker de ikke:
        </BodyLong>
        <Bullets>
          <li>{c("--pure")}: opencode laster ingen plugins, så ingen hooks kjører. nav-pilot sier fra ved oppstart.</li>
          <li>Vedlegg som bilder og PDF-er går til modellen uten maskering. Maskeringen virker på tekst.</li>
          <li>
            Kodemodus ({c("experimentalCodeMode")}): et program ser deler av et MCP-svar før maskeringen. Det programmet
            returnerer, blir maskert.
          </li>
          <li>Terminalen viser deg utdata uten maskering, som i Copilot. Maskeringen gjelder det modellen leser.</li>
          <li>Økten kan skrive i mappen der hookene holder rede på tilstand, som i Copilot.</li>
          <li>
            MCP-servere lagt til midt i en økt, konfig fra en {c(".well-known/opencode")}-adresse eller en organisasjon
            du har logget inn i, og administrerte innstillinger på macOS blir ikke sjekket mot registeret.
          </li>
        </Bullets>
        <BodyShort size="small" textColor="subtle">
          Detaljene står i{" "}
          <a href={`${GH}/blob/main/cli/nav-pilot/docs/opencode-hooks.md`} className={linkClass}>
            opencode-hooks.md
          </a>
          .
        </BodyShort>
      </Section>
    </DocPage>
  );
}
