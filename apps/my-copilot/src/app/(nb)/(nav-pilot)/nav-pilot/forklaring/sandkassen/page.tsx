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
  { id: "nar-strict-ikke-anbefales", label: "Når strict ikke anbefales" },
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
                . Det er det enkleste. Velger du noe annet, må du selv finne ut hvordan klienten isolerer agenten, og
                slå det på. Holder ikke det, må du isolere på en annen måte, for eksempel med en VM eller en container.
                Ikke kjør agenter med full tilgang til maskinen.
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
            Står du i et git-worktree, eller vil du at agenten skal lage worktrees til underagenter, se{" "}
            <NextLink href="/nav-pilot/guider/worktrees" className={linkClass}>
              Worktrees med nav-pilot og cplt
            </NextLink>
            .
          </BodyLong>
          <BodyLong>
            Trenger repoet mer enn standard, for eksempel localhost til Gradle eller en database, se{" "}
            <NextLink href="/nav-pilot/guider/cplt-oppsett" className={linkClass}>
              Sett opp cplt i et repo
            </NextLink>
            .
          </BodyLong>
          <BodyLong>
            Får du en feilmelding fra sandkassen, slå den opp i{" "}
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
            <code className={code}>nav-pilot doctor</code> sjekker sikkerhetsnivået og anbefaler{" "}
            <code className={code}>sandbox.preset = strict</code>. <code className={code}>gh_guard</code> og{" "}
            <code className={code}>git_guard</code> er på allerede i <code className={code}>standard</code>. I{" "}
            <code className={code}>standard</code> stopper <code className={code}>git_guard</code> push til
            standardgrenen og force push. Strict legger til tvungen proxy, en <code className={code}>git_guard</code>{" "}
            som stopper all push, og <code className={code}>proxy.default_allowlist</code>. Den siste er den viktige. Da
            når agenten bare hostene på lista til cplt og det <code className={code}>proxy.allowed_domains</code> peker
            på. Alt annet blokkeres. Hele sammenligningen står i{" "}
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
          <CodeBlock compact>{"nav-pilot config     # velg raden «cplt security posture»"}</CodeBlock>
          <BodyLong>
            nav-pilot skriver host-lista til <code className={code}>~/.nav-pilot/cplt-allowed-domains.txt</code>, peker{" "}
            <code className={code}>proxy.allowed_domains</code> dit, og setter presetet til slutt. Da blir låsen aldri
            aktiv uten hostene. Har du en egen <code className={code}>proxy.allowed_domains</code>, lar nav-pilot den
            være og sier at du må legge til hostene selv. Konfigen til cplt er din, så nav-pilot endrer den aldri uten å
            si fra, og nøkler du har satt selv, gjelder fortsatt foran presetet.
          </BodyLong>
          <BodyLong>
            Fila har hele lista, ikke bare Nav-hostene. Med <code className={code}>proxy.allowed_domains</code> slipper
            proxyen bare gjennom hostene i fila og agentens egne hoster. Pakkeregistrene kommer med bare når{" "}
            <code className={code}>proxy.default_allowlist</code> er på. I cplt fra før 29. september 2026 gjaldt det
            også agentens egne hoster. Og lista til cplt er per agent: bare lista for copilot har GitHub og Copilot,
            mens den for opencode har <code className={code}>opencode.ai</code> og{" "}
            <code className={code}>models.dev</code>.
          </BodyLong>
          <BodyLong>
            Den lokale modellen går gjennom en løkkevakt på <code className={code}>127.0.0.1</code>. cplt blokkerer
            localhost som standard, så nav-pilot sender porten med som{" "}
            <code className={code}>--allow-localhost &lt;port&gt;</code> ved hver oppstart. Det er én port, ikke
            bryteren for hele maskinen, som <code className={code}>proxy.forced</code> overstyrer. Én port slipper
            gjennom tvungen proxy på både macOS og Linux, så strict og lokal modell går fint sammen.
          </BodyLong>
          <LinkableHeading id="nar-strict-ikke-anbefales" size="small" level="3">
            Når strict ikke anbefales
          </LinkableHeading>
          <BodyLong>
            På Linux krever <code className={code}>proxy.forced</code> at kjernen kan begrense nettverket med Landlock:
            ABI v4, altså kjerne 6.7 eller nyere med Landlock slått på. Under det starter ikke cplt i det hele tatt. En
            anbefaling som stopper hver økt på maskinen, er verre enn problemet den løser, så{" "}
            <code className={code}>nav-pilot doctor</code> og innstillingssiden anbefaler ikke strict der, og sier
            hvorfor.
          </BodyLong>
          <BodyLong>
            nav-pilot spør kjernen direkte, med samme systemkall som cplt, i stedet for å lese{" "}
            <code className={code}>uname</code>. Landlock kan være kompilert bort eller slått av ved oppstart, og da
            ville en sjekk av versjonsnummeret sagt «går fint» rett før cplt nekter å starte. macOS har ingen slik
            grense. Der gjør Seatbelt samme jobben.
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
