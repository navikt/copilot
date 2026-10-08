import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Node, npm og pnpm i sandkassen",
  description:
    "Slik installerer, bygger og tester du Node-prosjekter med npm, pnpm og yarn når nav-pilot og Copilot kjører i cplt: @navikt-pakker, installasjonsskript, .env-filer, localhost, Playwright, Cypress, Puppeteer og npx.",
};

const TOC: TocItem[] = [
  { id: "github-packages", label: "@navikt-pakker fra GitHub Packages" },
  { id: "skript", label: "Skript som kjører ved installasjon" },
  { id: "env-filer", label: ".env-filer" },
  { id: "localhost", label: "Utviklingsserveren på localhost" },
  { id: "nettleser", label: "Playwright og Cypress" },
  { id: "mermaid", label: "Puppeteer og Mermaid-diagrammer" },
  { id: "pnpm-konfig", label: "pnpm-konfig og tokens" },
  { id: "globale", label: "Globale installasjoner" },
  { id: "tillatelsesliste", label: "Pakkeregistre og en liste over tillatte verter" },
  { id: "feil", label: "Når installasjonen feiler" },
];

const FAQ = "/nav-pilot/guider/cplt-feilmeldinger";
const OPPSETT = "/nav-pilot/guider/cplt-oppsett";
const KNOWN_IMPACTS = "https://github.com/navikt/cplt/blob/main/docs/known-impacts.md";

