import { BodyLong, Box, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Table, TableBody, TableRow, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, HeaderRow, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Sett opp cplt i et repo",
  description:
    "Første gang i et repo: la cplt init foreslå tilganger, se over forslaget, sjekk inn .cplt.toml og godkjenn det. Med det Go, Gradle, Next.js, pnpm, mise og Docker trenger.",
};

const TOC: TocItem[] = [
  { id: "forste-gang", label: "Første gang i et repo" },
  { id: "stakker", label: "Det stakken din trenger" },
  { id: "localhost", label: "Tjenester på localhost" },
  { id: "github-packages", label: "Pakker fra GitHub Packages" },
  { id: "tillatelsesliste", label: "Pakkeregistre og en liste over tillatte verter" },
  { id: "pnpm", label: "pnpm" },
  { id: "mise", label: "mise" },
  { id: "docker", label: "Docker" },
  { id: "hele-maskinen", label: "Brytere for hele maskinen" },
];

const FEIL = "/nav-pilot/guider/cplt-feilmeldinger";

type Stack = { stack: string; symptom: string; toml: string; anchor: string };

const STACKS: Stack[] = [
  {
    stack: "Go, tester med httptest",
    symptom: "dial tcp 127.0.0.1:…: connect: operation not permitted",
    toml: "allow_localhost_any = true",
    anchor: "localhost",
  },
  {
    stack: "Gradle",
    symptom: "Could not connect to the Gradle daemon.",
    toml: "allow_localhost_any = true",
    anchor: "gradle-connect",
  },
  {
    stack: "MockK, Mockito inline",
    symptom: "Could not self-attach to current VM",
    toml: "allow_jvm_attach = true",
    anchor: "self-attach",
  },
  {
    stack: "Next.js dev-server med Turbopack",
    symptom: "node process exited before we could connect to it",
    toml: "allow_localhost_any = true",
    anchor: "localhost",
  },
  {
    stack: "Database eller Docker Compose på egen maskin",
    symptom: "connect EPERM 127.0.0.1:5432",
    toml: "localhost = [5432]",
    anchor: "localhost",
  },
];

const EKSEMPEL = `[propose]
allow_localhost_any = true   # Gradle-daemonen, httptest og Turbopack bruker tilfeldige porter
allow_jvm_attach = true      # MockK

[propose.allow]
localhost = [5432]           # PostgreSQL fra docker compose`;

