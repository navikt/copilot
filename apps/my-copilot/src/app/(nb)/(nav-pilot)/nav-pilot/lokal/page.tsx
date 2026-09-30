import { BodyLong, Box, Label, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";
import { NAV_PILOT_BREW_INSTALL, NAV_PILOT_BREW_UPGRADE } from "@/lib/install-commands";
import { getLocalModels } from "@/lib/local-models";

// "AI" stays in the AI credit wording: AI credits is GitHub's name for the billing unit. Other Norwegian text says KI.

export const metadata: Metadata = {
  title: "Kom i gang med lokal modell på Mac",
  description:
    "Installer nav-pilot, last ned en kodemodell og kjør en første økt der hovedagenten i skyen sender en oppgave til modellen på Macen din.",
};

const TOC: TocItem[] = [
  { id: "sjekk-maskinen", label: "1. Sjekk maskinen" },
  { id: "installer", label: "2. Installer" },
  { id: "sett-opp-modellen", label: "3. Sett opp modellen" },
  { id: "forste-okt", label: "4. Første økt" },
  { id: "forste-decide", label: "5. Første decide" },
  { id: "videre", label: "6. Veien videre" },
];

const CHECK_MACHINE = `uname -m                                            # arm64
sysctl -n hw.memsize | awk '{print $1/2^30 " GB"}'  # minne
df -h ~                                             # ledig disk`;

const INSTALL = `${NAV_PILOT_BREW_INSTALL}
${NAV_PILOT_BREW_UPGRADE}    # har du den fra før`;

const SET_UP = `nav-pilot alpha local init
nav-pilot alpha local status`;

const FIRST_SESSION = `nav-pilot config set client opencode
cd ~/kode/mitt-repo
nav-pilot`;

const FIRST_DECIDE = `echo "Legg til retry i klienten" | nav-pilot alpha decide \\
  "Does the commit message explain why the change was made, beyond describing what the diff already shows?" \\
  --options yes,no --evidence -`;

export default async function LokalIntro() {
  const { models } = await getLocalModels();
  const m = models.find((x) => x.default) ?? models[0];

  return (
    <DocPage
      label="Introduksjon"
      title="Kom i gang med lokal modell på Mac"
      description={`Du installerer nav-pilot, laster ned en kodemodell og kjører en første økt der hovedagenten i skyen sender en oppgave til modellen på Macen din. Nedlastingen er på omtrent ${m.weights_gb + 1} GB. Hvor lang tid den tar, har vi ikke målt på en ny Mac.`}
      badge={
        <Tag variant="warning" size="small" className="uppercase tracking-wide">
          Alfa
        </Tag>
      }
      toc={TOC}
    >
      <div id="hva-du-far">
        <BodyLong>
          Hovedagenten i skyen planlegger og bestemmer. Mekaniske oppgaver, som å føre et nytt argument gjennom mange
          filer, sender den til <code className={code}>local-worker</code>, en underagent som kjører på Macen din og
          ikke bruker AI-kreditter. Med <code className={code}>nav-pilot alpha decide</code> stiller du den samme
          modellen et flervalgsspørsmål fra en hook eller et skript. Spørsmålet og grunnlaget forlater ikke maskinen.
          Ved utsending ser hovedagenten i skyen oppgaven den selv skrev, og det korte svaret fra den lokale modellen.
        </BodyLong>
      </div>
      <BodyLong>
        Alfa betyr at kommandoene ligger under <code className={code}>nav-pilot alpha</code> og kan endre seg uten
        varsel. Resten av nav-pilot er beta.
      </BodyLong>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="sjekk-maskinen" size="medium" level="2">
            1. Sjekk maskinen
          </LinkableHeading>
          <CodeBlock compact>{CHECK_MACHINE}</CodeBlock>
          <BodyLong>
            Du trenger Apple Silicon (<code className={code}>arm64</code>), minst {m.min_ram_gb} GB minne og omtrent{" "}
            {m.weights_gb + 1} GB ledig disk. Det er det <code className={code}>init</code> anslår å laste ned: vektene
            på {m.weights_gb} GB og omtrent 1 GB til Python-miljøet.
          </BodyLong>
          <BodyLong>
            Har Macen 64 GB minne eller mer, kan du etter oppsettet bytte til en større utgave av standardmodellen. Se{" "}
            <NextLink href="/nav-pilot/guider/lokal#modell-64-gb" className={linkClass}>
              Større modell for Macer med 64 GB
            </NextLink>
            . Hovedagenten sender den ingen oppgaver ennå.
          </BodyLong>
          <BodyLong>
            Har du Linux, en Intel-Mac eller mindre minne, gå til{" "}
            <NextLink href="/nav-pilot/lokal/egen-server" className={linkClass}>
              Kom i gang med egen server
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          {/* #kom-i-gang is the old anchor for this step (§2.2 in docs/nav-pilot-dokumentasjon-forslag.md). */}
          <div id="kom-i-gang">
            <LinkableHeading id="installer" size="medium" level="2">
              2. Installer
            </LinkableHeading>
          </div>
          <CodeBlock compact>{INSTALL}</CodeBlock>
          <BodyLong>
            Har du allerede gjort{" "}
            <NextLink href="/kom-i-gang" className={linkClass}>
              Kom i gang
            </NextLink>
            , hopp over dette steget.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="sett-opp-modellen" size="medium" level="2">
            3. Sett opp modellen
          </LinkableHeading>
          <CodeBlock compact>{SET_UP}</CodeBlock>
          <BodyLong>
            <code className={code}>init</code> viser modellen, hva den krever, hva den laster ned og hvor den henter det
            fra. Så spør den før den starter. Er minnegrensen i macOS for lav, ber den om passordet ditt én gang for å
            heve den. Når den er ferdig, kjører serveren.
          </BodyLong>
          <BodyLong>
            Vi har ikke testet <code className={code}>brew install</code> og <code className={code}>init</code> på en
            Mac uten modellen fra før.
          </BodyLong>
          <BodyLong>
            <code className={code}>status</code> skal vise linjene <code className={code}>Model</code>,{" "}
            <code className={code}>Server</code> og <code className={code}>Wired limit</code> uten advarsler.
          </BodyLong>
          <BodyLong>
            Etter en omstart av maskinen kjører du <code className={code}>nav-pilot alpha local start</code>.
            Minnegrensen nullstilles ved omstart, så <code className={code}>start</code> spør før den hever den igjen.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="forste-okt" size="medium" level="2">
            4. Første økt
          </LinkableHeading>
          <CodeBlock compact>{FIRST_SESSION}</CodeBlock>
          <BodyLong>
            Utsending krever opencode som klient. Copilot CLI har ingen underagent som kan kjøre lokalt.
          </BodyLong>
          <BodyLong>
            Be om en mekanisk endring over flere filer, for eksempel «legg til parameteren{" "}
            <code className={code}>ctx</code> i alle kall til <code className={code}>hentBruker</code>». Med{" "}
            <code className={code}>balanced</code> stopper nav-pilot hovedagenten én gang når den selv redigerer en
            femte fil eller et tiende kallsted, og ber den sende resten til <code className={code}>local-worker</code>.
            Når <code className={code}>local-worker</code> er ferdig, ber nav-pilot hovedagenten bygge prosjektet og
            kjøre testene før den godtar endringen.
          </BodyLong>
          <BodyLong>
            Med <code className={code}>aggressive</code> slipper redigeringen gjennom først når fila er sendt til{" "}
            <code className={code}>local-worker</code>. Nye filer går også dit. <code className={code}>aggressive</code>{" "}
            er standard. Har du satt <code className={code}>local_dispatch</code> selv, beholder du verdien din. Vil du
            tilbake til <code className={code}>balanced</code>, bruk{" "}
            <code className={code}>nav-pilot config set local_dispatch balanced</code>.
          </BodyLong>
          <BodyLong>
            I målingen fra september 2026 (re-probe 7, Sonnet 5 som hovedagent) sendte hovedagenten arbeid til den
            lokale modellen i alle 17 gyldige kjøringer med mange kallsteder eller nye filer, og alle 17 besto bygg og
            tester. Var endringen liten, sendte den ingenting (0 av 5). Det kostet 0,83–2,1 ganger så mange kreditter og
            tok 2,7–3,6 ganger så lang tid som når skymodellen gjorde alt selv. Hovedagenten gjorde likevel om 15 av 27
            oppgaver med nye filer selv. To kjøringer til ble avbrutt før de var ferdige, og i én av dem ble koden
            liggende i stykker. Med <code className={code}>balanced</code> sendte hovedagenten arbeid i 2 av 20
            kjøringer. En ny måling 29. september viste at <code className={code}>balanced</code> kostet 1,5–1,6 ganger
            så mye som når skymodellen gjorde alt selv, uten at flere oppgaver besto. Derfor er{" "}
            <code className={code}>aggressive</code> standard fra 30. september 2026.
          </BodyLong>
          <BodyLong>
            Etterpå viser <code className={code}>nav-pilot alpha local status</code> at serveren fortsatt kjører, og
            linja <code className={code}>Log</code> viser hvor serverloggen ligger. Kjører ikke serveren når du starter,
            sier nav-pilot fra før økten at den kommer til å gå helt i skyen, og gjentar etter økten at den gjorde det.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="forste-decide" size="medium" level="2">
            5. Første decide
          </LinkableHeading>
          <CodeBlock compact>{FIRST_DECIDE}</CodeBlock>
          <BodyLong>
            Du får en sannsynlighet per svar. Når serveren er varm, bruker modellen vanligvis under et halvt sekund.
            Neste steg er{" "}
            <NextLink href="/nav-pilot/lokal/decide" className={linkClass}>
              Din første decide-hook
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="videre" size="medium" level="2">
            6. Veien videre
          </LinkableHeading>
          <Bullets>
            <li>
              <NextLink href="/nav-pilot/guider/lokal#utsending" className={linkClass}>
                Styr utsendingen
              </NextLink>
            </li>
            <li>
              <NextLink href="/nav-pilot/guider/lokal#bytte-lokal-modell" className={linkClass}>
                Bytte lokal modell
              </NextLink>
            </li>
            <li>
              <NextLink href="/nav-pilot/guider/feilsoking#lokal" className={linkClass}>
                Når den lokale modellen henger
              </NextLink>
            </li>
            <li>
              <NextLink href="/nav-pilot/forklaring/lokal-modell#malte-grenser" className={linkClass}>
                Målte grenser
              </NextLink>
            </li>
          </Bullets>
          <Box background="neutral-soft" padding="space-16" borderRadius="8">
            <VStack gap="space-8">
              <Label size="small">Skru det av</Label>
              <CodeBlock compact>
                {`nav-pilot alpha local off     # alt går i skyen igjen, vektene blir liggende
nav-pilot alpha local purge   # viser hva som slettes, --yes sletter`}
              </CodeBlock>
            </VStack>
          </Box>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="lenker" size="medium" level="2">
            Les mer og gi tilbakemelding
          </LinkableHeading>
          <Bullets>
            <li>
              <NextLink href="/nyheter/nav-pilot-alpha-decide" className={linkClass}>
                Når du ikke trenger en agent, bare et svar
              </NextLink>{" "}
              (nyhetssak om decide)
            </li>
            <li>
              <NextLink href="/en/news/alpha-decide" className={linkClass}>
                When you need an answer, not an agent
              </NextLink>{" "}
              (samme sak på engelsk)
            </li>
            <li>
              <NextLink href="/nyheter/lokale-modeller-i-nav-pilot" className={linkClass}>
                Nyhetssaken om lokale modeller
              </NextLink>
            </li>
            <li>
              <NextLink href="/nav-pilot/forklaring/personvern" className={linkClass}>
                Personvern og telemetri
              </NextLink>
            </li>
            <li>
              Noe som ikke virker, eller et spørsmål du vil ha målt? Kjør{" "}
              <code className={code}>nav-pilot feedback</code> eller{" "}
              <a href="https://github.com/navikt/copilot/issues/new/choose" className={linkClass}>
                lag et issue i navikt/copilot
              </a>
              .
            </li>
          </Bullets>
        </VStack>
      </section>
    </DocPage>
  );
}
