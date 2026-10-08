import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Worktrees med nav-pilot og cplt",
  description:
    "Start nav-pilot i et git-worktree, la agenten lage worktrees til subagenter, og bruk worktrees fra et bare repo.",
};

const TOC: TocItem[] = [
  { id: "hvorfor", label: "Hvorfor worktrees" },
  { id: "starte-i-worktree", label: "Starte i et worktree" },
  { id: "subagenter", label: "Worktrees til subagenter" },
  { id: "bare-repo", label: "Bare repo og .git et annet sted" },
  { id: "begrensninger", label: "Begrensninger og sikkerhet" },
  { id: "feilsoking", label: "Feilsøking" },
];

const CPLT_CONFIG_DOCS =
  "https://github.com/navikt/cplt/blob/main/docs/configuration.md#worktrees-for-sub-agents-sandboxallow_git_worktrees";
const CPLT_SECURITY_DOCS =
  "https://github.com/navikt/cplt/blob/main/SECURITY.md#managed-worktree-root-sandboxallow_git_worktrees";

export default function Worktrees() {
  return (
    <DocPage
      label="Guider"
      upgrade
      title="Worktrees med nav-pilot og cplt"
      description="Med git worktree har du flere brancher sjekket ut samtidig, hver i sin mappe. Slik bruker du det i sandkassen."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="hvorfor" size="medium" level="2">
            Hvorfor worktrees
          </LinkableHeading>
          <BodyLong>
            Et worktree er en ekstra utsjekking av samme repo, på en annen branch og i en annen mappe. Agenten kan jobbe
            på én branch mens du jobber på en annen, og flere subagenter kan jobbe parallelt uten å skrive over
            hverandres filer. Objekter, refs og konfig ligger i én felles git-mappe, den{" "}
            <code className={code}>git rev-parse --git-common-dir</code> viser. Hvert worktree har i stedet for en{" "}
            <code className={code}>.git</code>-mappe en <code className={code}>.git</code>-fil som peker til sin egen
            admin-mappe under <code className={code}>&lt;felles&gt;/worktrees/&lt;navn&gt;</code>.
          </BodyLong>
          <BodyLong>Det er tre måter å bruke worktrees på med nav-pilot og cplt:</BodyLong>
          <Bullets>
            <li>Du starter nav-pilot i et worktree du har laget selv.</li>
            <li>Agenten lager worktrees til subagenter mens den jobber.</li>
            <li>Du jobber i et worktree fra et bare repo, eller med .git et annet sted.</li>
          </Bullets>
          <BodyLong>
            Den første og den tredje virker uten oppsett. Den andre må du slå på, og den virker bare på macOS.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="starte-i-worktree" size="medium" level="2">
            Starte i et worktree
          </LinkableHeading>
          <BodyLong>Lag worktreet utenfor sandkassen, og start nav-pilot i det:</BodyLong>
          <CodeBlock compact>
            {`cd ~/src/min-app
git worktree add ../min-app-ny-branch -b ny-branch
cd ../min-app-ny-branch
nav-pilot`}
          </CodeBlock>
          <BodyLong>
            nav-pilot gir cplt worktreet som prosjektkatalog. cplt finner den felles git-mappa, her{" "}
            <code className={code}>.git</code>-mappa i hovedutsjekkingen, og gir agenten tilgang til den, så{" "}
            <code className={code}>git status</code>, <code className={code}>git commit</code> og resten virker. Agenten
            kan skrive i worktreet, ikke i hovedutsjekkingen.
          </BodyLong>
          <BodyLong>
            Den felles git-mappa bør ligge under hjemmekatalogen din. Ligger den et annet sted, for eksempel under{" "}
            <code className={code}>/Users/Shared</code>, gir cplt ikke tilgang, og git feiler i sandkassen. cplt sier
            fra om det ved oppstart og i <code className={code}>cplt doctor</code>, se{" "}
            <a href="#git-feiler" className={linkClass}>
              Feilsøking
            </a>
            .
          </BodyLong>
          <BodyLong>
            Godkjenningen av repoets <code className={code}>.cplt.toml</code> gjelder hele repoet. Har du godkjent den i
            hovedutsjekkingen, spør ikke cplt på nytt i worktreet, med mindre branchen har endret forslaget.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="subagenter" size="medium" level="2">
            Worktrees til subagenter
          </LinkableHeading>
          <BodyLong>
            Som standard kan agenten bare skrive i prosjektkatalogen, så den har ikke noe sted å lage nye worktrees. Slå
            på en egen worktree-mappe for repoet:
          </BodyLong>
          <CodeBlock compact>
            {`cplt config set sandbox.allow_git_worktrees true           # alle repoer
cplt config set --local sandbox.allow_git_worktrees true   # bare denne utsjekkingen`}
          </CodeBlock>
          <BodyLong>
            Neste gang du starter nav-pilot, lager cplt mappa <code className={code}>~/.cplt-worktrees/&lt;id&gt;</code>
            , viser den ved oppstart og setter miljøvariabelen <code className={code}>CPLT_WORKTREE_ROOT</code>. Alle
            worktrees av samme repo får samme mappe, en annen klone får sin egen.
          </BodyLong>
          <BodyLong>
            Du trenger ikke be agenten om å bruke mappa. cplt forteller den om mappa i starten av økten, sammen med
            kommandoen:
          </BodyLong>
          <CodeBlock compact>{`git worktree add "$CPLT_WORKTREE_ROOT/<navn>" -b <branch>`}</CodeBlock>
          <BodyLong>
            Worktrees og brancher agenten lager, blir liggende etter økten. Rydd opp selv, utenfor cplt:
          </BodyLong>
          <CodeBlock compact>
            {`git worktree list
git worktree remove ~/.cplt-worktrees/<id>/<navn>
git branch -d <branch>`}
          </CodeBlock>
          <BodyLong>
            Innstillingen kan bare settes i din egen cplt-konfig. Et repo kan ikke slå den på for deg med{" "}
            <code className={code}>.cplt.toml</code>.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="bare-repo" size="medium" level="2">
            Bare repo og .git et annet sted
          </LinkableHeading>
          <BodyLong>
            Noen holder repoet som et bare repo, uten arbeidskatalog, med hver branch som et worktree ved siden av.
            Andre har git-mappa et annet sted enn koden. Begge virker som når du starter i et worktree: cplt følger{" "}
            <code className={code}>.git</code>-fila til den felles git-mappa og gir agenten tilgang dit.
          </BodyLong>
          <CodeBlock compact>
            {`git clone --bare git@github.com:navikt/min-app.git ~/src/min-app.git
git -C ~/src/min-app.git worktree add ~/src/min-app-main main
cd ~/src/min-app-main
nav-pilot`}
          </CodeBlock>
          <BodyLong>
            cplt godtar bare oppsett slik git selv lager dem. En <code className={code}>.git</code>-fil som er endret
            for hånd til å peke på et annet repo, gir ingen tilgang.
          </BodyLong>
          <BodyLong>
            Bruker du worktrees til subagenter i tillegg, kan ikke repoet bruke{" "}
            <code className={code}>worktree.useRelativePaths</code>. cplt nekter å starte når et worktree i repoet har
            relative stier, også utenfor worktree-mappa. Slå av innstillingen og skriv om stiene, utenfor cplt, for
            hvert worktree som har dem:
          </BodyLong>
          <CodeBlock compact>{`git config --unset worktree.useRelativePaths
git worktree repair --no-relative-paths ~/src/min-app-main`}</CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="begrensninger" size="medium" level="2">
            Begrensninger og sikkerhet
          </LinkableHeading>
          <Bullets>
            <li>
              <strong>Ingen isolasjon mellom subagenter.</strong> Alle worktrees i en økt deler samme tilgang. En
              subagent kan skrive i de andres worktrees og i utsjekkingen du startet fra.
            </li>
            <li>
              <strong>Bare macOS.</strong> På Linux nekter <code className={code}>cplt config set</code> å slå på
              worktrees til subagenter, fordi Landlock ikke kan stenge filer inne i mappa. De to andre måtene virker på
              Linux også.
            </li>
            <li>
              <strong>Samme beskyttelse som i prosjektet.</strong> Hooks og <code className={code}>config</code> i den
              felles git-mappa, <code className={code}>config.worktree</code> for hvert worktree,{" "}
              <code className={code}>.cplt.toml</code>, <code className={code}>.github/hooks</code> og de andre
              beskyttede filene er stengt for skriving i hvert worktree.
            </li>
            <li>
              <strong>cplt sjekker mappa ved hver oppstart og ved slutten av økten.</strong> Finner den en{" "}
              <code className={code}>.git</code> eller et bare repo der git ikke selv ville lagt det, eller en symlenke
              som peker ut av worktreet, nekter den å starte og sier hvilke mapper du ikke skal kjøre git i.
            </li>
            <li>
              <strong>Endringsrapporten dekker ikke worktrees.</strong> Rapporten cplt skriver etter økten, viser bare
              endringer i prosjektkatalogen. Se over det agenten har gjort i worktreene, før du kjører noe derfra
              utenfor sandkassen.
            </li>
          </Bullets>
          <BodyLong>
            Detaljene står i{" "}
            <a href={CPLT_CONFIG_DOCS} className={linkClass}>
              konfigurasjonsdokumentasjonen til cplt
            </a>{" "}
            og i{" "}
            <a href={CPLT_SECURITY_DOCS} className={linkClass}>
              SECURITY.md
            </a>{" "}
            (engelsk).
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="feilsoking" size="medium" level="2">
            Feilsøking
          </LinkableHeading>
          <LinkableHeading id="git-feiler" size="small" level="3">
            git feiler i sandkassen, men virker utenfor
          </LinkableHeading>
          <BodyLong>
            Den felles git-mappa (<code className={code}>git rev-parse --git-common-dir</code>) ligger utenfor
            hjemmekatalogen, eller stien har et tegn cplt ikke kan bruke: anførselstegn, parentes, semikolon eller
            omvendt skråstrek. cplt sier fra om begge deler ved oppstart, med{" "}
            <code className={code}>Not granting the shared git directory</code>. Flytt repoet inn under hjemmekatalogen,
            eller gi mappa et navn uten de tegnene. Ligger mappa utenfor hjemmekatalogen, kan du også gi tilgang til
            den:
          </BodyLong>
          <CodeBlock compact>{`cplt config set --local allow.write /Users/Shared/min-app/.git`}</CodeBlock>
          <LinkableHeading id="worktree-remove" size="small" level="3">
            cplt vil ikke starte etter git worktree remove
          </LinkableHeading>
          <BodyLong>
            <code className={code}>git worktree remove</code> feiler inne i sandkassen og etterlater et halvveis slettet
            worktree. Slett mappa og rydd opp utenfor cplt:
          </BodyLong>
          <CodeBlock compact>
            {`rm -rf ~/.cplt-worktrees/<id>/<navn>
git worktree prune`}
          </CodeBlock>
          <LinkableHeading id="for-mange-mapper" size="small" level="3">
            cplt sier at worktree-mappa har for mange mapper
          </LinkableHeading>
          <BodyLong>
            Sjekken går gjennom høyst 100 000 mapper. Noen få store JavaScript-worktrees kan være nok. Slett worktrees
            du er ferdig med, eller hev grensen:
          </BodyLong>
          <CodeBlock compact>{`cplt config set sandbox.worktree_walk_max_dirs 300000`}</CodeBlock>
          <LinkableHeading id="go-bygg" size="small" level="3">
            Go-bygg feiler med exit status 128 i et worktree
          </LinkableHeading>
          <BodyLong>
            Go leter oppover etter en <code className={code}>.git</code>-mappe for å stemple bygget med
            versjonsinformasjon. I et worktree er <code className={code}>.git</code> en fil, så Go går forbi den og kan
            finne en annen <code className={code}>.git</code>-mappe høyere opp, for eksempel i hjemmekatalogen din. Den
            har ikke agenten tilgang til, og bygget feiler med{" "}
            <code className={code}>error obtaining VCS status: exit status 128</code>. Slå av stemplingen:
          </BodyLong>
          <CodeBlock compact>{`go build -buildvcs=false`}</CodeBlock>
          <BodyLong>
            Bygger du gjennom mise, kan du sette det for hele repoet i <code className={code}>mise.toml</code>:
          </BodyLong>
          <CodeBlock compact>
            {`[env]
GOFLAGS = "-buildvcs=false"`}
          </CodeBlock>
          <BodyLong>
            <code className={code}>GOFLAGS</code> fra skallet ditt slipper også inn i sandkassen.
          </BodyLong>
          <BodyLong>
            Andre feilmeldinger fra sandkassen står i{" "}
            <NextLink href="/nav-pilot/guider/cplt-feilmeldinger" className={linkClass}>
              Feil i sandkassen
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