export default function CpltOppsett() {
  return (
    <DocPage
      label="Guider"
      upgrade
      title="Sett opp cplt i et repo"
      description="Første gang agenten skal jobbe i et repo, trenger sandkassen ofte litt mer enn standard. Legg det i .cplt.toml, så får hele teamet det samme."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="forste-gang" size="medium" level="2">
            Første gang i et repo
          </LinkableHeading>
          <BodyLong>
            Uten egen konfig slipper cplt agenten til prosjektkatalogen og de offentlige pakkeregistrene. Docker og
            filene med tokenene dine er stengt, og på macOS også localhost. Det repoet trenger i tillegg, skriver teamet
            i <code className={code}>.cplt.toml</code> i roten av repoet.
          </BodyLong>
          <BodyLong>
            Start med å se hva <code className={code}>cplt init</code> foreslår. Uten flagg skriver den ingenting:
          </BodyLong>
          <CodeBlock compact>{`cplt init`}</CodeBlock>
          <BodyLong>
            Se over forslaget. Under <code className={code}>[propose]</code> står det som åpner sandkassen. Linjer som
            er kommentert ut, peker på filer i hjemmekatalogen din og hører hjemme i din egen konfig,{" "}
            <code className={code}>~/.config/cplt/config.toml</code>, ikke i repoet.{" "}
            <code className={code}>cplt init</code> finner ikke alt. Sammenlign med{" "}
            <a href="#stakker" className={linkClass}>
              tabellen under
            </a>
            .
          </BodyLong>
          <BodyLong>Er forslaget riktig, skriv fila, sjekk den inn og godkjenn den:</BodyLong>
          <CodeBlock compact>
            {`cplt init --write
git add .cplt.toml
git commit -m "chore: add cplt sandbox config"
cplt trust accept --all`}
          </CodeBlock>
          <BodyLong>
            cplt leser fila slik den er i siste commit, så agenten kan ikke gi seg selv mer midt i en økt. Hver utvikler
            godkjenner forslaget på sin egen maskin med <code className={code}>cplt trust accept</code>.{" "}
            <code className={code}>cplt trust show</code> viser hva som er godkjent, og hva som venter.
          </BodyLong>
          <BodyLong>
            Har repoet allerede en <code className={code}>.cplt.toml</code>, legg til én nøkkel om gangen, eller ta med
            det init finner nå:
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set --repo allow.localhost 5432   # legger forslaget i .cplt.toml     
cplt init --write --merge                     # legger til nye funn, fjerner ingenting`}
          </CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="stakker" size="medium" level="2">
            Det stakken din trenger
          </LinkableHeading>
          <BodyLong>
            Feilen du ser i sandkassen på macOS, og hva du skriver i <code className={code}>.cplt.toml</code>.{" "}
            <code className={code}>localhost</code> står under <code className={code}>[propose.allow]</code>, de andre
            nøklene under <code className={code}>[propose]</code>. Feilmeldingene lenker til oppslaget i Feil i
            sandkassen.
          </BodyLong>
          <div className="overflow-x-auto">
            <Table size="small" className="table-stack" role="table">
              <HeaderRow stack cells={["Stakk", "Feilen", "I .cplt.toml"]} />
              <TableBody role="rowgroup">
                {STACKS.map((s) => (
                  <TableRow role="row" key={s.stack}>
                    <TableDataCell role="cell">{s.stack}</TableDataCell>
                    <TableDataCell role="cell" data-label="Feilen">
                      <NextLink href={`${FEIL}#${s.anchor}`} className={linkClass}>
                        <code className={code}>{s.symptom}</code>
                      </NextLink>
                    </TableDataCell>
                    <TableDataCell role="cell" data-label="I .cplt.toml">
                      <code className={code}>{s.toml}</code>
                    </TableDataCell>
                  </TableRow>
                ))}
                <TableRow role="row">
                  <TableDataCell role="cell">@navikt-pakker fra GitHub Packages</TableDataCell>
                  <TableDataCell role="cell" data-label="Feilen">
                    <code className={code}>401 Unauthorized … authentication token not provided</code>
                  </TableDataCell>
                  <TableDataCell role="cell" data-label="I .cplt.toml">
                    Ingenting. Se{" "}
                    <a href="#github-packages" className={linkClass}>
                      Pakker fra GitHub Packages
                    </a>
                    .
                  </TableDataCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>
          <BodyLong>En Kotlin-app med Gradle, MockK og PostgreSQL i Docker Compose får denne fila:</BodyLong>
          <CodeBlock filename=".cplt.toml">{EKSEMPEL}</CodeBlock>
          <BodyLong>
            Hvorfor noen stakker trenger alle portene, står under{" "}
            <a href="#localhost" className={linkClass}>
              Tjenester på localhost
            </a>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="localhost" size="medium" level="2">
            Tjenester på localhost
          </LinkableHeading>
          <BodyLong>
            På macOS stenger cplt localhost. Agenten kan starte en utviklingsserver, og du når den fra nettleseren som
            vanlig, men agenten selv når ikke tjenester på din maskin. Har tjenesten fast port, som en database, åpne
            bare den:
          </BodyLong>
          <CodeBlock compact>{`cplt config set allow.localhost 5432`}</CodeBlock>
          <BodyLong>
            Go-tester med <code className={code}>httptest</code>, Gradle-daemonen og arbeidsprosessene til Turbopack,
            Vite og esbuild lytter på tilfeldige porter, så én port er ikke nok. Med bare port 3000 åpen starter
            Next.js, men første side feiler. Da må du åpne alle portene:
          </BodyLong>
          <CodeBlock compact>{`cplt config set sandbox.allow_localhost_any true`}</CodeBlock>
          <Bullets>
            <li>
              Med <code className={code}>allow_localhost_any</code> når agenten alle tjenester som lytter på localhost,
              også en lokal database.
            </li>
            <li>
              På Linux koster den mer: kjernen der kan ikke skille localhost fra andre verter, så cplt slår av
              portfiltreringen for utgående TCP helt, og bare proxyen begrenser hvor agenten kan koble seg til.
            </li>
            <li>
              Med <code className={code}>proxy.forced</code> slått på, som i{" "}
              <code className={code}>--preset strict</code>, ser cplt bort fra{" "}
              <code className={code}>allow_localhost_any</code>. Enkeltporter fra{" "}
              <code className={code}>allow.localhost</code> virker fortsatt, men tilfeldige porter kan ikke åpnes. Da må
              du kjøre uten tvungen proxy.
            </li>
          </Bullets>
          <Box background="warning-soft" borderRadius="8" padding="space-16">
            <BodyLong>
              Bruk <code className={code}>localhost</code>, ikke <code className={code}>ports</code>, for tjenester på
              din egen maskin. <code className={code}>ports</code> åpner porten mot alle maskiner på nettet, og på macOS
              gir den ikke tilgang til localhost.
            </BodyLong>
          </Box>
          <BodyLong>
            Skal hele teamet ha innstillingen, bruk <code className={code}>cplt config set --repo …</code> og sjekk inn{" "}
            <code className={code}>.cplt.toml</code>, som beskrevet i{" "}
            <a href="#forste-gang" className={linkClass}>
              Første gang i et repo
            </a>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="github-packages" size="medium" level="2">
            Pakker fra GitHub Packages
          </LinkableHeading>
          <BodyLong>
            @navikt-pakker på npm og Maven ligger i GitHub Packages, og der må du logge inn. Tokenet ligger i{" "}
            <code className={code}>~/.npmrc</code>, <code className={code}>~/.gradle/gradle.properties</code> eller{" "}
            <code className={code}>~/.m2/settings.xml</code>. cplt stenger disse filene, så{" "}
            <code className={code}>npm install</code> svarer <code className={code}>401 Unauthorized</code> og Gradle
            finner ikke pakken. På Linux er bare <code className={code}>~/.npmrc</code> stengt. De to andre ligger i
            mapper agenten har tilgang til. <code className={code}>cplt init</code> advarer om dette når{" "}
            <code className={code}>.npmrc</code> i repoet henter pakker fra GitHub Packages.
          </BodyLong>
          <BodyLong>For npm og pnpm er det enklest å installere utenfor cplt før du starter agenten:</BodyLong>
          <CodeBlock compact>
            {`pnpm install        # i terminalen, utenfor cplt
nav-pilot`}
          </CodeBlock>
          <BodyLong>Må agenten hente pakker selv, gi den lesetilgang til de tre filene:</BodyLong>
          <CodeBlock compact>{`cplt config set sandbox.allow_build_credentials true --force`}</CodeBlock>
          <BodyLong>
            Da kan agenten lese alle tokenene i filene, ikke bare det prosjektet trenger. Derfor krever cplt{" "}
            <code className={code}>--force</code>, og derfor kan ikke nøkkelen stå i{" "}
            <code className={code}>.cplt.toml</code>. Bruk et token som bare kan lese pakker (
            <code className={code}>read:packages</code>). Trenger du bare én av filene, gi lesetilgang til den, for
            eksempel <code className={code}>cplt config set allow.read ~/.gradle/gradle.properties</code>.
          </BodyLong>
          <BodyLong>
            Under en liste over tillatte verter når Copilot <code className={code}>npm.pkg.github.com</code> og{" "}
            <code className={code}>maven.pkg.github.com</code>, fordi lista har med{" "}
            <code className={code}>github.com</code> og alle undervertene. Får du <code className={code}>401</code>, har
            forespørselen kommet fram. Da er det tokenet som mangler, ikke nettverket.
          </BodyLong>
          <BodyLong>
            Hvert verktøy har sin egen vei rundt dette: npm kan lese tokenet fra en miljøvariabel, pnpm og yarn 1
            trenger <code className={code}>~/.npmrc</code>, og Gradle kan hente fra Navs speil uten token. Se{" "}
            <NextLink href="/nav-pilot/guider/cplt-node#github-packages" className={linkClass}>
              Node, npm og pnpm
            </NextLink>{" "}
            og{" "}
            <NextLink href="/nav-pilot/guider/cplt-gradle#github-packages" className={linkClass}>
              Kotlin og Gradle
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="tillatelsesliste" size="medium" level="2">
            Pakkeregistre og en liste over tillatte verter
          </LinkableHeading>
          <BodyLong>
            I standardoppsettet stopper cplt bare kjente skadelige verter og verter med private adresser, og
            installasjoner virker som vanlig. Har du slått på en liste over tillatte verter, med{" "}
            <code className={code}>--preset strict</code>, <code className={code}>proxy.default_allowlist</code> eller{" "}
            <code className={code}>proxy.allowed_domains</code>, slipper cplt bare gjennom det som står på lista.
            Pakkeregistrene er med når <code className={code}>proxy.default_allowlist</code> er på, og strict slår den
            på. Med bare <code className={code}>proxy.allowed_domains</code> er de med hvis fila lister dem. Fila
            nav-pilot skriver, har alle når cplt støtter <code className={code}>cplt config hosts</code>. Er cplt for
            gammel til det, mangler Gradle-plugin-artefaktene, Confluent og JitPack:
          </BodyLong>
          <Bullets>
            <li>
              <code className={code}>registry.npmjs.org</code> og <code className={code}>registry.yarnpkg.com</code>{" "}
              (npm og yarn)
            </li>
            <li>
              <code className={code}>repo.maven.apache.org</code> (Maven Central)
            </li>
            <li>
              <code className={code}>plugins.gradle.org</code> og{" "}
              <code className={code}>plugins-artifacts.gradle.org</code> (Gradle-plugins)
            </li>
            <li>
              <code className={code}>crates.io</code> og <code className={code}>static.crates.io</code> (Cargo)
            </li>
            <li>
              <code className={code}>pypi.org</code> og <code className={code}>files.pythonhosted.org</code> (PyPI)
            </li>
            <li>
              <code className={code}>packages.confluent.io</code> og <code className={code}>jitpack.io</code>
            </li>
          </Bullets>
          <BodyLong>
            Mangler en vert, sjekk den og legg den til med <code className={code}>allow.domains</code>. Den legger til
            på en liste som allerede er slått på, og slår ikke på lista selv:
          </BodyLong>
          <CodeBlock compact>
            {`cplt check net min.vert.no
cplt config set allow.domains min.vert.no`}
          </CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="pnpm" size="medium" level="2">
            pnpm
          </LinkableHeading>
          <BodyLong>
            I sandkassen legger pnpm lageret i repoet, i <code className={code}>.pnpm-store/</code> eller{" "}
            <code className={code}>node_modules/.pnpm-store/</code>, og laster ned alle pakkene på nytt i hvert repo og
            hvert worktree. Årsaken er at pnpm vil lage en mappe i <code className={code}>$PNPM_HOME</code>, der agenten
            ikke får skrive (
            <a href="https://github.com/navikt/cplt/issues/637" className={linkClass}>
              cplt#637
            </a>
            ). Pek pnpm på det felles lageret, her på macOS:
          </BodyLong>
          <CodeBlock compact>
            {`export pnpm_config_store_dir=$HOME/Library/pnpm/store   # i skallet, for eksempel ~/.zshrc
cplt config set sandbox.pass_env pnpm_config_store_dir   # send den inn i sandkassen`}
          </CodeBlock>
          <BodyLong>
            Ellers legger du lageret i <code className={code}>.gitignore</code>:
          </BodyLong>
          <CodeBlock compact>{`echo ".pnpm-store/" >> .gitignore`}</CodeBlock>
          <BodyLong>
            Skript som kjører når en pakke installeres, er slått av i sandkassen for npm, pnpm og yarn 1 (ikke yarn 2 og
            nyere), se{" "}
            <NextLink href="/nav-pilot/guider/cplt-node#skript" className={linkClass}>
              Skript som kjører ved installasjon
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="mise" size="medium" level="2">
            mise
          </LinkableHeading>
          <BodyLong>
            Verktøy mise allerede har installert, kan agenten bruke, og <code className={code}>mise run</code> virker.
            Mangler en versjon, stopper mise med denne feilen:
          </BodyLong>
          <CodeBlock
            compact
          >{`failed create_dir_all ~/.local/share/mise/installs/…: Operation not permitted`}</CodeBlock>
          <BodyLong>
            Det er med vilje. Et program agenten laster ned og kjører selv, kan gjøre det sandkassen ellers stopper.
            Installer utenfor cplt før du starter agenten:
          </BodyLong>
          <CodeBlock compact>{`mise install`}</CodeBlock>
          <BodyLong>
            <code className={code}>cplt init</code> minner deg om dette når repoet har en mise-konfig med{" "}
            <code className={code}>[tools]</code>. <code className={code}>cplt doctor</code> gjør det samme når en
            versjon mangler.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="docker" size="medium" level="2">
            Docker
          </LinkableHeading>
          <BodyLong>
            Docker er stengt i sandkassen. Den som kan snakke med Docker-daemonen, kan starte en container som monterer
            hele disken, og da er sandkassen borte. Det gjelder også Testcontainers, se{" "}
            <NextLink href="/nav-pilot/guider/cplt-gradle#testcontainers" className={linkClass}>
              Testcontainers og Docker
            </NextLink>
            .
          </BodyLong>
          <BodyLong>
            <code className={code}>cplt init</code> foreslår <code className={code}>allow_docker</code> når repoet har
            en compose-fil. Ta den ut av forslaget. For en <code className={code}>Dockerfile</code> alene foreslår den
            ikke Docker, men advarer om at <code className={code}>docker build</code> ikke virker i sandkassen. I Nav
            bygger CI-en imagene, så agenten trenger ikke Docker. Start tjenestene selv i en egen terminal, og åpne
            portene agenten skal nå:
          </BodyLong>
          <CodeBlock compact>
            {`docker compose up -d                         # i en terminal utenfor cplt
cplt config set --repo allow.localhost 5432   # agenten når databasen på localhost`}
          </CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="hele-maskinen" size="medium" level="2">
            Brytere for hele maskinen
          </LinkableHeading>
          <BodyLong>
            Det som gjelder alle repoene dine, hører hjemme i din egen konfig,{" "}
            <code className={code}>~/.config/cplt/config.toml</code>.{" "}
            <code className={code}>nav-pilot config sandbox</code> viser bryterne for hele maskinen, for eksempel
            localhost og Docker, og skriver dit. Det samme gjør <code className={code}>cplt config set</code>:
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot config sandbox
cplt config set sandbox.allow_localhost_any true`}
          </CodeBlock>
          <BodyLong>
            Sikkerhetsnivået setter du med <code className={code}>nav-pilot config</code>, se{" "}
            <NextLink href="/nav-pilot/forklaring/sandkassen#sikkerhetsniva" className={linkClass}>
              Sandkassen
            </NextLink>
            . Får du en feil du ikke finner her, slå den opp i{" "}
            <NextLink href={FEIL} className={linkClass}>
              Feil i sandkassen
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
