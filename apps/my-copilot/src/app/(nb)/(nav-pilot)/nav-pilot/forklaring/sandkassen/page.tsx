import { BodyLong, Box, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Sandkassen",
  description:
    "Hvorfor en agent må kjøre isolert på Nav-utstyr, hva cplt låser, og hva sikkerhetsnivået strict legger til.",
};

const TOC: TocItem[] = [
  { id: "isolasjon-er-pakrevd", label: "Isolasjon er påkrevd" },
  { id: "prosjektkatalogen", label: "Prosjektkatalogen" },
  { id: "sikkerhetsniva", label: "Sikkerhetsnivå i cplt" },
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
            cplt bruker <code className={code}>standard</code> hvis du ikke velger noe annet. Der er{" "}
            <code className={code}>gh_guard</code> og <code className={code}>git_guard</code> på:{" "}
            <code className={code}>git_guard</code> stopper push til standardgrenen og force push, og{" "}
            <code className={code}>gh_guard</code> stopper <code className={code}>gh pr merge</code>. Agenten kan pushe
            egne grener og åpne pull requests.
          </BodyLong>
          <BodyLong>
            <code className={code}>strict</code> blokkerer all push. Agenten kan ikke pushe en gren eller åpne en pull
            request fra den, så du må pushe selv. Strict legger også til tvungen proxy og{" "}
            <code className={code}>proxy.default_allowlist</code>: da når agenten bare hostene på lista til cplt og det{" "}
            <code className={code}>proxy.allowed_domains</code> peker på. Verken{" "}
            <code className={code}>nav-pilot doctor</code> eller innstillingssiden anbefaler strict. Velg det bare hvis
            du vil låse nettverket og kan leve med å pushe selv. I nav-pilot velger du det med{" "}
            <code className={code}>nav-pilot config setup --advanced</code>, se{" "}
            <NextLink href="/nav-pilot/guider/kom-i-gang#autonomi" className={linkClass}>
              Hvor mye skal agenten gjøre selv?
            </NextLink>{" "}
            Hele sammenligningen står i{" "}
            <NextLink href="/nav-pilot/referanse#sikkerhetsniva" className={linkClass}>
              referansen
            </NextLink>
            .
          </BodyLong>
          <Box background="warning-soft" borderRadius="8" padding="space-16">
            <BodyLong>
              Lista til cplt dekker GitHub Copilot og de offentlige pakkeregistrene, men ingenting hos Nav. Setter du
              strict for hånd, slutter telemetrien fra nav-pilot å komme fram. Skills som{" "}
              <code className={code}>aksel-builder</code>, <code className={code}>observability-debugging</code> og{" "}
              <code className={code}>nav-auth</code> mister hostene de er bygget rundt, og ingenting på skjermen sier
              hvorfor.
            </BodyLong>
          </Box>
          <BodyLong>Sett det derfor med nav-pilot:</BodyLong>
          <CodeBlock compact>
            {`nav-pilot config setup --advanced   # velg «Allowlist only (cplt strict)»
nav-pilot config                    # eller raden «cplt strict preset (blocks all pushes)»`}
          </CodeBlock>
          <BodyLong>
            nav-pilot skriver host-lista til <code className={code}>~/.nav-pilot/cplt-allowed-domains.txt</code>, peker{" "}
            <code className={code}>proxy.allowed_domains</code> dit, og setter presetet til slutt, så låsen aldri blir
            aktiv uten hostene. Har du en egen <code className={code}>proxy.allowed_domains</code>, lar nav-pilot den
            være og sier at du må legge til hostene selv. Nøkler du har satt selv, gjelder foran presetet.
          </BodyLong>
          <BodyLong>
            Fila har hele lista, ikke bare Nav-hostene. Med <code className={code}>proxy.allowed_domains</code> slipper
            proxyen gjennom hostene i fila og agentens egne hoster, og pakkeregistrene bare når{" "}
            <code className={code}>proxy.default_allowlist</code> er på. Lista til cplt er per agent: lista for copilot
            har GitHub og Copilot, den for opencode har <code className={code}>opencode.ai</code> og{" "}
            <code className={code}>models.dev</code>.
          </BodyLong>
          <BodyLong>
            Den lokale modellen går gjennom en løkkevakt på <code className={code}>127.0.0.1</code>. cplt blokkerer
            localhost som standard, så nav-pilot sender porten med som{" "}
            <code className={code}>--allow-localhost &lt;port&gt;</code> ved hver oppstart. Én port slipper gjennom
            tvungen proxy på både macOS og Linux, så strict og lokal modell går sammen.
          </BodyLong>
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
