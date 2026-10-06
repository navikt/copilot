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

// The first nav-pilot release with the two autonomy questions (navikt/copilot#1359).
const MIN_VERSION = "2026.09.30-072123";
// The first release that offers to remove the strict allowlist (navikt/copilot#1351).
const LIST_REMOVAL_VERSION = "2026.09.29-223958";

// One block per copy target: the copy button copies the whole block, and the
// two clients are alternatives. No trailing comments: zsh on macOS does not
// allow them in an interactive shell by default.
const INSTALL = `${NAV_PILOT_BREW_INSTALL}
brew install gh`;

const COPILOT_INSTALL = `curl -fsSL https://gh.io/copilot-install | bash
export PATH="$HOME/.local/bin:$PATH"`;

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
          <BodyLong>Installer så én av klientene. OpenCode:</BodyLong>
          <CodeBlock compact>{OPENCODE_INSTALL.mac}</CodeBlock>
          <BodyLong>
            Eller Copilot CLI. Den havner i <code className={code}>~/.local/bin</code>, så den andre linja legger den
            mappa til i PATH:
          </BodyLong>
          <CodeBlock compact>{COPILOT_INSTALL}</CodeBlock>
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
            <strong>OpenCode eller pi på Mac:</strong> gh legger tokenet i nøkkelringen til macOS, og cplt slipper ikke
            disse klientene til nøkkelringen. Da feiler push og pull requests i sandkassen. Logg inn slik at gh lagrer
            tokenet i sin egen fil i stedet:
          </BodyLong>
          <CodeBlock compact>{`gh auth login --insecure-storage`}</CodeBlock>
          <BodyLong>
            <strong>Kloner du med SSH</strong> (<code className={code}>git@github.com:…</code>)? cplt stenger{" "}
            <code className={code}>~/.ssh</code>, så push over SSH feiler i sandkassen. Få git til å bruke HTTPS mot
            GitHub. Kjør dette utenfor cplt, siden git-vakta stopper endringer i remote-oppsettet:
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
              <strong>How should the agent run commands?</strong> og <strong>What may the agent do with git?</strong> Se{" "}
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
            Kjenner cplt igjen verktøyene i repoet, kommer spørsmålet om <code className={code}>.cplt.toml</code> nå, se{" "}
            <a href="#forste-repo" className={linkClass}>
              Første gang i et repo
            </a>
            . Så spør nav-pilot hvor agentpakka skal ligge (<strong>Where to install?</strong>): i repoet, som hele
            teamet får, eller i hjemmekatalogen, bare for deg.{" "}
            <NextLink href="/nav-pilot/guider/installere-og-oppgradere#velg-installasjonssted" className={linkClass}>
              Installere og oppgradere
            </NextLink>{" "}
            forklarer forskjellen.
          </BodyLong>
          <BodyLong>
            Ber agentpakka om et unntak i sandkassen, spør nav-pilot før den installerer (
            <strong>agentpakke nav-pilot asks for more than the sandbox gives it. Allow it?</strong>). Nav-pakka ber om
            å nå Grafana-verktøyene Mimir, Loki og Tempo. Enter svarer Decline, og da virker alt annet i pakka.{" "}
            <NextLink href="/nav-pilot/agentpakker#sandkasse" className={linkClass}>
              Agentpakker
            </NextLink>{" "}
            forklarer unntakene.
          </BodyLong>
          <BodyLong>
            Har du lagt inn MCP-tjenere fra Navs MCP-register i Copilot CLI eller OpenCode, spør nav-pilot om de får nå
            vertene sine fra sandkassen (
            <strong>MCP server … connects to these hosts. Allow them in the sandbox?</strong>
            ). Enter svarer nei. nav-pilot henter registeret i bakgrunnen, så spørsmålet kan komme først ved neste
            oppstart. Deretter starter klienten i sandkassen.
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
            <li>
              skriving utenfor prosjektkatalogen, og lesing av SSH-nøkler, nøkler til skytjenester og andre
              hemmeligheter
            </li>
          </Bullets>
          <BodyLong>
            I tillegg ber nav-pilot agenten om å spørre deg før den sletter grener eller filer utenfor oppgaven,
            deployer, endrer CI eller tilganger, legger til avhengigheter, og når kravene er uklare. Det er en instruks,
            ikke en sperre. Det er cplt som sperrer.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="autonomi" size="medium" level="2">
            Hvor mye skal agenten gjøre selv?
          </LinkableHeading>
          <BodyLong>
            Veiviseren stiller to spørsmål. Når klienten kjører i cplt med nivået <code className={code}>standard</code>{" "}
            eller <code className={code}>strict</code>, stopper cplt <code className={code}>gh pr merge</code>, push til
            standardgrenen og force push, uansett hva du svarer. Med <code className={code}>permissive</code> eller{" "}
            <code className={code}>full-trust</code> er sperrene av, og ingenting blir stoppet.
          </BodyLong>
          <BodyLong>
            <strong>How should the agent run commands?</strong> Bare for Copilot CLI.
          </BodyLong>
          <Bullets>
            <li>
              <strong>On its own inside the sandbox, and ask you when it needs to (recommended).</strong> Agenten kjører
              kommandoer uten å spørre hver gang, og spør deg når den trenger det. nav-pilot sender{" "}
              <code className={code}>--allow-all-tools --allow-all-paths --allow-all-urls</code>, men bare når klienten
              kjører i cplt.
            </li>
            <li>
              <strong>Ask before each command.</strong> Copilot CLI spør før hver kommando.
            </li>
          </Bullets>
          <BodyLong>
            Bruker du OpenCode eller pi, får du ikke dette spørsmålet. Der styrer klienten selv hva den spør om. For
            OpenCode er det nøkkelen <code className={code}>permission</code> i{" "}
            <code className={code}>opencode.json</code>.
          </BodyLong>
          <BodyLong>
            <strong>What may the agent do with git?</strong> cplt håndhever svaret.
          </BodyLong>
          <Bullets>
            <li>
              <strong>Commit, push branches and open pull requests (recommended).</strong> Agenten committer, pusher
              egne grener og åpner pull requests.
            </li>
            <li>
              <strong>Commit only (no pushes).</strong> Agenten kan ikke pushe, så du pusher selv.
            </li>
          </Bullets>
          <BodyLong>
            Har du satt cplt til <code className={code}>permissive</code> eller <code className={code}>full-trust</code>
            , får du ikke git-spørsmålet, fordi git- og gh-vaktene er av der.
          </BodyLong>
          <BodyLong>
            Brukte du nav-pilot med Copilot CLI før, og valgte aldri selv at den skulle spørre før hver kommando, kjører
            den nå kommandoer på egen hånd i cplt. nav-pilot sier fra om det én gang. En{" "}
            <code className={code}>autonomy = &quot;conservative&quot;</code> som nav-pilot skrev selv, teller ikke som
            et valg. Vil du at Copilot CLI skal spørre før hver kommando:
          </BodyLong>
          <CodeBlock compact>{`nav-pilot config set autonomy conservative`}</CodeBlock>
          <BodyLong>Tilbake til at agenten kjører kommandoer selv:</BodyLong>
          <CodeBlock compact>{`nav-pilot config set autonomy sandbox`}</CodeBlock>
          <BodyLong>
            Veiviseren endrer ikke cplt-nivået du har. Har du ikke satt noe, er det{" "}
            <code className={code}>standard</code>. Vil du bytte nivå, for eksempel til{" "}
            <code className={code}>strict</code>, der agenten bare når verter på en liste, får du spørsmålet om
            nettverket med:
          </BodyLong>
          <CodeBlock compact>{`nav-pilot config setup --advanced`}</CodeBlock>
          <Box background="warning-soft" borderRadius="8" padding="space-16">
            <VStack gap="space-8">
              <BodyLong>
                <strong>strict koster noe.</strong> Agenten kan ikke pushe med mindre du tillater det i git-spørsmålet.
                Verter som ikke står på lista, blir blokkert. Det gjelder også interne verter du tar i bruk senere.
                nav-pilot legger Navs verter i <code className={code}>~/.nav-pilot/cplt-allowed-domains.txt</code> før
                nivået blir satt. Har du en egen <code className={code}>proxy.allowed_domains</code>, må du legge dem
                inn i den selv. På Linux krever nivået kjerne 6.7 eller nyere med Landlock slått på.
              </BodyLong>
              <BodyLong>
                Bytter du fra strict til <code className={code}>standard</code>, spør nav-pilot fra versjon{" "}
                {LIST_REMOVAL_VERSION}: <strong>Remove the network allowlist nav-pilot set up for strict?</strong>{" "}
                Spørsmålet kommer bare for lista nav-pilot la inn selv. Svarer du nei, har du en eldre versjon eller en
                egen liste, blir lista stående, og agenten når fortsatt bare vertene på den. Kommandoen under fjerner
                lista som gjelder, uansett hvem som la den inn:
              </BodyLong>
              <CodeBlock compact>{`cplt config set proxy.allowed_domains --unset --global`}</CodeBlock>
            </VStack>
          </Box>
          <BodyLong>
            Kjører du <code className={code}>nav-pilot config setup</code> på nytt, starter hvert spørsmål på det du har
            nå. Trykker du bare Enter, blir fila som den er. Se over oppsummeringen før du lagrer.
          </BodyLong>
          <BodyLong>
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
              Trenger agenten verktøy utenfor klienten, som Playwright? Finn navnet med{" "}
              <code className={code}>nav-pilot mcp list</code>, legg serveren til med{" "}
              <code className={code}>nav-pilot mcp enable &lt;navn&gt;</code>, og se{" "}
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
