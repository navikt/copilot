import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Feil i sandkassen",
  description:
    "Feilmeldinger fra cplt og verktøy som kjører i sandkassen: hva de betyr og kommandoen som løser dem. Slå opp på teksten du ser.",
};

const TOC: TocItem[] = [
  { id: "start-her", label: "Start her" },
  { id: "versjon", label: "Versjon og konfig" },
  { id: "nettverk", label: "Nettverk" },
  { id: "filer", label: "Filer og programmer" },
  { id: "jvm", label: "Java, Kotlin og Gradle" },
  { id: "nettleser", label: "Playwright, Cypress og Puppeteer" },
  { id: "git", label: "Git og GitHub" },
  { id: "agent", label: "Agenten og innlogging" },
  { id: "rapporter", label: "Rapporter et problem" },
];

const KNOWN_IMPACTS = "https://github.com/navikt/cplt/blob/main/docs/known-impacts.md";

export default function CpltFeilmeldinger() {
  return (
    <DocPage
      label="Guider"
      upgrade
      title="Feil i sandkassen"
      description="Slå opp på feilmeldingen du ser. Hver oppføring sier hva som skjer, og hvilken kommando som løser det."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="start-her" size="medium" level="2">
            Start her
          </LinkableHeading>
          <BodyLong id="oppgrader">
            {/* Gamle anker for feil som er rettet i cplt fra 29. september 2026. */}
            <span id="sign-in-failed" />
            <span id="pnpm-config" />
            <span id="pnpm-claude" />
            <strong>Eldre cplt? Oppgrader først.</strong> Flere feil er rettet i cplt fra 29. september 2026: pnpm fikk
            ikke lese sin egen konfig (<code className={code}>EPERM … pnpm/config.yaml</code>), pakker med en{" "}
            <code className={code}>.claude</code>-mappe kunne ikke installeres (
            <code className={code}>ERR_PNPM_EPERM</code>), og en egen host-liste stengte Copilot ute fra innloggingen (
            <code className={code}>Sign-in failed</code>).
          </BodyLong>
          <BodyLong>Finner du ikke feilen under, spør cplt selv:</BodyLong>
          <CodeBlock compact>
            {`cplt check                               # virker sandkassen?
cplt check net registry.npmjs.org        # slipper proxyen gjennom denne hosten?
cplt check path ~/.npmrc                 # får agenten lese eller skrive denne fila?
cplt config set proxy.log_level blocked  # vis blokkerte nettverkskall mens agenten jobber
cplt config explain sandbox.allow_env_files  # hva gjør en nøkkel?`}
          </CodeBlock>
          <BodyLong>
            Svarene fra <code className={code}>cplt check</code> har en linje som begynner med{" "}
            <code className={code}>Reason:</code>, og ofte en med <code className={code}>Fix:</code>. Flere detaljer
            står i{" "}
            <a href={KNOWN_IMPACTS} className={linkClass}>
              known-impacts.md
            </a>{" "}
            (engelsk).
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="versjon" size="medium" level="2">
            Versjon og konfig
          </LinkableHeading>

          <LinkableHeading id="unknown-config-key" size="small" level="3">
            unknown config key
          </LinkableHeading>
          <BodyLong>
            cplt kjenner ikke nøkkelen. Meldingen lister de gyldige. Står nøkkelen der, er den skrevet feil. Står den
            ikke der, er din cplt eldre enn dokumentasjonen. Oppgrader.
          </BodyLong>

          <LinkableHeading id="does-not-understand" size="small" level="3">
            .cplt.toml uses a key this version of cplt does not understand
          </LinkableHeading>
          <BodyLong>
            Repoets <code className={code}>.cplt.toml</code> er skrevet for en nyere cplt enn din, som hopper over
            nøkkelen. Står den under <code className={code}>[deny]</code>, blir en begrensning repoet ber om, ikke
            brukt. Oppgrader cplt. Gjelder meldingen din egen konfig (
            <code className={code}>unknown key &apos;…&apos; in [seksjon]</code>), sjekk fila:
          </BodyLong>
          <CodeBlock compact>{`cplt config validate`}</CodeBlock>

          <LinkableHeading id="audit-fjernet" size="small" level="3">
            the [audit] section was removed
          </LinkableHeading>
          <BodyLong>
            <code className={code}>[audit]</code> gjorde aldri noe, og cplt avviser nå alle{" "}
            <code className={code}>audit.*</code>-nøkler. Slett seksjonen fra{" "}
            <code className={code}>~/.config/cplt/config.toml</code>. Vil du ha en logg over nettverkskallene, bruk{" "}
            <code className={code}>proxy.log_file</code>:
          </BodyLong>
          <CodeBlock compact>{`cplt config set proxy.log_file ~/cplt-proxy.log`}</CodeBlock>
          <BodyLong>
            Rapporten over filene agenten endret, <code className={code}>sandbox.audit</code>, er på som standard.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="nettverk" size="medium" level="2">
            Nettverk
          </LinkableHeading>

          <LinkableHeading id="blocked-allowlist" size="small" level="3">
            BLOCKED-ALLOWLIST
          </LinkableHeading>
          <BodyLong>
            Hosten står ikke på host-lista. Når <code className={code}>proxy.allowed_domains</code> er satt, slipper
            proxyen bare gjennom agentens egne hoster, hostene i fila og i <code className={code}>allow.domains</code>,
            og pakkeregistrene hvis <code className={code}>proxy.default_allowlist</code> er på. Fila nav-pilot skriver,
            har pakkeregistrene. Finn fila og legg til hosten:
          </BodyLong>
          <CodeBlock compact>
            {`cplt config get proxy.allowed_domains   # viser hvilken fil
echo min.host.no >> <fila>
cplt check net min.host.no               # BLOCKED før, ALLOWED etter`}
          </CodeBlock>
          <BodyLong>
            Én host per linje. Mangler npm, Maven Central eller andre pakkeregistre, slå på{" "}
            <code className={code}>proxy.default_allowlist</code>:
          </BodyLong>
          <CodeBlock compact>{`cplt config set proxy.default_allowlist true`}</CodeBlock>
          <BodyLong>
            <code className={code}>cplt doctor</code> sier fra når nettverksreglene stenger ute agentens hoster eller
            pakkeregistrene.
          </BodyLong>

          <LinkableHeading id="private-target" size="small" level="3">
            Failed to CONNECT: 403 Forbidden — Private target blocked by cplt
          </LinkableHeading>
          <BodyLong>
            Hosten peker til en privat IP-adresse, for eksempel <code className={code}>repo.adeo.no</code> eller en
            tjeneste på <code className={code}>intern.nav.no</code>. cplt stopper slike kall som vern mot SSRF.
            Proxyloggen viser <code className={code}>BLOCKED-PRIVATE</code> eller{" "}
            <code className={code}>BLOCKED-PRIVATE-RESOLVED</code>. Tillat domenet ved navn. Underdomener blir med:
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set proxy.allow_private_domains repo.adeo.no
cplt config set proxy.allow_private_domains intern.nav.no`}
          </CodeBlock>
          <BodyLong>
            En adresse skrevet som IP, for eksempel <code className={code}>https://10.20.30.40/</code>, kan ikke åpnes.
            Bruk DNS-navnet i stedet.
          </BodyLong>

          <LinkableHeading id="not-enforcing" size="small" level="3">
            Sandbox is NOT ENFORCING
          </LinkableHeading>
          <BodyLong>
            Sandkassen slapp gjennom noe den skulle stoppe. Meld det i{" "}
            <a href="https://github.com/navikt/cplt/issues" className={linkClass}>
              navikt/cplt
            </a>
            . Oppgrader og kjør <code className={code}>cplt check</code> på nytt før du melder, for eldre cplt viste
            meldingen også når sandkassen blokkerte noe den skulle slippe gjennom.
          </BodyLong>

          <LinkableHeading id="policy-too-strict" size="small" level="3">
            policy too strict
          </LinkableHeading>
          <BodyLong>
            Sandkassen virker, men konfigen din stenger noe agenten trenger. Linja under den feilede sjekken sier hva og
            hvorfor. En vanlig årsak er en egen host-liste, se{" "}
            <a href="#blocked-allowlist" className={linkClass}>
              BLOCKED-ALLOWLIST
            </a>
            .
          </BodyLong>

          <LinkableHeading id="localhost" size="small" level="3">
            connect EPERM 127.0.0.1
          </LinkableHeading>
          <BodyLong>
            cplt stenger localhost på macOS. Åpne porten til tjenesten du trenger, for eksempel PostgreSQL:
          </BodyLong>
          <CodeBlock compact>{`cplt config set allow.localhost 5432`}</CodeBlock>
          <BodyLong>
            Verktøy som bruker tilfeldige porter, som Next.js, Vite, esbuild og Gradle, trenger alle portene:
          </BodyLong>
          <CodeBlock compact>{`cplt config set sandbox.allow_localhost_any true`}</CodeBlock>
          <BodyLong>
            Gjelder det hele teamet, legg det i repoet, se{" "}
            <NextLink href="/nav-pilot/guider/cplt-oppsett#stakker" className={linkClass}>
              Det stakken din trenger
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="filer" size="medium" level="2">
            Filer og programmer
          </LinkableHeading>

          <LinkableHeading id="operation-not-permitted" size="small" level="3">
            Operation not permitted, EPERM eller EACCES
          </LinkableHeading>
          <BodyLong>Sandkassen nektet tilgang til en fil. Spør cplt om stien:</BodyLong>
          <CodeBlock compact>{`cplt check path <sti>`}</CodeBlock>
          <BodyLong>Stien sier som regel hvor du skal lete videre:</BodyLong>
          <Bullets>
            <li>
              <code className={code}>~/.npmrc</code>, <code className={code}>~/.gradle/gradle.properties</code> eller{" "}
              <code className={code}>~/.m2/settings.xml</code>: filene har tokens og er stengt med vilje. Se{" "}
              <NextLink href="/nav-pilot/guider/cplt-oppsett#github-packages" className={linkClass}>
                Pakker fra GitHub Packages
              </NextLink>
              .
            </li>
            <li>
              <code className={code}>~/Library/Preferences/pnpm</code>: se{" "}
              <NextLink href="/nav-pilot/guider/cplt-node#pnpm-konfig" className={linkClass}>
                pnpm-konfig og tokens
              </NextLink>
              .
            </li>
            <li>
              <code className={code}>.env</code>, <code className={code}>.env.local</code> og lignende: se{" "}
              <a href="#env-filer" className={linkClass}>
                .env-filer
              </a>
              .
            </li>
            <li>
              <code className={code}>.git/config</code>: se{" "}
              <a href="#git-config" className={linkClass}>
                git config
              </a>
              .
            </li>
            <li>
              <code className={code}>/tmp</code> eller <code className={code}>/var/folders</code> når en JVM starter: se{" "}
              <a href="#jvm-oppstart" className={linkClass}>
                JVM-oppstart
              </a>
              .
            </li>
            <li>
              <code className={code}>~/Library/Caches</code> når et program skal kjøres: se{" "}
              <a href="#playwright" className={linkClass}>
                Playwright
              </a>{" "}
              og{" "}
              <a href="#cypress" className={linkClass}>
                Cypress
              </a>
              .
            </li>
          </Bullets>

          <LinkableHeading id="yarnrc" size="small" level="3">
            EPERM: operation not permitted, open …/.yarnrc
          </LinkableHeading>
          <BodyLong>
            yarn 1 stopper når den ikke får lese <code className={code}>~/.yarnrc</code>. Gi lesetilgang til fila:
          </BodyLong>
          <CodeBlock compact>{`cplt config set allow.read ~/.yarnrc`}</CodeBlock>
          <BodyLong>
            Samme feil for <code className={code}>~/.npmrc</code> (på Linux <code className={code}>EACCES</code>)
            trenger det samme, også når prosjektet bare henter offentlige pakker:
          </BodyLong>
          <CodeBlock compact>{`cplt config set allow.read ~/.npmrc`}</CodeBlock>

          <LinkableHeading id="cannot-hash" size="small" level="3">
            fatal: cannot hash .env.local
          </LinkableHeading>
          <BodyLong>
            Gjelder macOS. Fila er sporet i git og endret, og cplt stenger lesing av <code className={code}>.env</code>
            -filer, så <code className={code}>git diff</code>, <code className={code}>git add</code> og{" "}
            <code className={code}>git commit -a</code> stopper. En fil med hemmeligheter hører ikke hjemme i git. Slutt
            å spore den:
          </BodyLong>
          <CodeBlock compact>
            {`git rm --cached .env.local
echo ".env.local" >> .gitignore`}
          </CodeBlock>

          <LinkableHeading id="env-filer" size="small" level="3">
            Operation not permitted på .env-filer
          </LinkableHeading>
          <BodyLong>
            Gjelder macOS. På Linux kan agenten lese og skrive disse filene. cplt stenger både lesing og skriving av{" "}
            <code className={code}>.env</code> og <code className={code}>.env.*</code>, og av filer som heter{" "}
            <code className={code}>.pem</code>, <code className={code}>.key</code> og lignende. Next.js og dotenv får
            tomme variabler, og tester som skriver slike filer, feiler. Åpne dem hvis prosjektet trenger det:
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set sandbox.allow_env_files true   # alltid
cplt --allow-env-files                          # bare denne økten`}
          </CodeBlock>
          <BodyLong>Da kan agenten lese alle slike filer i prosjektet, også hemmelighetene i dem.</BodyLong>

          <LinkableHeading id="includeif" size="small" level="3">
            fatal: unable to access ~/.config/git/work: Permission denied
          </LinkableHeading>
          <BodyLong>
            Git-konfigen din har en <code className={code}>includeIf</code> som peker på en fil i{" "}
            <code className={code}>~/.config/git</code>. Der får agenten bare lese <code className={code}>config</code>,{" "}
            <code className={code}>ignore</code> og <code className={code}>attributes</code>, og git stopper når en
            include ikke kan leses. På macOS står det <code className={code}>Operation not permitted</code>. Gi
            lesetilgang til fila, ikke hele mappa:
          </BodyLong>
          <CodeBlock compact>{`cplt config set allow.read ~/.config/git/work`}</CodeBlock>

          <LinkableHeading id="setuid" size="small" level="3">
            execvp() of /bin/ps failed: Operation not permitted
          </LinkableHeading>
          <BodyLong>
            Gjelder macOS. Programmer som kjører med rettighetene til eieren (setuid), som{" "}
            <code className={code}>ps</code>, <code className={code}>top</code> og <code className={code}>sudo</code>,
            kan ikke startes i sandkassen. Fra Go ser feilen ut som{" "}
            <code className={code}>fork/exec /bin/ps: operation not permitted</code>. Ingen innstilling åpner det. Kjør
            kommandoen utenfor cplt. <code className={code}>cplt check exec /bin/ps</code> svarer{" "}
            <code className={code}>BLOCKED</code>.
          </BodyLong>

          <LinkableHeading id="openpty" size="small" level="3">
            openpty: Operation not permitted
          </LinkableHeading>
          <BodyLong>
            Programmer som lager sin egen terminal, som <code className={code}>script</code>,{" "}
            <code className={code}>tmux</code>, <code className={code}>screen</code>, pexpect og node-pty, får ikke lov
            i sandkassen. Den kunne ellers lest det du skriver i andre terminalvinduer. Vanlige programmer i terminalen
            din virker. Kjør slike kommandoer utenfor cplt.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="jvm" size="medium" level="2">
            Java, Kotlin og Gradle
          </LinkableHeading>
          <BodyLong>
            Oppsettet for Gradle-prosjekter står i{" "}
            <NextLink href="/nav-pilot/guider/cplt-gradle" className={linkClass}>
              Kotlin og Gradle i sandkassen
            </NextLink>
            .
          </BodyLong>

          <LinkableHeading id="gradle-connect" size="small" level="3">
            Could not connect to the Gradle daemon / ConnectException: Could not connect to server
          </LinkableHeading>
          <BodyLong>
            Ser du bare <code className={code}>Last 20 lines from daemon log file</code>, som slutter med{" "}
            <code className={code}>Daemon server started</code>, står feilen lenger opp i utdataene. Gradle-daemonen og
            Kotlin-daemonen snakker sammen over tilfeldige porter på localhost, og cplt stenger localhost.{" "}
            <code className={code}>--no-daemon</code> hjelper ikke, for Gradle starter da som regel en daemon for ett
            enkelt bygg og kobler seg til den over localhost. Åpne alle portene:
          </BodyLong>
          <CodeBlock compact>{`cplt config set sandbox.allow_localhost_any true`}</CodeBlock>
          <BodyLong>
            Kommer feilen fortsatt med <code className={code}>addresses:[/127.0.0.1]</code>, er det som regel ktlint
            eller detekt. Arbeidsprosessene deres får ikke med innstillingene cplt setter. Kjør disse oppgavene utenfor
            cplt, for eksempel <code className={code}>./gradlew ktlintFormat</code>.
          </BodyLong>

          <LinkableHeading id="jvm-oppstart" size="small" level="3">
            Operation not permitted når JVM-en starter
          </LinkableHeading>
          <BodyLong>
            Gjelder macOS. Noen JVM-biblioteker lastes fra systemets temp-mappe før cplt har flyttet den. Bruk dette
            bare når feilen kommer mens JVM-en starter, ikke senere i bygget. Programmer i{" "}
            <code className={code}>/tmp</code> kan da kjøres, så cplt krever <code className={code}>--force</code>:
          </BodyLong>
          <CodeBlock compact>{`cplt config set sandbox.allow_tmp_exec true --force`}</CodeBlock>

          <LinkableHeading id="self-attach" size="small" level="3">
            Could not self-attach to current VM using external process
          </LinkableHeading>
          <BodyLong>
            Gjelder macOS. MockK, Mockito og ByteBuddy kobler seg til JVM-en mens testene kjører, og det trenger en
            socket cplt stenger. Åpne den:
          </BodyLong>
          <CodeBlock compact>{`cplt config set sandbox.allow_jvm_attach true`}</CodeBlock>
          <BodyLong>På Linux stenger ikke cplt denne socketen, så der skyldes feilen noe annet.</BodyLong>

          <LinkableHeading id="jdk-library" size="small" level="3">
            JDK i ~/Library/Java
          </LinkableHeading>
          <BodyLong>
            cplt støtter ikke JDK-er i <code className={code}>~/Library/Java/JavaVirtualMachines</code> ennå. Pek{" "}
            <code className={code}>JAVA_HOME</code> på en JDK et annet sted, for eksempel under{" "}
            <code className={code}>/Library/Java/JavaVirtualMachines</code>, SDKMAN eller jenv.
          </BodyLong>

          <LinkableHeading id="foojay" size="small" level="3">
            Unable to download toolchain
          </LinkableHeading>
          <BodyLong>
            Gradle prøver å laste ned en JDK til <code className={code}>~/.gradle/jdks</code>, og den mappa er
            skrivebeskyttet i cplt på macOS. Feilen ser ut som et nettverksproblem, men det er skrivingen som stoppes.
            Kjør bygget én gang utenfor cplt, så ligger JDK-en klar. Eller bruk en JDK du har installert, i{" "}
            <code className={code}>~/.gradle/gradle.properties</code>:
          </BodyLong>
          <CodeBlock compact>
            {`org.gradle.java.installations.auto-download=false
org.gradle.java.installations.paths=/Library/Java/JavaVirtualMachines/temurin-25.jdk/Contents/Home`}
          </CodeBlock>
          <BodyLong>
            cplt stenger <code className={code}>~/.gradle/gradle.properties</code>. Legger du innstillingene der, må du
            åpne fila, se neste oppføring.
          </BodyLong>
          <BodyLong>
            Har du en liste over tillatte hoster, stopper cplt også oppslaget mot{" "}
            <code className={code}>api.foojay.io</code> med <code className={code}>BLOCKED-ALLOWLIST</code> i
            proxyloggen, og Gradle kan melde det som tidsavbrudd. Legg til hosten, og hostene JDK-en hentes fra. For
            Temurin er det <code className={code}>github.com</code> og{" "}
            <code className={code}>release-assets.githubusercontent.com</code>, de samme som Gradle-wrapperen bruker:
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set allow.domains api.foojay.io
cplt config set allow.domains github.com
cplt config set allow.domains release-assets.githubusercontent.com`}
          </CodeBlock>

          <LinkableHeading id="gradle-properties" size="small" level="3">
            Error when loading properties file
          </LinkableHeading>
          <BodyLong>
            Hele meldingen er{" "}
            <code className={code}>Error when loading properties file=/Users/…/.gradle/gradle.properties</code> med{" "}
            <code className={code}>(Operation not permitted)</code>. Gjelder macOS. cplt stenger fila fordi den ofte har
            tokens i seg, og Gradle stopper når fila finnes, men ikke kan leses. Ta tokenet ut og hent pakkene fra{" "}
            <NextLink href="/nav-pilot/guider/cplt-gradle#speilet" className={linkClass}>
              Navs speil
            </NextLink>
            , eller gi agenten lesetilgang til fila og alle tokenene i den:
          </BodyLong>
          <CodeBlock compact>{`cplt config set allow.read ~/.gradle/gradle.properties`}</CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="nettleser" size="medium" level="2">
            Playwright, Cypress og Puppeteer
          </LinkableHeading>

          <LinkableHeading id="playwright" size="small" level="3">
            forbidden-sandbox-reinit
          </LinkableHeading>
          <BodyLong>
            Chromium prøver å starte sin egen sandkasse, og det går ikke inne i cplt. Playwright MCP slår den på. Selve
            Playwright-biblioteket kjører uten. Playwright kjører dessuten nettleseren fra{" "}
            <code className={code}>ms-playwright</code> i cache-mappa, som cplt ikke lar programmer kjøre fra. Da står
            det <code className={code}>Operation not permitted</code>. Én innstilling løser begge:
          </BodyLong>
          <CodeBlock compact>{`cplt config set sandbox.allow_cache_exec ms-playwright`}</CodeBlock>
          <BodyLong>
            Mappa blir både skrivbar og kjørbar for agenten, og cplt slår av sandkassen til Chromium for Playwright MCP.
            cplt er da den eneste sandkassen rundt nettleseren. Starter du Chromium på en annen måte, sett{" "}
            <code className={code}>--no-sandbox</code> selv.
          </BodyLong>

          <LinkableHeading id="cypress" size="small" level="3">
            EPERM (1100)
          </LinkableHeading>
          <BodyLong>
            <code className={code}>cypress verify</code> stopper under oppstart fordi Cypress kjører fra cache-mappa og
            trenger sitt eget område under <code className={code}>~/Library/Application Support/Cypress</code>.
            Innstillingen <code className={code}>Cypress</code> åpner begge. Testkjøringer trenger i tillegg tilfeldige
            porter på localhost. Foreslå det i repoet, slik at alle i teamet får det:
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set sandbox.allow_cache_exec Cypress
cplt config set --repo sandbox.allow_localhost_any true   # skriver til .cplt.toml
cplt trust accept --all                                    # godkjenn forslaget`}
          </CodeBlock>

          <LinkableHeading id="puppeteer" size="small" level="3">
            spawn EPERM og [object Object] fra Puppeteer og mmdc
          </LinkableHeading>
          <BodyLong>
            <code className={code}>Error: spawn EPERM</code> betyr at Puppeteer prøver å starte Chrome fra{" "}
            <code className={code}>~/.cache/puppeteer</code>. På macOS får ingenting kjøre derfra, og{" "}
            <code className={code}>--allow-cache-exec</code> når ikke den mappa. Skriver{" "}
            <code className={code}>mmdc</code> bare <code className={code}>[object Object]</code>, har Puppeteer prøvd å
            koble til nettleseren over localhost. Løsningen er en <code className={code}>puppeteer.json</code> som peker
            på nettleseren fra Playwright og har <code className={code}>{'"pipe": true'}</code> og{" "}
            <code className={code}>--no-sandbox</code>. Oppskriften står under{" "}
            <NextLink href="/nav-pilot/guider/cplt-node#mermaid" className={linkClass}>
              Puppeteer og Mermaid-diagrammer
            </NextLink>{" "}
            i Node-guiden.
          </BodyLong>
          <BodyLong>
            <code className={code}>npx -y @mermaid-js/mermaid-cli</code> stopper før det, med{" "}
            <code className={code}>bad interpreter: Operation not permitted</code>, fordi ingenting får kjøre fra{" "}
            <code className={code}>~/.npm</code>. Installer pakken i prosjektet med{" "}
            <code className={code}>npm install -D @mermaid-js/mermaid-cli</code>.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="git" size="medium" level="2">
            Git og GitHub
          </LinkableHeading>

          <LinkableHeading id="git-config" size="small" level="3">
            error: could not write config file .git/config: Operation not permitted
          </LinkableHeading>
          <BodyLong>
            Gjelder macOS. <code className={code}>.git/config</code> er skrivebeskyttet, fordi den kan få git til å
            kjøre kode utenfor sandkassen. Derfor fjerner cplt <code className={code}>-u</code> fra{" "}
            <code className={code}>git push -u</code> og skriver <code className={code}>cplt: pushing without -u</code>.
            Branchen blir pushet, og cplt setter upstream når økta er ferdig hvis pushen oppfyller kravene i{" "}
            <NextLink href="/nav-pilot/guider/cplt-git#push" className={linkClass}>
              Git og GitHub i sandkassen
            </NextLink>
            . Ellers skriver cplt en <code className={code}>git branch -u</code>-kommando du kjører selv utenfor cplt:
            når økta slutter, eller i meldingen når du pusher i en stille økt. Du klarer deg også uten upstream. Skriv
            branchnavnet når du pusher, og oppgi det når du lager pull requesten:
          </BodyLong>
          <CodeBlock compact>
            {`git push origin HEAD:min-branch
gh pr create --head min-branch`}
          </CodeBlock>
          <BodyLong>
            <code className={code}>git config</code>, <code className={code}>git remote add</code>,{" "}
            <code className={code}>git clone</code> og <code className={code}>git init</code> må du kjøre utenfor cplt.
          </BodyLong>

          <LinkableHeading id="git-push" size="small" level="3">
            BLOCKED by sandbox: &apos;git push&apos; is not allowed in this environment.
          </LinkableHeading>
          <BodyLong>
            Git-vakta stopper push til default branch og force push. <code className={code}>main</code> og{" "}
            <code className={code}>master</code> er alltid beskyttet, i tillegg til branchen{" "}
            <code className={code}>origin</code> peker på. Med <code className={code}>strict</code> stopper vakta all
            push. Lag en egen branch og push den:
          </BodyLong>
          <CodeBlock compact>{`git push origin HEAD:min-branch`}</CodeBlock>
          <BodyLong>
            <code className={code}>cplt doctor</code> viser om vakta slipper gjennom push til en ny branch i repoet
            ditt. Stopper vakta også push til en ny branch, og sier at{" "}
            <code className={code}>the default branch of remote &apos;origin&apos; could not be determined</code>, vet
            den ikke hvilken branch som er default branch. cplt spør remoten når{" "}
            <code className={code}>origin/HEAD</code> mangler lokalt, og feilen kommer når remoten ikke svarer. Kjør
            dette utenfor cplt, og start en ny økt:
          </BodyLong>
          <CodeBlock compact>{`git remote set-head origin -a`}</CodeBlock>

          <LinkableHeading id="git-remote" size="small" level="3">
            BLOCKED by sandbox: &apos;git remote&apos; would change a remote&apos;s URL.
          </LinkableHeading>
          <BodyLong>
            gh-vakta bruker <code className={code}>origin</code> til å avgjøre hvilket repo agenten får jobbe mot.
            Derfor stopper git-vakta <code className={code}>git remote add origin</code>,{" "}
            <code className={code}>git remote set-url origin</code> og <code className={code}>git config</code> med{" "}
            <code className={code}>remote.origin.url</code> eller <code className={code}>url.*.insteadOf</code>. Det
            gjelder overalt i sandkassen, også i test-repoer i en temp-mappe, og det er med vilje. Andre remotes kan du
            endre. Lager testene dine egne repoer med <code className={code}>origin</code>, kjør dem uten git-vakta:
          </BodyLong>
          <CodeBlock compact>{`cplt --no-git-guard exec -- <kommando>`}</CodeBlock>
          <BodyLong>
            Alternativt kan vakta bare advare. Det gjelder alle øktene dine, så slå den på igjen etterpå:
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set git_guard.mode warn --force
cplt config set git_guard.mode block`}
          </CodeBlock>

          <LinkableHeading id="gh-scope" size="small" level="3">
            which is outside the startup repo
          </LinkableHeading>
          <BodyLong>
            gh-vakta lar agenten jobbe mot repoet den startet i. Skal agenten jobbe i et repo til, legg det til. cplt
            finner utsjekkingen på maskinen din og sjekker at den er riktig repo:
          </BodyLong>
          <CodeBlock compact>{`cplt link navikt/annet-repo`}</CodeBlock>

          <LinkableHeading id="must-push" size="small" level="3">
            aborted: you must first push the current branch to a remote, or use the --head flag
          </LinkableHeading>
          <BodyLong>
            Uten terminal pusher ikke <code className={code}>gh pr create</code> for deg. Push først, og oppgi branchen:
          </BodyLong>
          <CodeBlock compact>
            {`git push origin HEAD:min-branch
