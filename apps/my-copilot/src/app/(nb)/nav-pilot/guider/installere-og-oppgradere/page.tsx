import { BodyLong, BodyShort, Box, Heading, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
import { AltInstall } from "@/components/alt-install";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import { GUIDE_PAGES } from "@/components/nav-pilot/doc-pages";
import type { TocItem } from "@/components/table-of-contents";
import { NAV_PILOT_BREW_UPGRADE } from "@/lib/install-commands";

export const metadata: Metadata = {
  title: "Installere og oppgradere — nav-pilot",
  description:
    "Velg hvor agentpakka skal ligge, installer i CI, oppgrader nav-pilot og fjern alt nav-pilot har lagt på maskinen.",
};

const TOC: TocItem[] = [
  { id: "velg-installasjonssted", label: "Velg installasjonssted" },
  { id: "vanlige-oppgaver", label: "Vanlige oppgaver" },
  { id: "installere-i-ci", label: "Installere i CI" },
  { id: "oppgradere", label: "Oppgradere" },
  { id: "avinstallere", label: "Avinstallere" },
];

const TASKS = [
  { task: "Bygge ny tjeneste", prompt: "Jeg trenger en ny tjeneste for dagpenger" },
  { task: "Legge til autentisering", prompt: "Legg til TokenX-validering i API-et" },
  { task: "Feilsøke en deploy", prompt: "Poden min krasjer i dev, hjelp meg feilsøke" },
  { task: "Gjennomgå før PR", prompt: "Gjør en sikkerhetsgjennomgang av disse endringene" },
  { task: "Sette opp Kafka", prompt: "Vi trenger en Kafka-consumer for vedtakshendelser" },
  { task: "Legge til observerbarhet", prompt: "Sett opp metrikker og tracing for tjenesten" },
  { task: "Migrere Java til Kotlin", prompt: "Hjelp meg migrere denne klassen til Kotlin" },
  { task: "Få kortere svar", prompt: "$terse-mode" },
  { task: "Planlegge arkitektur", prompt: "Planlegg arkitekturen for nytt saksbehandlersystem" },
];

const UNINSTALL = `nav-pilot alpha local stop              # bare hvis du har brukt lokal modell
nav-pilot alpha local purge --all --yes # Python-miljøet og alle nedlastede vekter
nav-pilot uninstall                     # i hvert repo der du installerte med --repo
nav-pilot uninstall --user              # ~/.copilot
brew uninstall nav-pilot                # eller: sudo apt remove nav-pilot
rm -rf ~/.nav-pilot                     # konfig, cache og lokale data`;

export default function InstallereOgOppgradere() {
  return (
    <DocPage
      label="Guider"
      title="Installere og oppgradere"
      description="Har du ikke installert nav-pilot ennå, start med Kom i gang. Her står det du trenger etterpå."
      toc={TOC}
      siblings={{ pages: GUIDE_PAGES, current: "/nav-pilot/guider/installere-og-oppgradere" }}
    >
      <BodyLong>
        Alle kommandoene og flaggene står i{" "}
        <NextLink href="/nav-pilot/referanse#kommandoer" className={linkClass}>
          referansen
        </NextLink>
        .
      </BodyLong>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="velg-installasjonssted" size="medium" level="2">
            Velg installasjonssted
          </LinkableHeading>
          <BodyLong>
            <code className={code}>install</code> spør hvor agentpakka skal ligge, hvis du ikke svarer på forhånd med{" "}
            <code className={code}>--repo</code> eller <code className={code}>--user</code>. Tre former er i bruk i Nav:
          </BodyLong>
          <Bullets>
            <li>
              <strong>I repoet</strong> (<code className={code}>--repo</code>, skriver til{" "}
              <code className={code}>.github/</code>). Hele teamet får det samme, prompts virker, og Copilot på
              github.com ser filene fordi de er sjekket inn. Til gjengjeld ligger de i repoet og i hver diff.
            </li>
            <li>
              <strong>Personlig</strong> (<code className={code}>--user</code>, skriver til{" "}
              <code className={code}>~/.copilot/</code>). Følger deg i alle repoer, og ingenting sjekkes inn. Du får
              ikke prompts, og verken github.com eller resten av teamet ser filene.
            </li>
            <li>
              <strong>Hub-repo.</strong> En repo-installasjon i et repo som ikke er en app, med teamets egne skills lagt
              for hånd i samme <code className={code}>.github/</code>. Konteksten gjelder mens du står i hub-repoet.
            </li>
          </Bullets>
          <BodyLong>
            Du kan bruke flere. <code className={code}>nav-pilot sync</code> uten flagg synkroniserer alle stedene som
            har en tilstandsfil. Hele avveiningen står i{" "}
            <a
              href="https://github.com/navikt/copilot/blob/main/docs/README.nav-pilot.md#hvor-skal-artefaktene-installeres"
              className={linkClass}
            >
              README.nav-pilot.md
            </a>
            .
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot install --dry-run nav-pilot            # se hva som installeres
nav-pilot install nav-pilot --repo               # i repoet
nav-pilot install --user                         # personlig
nav-pilot install --target /sti/til/repo nav-pilot   # i et annet repo
nav-pilot install --force nav-pilot              # overskriv filer du har endret (din kopi lagres som .orig)`}
          </CodeBlock>
          <BodyLong>
            Copilot finner agenter og skills i <code className={code}>~/.copilot/</code> selv. Instruksjonene krever{" "}
            <code className={code}>COPILOT_CUSTOM_INSTRUCTIONS_DIRS</code> og virker bare i Copilot CLI. nav-pilot
            setter variabelen når den starter Copilot. Starter du cplt selv, legg dette i skallprofilen:
          </BodyLong>
          <CodeBlock compact>{`eval "$(nav-pilot env)"`}</CodeBlock>
          <BodyShort size="small" textColor="subtle">
            opencode får Nav-konteksten på en annen måte, se{" "}
            <NextLink href="/nav-pilot/klienter#opencode" className={linkClass}>
              opencode
            </NextLink>
            .
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="vanlige-oppgaver" size="medium" level="2">
            Vanlige oppgaver
          </LinkableHeading>
          <BodyLong>
            Du trenger ikke huske navnet på skillene. Beskriv oppgaven, så henter nav-pilot kunnskapen som trengs. Du
            kan spørre fra tre steder:
          </BodyLong>
          <CodeBlock compact>
            {`# Terminalen: nav-pilot starter Copilot med agenten nav-pilot
nav-pilot

# Copilot CLI direkte, i sandkassen
cplt --project-dir . -- --agent nav-pilot --prompt "Jeg trenger en ny tjeneste som behandler dagpengesøknader"

# Copilot Chat i VS Code eller JetBrains
@nav-pilot Jeg trenger en ny tjeneste som behandler dagpengesøknader`}
          </CodeBlock>
          <div className="overflow-x-auto">
            <Table size="small">
              <TableHeader>
                <TableRow>
                  <TableHeaderCell scope="col">Oppgave</TableHeaderCell>
                  <TableHeaderCell scope="col">Eksempel</TableHeaderCell>
                </TableRow>
              </TableHeader>
              <TableBody>
                {TASKS.map((t) => (
                  <TableRow key={t.task}>
                    <TableDataCell>{t.task}</TableDataCell>
                    <TableDataCell>«{t.prompt}»</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="installere-i-ci" size="medium" level="2">
            Installere i CI
          </LinkableHeading>
          <BodyLong>
            Første gang du installerer en agentpakke i et repo, skriver nav-pilot{" "}
            <code className={code}>.nav-pilot/agentpakke.lock.json</code> med versjonen den installerte. Sjekk inn fila.
            I CI installerer du så nøyaktig den versjonen:
          </BodyLong>
          <CodeBlock compact>{`- run: nav-pilot install <navn> --frozen --force`}</CodeBlock>
          <BodyLong>Runneren trenger nav-pilot. Bruk installasjonsskriptet eller apt:</BodyLong>
          <AltInstall />
          <BodyLong>
            <code className={code}>--frozen</code> spør aldri og flytter aldri låsen. Kode 3 betyr at låsen ikke ble
            fulgt: fila mangler, den peker ikke på en versjon, en annen versjon kom inn, eller bare en del ble
            installert. Kode 1 betyr at noe annet feilet, for eksempel at kilden ikke svarte.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot install nav-pilot --repo --yes --json   # installer uten terminal, resultat som JSON
nav-pilot sync --json                             # kode 1: en oppdatering finnes
nav-pilot list --installed --json | jq .`}
          </CodeBlock>
          <BodyShort size="small" textColor="subtle">
            Uten terminal installerer <code className={code}>install</code> hele agentpakka bare med{" "}
            <code className={code}>--yes</code>, <code className={code}>--all</code> eller{" "}
            <code className={code}>--frozen</code>. Ellers avslutter den med kode 2 og sier hva den ville ha skrevet.
            Alle kodene står i{" "}
            <NextLink href="/nav-pilot/referanse#avslutningskoder" className={linkClass}>
              referansen
            </NextLink>
            .
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="oppgradere" size="medium" level="2">
            Oppgradere
          </LinkableHeading>
          <BodyLong>nav-pilot sjekker ved oppstart om det finnes en nyere versjon. Oppgrader med én av disse:</BodyLong>
          <CodeBlock compact>
            {`nav-pilot upgrade                            # selvoppdatering
${NAV_PILOT_BREW_UPGRADE}   # Homebrew
sudo apt update && sudo apt upgrade nav-pilot  # Debian og Ubuntu`}
          </CodeBlock>
          <BodyLong>
            <code className={code}>nav-pilot upgrade</code> spør ikke, og installerer alltid nyeste versjon. Har du
            installert med Homebrew eller apt, lar den binæren være og skriver kommandoen som virker.{" "}
            <code className={code}>--dry-run</code> sjekker bare. Vil du ha en bestemt versjon, bruk pakkebehandleren
            eller last den ned fra{" "}
            <a href="https://github.com/navikt/copilot/releases?q=nav-pilot" className={linkClass}>
              GitHub-releasene
            </a>
            .
          </BodyLong>
          <BodyLong>
            Med <code className={code}>auto_update = true</code> oppgraderer nav-pilot seg selv før andre kommandoer.
            Feiler det, får du en advarsel, kommandoen kjører på versjonen du har, og neste forsøk kommer etter 24
            timer. Slå det av med <code className={code}>nav-pilot config set auto_update false</code>.
          </BodyLong>
          <Box background="neutral-soft" padding="space-16" borderRadius="8">
            <VStack gap="space-8">
              <Heading size="xsmall" level="3">
                Homebrew sier «already installed», men versjonen er gammel
              </Heading>
              <BodyShort size="small">
                Tap-cachen er ikke oppdatert. Kjør <code className={code}>brew update</code> først, og så{" "}
                <code className={code}>brew upgrade navikt/tap/nav-pilot</code>. Feiler{" "}
                <code className={code}>brew update</code> med tilgangsfeil, kjør{" "}
                <code className={code}>brew doctor</code>. Den sier hvilke mapper som har feil eier, og hvordan du
                retter dem.
              </BodyShort>
            </VStack>
          </Box>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="avinstallere" size="medium" level="2">
            Avinstallere
          </LinkableHeading>
          <BodyLong>
            <code className={code}>uninstall</code> fjerner ett sted om gangen: repoet du står i, eller{" "}
            <code className={code}>~/.copilot</code> med <code className={code}>--user</code>. Den viser alt den
            fjerner, også tilstandsfila og låsen, og spør først. Filer du har endret, blir stående, og den sier hvilke.{" "}
            <code className={code}>--force</code> fjerner dem også. Skal alt bort, gjør du det i denne rekkefølgen:
          </BodyLong>
          <CodeBlock compact>{UNINSTALL}</CodeBlock>
          <BodyShort size="small" textColor="subtle">
            Vektene til de lokale modellene ligger i <code className={code}>~/.cache/huggingface</code>, eller der{" "}
            <code className={code}>HF_HOME</code> peker. <code className={code}>purge</code> tar dem, så kjør den før du
            fjerner nav-pilot. cplt kan du beholde. Vil du fjerne den også:{" "}
            <code className={code}>brew uninstall cplt</code>.
          </BodyShort>
        </VStack>
      </section>
    </DocPage>
  );
}
