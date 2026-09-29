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
    "Start nav-pilot i en git worktree, la agenten lage worktrees til underagenter, og bruk worktrees fra et bare repo.",
};

const TOC: TocItem[] = [
  { id: "hvorfor", label: "Hvorfor worktrees" },
  { id: "forutsetninger", label: "Før du starter" },
  { id: "starte-i-worktree", label: "Starte i en worktree" },
  { id: "underagenter", label: "Worktrees til underagenter" },
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
      title="Worktrees med nav-pilot og cplt"
      description="Med git worktree har du flere grener sjekket ut samtidig, hver i sin mappe. Slik bruker du det i sandkassen."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="hvorfor" size="medium" level="2">
            Hvorfor worktrees
          </LinkableHeading>
          <BodyLong>
            En worktree er en ekstra utsjekking av samme repo, på en annen gren og i en annen mappe. Du kan la agenten
            jobbe på én gren mens du selv jobber på en annen, eller la flere underagenter jobbe parallelt uten å skrive
            over hverandres filer. Alle deler den samme <code className={code}>.git</code>-mappa.
          </BodyLong>
          <BodyLong>Det er tre måter å bruke worktrees på med nav-pilot og cplt:</BodyLong>
          <Bullets>
            <li>Du starter nav-pilot i en worktree du har laget selv.</li>
            <li>Agenten lager worktrees til underagenter mens den jobber.</li>
            <li>Du jobber i en worktree fra et bare repo, eller med .git et annet sted.</li>
          </Bullets>
          <BodyLong>
            Den første og den tredje virker uten oppsett. Den andre må du slå på, og den virker bare på macOS.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="forutsetninger" size="medium" level="2">
            Før du starter
          </LinkableHeading>
          <BodyLong>
            Worktrees til underagenter krever cplt <code className={code}>2026.09.24-164900-53d4462</code> eller nyere,
            på macOS. Sjekk versjonen:
          </BodyLong>
          <CodeBlock compact>{`cplt --version`}</CodeBlock>
          <BodyLong>
            cplt er en egen pakke. <code className={code}>brew upgrade navikt/tap/nav-pilot</code> oppgraderer bare
            nav-pilot, og <code className={code}>nav-pilot upgrade</code> foreslår bare å oppgradere cplt når nav-pilot
            også er utdatert. Oppgrader cplt selv:
          </BodyLong>
          <CodeBlock compact>{`brew update
brew upgrade navikt/tap/cplt`}</CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="starte-i-worktree" size="medium" level="2">
            Starte i en worktree
          </LinkableHeading>
          <BodyLong>Lag worktreen utenfor sandkassen, og start nav-pilot i den:</BodyLong>
          <CodeBlock compact>
            {`cd ~/src/min-app
git worktree add ../min-app-ny-gren -b ny-gren
cd ../min-app-ny-gren
nav-pilot`}
          </CodeBlock>
          <BodyLong>
            nav-pilot gir cplt worktreen som prosjektkatalog. cplt finner den delte <code className={code}>.git</code>
            -mappa i hovedutsjekkingen og gir agenten tilgang til den, så <code className={code}>git status</code>,{" "}
            <code className={code}>git commit</code> og resten virker. På macOS er hooks og konfig i{" "}
            <code className={code}>.git</code> fortsatt stengt for skriving. Agenten kan ikke skrive i selve
            hovedutsjekkingen, bare i worktreen.
          </BodyLong>
          <BodyLong>
            Den delte <code className={code}>.git</code>-mappa må ligge under hjemmekatalogen din. Ligger den et annet
            sted, for eksempel under <code className={code}>/tmp</code>, gir cplt ikke tilgang, og git feiler i
            sandkassen.
          </BodyLong>
          <BodyLong>
            Godkjenningen av repoets <code className={code}>.cplt.toml</code> gjelder hele repoet, ikke én utsjekking.
            Har du godkjent den i hovedutsjekkingen, spør ikke cplt på nytt i worktreen, med mindre grenen har endret
            forslaget.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="underagenter" size="medium" level="2">
            Worktrees til underagenter
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
            , viser den når cplt starter, og setter miljøvariabelen <code className={code}>CPLT_WORKTREE_ROOT</code>.
            Mappa hører til dette repoet. Alle worktrees av samme repo får samme mappe, mens en annen klone får sin
            egen.
          </BodyLong>
          <BodyLong>
            Du trenger ikke be agenten om å bruke mappa. cplt forteller agenten om den i starten av økten, sammen med
            kommandoen den skal bruke:
          </BodyLong>
          <CodeBlock compact>{`git worktree add "$CPLT_WORKTREE_ROOT/<navn>" -b <gren>`}</CodeBlock>
          <BodyLong>
            Worktrees og grener agenten lager, blir liggende etter økten. Rydd opp selv, utenfor cplt:
          </BodyLong>
          <CodeBlock compact>
            {`git worktree list
git worktree remove ~/.cplt-worktrees/<id>/<navn>
git branch -d <gren>`}
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
            Noen holder repoet som et bare repo, uten arbeidskatalog, og har hver gren som en worktree ved siden av.
            Andre har <code className={code}>.git</code>-mappa et annet sted enn koden. Begge deler virker på samme måte
            som når du starter i en worktree: cplt følger <code className={code}>.git</code>-fila til den delte mappa og
            gir agenten tilgang dit.
          </BodyLong>
          <CodeBlock compact>
            {`git clone --bare git@github.com:navikt/min-app.git ~/src/min-app.git
git -C ~/src/min-app.git worktree add ~/src/min-app-main main
cd ~/src/min-app-main
nav-pilot`}
          </CodeBlock>
          <BodyLong>
            Det samme kravet gjelder: den delte mappa må ligge under hjemmekatalogen din. cplt godtar bare oppsett slik
            git selv lager dem, så en <code className={code}>.git</code>-fil som er endret for hånd til å peke på et
            annet repo, gir ingen tilgang.
          </BodyLong>
          <BodyLong>
            Bruker du worktrees til underagenter i tillegg, kan ikke repoet bruke{" "}
            <code className={code}>worktree.useRelativePaths</code>. cplt nekter å starte når worktree-mappa inneholder
            worktrees med relative stier. Slå det av der du har satt det:
          </BodyLong>
          <CodeBlock compact>{`git config --unset worktree.useRelativePaths`}</CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="begrensninger" size="medium" level="2">
            Begrensninger og sikkerhet
          </LinkableHeading>
          <Bullets>
            <li>
              <strong>Ingen isolasjon mellom underagenter.</strong> Alle worktrees i en økt deler samme tilgang. En
              underagent kan skrive i de andres worktrees og i utsjekkingen du startet fra. Worktrees holder parallelt
              arbeid fra hverandre, men beskytter det ikke mot en annen agent.
            </li>
            <li>
              <strong>Bare macOS.</strong> På Linux nekter <code className={code}>cplt config set</code> å slå på
              worktrees til underagenter, fordi Landlock ikke kan stenge filer inne i mappa. De to andre måtene virker
              på Linux også.
            </li>
            <li>
              <strong>Samme beskyttelse som i prosjektet.</strong> Hooks og konfig i <code className={code}>.git</code>,{" "}
              <code className={code}>.cplt.toml</code>, <code className={code}>.github/hooks</code> og de andre
              beskyttede filene er stengt for skriving i hver worktree.
            </li>
            <li>
              <strong>cplt sjekker mappa ved hver oppstart og ved slutten av økten.</strong> Finner den en{" "}
              <code className={code}>.git</code> eller et bare repo der git ikke selv ville lagt det, eller en symlenke
              som peker ut av worktreen, nekter den å starte og sier hvilke mapper du ikke skal kjøre git i.
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
          <LinkableHeading id="unknown-config-key" size="small" level="3">
            unknown config key &apos;sandbox.allow_git_worktrees&apos;
          </LinkableHeading>
          <BodyLong>
            cplt er for gammel. Oppgrader med <code className={code}>brew update</code> og{" "}
            <code className={code}>brew upgrade navikt/tap/cplt</code>, og sjekk at{" "}
            <code className={code}>cplt --version</code> viser <code className={code}>2026.09.24-164900-53d4462</code>{" "}
            eller nyere.
          </BodyLong>
          <LinkableHeading id="git-feiler" size="small" level="3">
            git feiler i sandkassen, men virker utenfor
          </LinkableHeading>
          <BodyLong>
            Den delte <code className={code}>.git</code>-mappa ligger utenfor hjemmekatalogen, eller stien har et tegn
            cplt ikke kan bruke: anførselstegn, parentes, semikolon eller omvendt skråstrek. Da får ikke agenten tilgang
            til den, og cplt sier fra ved oppstart. Flytt repoet inn under hjemmekatalogen, eller gi mappa et navn uten
            de tegnene.
          </BodyLong>
          <LinkableHeading id="worktree-remove" size="small" level="3">
            cplt vil ikke starte etter git worktree remove
          </LinkableHeading>
          <BodyLong>
            <code className={code}>git worktree remove</code> feiler inne i sandkassen og etterlater en halvveis slettet
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
            Go-bygg feiler med exit status 128 i en worktree
          </LinkableHeading>
          <BodyLong>
            Go leter oppover etter en <code className={code}>.git</code>-mappe for å stemple bygget med
            versjonsinformasjon. I en worktree er <code className={code}>.git</code> en fil, så Go går forbi den og kan
            finne en annen <code className={code}>.git</code>-mappe høyere opp, for eksempel i hjemmekatalogen din hvis
            den er et git-repo. Den har ikke agenten tilgang til, og bygget feiler med{" "}
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
            <code className={code}>GOFLAGS</code> satt i skallet ditt hjelper ikke, fordi cplt ikke sender variabelen
            inn i sandkassen. Se{" "}
            <a href="https://github.com/navikt/cplt/issues/617" className={linkClass}>
              cplt-sak 617
            </a>
            .
          </BodyLong>
          <BodyLong>
            Andre problemer med cplt står i{" "}
            <NextLink href="/nav-pilot/guider/feilsoking" className={linkClass}>
              feilsøkingsguiden
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