gh pr create --head min-branch`}
          </CodeBlock>

          <LinkableHeading id="publickey" size="small" level="3">
            Permission denied (publickey)
          </LinkableHeading>
          <BodyLong>
            SSH-nøklene dine er stengt i sandkassen. Bruk HTTPS i stedet. Kjør dette utenfor cplt, fordi det skriver til
            git-konfigen:
          </BodyLong>
          <CodeBlock compact>
            {`git remote set-url origin https://github.com/navikt/<repo>.git
gh auth setup-git`}
          </CodeBlock>

          <LinkableHeading id="go-vcs" size="small" level="3">
            error obtaining VCS status: exit status 128
          </LinkableHeading>
          <BodyLong>
            Go-bygg i et git-worktree kan finne en annen <code className={code}>.git</code>-mappe lenger opp, som
            agenten ikke har tilgang til. Slå av versjonsstemplingen:
          </BodyLong>
          <CodeBlock compact>{`go build -buildvcs=false`}</CodeBlock>
          <BodyLong>
            <code className={code}>GOFLAGS</code> fra skallet ditt slipper inn i sandkassen. Hvordan du setter det for
            hele repoet med mise, står i{" "}
            <NextLink href="/nav-pilot/guider/worktrees#go-bygg" className={linkClass}>
              worktree-guiden
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="agent" size="medium" level="2">
            Agenten og innlogging
          </LinkableHeading>

          <LinkableHeading id="keyring" size="small" level="3">
            System vault not available
          </LinkableHeading>
          <BodyLong>
            Gjelder Linux og WSL. Nøkkelringen nås over D-Bus, som cplt stenger, så agenten ber deg lagre tokenet i
            klartekst. Logg inn på maskinen din, og la cplt sende tokenet inn:
          </BodyLong>
          <CodeBlock compact>
            {`gh auth login                                        # utenfor cplt, én gang
