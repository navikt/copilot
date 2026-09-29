import { BodyLong, Box, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Sett opp cplt i et repo",
  description:
    "Første gang i et repo: la cplt init foreslå tilganger, se over forslaget, sjekk inn .cplt.toml og godkjenn det. Med det Go, Gradle, Next.js, pnpm, mise og Docker trenger.",
};

const TOC: TocItem[] = [
  { id: "forste-gang", label: "Første gang i et repo" },
  { id: "stakker", label: "Det stakken din trenger" },
  { id: "github-packages", label: "Pakker fra GitHub Packages" },
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
    stack: "Next.js dev-server",
    symptom: "connect EPERM 127.0.0.1:3000",
    toml: "localhost = [3000]",
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
allow_localhost_any = true   # Gradle-daemonen og httptest bruker tilfeldige porter
allow_jvm_attach = true      # MockK

[propose.allow]
localhost = [5432]           # PostgreSQL fra docker compose`;

export default function CpltOppsett() {
  return (
    <DocPage
      label="Guider"
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
            i <code className={code}>.cplt.toml</code> i roten av repoet. Da får alle på teamet den samme sandkassen.
          </BodyLong>
          <BodyLong>
            Start med å se hva <code className={code}>cplt init</code> foreslår. Uten flagg skriver den ingenting:
          </BodyLong>
          <CodeBlock compact>{`cplt init`}</CodeBlock>
          <BodyLong>
            Se over forslaget. Under <code className={code}>[propose]</code> står det som åpner sandkassen. Linjer som
            er kommentert ut, peker på filer i hjemmekatalogen din. De hører ikke hjemme i repoet, men i din egen
            konfig, <code className={code}>~/.config/cplt/config.toml</code>. <code className={code}>cplt init</code>{" "}
            finner ikke alt. For Go foreslår den ingenting, og for Gradle mangler localhost. Sammenlign med{" "}
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
            cplt leser bare fila slik den er i siste commit. Før den er sjekket inn, gir den ingen tilganger. Da kan
            ikke agenten gi seg selv mer midt i en økt. Hver utvikler godkjenner forslaget på sin egen maskin med{" "}
            <code className={code}>cplt trust accept</code>. <code className={code}>cplt trust show</code> viser hva som
            er godkjent, og hva som venter.
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
            Tabellen viser feilen du ser i sandkassen på macOS, og hva du skriver i{" "}
            <code className={code}>.cplt.toml</code>. <code className={code}>localhost</code> står under{" "}
            <code className={code}>[propose.allow]</code>, de andre nøklene under{" "}
            <code className={code}>[propose]</code>. Feilmeldingene lenker til forklaringen i Feil i sandkassen.
          </BodyLong>
          <div className="overflow-x-auto">
            <Table size="small">
              <TableHeader>
                <TableRow>
                  <TableHeaderCell scope="col">Stakk</TableHeaderCell>
                  <TableHeaderCell scope="col">Feilen</TableHeaderCell>
                  <TableHeaderCell scope="col">I .cplt.toml</TableHeaderCell>
                </TableRow>
              </TableHeader>
              <TableBody>
                {STACKS.map((s) => (
                  <TableRow key={s.stack}>
                    <TableDataCell>{s.stack}</TableDataCell>
                    <TableDataCell>
                      <NextLink href={`${FEIL}#${s.anchor}`} className={linkClass}>
                        <code className={code}>{s.symptom}</code>
                      </NextLink>
                    </TableDataCell>
                    <TableDataCell>
                      <code className={code}>{s.toml}</code>
                    </TableDataCell>
                  </TableRow>
                ))}
                <TableRow>
                  <TableDataCell>@navikt-pakker fra GitHub Packages</TableDataCell>
                  <TableDataCell>
                    <code className={code}>401 Unauthorized … authentication token not provided</code>
                  </TableDataCell>
                  <TableDataCell>
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
            Go-tester med <code className={code}>httptest</code> og Gradle-daemonen lytter på en tilfeldig port, så én
            port er ikke nok. Next.js og databasen har faste porter, og da holder det å åpne dem.
          </BodyLong>
          <Box background="warning-soft" borderRadius="8" padding="space-16">
            <BodyLong>
              Bruk <code className={code}>localhost</code>, ikke <code className={code}>ports</code>, for tjenester på
              din egen maskin. <code className={code}>ports</code> åpner porten mot alle maskiner på nettet, og på macOS
              gir den ikke tilgang til localhost. Foreslår <code className={code}>cplt init</code>{" "}
              <code className={code}>ports</code>, eller <code className={code}>cplt check net</code>{" "}
              <code className={code}>--allow-port</code>, for en tjeneste på din egen maskin, bruk{" "}
              <code className={code}>localhost</code> eller <code className={code}>--allow-localhost</code> i stedet.
            </BodyLong>
          </Box>
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
            finner ikke pakken.
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
            <code className={code}>.cplt.toml</code>. Den gjelder bare din egen maskin. Bruk et token som bare kan lese
            pakker (<code className={code}>read:packages</code>).
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="pnpm" size="medium" level="2">
            pnpm
          </LinkableHeading>
          <BodyLong>
            Stopper <code className={code}>pnpm install</code> på{" "}
            <code className={code}>~/Library/Preferences/pnpm/config.yaml</code>, se{" "}
            <NextLink href={`${FEIL}#pnpm-config`} className={linkClass}>
              pnpm-konfig
            </NextLink>
            .
          </BodyLong>
          <BodyLong>
            I sandkassen kan ikke pnpm lenke filer fra det felles lageret i hjemmekatalogen inn i prosjektet. Da lager
            den et eget lager inne i repoet, for eksempel <code className={code}>.pnpm-store/</code> eller{" "}
            <code className={code}>node_modules/.pnpm-store/</code>, og laster ned alle pakkene på nytt i hvert repo og
            hver worktree. Legg lageret i <code className={code}>.gitignore</code>:
          </BodyLong>
          <CodeBlock compact>{`echo ".pnpm-store/" >> .gitignore`}</CodeBlock>
          <BodyLong>
            Kjører du <code className={code}>pnpm install</code> utenfor cplt, bruker pnpm det felles lageret som før.
          </BodyLong>
          <BodyLong>
            Skript som kjører når en pakke installeres, er slått av i sandkassen. Trenger en pakke dem, installer
            utenfor cplt.
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
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="docker" size="medium" level="2">
            Docker
          </LinkableHeading>
          <BodyLong>
            Docker er stengt i sandkassen. Den som kan snakke med Docker-daemonen, kan starte en container som monterer
            hele disken, og da er sandkassen borte. <code className={code}>cplt check exec docker</code> svarer{" "}
            <code className={code}>BLOCKED</code> og sier det samme.
          </BodyLong>
          <BodyLong>
            <code className={code}>cplt init</code> foreslår <code className={code}>allow_docker</code> når repoet har
            en <code className={code}>Dockerfile</code>. Ta den ut av forslaget. I Nav bygger CI-en imagene, så agenten
            trenger ikke Docker. Start tjenestene selv i en egen terminal, og åpne portene agenten skal nå:
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
            Det som gjelder alle repoene dine, hører hjemme i din egen konfig, ikke i{" "}
            <code className={code}>.cplt.toml</code>. <code className={code}>nav-pilot config sandbox</code> viser
            bryterne for hele maskinen, for eksempel localhost og Docker, og skriver valgene til{" "}
            <code className={code}>~/.config/cplt/config.toml</code>. Det samme gjør{" "}
            <code className={code}>cplt config set</code>:
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