export default function CpltNode() {
  return (
    <DocPage
      label="Guider"
      upgrade
      title="Node, npm og pnpm i sandkassen"
      description="Det du trenger for at npm, pnpm og yarn skal virke når agenten kjører i cplt."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="github-packages" size="medium" level="2">
            @navikt-pakker fra GitHub Packages
          </LinkableHeading>
          <BodyLong>
            <code className={code}>npm.pkg.github.com</code> svarer <code className={code}>401 Unauthorized</code> uten
            token, også for offentlige pakker, og cplt stenger <code className={code}>~/.npmrc</code> der tokenet
            ligger. Det enkleste er å kjøre <code className={code}>pnpm install</code> utenfor cplt før du starter
            agenten. Hvorfor, og hva det koster å åpne tokenfilene, står i{" "}
            <NextLink href={`${OPPSETT}#github-packages`} className={linkClass}>
              Pakker fra GitHub Packages
            </NextLink>
            . Må agenten installere selv, kommer det an på pakkebehandleren.
          </BodyLong>

          <LinkableHeading id="npm-token" size="small" level="3">
            npm: token fra en miljøvariabel
          </LinkableHeading>
          <BodyLong>
            npm leser en miljøvariabel i prosjektets <code className={code}>.npmrc</code>. Da trenger agenten ikke{" "}
            <code className={code}>~/.npmrc</code>:
          </BodyLong>
          <CodeBlock filename=".npmrc">
            {`@navikt:registry=https://npm.pkg.github.com
//npm.pkg.github.com/:_authToken=\${NODE_AUTH_TOKEN}`}
          </CodeBlock>
          <BodyLong>
            cplt fjerner miljøvariabler som kan være hemmeligheter, så variabelen må sendes inn. Bruk et token som bare
            kan lese pakker (<code className={code}>read:packages</code>):
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set sandbox.pass_env NODE_AUTH_TOKEN   # alltid
cplt --pass-env NODE_AUTH_TOKEN                    # bare denne økten`}
          </CodeBlock>
          <BodyLong>
            Uten variabelen sender npm teksten <code className={code}>{"${NODE_AUTH_TOKEN}"}</code> som token, og får
            401.
          </BodyLong>

          <LinkableHeading id="pnpm-token" size="small" level="3">
            pnpm: trenger ~/.npmrc
          </LinkableHeading>
          <BodyLong>
            pnpm leser ikke miljøvariabler i prosjektets <code className={code}>.npmrc</code>, så tokenet må ligge i{" "}
            <code className={code}>~/.npmrc</code>. Gi agenten lesetilgang til fila, og dermed til alle tokenene i den:
          </BodyLong>
          <CodeBlock compact>{`cplt config set allow.read ~/.npmrc`}</CodeBlock>
          <BodyLong>
            <code className={code}>cplt init</code> og <code className={code}>cplt doctor</code> sier fra når
            prosjektets <code className={code}>.npmrc</code> henter et scope fra GitHub Packages.
          </BodyLong>

          <LinkableHeading id="yarn" size="small" level="3">
            yarn 1
          </LinkableHeading>
          <BodyLong>
            yarn 1 stopper med <code className={code}>EPERM: operation not permitted, open …/.npmrc</code> når{" "}
            <code className={code}>~/.npmrc</code> finnes, også når prosjektet bare henter offentlige pakker. Det
            hjelper ikke å peke <code className={code}>NPM_CONFIG_USERCONFIG</code> et annet sted, for yarn 1 leter også
            oppover fra prosjektmappa. Gi lesetilgang til fila, som for pnpm. Har du en{" "}
            <code className={code}>~/.yarnrc</code>, trenger den det samme, se{" "}
            <NextLink href={`${FAQ}#yarnrc`} className={linkClass}>
              .yarnrc
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="skript" size="medium" level="2">
            Skript som kjører ved installasjon
          </LinkableHeading>
          <BodyLong>
            Skript som <code className={code}>postinstall</code> og <code className={code}>prepare</code> kan kjøre hva
            som helst når en pakke installeres, og er en vanlig vei inn for skadelig kode. cplt slår dem av for npm,
            pnpm og yarn 1. <code className={code}>npm run build</code>, <code className={code}>npm test</code> og andre
            skript du kjører selv, virker som vanlig.
          </BodyLong>
          <BodyLong>
            Pakker som bygger noe ved installasjon, som <code className={code}>sharp</code>,{" "}
            <code className={code}>bcrypt</code> og <code className={code}>esbuild</code>, kan da feile. Installer dem
            utenfor cplt, eller slå skriptene på:
          </BodyLong>
          <CodeBlock compact>
            {`cplt --allow-lifecycle-scripts                                        # bare denne økten
cplt config set sandbox.allow_lifecycle_scripts true --force          # alltid
cplt config set --repo sandbox.allow_lifecycle_scripts true --force   # i .cplt.toml`}
          </CodeBlock>
          <BodyLong>
            cplt krever <code className={code}>--force</code>, fordi alle pakkene i treet da kan kjøre kode i
            sandkassen.
          </BodyLong>
          <BodyLong>
            Sperren gjelder også pnpm 11 og nyere og yarn 1, men ikke yarn 2 og nyere: de kjører prosjektets eget
            postinstall. Bruker du cplt fra før 30. september 2026, kjører de prosjektets egne skript. Da må du si fra
            selv:
          </BodyLong>
          <CodeBlock compact>
            {`pnpm install --ignore-scripts
yarn install --ignore-scripts`}
          </CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="env-filer" size="medium" level="2">
            .env-filer
          </LinkableHeading>
          <BodyLong>
            På macOS kan agenten verken lese eller skrive <code className={code}>.env</code> og{" "}
            <code className={code}>.env.*</code> i prosjektet. Next.js og dotenv får tomme variabler, og tester som
            lager slike filer, feiler med <code className={code}>Operation not permitted</code>. På Linux gjelder ikke
            sperren.
          </BodyLong>
          <BodyLong>Trenger agenten filene, åpne dem:</BodyLong>
          <CodeBlock compact>
            {`cplt --allow-env-files                          # bare denne økten
cplt config set sandbox.allow_env_files true    # alltid`}
          </CodeBlock>
          <BodyLong>
            Da kan agenten lese alle hemmelighetene i filene, også <code className={code}>.pem</code>- og{" "}
            <code className={code}>.key</code>-filer. Er en <code className={code}>.env</code>-fil sporet i git, stopper{" "}
            <code className={code}>git diff</code> og <code className={code}>git add</code>, se{" "}
            <NextLink href={`${FAQ}#cannot-hash`} className={linkClass}>
              cannot hash .env.local
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="localhost" size="medium" level="2">
            Utviklingsserveren på localhost
          </LinkableHeading>
          <BodyLong>
            Agenten kan starte <code className={code}>npm run dev</code> i sandkassen, og du når serveren fra
            nettleseren som vanlig. Men agenten selv når ikke localhost, så den kan ikke hente sider fra serveren eller
            kjøre tester mot den. Åpne porten:
          </BodyLong>
          <CodeBlock compact>{`cplt config set allow.localhost 3000`}</CodeBlock>
          <BodyLong>
            Next.js med Turbopack, Vite og esbuild starter arbeidsprosesser som snakker sammen over tilfeldige porter på
            localhost. Med bare port 3000 åpen starter Next.js, men første side feiler. Da må alle portene åpnes:
          </BodyLong>
          <CodeBlock compact>{`cplt config set sandbox.allow_localhost_any true`}</CodeBlock>
          <BodyLong>
            Hva det åpner, hva det koster på Linux, hvorfor det ikke virker med tvungen proxy, og hvordan du legger det
            i repoet for hele teamet, står i{" "}
            <NextLink href={`${OPPSETT}#localhost`} className={linkClass}>
              Tjenester på localhost
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="nettleser" size="medium" level="2">
            Playwright og Cypress
          </LinkableHeading>
          <BodyLong>
            Begge kjører nettleseren fra cache-mappa, og cplt lar ikke programmer kjøre derfra. Åpne mappa for verktøyet
            du bruker:
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set sandbox.allow_cache_exec ms-playwright
cplt config set sandbox.allow_cache_exec Cypress`}
          </CodeBlock>
          <BodyLong>
            Innstillingen gjelder maskinen din og kan ikke stå i <code className={code}>.cplt.toml</code>. Cypress
            trenger i tillegg <code className={code}>allow_localhost_any</code> for å kjøre testene.
          </BodyLong>
          <BodyLong>
            Chromium prøver å starte sin egen sandkasse, og det går ikke inne i cplt. For Playwright MCP slår cplt den
            av når <code className={code}>ms-playwright</code> er åpnet. Starter du Chromium på en annen måte, sett{" "}
            <code className={code}>--no-sandbox</code> selv. Feilmeldingene står under{" "}
            <NextLink href={`${FAQ}#nettleser`} className={linkClass}>
              Playwright og Cypress
            </NextLink>{" "}
            i Feil i sandkassen.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="mermaid" size="medium" level="2">
            Puppeteer og Mermaid-diagrammer
          </LinkableHeading>
          <BodyLong>
            Med mermaid-cli (<code className={code}>mmdc</code>) kan agenten gjøre Mermaid-diagrammene sine om til PNG
            og se på bildet selv. <code className={code}>mmdc</code> starter Chrome gjennom Puppeteer, og det stopper i
            cplt på tre steder:
          </BodyLong>
          <Bullets>
            <li>
              Puppeteer laster ned Chrome til <code className={code}>~/.cache/puppeteer</code>. På macOS får ingenting
              kjøre derfra, og <code className={code}>--allow-cache-exec</code> når ikke den mappa. Feilen er{" "}
              <code className={code}>Error: spawn EPERM</code>.
            </li>
            <li>
              Chromium må startes med <code className={code}>--no-sandbox</code>.
            </li>
            <li>
              Puppeteer snakker med nettleseren over en WebSocket på localhost, og cplt stenger localhost.{" "}
              <code className={code}>mmdc</code> skriver da bare <code className={code}>[object Object]</code>.
            </li>
          </Bullets>
          <BodyLong>
            Bruk nettleseren fra Playwright i stedet. Installer <code className={code}>mmdc</code> i prosjektet,
            installer nettleseren utenfor cplt og åpne mappa:
          </BodyLong>
          <CodeBlock compact>
            {`npm install -D @mermaid-js/mermaid-cli
npx playwright install chromium-headless-shell
cplt config set sandbox.allow_cache_exec ms-playwright
ls ~/Library/Caches/ms-playwright     # finn versjonen, f.eks. chromium_headless_shell-1243`}
          </CodeBlock>
          <BodyLong>
            Pek Puppeteer på nettleseren med full sti. <code className={code}>{'"pipe": true'}</code> gjør at Puppeteer
            snakker med nettleseren over en pipe i stedet for localhost. Stien under gjelder Mac med Apple-brikke; på
            Mac med Intel heter mappa <code className={code}>chrome-headless-shell-mac-x64</code>:
          </BodyLong>
          <CodeBlock filename="puppeteer.json">
            {`{
  "executablePath": "/Users/<deg>/Library/Caches/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell",
  "args": ["--no-sandbox"],
  "pipe": true
}`}
          </CodeBlock>
          <CodeBlock compact>{`npx mmdc -p puppeteer.json -i diagram.mmd -o diagram.png`}</CodeBlock>
          <BodyLong>
            Vil du heller bruke Puppeteers egen Chrome, legg cachen under Playwright-mappa, som allerede er åpnet, og
            last ned nettleseren dit utenfor cplt. Da trenger du ikke <code className={code}>executablePath</code>, men{" "}
            <code className={code}>args</code> og <code className={code}>pipe</code> må fortsatt stå i{" "}
            <code className={code}>puppeteer.json</code>:
          </BodyLong>
          <CodeBlock compact>
            {`export PUPPETEER_CACHE_DIR=~/Library/Caches/ms-playwright/puppeteer
npx puppeteer browsers install chrome-headless-shell
cplt --pass-env PUPPETEER_CACHE_DIR`}
          </CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="pnpm-konfig" size="medium" level="2">
            pnpm-konfig og tokens
          </LinkableHeading>
          <BodyLong>
            <code className={code}>pnpm login</code> lagrer tokenet i <code className={code}>auth.ini</code> i pnpms
            konfigmappe (pnpm 10 og eldre: <code className={code}>rc</code>). På macOS er det{" "}
            <code className={code}>~/Library/Preferences/pnpm</code>, der agenten bare får lese{" "}
            <code className={code}>config.yaml</code>. Installerer du fra et privat register med det tokenet, gi
            lesetilgang til fila:
          </BodyLong>
          <CodeBlock compact>{`cplt config set allow.read ~/Library/Preferences/pnpm/auth.ini`}</CodeBlock>
          <BodyLong>
            På Linux, og på macOS med <code className={code}>XDG_CONFIG_HOME</code> satt, ligger mappa i{" "}
            <code className={code}>$XDG_CONFIG_HOME/pnpm</code>, som regel <code className={code}>~/.config/pnpm</code>.
            Den kan agenten både lese og skrive, også tokenfilene. Vil du stenge dem, slå på:
          </BodyLong>
          <CodeBlock compact>{`cplt config set sandbox.protect_pnpm_config true`}</CodeBlock>
          <BodyLong>
            Den er av som standard, fordi <code className={code}>pnpm install</code> fra et privat register da feiler
            med 401 hvis tokenet bare ligger i tokenfila, og <code className={code}>pnpm login</code> ikke virker i
            sandkassen. <code className={code}>config.yaml</code> kan pnpm fortsatt lese.
          </BodyLong>
          <BodyLong>
            Hvorfor pnpm laster ned alle pakkene på nytt i hvert repo, og hvordan du bruker det felles lageret, står
            under{" "}
            <NextLink href={`${OPPSETT}#pnpm`} className={linkClass}>
              pnpm
            </NextLink>{" "}
            i oppsettguiden.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="globale" size="medium" level="2">
            Globale installasjoner
          </LinkableHeading>
          <BodyLong>
            <code className={code}>pnpm add -g</code> og <code className={code}>npm install -g</code> feiler i cplt, det
            samme gjør <code className={code}>pnpm setup</code> og <code className={code}>pnpm env use -g</code>. Det er
            med vilje. Mappene de skriver til, ligger foran <code className={code}>/usr/bin</code> i{" "}
            <code className={code}>PATH</code>, så et program agenten la der, ville du selv kjørt neste gang, utenfor
            sandkassen. Kjør globale installasjoner i en vanlig terminal.
          </BodyLong>
          <BodyLong>
            Installasjoner i prosjektet, som <code className={code}>npm install</code> og{" "}
            <code className={code}>pnpm install</code>, virker som vanlig.
          </BodyLong>
          <BodyLong>
            <code className={code}>npx &lt;pakke&gt;</code> for en pakke som ikke ligger i prosjektet, stopper med{" "}
            <code className={code}>/usr/bin/env: bad interpreter: Operation not permitted</code>, fordi npx legger
            pakken i <code className={code}>~/.npm/_npx</code>, og der får agenten ikke kjøre noe. Legg pakken i
            prosjektet først, så virker <code className={code}>npx</code>:
          </BodyLong>
          <CodeBlock compact>
            {`npm install -D semver
npx semver 1.2.3`}
          </CodeBlock>
          <BodyLong>
            Det samme gjelder <code className={code}>npx -y @mermaid-js/mermaid-cli</code>, se{" "}
            <NextLink href="#mermaid" className={linkClass}>
              Puppeteer og Mermaid-diagrammer
            </NextLink>
            . Trenger agenten npx for pakker utenfor prosjektet, flytt npm-cachen inn i cache-mappa og åpne bare
            npx-mappa der. Legg variabelen i <code className={code}>~/.zshrc</code> eller{" "}
            <code className={code}>~/.bashrc</code>. <code className={code}>cplt config set</code>-linjene kjører du én
            gang:
          </BodyLong>
          <CodeBlock compact>
            {`export npm_config_cache="$HOME/Library/Caches/npm"   # macOS
cplt config set sandbox.pass_env npm_config_cache
cplt config set sandbox.allow_cache_exec npm/_npx`}
          </CodeBlock>
          <BodyLong>
            npm starter da med en ny cache, og alle pakkene npx henter, kan kjøre i sandkassen. På Linux er cache-mappa{" "}
            <code className={code}>~/.cache</code>, så der blir stien <code className={code}>$HOME/.cache/npm</code>.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="tillatelsesliste" size="medium" level="2">
            Pakkeregistre og en liste over tillatte verter
          </LinkableHeading>
          <BodyLong>
            Uten en liste over tillatte verter virker installasjoner som vanlig. Hvilke registre lista har med, står i{" "}
            <NextLink href={`${OPPSETT}#tillatelsesliste`} className={linkClass}>
              Pakkeregistre og en liste over tillatte verter
            </NextLink>
            . npm og yarn er med. Vertene Playwright og Cypress laster ned nettlesere fra, er ikke med. Sjekk en vert og
            legg den til:
          </BodyLong>
          <CodeBlock compact>
            {`cplt check net cdn.playwright.dev
cplt config set allow.domains cdn.playwright.dev`}
          </CodeBlock>
          <BodyLong>
            Enklere er det å installere nettleserne utenfor cplt, med{" "}
            <code className={code}>npx playwright install</code> eller <code className={code}>npx cypress install</code>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="feil" size="medium" level="2">
            Når installasjonen feiler
          </LinkableHeading>
          <BodyLong>Slå på loggen over blokkerte forbindelser, og kjør installasjonen i sandkassen selv:</BodyLong>
          <CodeBlock compact>
            {`cplt config set proxy.log_level blocked
cplt exec -- pnpm install`}
          </CodeBlock>
          <BodyLong>
            Linjer med <code className={code}>[proxy]</code> og <code className={code}>BLOCKED</code> viser hvilken vert
            som ble stoppet. Nektes du en fil, spør cplt om stien med{" "}
            <code className={code}>cplt check path &lt;sti&gt;</code>. Feilmeldingene står i{" "}
            <NextLink href={`${FAQ}#filer`} className={linkClass}>
              Feil i sandkassen
            </NextLink>
            . Mer om hvorfor står i{" "}
            <a href={KNOWN_IMPACTS} className={linkClass}>
              known-impacts.md
            </a>{" "}
            (engelsk).
          </BodyLong>
          <BodyLong>
            Kommandoene på denne siden er testet med cplt fra 30. september 2026 på macOS, med npm 11, pnpm 10, 11 og 12
            og yarn 1.
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