cplt config set gh_guard.inject_token true --force`}
          </CodeBlock>
          <BodyLong>
            Tokenet ligger da i miljøet til alle prosesser i sandkassen, og alle kan lese det. Derfor krever cplt{" "}
            <code className={code}>--force</code>.
          </BodyLong>

          <LinkableHeading id="copilot-sandbox" size="small" level="3">
            Sandboxing is disabled for this session because this host does not support it
          </LinkableHeading>
          <BodyLong>
            Copilot CLI har sin egen sandkasse, og den kan ikke starte inne i cplt. cplt slår den av for økten, og
            Copilot sier fra med denne linja. Det er ventet, og innstillingene dine endres ikke. cplt er fortsatt
            sandkassen rundt agenten.
          </BodyLong>
          <BodyLong>
            Henger Copilot rett etter oppstartsmeldingen, og organisasjonen din styrer Copilot-innstillingene, kan en
            policy kreve Copilots egen sandkasse. Det kan ikke cplt endre.
          </BodyLong>

          <LinkableHeading id="tcc" size="small" level="3">
            Operation not permitted i ~/Desktop eller ~/Documents
          </LinkableHeading>
          <BodyLong>
            Gjelder macOS, og skjer også uten cplt. macOS beskytter disse mappene. Gi terminalen full disktilgang i{" "}
            <strong>Systeminnstillinger → Personvern og sikkerhet → Full disktilgang</strong>, og start terminalen på
            nytt. Eller kopier fila inn i prosjektet.
          </BodyLong>

          <BodyLong>
            Andre problemer med nav-pilot står i{" "}
            <NextLink href="/nav-pilot/guider/feilsoking" className={linkClass}>
              feilsøkingsguiden
            </NextLink>
            , og hvordan sandkassen er satt opp, står i{" "}
            <NextLink href="/nav-pilot/forklaring/sandkassen" className={linkClass}>
              Sandkassen
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="rapporter" size="medium" level="2">
            Rapporter et problem
          </LinkableHeading>
          <BodyLong>
            Kjør <code className={code}>cplt doctor</code>, eller{" "}
            <code className={code}>cplt doctor --agent claude</code> for en bestemt agent, og lim inn hele utskriften
            sammen med kommandoen som feilet. Utskriften viser hjemmekatalogen som <code className={code}>~</code> og
            tokener bare med navn. Ikke send <code className={code}>--verbose</code>: den tar med fulle stier.
          </BodyLong>
          <CodeBlock compact>{`cplt doctor`}</CodeBlock>
        </VStack>
      </section>
    </DocPage>
  );
}
