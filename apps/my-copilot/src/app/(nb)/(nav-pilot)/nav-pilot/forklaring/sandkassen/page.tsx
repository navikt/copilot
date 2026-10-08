import { BodyLong, Box, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Sandkassen",
  description:
    "Hvorfor en agent må kjøre isolert på Nav-utstyr, hva cplt låser, og forskjellen på sikkerhetsnivåene standard og strict.",
};

const TOC: TocItem[] = [
  { id: "isolasjon-er-pakrevd", label: "Isolasjon er påkrevd" },
  { id: "prosjektkatalogen", label: "Prosjektkatalogen" },
  { id: "sikkerhetsniva", label: "Sikkerhetsnivå i cplt" },
  { id: "autonomi", label: "Kommandoer og git" },
  { id: "strict", label: "Strict i detalj" },
  { id: "nar-strict-ikke-anbefales", label: "Strict på Linux" },
];

export default function Sandkassen() {
  return (
    <DocPage
      label="Forklaring"
      title="Sandkassen"
      description="nav-pilot starter klienten i cplt, sandkassen for kodeagenter. Her er hvorfor, og hva sikkerhetsnivåene gjør."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="isolasjon-er-pakrevd" size="medium" level="2">
            Isolasjon er påkrevd på Nav-utstyr
          </LinkableHeading>
          <Box background="warning-soft" borderRadius="8" padding="space-16">
            <VStack gap="space-8">
              <BodyLong>
                En KI-agent på Nav-utstyr skal kjøre i en sandkasse eller tilsvarende isolasjon. Kravet gjelder både
                arbeid for Nav og eget arbeid.
              </BodyLong>
              <BodyLong>
                Bruk{" "}
                <NextLink href="/cplt" className={linkClass}>
                  cplt
                </NextLink>
                . Det er det enkleste. Velger du noe annet, må du selv slå på isolasjonen i klienten, eller isolere på
                en annen måte, for eksempel med en VM eller en container. Ikke kjør agenter med full tilgang til
                maskinen.
              </BodyLong>
              <BodyLong>
                <NextLink href="/nyheter/sandboxing-er-pakrevd-pa-nav-utstyr" className={linkClass}>
                  Kortversjonen av kravet
                </NextLink>{" "}
                er en lenke du kan dele med andre.
              </BodyLong>
            </VStack>
          </Box>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="prosjektkatalogen" size="medium" level="2">
            Prosjektkatalogen
          </LinkableHeading>
          <BodyLong>
            nav-pilot gir cplt katalogen du står i som prosjektkatalog, med <code className={code}>--project-dir</code>.
            Agenten kan lese og skrive der og under, ikke i mapper ved siden av. Står du i en undermappe, gjelder
            sandkassen bare den, men instruksjonene i roten av repoet, som <code className={code}>.github/</code> og{" "}
            <code className={code}>AGENTS.md</code>, kan agenten likevel lese. Trenger den hele repoet, start fra roten
            eller kjør <code className={code}>nav-pilot --project-dir &lt;katalog&gt;</code>. Hjemmekatalogen og{" "}
            <code className={code}>/</code> avviser cplt, fordi de gir for mye tilgang.
          </BodyLong>
          <BodyLong>
            Trenger repoet mer enn standard, for eksempel localhost til Gradle eller en database, se{" "}
            <NextLink href="/nav-pilot/guider/cplt-oppsett" className={linkClass}>
              Sett opp cplt i et repo
            </NextLink>
            . Står du i et git-worktree, se{" "}
            <NextLink href="/nav-pilot/guider/worktrees" className={linkClass}>
              Worktrees med nav-pilot og cplt
            </NextLink>
            . Feilmeldinger fra sandkassen slår du opp i{" "}
            <NextLink href="/nav-pilot/guider/cplt-feilmeldinger" className={linkClass}>
              Feil i sandkassen
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="sikkerhetsniva" size="medium" level="2">
            Sikkerhetsnivå i cplt
          </LinkableHeading>
          <BodyLong>
            Nivået er <code className={code}>sandbox.preset</code> i cplt. Det avgjør hva cplt stopper:
          </BodyLong>
          <Bullets>
            <li>
              <strong>
                <code className={code}>standard</code>
              </strong>{" "}
              (anbefalt, og det du har hvis du ikke har valgt noe): agenten kan committe, pushe egne grener og åpne pull
              requests. cplt stopper <code className={code}>gh pr merge</code>, push til standardgrenen og force push.
            </li>
            <li>
              <strong>
                <code className={code}>strict</code>
              </strong>
              : som standard, men agenten når bare verter på en liste, og all push er stoppet. Velg det bare hvis du vil
              låse nettverket. Se{" "}
              <a href="#strict" className={linkClass}>
                Strict i detalj
              </a>
              .
            </li>
            <li>
              <strong>
                <code className={code}>permissive</code> og <code className={code}>full-trust</code>
              </strong>
              : vaktene er av, og cplt stopper ingenting av dette.
            </li>
          </Bullets>
          <BodyLong>
            Veiviseren i nav-pilot endrer ikke nivået du har. Vil du bytte, kjør{" "}
            <code className={code}>nav-pilot config setup --advanced</code>. Tabellen over innstillingene står i{" "}
            <NextLink href="/nav-pilot/referanse#sikkerhetsniva" className={linkClass}>
              referansen
            </NextLink>
            .
          </BodyLong>

          <LinkableHeading id="autonomi" size="small" level="3">
            Kommandoer og git i sandkassen
          </LinkableHeading>
          <BodyLong>
            Oppsettet i nav-pilot stiller to spørsmål om hva agenten får gjøre selv. Med{" "}
            <code className={code}>standard</code> og <code className={code}>strict</code> stopper cplt likevel{" "}
            <code className={code}>gh pr merge</code>, push til standardgrenen og force push, uansett hva du svarer.
          </BodyLong>
          <Bullets>
            <li>
              <strong>How should the agent run commands?</strong> (bare Copilot CLI). Det anbefalte valget lar agenten
              kjøre kommandoer uten å spørre hver gang. nav-pilot sender da{" "}
              <code className={code}>--allow-all-tools --allow-all-paths --allow-all-urls</code>, men bare når klienten
              kjører i cplt. «Ask before each command» gjør at Copilot CLI spør før hver kommando. OpenCode og pi styrer
              dette selv. For OpenCode er det nøkkelen <code className={code}>permission</code> i{" "}
              <code className={code}>opencode.json</code>.
            </li>
            <li>
              <strong>What may the agent do with git?</strong> Det anbefalte valget lar agenten committe, pushe egne
              grener og åpne pull requests. «Commit only» stopper all push. Med <code className={code}>permissive</code>{" "}
              eller <code className={code}>full-trust</code> kommer ikke spørsmålet, fordi git- og gh-vaktene er av.
            </li>
          </Bullets>
          <BodyLong>
            Svaret på det første spørsmålet er nøkkelen <code className={code}>autonomy</code>:
          </BodyLong>
          <CodeBlock compact>{`nav-pilot config set autonomy conservative   # spør før hver kommando
nav-pilot config set autonomy sandbox        # kjør kommandoer selv`}</CodeBlock>
          <BodyLong>
            Brukte du Copilot CLI med nav-pilot før, og valgte aldri selv at den skulle spørre, kjører den nå kommandoer
            på egen hånd i cplt. nav-pilot sier fra om det én gang. En{" "}
            <code className={code}>autonomy = &quot;conservative&quot;</code> som en eldre nav-pilot skrev selv, teller
            ikke som et valg. <code className={code}>nav-pilot config set autonomy</code> skriver også{" "}
            <code className={code}>autonomy_chosen = true</code>, så valget ditt blir stående.
          </BodyLong>
          <BodyLong>
            Kjører du <code className={code}>nav-pilot config setup</code> på nytt, starter hvert spørsmål på det du har
            nå. Trykker du bare Enter, blir fila som den er.
          </BodyLong>

          <LinkableHeading id="strict" size="small" level="3">
            Strict i detalj
          </LinkableHeading>
          <BodyLong>
            <code className={code}>strict</code> slår på tvungen proxy og{" "}
            <code className={code}>proxy.default_allowlist</code>. Da når agenten bare vertene på lista til cplt og det{" "}
            <code className={code}>proxy.allowed_domains</code> peker på. Verter som ikke står der, blir blokkert, også
            interne verter du tar i bruk senere. Verken <code className={code}>nav-pilot doctor</code> eller
            innstillingssiden anbefaler strict.
          </BodyLong>
          <Box background="warning-soft" borderRadius="8" padding="space-16">
            <BodyLong>
              Lista til cplt dekker GitHub Copilot og de offentlige pakkeregistrene, men ingenting hos Nav. Setter du
              strict for hånd, slutter telemetrien fra nav-pilot å komme fram. Skills som{" "}
              <code className={code}>aksel-builder</code>, <code className={code}>observability-debugging</code> og{" "}
              <code className={code}>nav-auth</code> mister vertene de er bygget rundt, og ingenting på skjermen sier
              hvorfor.
            </BodyLong>
          </Box>
          <BodyLong>Sett det derfor med nav-pilot:</BodyLong>
          <CodeBlock compact>
            {`nav-pilot config setup --advanced   # velg «Allowlist only (cplt strict)»
nav-pilot config                    # eller raden «cplt strict preset (blocks all pushes)»`}
          </CodeBlock>
          <BodyLong>
            nav-pilot skriver vertslista til <code className={code}>~/.nav-pilot/cplt-allowed-domains.txt</code>, peker{" "}
            <code className={code}>proxy.allowed_domains</code> dit, og setter nivået til slutt, så låsen aldri blir
            aktiv uten vertene. Har du en egen <code className={code}>proxy.allowed_domains</code>, lar nav-pilot den
            være og sier at du må legge til vertene selv. Nøkler du har satt selv, gjelder foran nivået.
          </BodyLong>
          <BodyLong>
            Fila har hele lista: Nav-vertene, cplts egen liste og pakkeregistrene. Agenten når derfor pakkeregistrene
            selv om <code className={code}>proxy.default_allowlist</code> er av. nav-pilot henter lista med{" "}
            <code className={code}>cplt config hosts</code>. Er cplt for gammel til det, skriver nav-pilot en frosset
            liste med GitHub, Copilot og pakkeregistrene som fantes da, uten dem cplt har lagt til siden. Lista til cplt
            er per agent: lista for copilot har GitHub og Copilot, den for opencode har{" "}
            <code className={code}>opencode.ai</code> og <code className={code}>models.dev</code>.
          </BodyLong>
          <BodyLong>
            <strong>Push.</strong> Strict stopper all push, så agenten kan ikke pushe en gren eller åpne en pull
            request. Velger du strict med <code className={code}>nav-pilot config setup --advanced</code>, kan du i
            neste spørsmål la agenten pushe grener likevel. Da setter nav-pilot{" "}
            <code className={code}>git_guard.protect_default_branch_only = true</code>, og cplt stopper bare push til
            standardgrenen og force push. Har du satt den nøkkelen fra før, gjelder den foran nivået, også når du slår
            på strict fra <code className={code}>nav-pilot config</code>. Vil du stoppe all push, velg «Commit only» i
            git-spørsmålet.
          </BodyLong>
          <BodyLong>
            <strong>Lokal modell.</strong> Den lokale modellen går gjennom en løkkevakt på{" "}
            <code className={code}>127.0.0.1</code>. cplt blokkerer localhost som standard, så nav-pilot sender porten
            med som <code className={code}>--allow-localhost &lt;port&gt;</code> ved hver oppstart. Én port slipper
            gjennom tvungen proxy på både macOS og Linux, så strict og lokal modell går sammen.
          </BodyLong>
          <BodyLong>
            <strong>Tilbake til standard.</strong> Bytter du fra strict til <code className={code}>standard</code>, spør
            nav-pilot (fra 29. september 2026):{" "}
            <strong>Remove the network allowlist nav-pilot set up for strict?</strong> Spørsmålet kommer bare for lista
            nav-pilot la inn selv. Svarer du nei, eller har du en egen liste, blir lista stående, og agenten når
            fortsatt bare vertene på den. Kommandoen under fjerner lista som gjelder, uansett hvem som la den inn:
          </BodyLong>
          <CodeBlock compact>{`cplt config set proxy.allowed_domains --unset --global`}</CodeBlock>
          <LinkableHeading id="nar-strict-ikke-anbefales" size="small" level="3">
            Strict på Linux
          </LinkableHeading>
          <BodyLong>
            På Linux krever <code className={code}>proxy.forced</code> at kjernen kan begrense nettverket med Landlock:
            ABI v4, altså kjerne 6.7 eller nyere med Landlock slått på. Under det starter ikke cplt med strict. Derfor
            nekter nav-pilot å sette strict der, både i oppsettet og på innstillingssiden, og sier hvorfor. Har du
            allerede strict på en slik maskin, melder <code className={code}>nav-pilot doctor</code> det som en feil.
          </BodyLong>
          <BodyLong>
            nav-pilot spør kjernen med samme systemkall som cplt, ikke <code className={code}>uname</code>. Landlock kan
            være kompilert bort eller slått av ved oppstart, og da ville versjonsnummeret sagt «går fint» rett før cplt
            nektet å starte. macOS har ingen slik grense. Der gjør Seatbelt jobben.
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
