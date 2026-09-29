import { BodyLong, Box, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";
import { NAV_PILOT_BREW_INSTALL, OPENCODE_INSTALL } from "@/lib/install-commands";

export const metadata: Metadata = {
  title: "Kom i gang på 5 minutter",
  description:
    "Installer nav-pilot og cplt, logg inn med gh, start agenten i et repo og velg hvor mye den skal gjøre selv.",
};

const TOC: TocItem[] = [
  { id: "installer", label: "Installer" },
  { id: "logg-inn", label: "Logg inn på GitHub" },
  { id: "forste-kjoring", label: "Første kjøring" },
  { id: "forste-repo", label: "Første gang i et repo" },
  { id: "hva-agenten-kan", label: "Hva agenten kan gjøre" },
  { id: "autonomi", label: "Hvor mye skal agenten gjøre selv?" },
  { id: "videre", label: "Videre" },
];

// The first nav-pilot release with the autonomy presets (navikt/copilot#1350).
const MIN_VERSION = "2026.09.29-221653";
// The first release that offers to remove the strict allowlist (navikt/copilot#1351).
const LIST_REMOVAL_VERSION = "2026.09.29-223958";

const INSTALL = `${NAV_PILOT_BREW_INSTALL}
brew install gh

# Velg klienten du vil bruke:
${OPENCODE_INSTALL.mac}                        # OpenCode
curl -fsSL https://gh.io/copilot-install | bash   # Copilot CLI
export PATH="$HOME/.local/bin:$PATH"              # der ligger copilot`;

const SSH_TO_HTTPS = `git config --global url."https://github.com/".insteadOf "git@github.com:"`;

export default function KomIGang() {
  return (
    <DocPage
      label="Guider"
      title="Kom i gang på 5 minutter"
      description="Fra installasjon til en agent som jobber i repoet ditt, med Copilot CLI eller OpenCode i sandkassen cplt."
      toc={TOC}
    >
      <BodyLong>
        Guiden gjelder nav-pilot {MIN_VERSION} eller nyere. Sjekk med <code className={code}>nav-pilot --version</code>.
        Har du en eldre versjon, kjør <code className={code}>brew update</code> og{" "}
        <code className={code}>brew upgrade navikt/tap/nav-pilot navikt/tap/cplt</code>.
      </BodyLong>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="installer" size="medium" level="2">
            Installer
          </LinkableHeading>
          <BodyLong>På Mac med Homebrew:</BodyLong>
          <CodeBlock compact>{INSTALL}</CodeBlock>
          <BodyLong>
            På Linux og i WSL bruker du installasjonsskriptet. Kommandoene står på{" "}
            <NextLink href="/kom-i-gang#installer" className={linkClass}>
              Kom i gang
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="logg-inn" size="medium" level="2">
            Logg inn på GitHub
          </LinkableHeading>
          <BodyLong>
            Agenten pusher og åpner pull requests med gh-innloggingen din. Logg inn én gang, utenfor sandkassen:
          </BodyLong>
          <CodeBlock compact>{`gh auth login`}</CodeBlock>
          <BodyLong>
            <strong>OpenCode eller pi på Mac:</strong> gh legger tokenet i nøkkelringen til macOS, og den slipper ikke
            cplt disse klientene til. Da feiler push og pull requests i sandkassen. Logg inn slik at gh lagrer tokenet i
            sin egen fil i stedet:
          </BodyLong>
          <CodeBlock compact>{`gh auth login --insecure-storage`}</CodeBlock>
          <BodyLong>
            <strong>Kloner du med SSH</strong> (<code className={code}>git@github.com:…</code>)? cplt stenger{" "}
            <code className={code}>~/.ssh</code>, så push over SSH feiler i sandkassen. Få git til å bruke HTTPS mot
            GitHub:
          </BodyLong>
          <CodeBlock compact>{SSH_TO_HTTPS}</CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="forste-kjoring" size="medium" level="2">
            Første kjøring
          </LinkableHeading>
          <BodyLong>Gå til et repo og start nav-pilot:</BodyLong>
          <CodeBlock compact>{`cd ~/src/min-app
nav-pilot`}</CodeBlock>
          <BodyLong>
            Første gang spør nav-pilot om noen innstillinger. Enter gir det anbefalte valget. Spørsmålene er på engelsk:
          </BodyLong>
          <Bullets>
            <li>
              <strong>Which coding agent?</strong> OpenCode, GitHub Copilot eller pi. OpenCode er valgt på forhånd.
            </li>
            <li>
              <strong>Conversation mode</strong>: vanlig samtale, plan eller autopilot.
            </li>
            <li>
              <strong>How much should the agent do on its own?</strong> Se{" "}
              <a href="#autonomi" className={linkClass}>
                Hvor mye skal agenten gjøre selv?
              </a>
            </li>
            <li>
              <strong>Model</strong>, <strong>Reasoning effort</strong> og <strong>Auto-update nav-pilot</strong>.
            </li>
          </Bullets>
          <BodyLong>
            Til slutt viser <strong>Save these settings?</strong> hva som blir endret, både i nav-pilot og i cplt.
            Velger du Cancel, lagres ingenting. Etterpå sjekker nav-pilot gh-innloggingen og sier hvilken kommando du må
            kjøre hvis push eller pull requests ikke vil virke.
          </BodyLong>
          <BodyLong>
            Så spør nav-pilot hvor agentpakka skal ligge (<strong>Where to install?</strong>): i repoet, som hele teamet
            får, eller i hjemmekatalogen, bare for deg.{" "}
            <NextLink href="/nav-pilot/guider/installere-og-oppgradere#velg-installasjonssted" className={linkClass}>
              Installere og oppgradere
            </NextLink>{" "}
            forklarer forskjellen. Deretter starter klienten i sandkassen.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="forste-repo" size="medium" level="2">
            Første gang i et repo
          </LinkableHeading>
          <BodyLong>
            Står du i et repo uten <code className={code}>.cplt.toml</code>, og cplt kjenner igjen verktøyene, for
            eksempel Gradle, spør nav-pilot: <strong>Apply the suggested sandbox rules for this repo?</strong> Svarer du
            ja, skriver <code className={code}>cplt init --write</code> fila. Den gir ingen tilgang før den er sjekket
            inn og godkjent:
          </BodyLong>
          <CodeBlock compact>{`git add .cplt.toml
git commit -m "chore: add cplt sandbox config"
cplt trust accept`}</CodeBlock>
          <BodyLong>
            <code className={code}>cplt trust accept</code> viser hva fila ber om, og spør før den godkjenner. Har
            repoet allerede en <code className={code}>.cplt.toml</code>, sier cplt fra ved oppstart om at den ikke er
            godkjent. Da kjører du bare <code className={code}>cplt trust accept</code>. Mer om fila står i{" "}
            <NextLink href="/nav-pilot/guider/cplt-oppsett" className={linkClass}>
              Sett opp cplt i et repo
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="hva-agenten-kan" size="medium" level="2">
            Hva agenten kan gjøre
          </LinkableHeading>
          <BodyLong>Med det anbefalte valget kan agenten:</BodyLong>
          <Bullets>
            <li>lese og skrive filer i katalogen du startet i, og kjøre kommandoer der</li>
            <li>committe, pushe egne grener og åpne pull requests</li>
          </Bullets>
          <BodyLong>cplt stopper:</BodyLong>
          <Bullets>
            <li>
              merging av pull requests (<code className={code}>gh pr merge</code>)
            </li>
            <li>push til standardgrenen, for eksempel main, og force push</li>
            <li>skriving utenfor prosjektkatalogen, og lesing av SSH-nøkler, skynøkler og andre hemmeligheter</li>
          </Bullets>
          <BodyLong>
            I tillegg er agenten nav-pilot bedt om å spørre deg før den sletter grener eller filer utenfor oppgaven,
            deployer, endrer CI eller tilganger, legger til avhengigheter, og når kravene er uklare. Det er en instruks,
            ikke en sperre. Det er cplt som sperrer.
          </BodyLong>
          <BodyLong>
            Starter du uten cplt, med <code className={code}>--no-sandbox</code>, sender nav-pilot ingen flagg som lar
            agenten jobbe uten å spørre. Da spør Copilot CLI før hver handling, og OpenCode følger sine egne
            tillatelser.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="autonomi" size="medium" level="2">
            Hvor mye skal agenten gjøre selv?
          </LinkableHeading>
          <BodyLong>
            nav-pilot tilbyr fire nivåer. På alle fire stopper cplt merging, push til standardgrenen og force push.
            Unntaket er hvis du under Custom beholder cplt-nivået <code className={code}>permissive</code> eller{" "}
            <code className={code}>full-trust</code>. Der er sperrene av, og ingenting blir stoppet.
          </BodyLong>
          <Bullets>
            <li>
              <strong>Autonomous in the sandbox (recommended).</strong> Agenten jobber uten å spørre for hver kommando,
              innenfor sandkassen. Den committer, pusher egne grener og åpner pull requests, og kan fortsatt spørre deg
              når den er usikker. For Copilot CLI sender nav-pilot{" "}
              <code className={code}>--allow-all-tools --allow-all-paths --allow-all-urls</code>, men bare når klienten
              kjører i cplt.
            </li>
            <li>
              <strong>Ask before each command.</strong> Samme git-regler, men Copilot CLI spør før hver kommando. Bare
              for Copilot CLI.
            </li>
            <li>
              <strong>Locked down.</strong> Agenten kan ikke pushe i det hele tatt, så du pusher selv. Nettverket er
              begrenset til en liste over tillatte verter, og Copilot CLI spør før hver kommando. nav-pilot setter cplt
              til <code className={code}>strict</code> og legger Navs verter i lista først.
            </li>
            <li>
              <strong>Custom.</strong> Du velger hver del selv: om Copilot CLI skal spørre før hver kommando, vanlig
              nettverk eller bare lista, og om agenten kan pushe og åpne pull requests eller bare committe.
            </li>
          </Bullets>
          <BodyLong>
            Bruker du OpenCode eller pi, får du ikke valget Ask before each command. Der styrer klienten selv hva den
            spør om. For OpenCode er det nøkkelen <code className={code}>permission</code> i{" "}
            <code className={code}>opencode.json</code>.
          </BodyLong>
          <Box background="warning-soft" borderRadius="8" padding="space-16">
            <VStack gap="space-8">
              <BodyLong>
                <strong>Locked down koster noe.</strong> Agenten kan ikke pushe en gren eller åpne en pull request fra
                den, så du må pushe selv. Verter som ikke står på lista, blir blokkert. Det gjelder også interne verter
                du tar i bruk senere. Lista ligger i <code className={code}>~/.nav-pilot/cplt-allowed-domains.txt</code>
                . På Linux krever nivået kjerne 6.7 eller nyere med Landlock slått på.
              </BodyLong>
              <BodyLong>
                Bytter du fra Locked down til et annet nivå, spør nav-pilot fra versjon {LIST_REMOVAL_VERSION}:{" "}
                <strong>Remove the network allowlist nav-pilot set up for strict?</strong> Svarer du nei, eller har du
                en eldre versjon, blir lista stående, og agenten når fortsatt bare vertene på den. Fjern den slik:
              </BodyLong>
              <CodeBlock compact>{`cplt config set proxy.allowed_domains --unset --global`}</CodeBlock>
            </VStack>
          </Box>
          <BodyLong>Du kan bytte nivå når som helst:</BodyLong>
          <CodeBlock compact>{`nav-pilot config setup`}</CodeBlock>
          <BodyLong>
            nav-pilot spør om du vil erstatte innstillingsfila. Velg Replace. Valgene du har nå, er valgt på forhånd, så
            du trenger bare endre det du vil endre. Før noe lagres, ser du hva som blir endret.
          </BodyLong>
          <BodyLong>
            Brukte du nav-pilot med Copilot CLI før nivåene kom, har du Ask before each command til du velger noe annet.
            Hele sammenligningen av <code className={code}>standard</code> og <code className={code}>strict</code> står
            i{" "}
            <NextLink href="/nav-pilot/forklaring/sandkassen#sikkerhetsniva" className={linkClass}>
              Sandkassen
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="videre" size="medium" level="2">
            Videre
          </LinkableHeading>
          <Bullets>
            <li>
              Stopper cplt noe agenten trenger? Slå opp meldingen i{" "}
              <NextLink href="/nav-pilot/guider/cplt-feilmeldinger" className={linkClass}>
                Feil i sandkassen
              </NextLink>
              .
            </li>
            <li>
              Vil du la agenten jobbe på flere grener samtidig? Se{" "}
              <NextLink href="/nav-pilot/guider/worktrees" className={linkClass}>
                Worktrees med nav-pilot og cplt
              </NextLink>
              .
            </li>
            <li>
              Trenger agenten verktøy utenfor klienten, som Playwright? Se{" "}
              <NextLink href="/nav-pilot/klienter#mcp-register" className={linkClass}>
                Navs MCP-register
              </NextLink>
              .
            </li>
          </Bullets>
        </VStack>
      </section>
    </DocPage>
  );
}
