import { BodyLong, BodyShort, Box, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import type { ReactNode } from "react";
import { Table, TableBody, TableRow, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, HeaderRow, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

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
const OPENCODE_RANGE = ">=1.18.20,<1.19"; // OpenCodeTestedRange in cli/nav-pilot/internal/provider/opencode_policy.go
const c = (s: string) => <code className={code}>{s}</code>;

const CLIENTS = [
  {
    name: "opencode",
    tag: { text: "Standard for nye installasjoner", variant: "info" as const },
    desc: "opencode med Copilot-abonnementet ditt. Den eneste klienten der hovedagenten kan kjøre i skyen og sende jobber til en lokal modell.",
  },
  {
    name: "copilot",
    tag: { text: "Støttet", variant: "success" as const },
    desc: "GitHub Copilot CLI, i sandkassen cplt når den er installert. Agentpakka virker også i VS Code, JetBrains og på github.com.",
  },
  {
    name: "pi",
    tag: { text: "Eksperimentell", variant: "warning" as const },
    desc: "pi i cplt, med skills, persona og AGENTS.md lagt inn ved oppstart. Mangler blant annet hooks.",
  },
];

// The final parity status of navikt/copilot#1022 (posted on #1037), as of
// 2026-09-28 after #1038, #1039, #1057, #1061, #1062, #1113, #1114 and #1136,
// with pi added from pi_launch.go.
const PARITY: { what: ReactNode; copilot: string; opencode: string; pi: string; note?: ReactNode }[] = [
  {
    what: "Maskering av hemmeligheter og fødselsnumre",
    copilot: "Ja",
    opencode: "Ja",
    pi: "Nei",
    note: (
      <>
        Feiler maskeringen i opencode, holder nav-pilot verktøyresultatet tilbake. Med {c("--pure")} kjører ingen hooks.
      </>
    ),
  },
  { what: "Merknad om instruksjoner i verktøyresultater (prompt-injeksjon)", copilot: "Ja", opencode: "Ja", pi: "Nei" },
  { what: "Løkkevakt i skyøkter", copilot: "Ja", opencode: "Ja", pi: "Nei" },
  {
    what: "Gates du har installert, også agentpakke-hooks",
    copilot: "Ja",
    opencode: "Ja",
    pi: "Nei",
    note: "Gates fra repoet kjører i opencode uten at du først har godtatt mappa, slik Copilot CLI krever.",
  },
  {
    what: <>{c("tools:")} i agentene</>,
    copilot: "Ja",
    opencode: "Ja, som permission per agent",
    pi: "Nei",
    note: <>nav-pilot oversetter ikke MCP-verktøy og {c("task")}.</>,
  },
  {
    what: "Navs MCP-register",
    copilot: "Ja, GitHub håndhever",
    opencode: "Ja, nav-pilot håndhever",
    pi: "Nei",
    note: "nav-pilot slår av servere utenfor registeret og avviser verktøyene deres.",
  },
  {
    what: "Testet klientversjon",
    copilot: "Ikke aktuelt",
    opencode: `Ja, ${OPENCODE_RANGE}`,
    pi: "Nei",
    note: <>Utenfor dette området får du en advarsel ved oppstart og i {c("nav-pilot doctor")}.</>,
  },
  {
    what: "Deling slått av, oppdateringer som varsel",
    copilot: "Ikke aktuelt",
    opencode: "Ja, i hver økt",
    pi: "Ikke aktuelt",
  },
  {
    what: <>Uten cplt ({c("--no-sandbox")}, CI)</>,
    copilot: "Ja",
    opencode: "Ja",
    pi: "Nei",
    note: "Tier 2 krever fortsatt cplt.",
  },
  { what: "Installasjon i WSL2 på Windows", copilot: "Ja", opencode: "Ja", pi: "Nei" },
  {
    what: <>Skyorkestrator med {c("local-worker")}</>,
    copilot: "Nei",
    opencode: "Ja",
    pi: "Nei",
    note: "Copilot CLI kjører en økt helt lokalt eller helt i skyen, aldri blandet.",
  },
  {
    what: <>Stopper hovedagenten som redigerer selv ({c("local_dispatch")} balanced og aggressive)</>,
    copilot: "Nei",
    opencode: "Ja",
    pi: "Nei",
    note: (
      <>Teller kallsteder, ikke bare filer, og ber hovedagenten bygge og teste etter en jobb fra {c("local-worker")}.</>
    ),
  },
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
    note: "nav-pilot advarer ved oppstart, én advarsel per innstilling.",
  },
  { what: "Utvidelser (extensions) for Copilot CLI", copilot: "Ja", opencode: "Nei", pi: "Nei" },
];

const MCP_WARNING = `⚠ MCP servers turned off for this session (not in Nav's MCP registry): <navn>. See https://ki-utvikling.nav.no/verktoy (approved servers); to add one: https://github.com/navikt/copilot/blob/main/apps/mcp-registry/README.md#adding-servers`;

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
      description="Hva Copilot CLI, opencode og pi kan når nav-pilot starter dem, og hva som mangler. Søk på siden med Ctrl+F (Cmd+F på Mac)."
      toc={TOC}
      wide
    >
      <Section>
        <LinkableHeading id="stotte-klienter" size="medium" level="2">
          Klientene
        </LinkableHeading>
        <BodyLong>
          En klient er programmet nav-pilot starter. På en ny installasjon er standarden opencode. Velg klient for én
          økt med {c("--client copilot")}, eller for godt med {c("nav-pilot config set client copilot")}. Copilot CLI er
          fortsatt fullt støttet.
        </BodyLong>
        <BodyLong>
          Klienten du bruker, står i {c("~/.nav-pilot/config.toml")}. Har du brukt nav-pilot før, beholder du klienten
          du har: en {c("config.toml")} uten {c("client")} betyr copilot, og nav-pilot skriver linja inn første gang du
          starter en økt i en terminal.
        </BodyLong>
        <BodyLong>
          Første gang du kjører nav-pilot, spør den hvilken klient du vil ha, med opencode valgt. Mangler opencode, og
          du har Homebrew, tilbyr nav-pilot å installere den med {c("brew install anomalyco/tap/opencode")}. Uten
          Homebrew, eller om du sier nei, bruker nav-pilot Copilot CLI og forteller hvordan du bytter senere. I CI og
          uten terminal starter nav-pilot Copilot CLI når {c("config.toml")} mangler, siden den ikke kan se om du har
          brukt nav-pilot før. Vil du ha opencode i CI, bruk {c("--client opencode")}.
        </BodyLong>
        <BodyLong>
          Bruker du Copilot CLI med lokale modeller på og har opencode installert, viser nav-pilot én gang et tips om
          opencode: bare opencode lar en skymodell sende oppgaver til en lokal modell. Tipset kommer aldri i samme økt
          som en brukerundersøkelse, og {c("nav-pilot config set surveys false")} slår det av.
        </BodyLong>
        <div className="overflow-x-auto">
          <Table size="small" className="table-stack" role="table">
            <HeaderRow stack cells={["Klient", "Status", "Hva du får"]} />
            <TableBody role="rowgroup">
              {CLIENTS.map((k) => (
                <TableRow role="row" key={k.name}>
                  <TableDataCell role="cell">{c(k.name)}</TableDataCell>
                  <TableDataCell role="cell" data-label="Status">
                    <Tag size="small" variant={k.tag.variant}>
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
      </Section>

      <Section>
        <LinkableHeading id="paritet" size="medium" level="2">
          Hva hver klient kan
        </LinkableHeading>
        <BodyLong>
          Tabellen viser sluttstatus per 28. september 2026 for paritetsarbeidet i{" "}
          <a href={`${GH}/issues/1022`} className={linkClass}>
            #1022
          </a>
          , som skulle gjøre opencode like trygg som Copilot CLI. Det som gjenstår i opencode, står under{" "}
          <a href="#kjente-hull" className={linkClass}>
            Kjente hull i opencode
          </a>
          .
        </BodyLong>
        <div className="overflow-x-auto">
          <Table size="small" className="table-stack" role="table">
            <HeaderRow stack cells={cols} />
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
          MCP-servere gir agenten verktøy utenfor klienten, for eksempel Playwright eller GitHub. Nav tillater bare
          servere som står i Navs MCP-register, lista over godkjente servere (se{" "}
          <NextLink href="/verktoy?type=mcp" className={linkClass}>
            verktøykatalogen
          </NextLink>
          ). Copilot CLI spør GitHub om policyen og kjører bare serverne registeret lister. opencode har ingen slik
          sjekk, så nav-pilot gjør den når den starter opencode:
        </BodyLong>
        <Bullets>
          <li>
            nav-pilot leser MCP-serverne i opencode-konfigen din, i samme rekkefølge som opencode: den globale fila i{" "}
            {c("~/.config/opencode/")}, fila {c("OPENCODE_CONFIG")} peker på, {c("opencode.json")} og{" "}
            {c(".opencode/opencode.json")} fra roten av repoet ned til katalogen du står i, {c("~/.opencode/")},{" "}
            {c("OPENCODE_CONFIG_DIR")} og {c("OPENCODE_CONFIG_CONTENT")}.
          </li>
          <li>
            Den spør GitHub med din gh-innlogging ({c("gh api /copilot/mcp_registry")}) hvilket register policyen peker
            på, og henter lista derfra.
          </li>
          <li>
            Hver server som er på og ikke står i registeret, slås av for økten med {c('"enabled": false')}. En ekstern
            server må ha samme URL som i registeret. En lokal server må starte en pakke registeret lister, for eksempel{" "}
            {c("npx @playwright/mcp")}.
          </li>
          <li>
            Kobler du til en avslått server med {c("/mcp")} i økten, blir verktøyene dens avvist likevel. Det gjør
            hooks-pluginen, så med {c("--pure")} kan serveren kobles til og brukes. nav-pilot advarer om det ved
            oppstart.
          </li>
        </Bullets>
        <BodyLong>
          MCP-oppføringene i {c("opencode.json")} rører nav-pilot ikke, og opencode du starter uten nav-pilot, blir ikke
          berørt. Ved oppstart ser du hvilke servere som ble slått av:
        </BodyLong>
        {/* The URLs have no break points; let them wrap on a phone. */}
        <div className="[&_pre]:[overflow-wrap:anywhere]">
          <CodeBlock compact>{MCP_WARNING}</CodeBlock>
        </div>
        <BodyLong>
          {c("nav-pilot doctor")} viser det samme før du starter. Har du ingen MCP-servere, gjør nav-pilot ingen
          nettkall. Mangler {c("gh")}, eller svarer ikke GitHub eller registeret, slår nav-pilot ingenting av. Da får du
          en advarsel, og serverne kjører som du har satt dem opp.
        </BodyLong>
        <LinkableHeading id="legg-til-server" size="small" level="3">
          Få en server inn i registeret
        </LinkableHeading>
        <BodyLong>
          Godkjente servere står i{" "}
          <NextLink href="/verktoy" className={linkClass}>
            verktøykatalogen
          </NextLink>
          . Mangler serveren du trenger, legger du den til i {c("apps/mcp-registry/allowlist.json")} i navikt/copilot og
          lager en pull request. Den må gjennom en sikkerhetsgjennomgang. Hvordan du skriver oppføringen, står i{" "}
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
            nav-pilot sjekker mot en digest og låser den som én revisjon per bruker, se{" "}
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
          og starter likevel. Hooks, utsendingsstoppet og innstillingene nav-pilot setter per økt, virker da kanskje
          ikke som beskrevet her. {c("nav-pilot doctor")} viser versjonen og om den er testet.
        </BodyLong>
        <LinkableHeading id="deling-og-oppdatering" size="small" level="3">
          Deling og oppdatering
        </LinkableHeading>
        <BodyLong>
          Hver økt nav-pilot starter, får {c('"share": "disabled"')} og {c('"autoupdate": "notify"')}, uansett hva{" "}
          {c("opencode.json")} sier. Deling ville lastet opp økten til opencode.ai. Oppdateringer kommer som varsel, så
          du ikke får en versjon utenfor det testede området midt i en økt. I tillegg setter nav-pilot{" "}
          {c('"share": "disabled"')} i {c("~/.config/opencode/opencode.json")} når fila ikke sier noe om deling, så det
          gjelder også når du starter opencode selv. Det skjer når nav-pilot starter en agentpakke i Tier 1 og i
          oppsettet, ikke med Tier 2, som ikke rører {c("opencode.json")}. Står det {c('"auto"')} der, får du en
          advarsel. Har {c("opencode.json")} kommentarer, lar nav-pilot fila være, siden kommentarene ville forsvunnet
          ved omskriving. Innstillingene nav-pilot trenger, gjelder da bare øktene nav-pilot starter. Må noe ut av fila,
          for eksempel ved {c("nav-pilot alpha local off")}, sier nav-pilot hva du må fjerne selv.
        </BodyLong>
        <BodyLong>
          Instruksjonene, agentene og skillene nav-pilot installerer, ligger i {c("~/.config/opencode/")}, utenfor
          prosjektet. Økter nav-pilot starter, får lese dem uten å spørre om {c("external_directory")}, men ikke endre
          dem: det er jobben til {c("nav-pilot sync")}. Uten dette må opencode spørre når modellen åpner en av dem, og{" "}
          {c("opencode run")} svarer nei og avslutter økten. For andre kataloger utenfor prosjektet gjelder det du har
          satt selv. Har du avslått {c("external_directory")} helt, med {c('"deny"')}, legger nav-pilot ikke til noe.
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
          CLI. Har du satt en av de tre innstillingene og bruker opencode, skriver nav-pilot én advarsel per
          innstilling. For utvidelser kommer ingen advarsel. I Copilot CLI tar en lokal modell hele økten, uten
          utsending, se{" "}
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
            {c("--agent")}: pi kan ikke bytte agent. Personaen blir en del av systemprompten, og {c("tools:")} gjelder
            ikke.
          </li>
          <li>
            Lokal modell: ingen utsending til {c("local-worker")}, ingen {c("local_dispatch")} og ingen{" "}
            {c("local_endpoint")}.
          </li>
          <li>Sjekk mot Navs MCP-register.</li>
          <li>Start uten cplt. pi starter bare i sandkassen.</li>
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
            nav-pilot sjekker ikke disse mot registeret: en server du legger til midt i økten og som ikke sto i konfigen
            ved oppstart, konfig fra en {c(".well-known/opencode")}-adresse eller en organisasjon du har logget inn i,
            og administrerte innstillinger på macOS.
          </li>
        </Bullets>
        <BodyShort size="small" textColor="subtle">
          Resten, blant annet kodemodus og hva terminalen viser, står i{" "}
          <a href={`${GH}/blob/main/cli/nav-pilot/docs/opencode-hooks.md`} className={linkClass}>
            opencode-hooks.md
          </a>
          .
        </BodyShort>
      </Section>
    </DocPage>
  );
}
