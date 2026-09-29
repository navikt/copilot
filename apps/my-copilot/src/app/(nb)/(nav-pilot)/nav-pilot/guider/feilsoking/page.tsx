import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Feilsøking",
  description: "Sjekk maskinen med nav-pilot doctor, se hva cplt blokkerer, og få liv i en lokal modell som henger.",
};

const TOC: TocItem[] = [
  { id: "doctor", label: "Sjekk maskinen" },
  { id: "cplt-feil", label: "Feil fra cplt" },
  { id: "blokkeringer", label: "Se hva cplt blokkerer" },
  { id: "kjernen", label: "Filer, programmer og localhost" },
  { id: "lokal", label: "Når den lokale modellen henger" },
];

export default function Feilsoking() {
  return (
    <DocPage
      label="Guider"
      upgrade
      title="Feilsøking"
      description="Start med nav-pilot doctor. Den finner det meste og sier hva du skal gjøre."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="doctor" size="medium" level="2">
            Sjekk maskinen
          </LinkableHeading>
          <CodeBlock compact>
            {`nav-pilot doctor              # konfig, installasjon, hooks, klienter, cplt og git
nav-pilot config validate     # bare konfigfila: syntaks, nøkler og verdier
nav-pilot alpha local doctor  # bare egen server (local_endpoint)`}
          </CodeBlock>
          <BodyLong>
            <code className={code}>doctor</code> sjekker konfigfila, det som er installert i{" "}
            <code className={code}>~/.copilot</code> og i repoet, hooks, klientene (Copilot CLI, opencode og pi), cplt,
            modellvalgene, sandkassen og git. Hvert problem kommer med kommandoen som løser det. Den endrer ingenting,
            og avslutter med kode 0 også når den finner noe, så les linjene med «Solution».
          </BodyLong>
          <BodyLong>
            <code className={code}>alpha local doctor</code> sjekker verktøykall, logprobs, kontekstlengde og tid til
            første token på serveren du kjører selv. Kontekstsjekken kan ta minutter på en CPU. Den gjelder ikke
            modellen nav-pilot setter opp på Mac. Der bruker du{" "}
            <code className={code}>nav-pilot alpha local status</code>.
          </BodyLong>
          <BodyLong>
            Finner du en feil, meld den med <code className={code}>nav-pilot feedback</code>. Den åpner et issue i
            navikt/copilot med versjon og systeminformasjon fylt inn. Ingenting går ut før du sender inn issuet.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="cplt-feil" size="medium" level="2">
            Feil fra cplt
          </LinkableHeading>
          <BodyLong>
            Stopper sandkassen noe agenten skal gjøre, slå opp feilmeldingen i{" "}
            <NextLink href="/nav-pilot/guider/cplt-feilmeldinger" className={linkClass}>
              Feil i sandkassen
            </NextLink>
            . Der står årsaken og kommandoen som løser det. Setter du opp et nytt repo, se{" "}
            <NextLink href="/nav-pilot/guider/cplt-oppsett" className={linkClass}>
              Sett opp cplt i et repo
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="blokkeringer" size="medium" level="2">
            Se hva cplt blokkerer
          </LinkableHeading>
          <BodyLong>
            cplt stopper ting på to steder. Proxyen stopper nettverkskall til hoster på internett. Kjernen stopper
            filer, programmer og localhost. Proxyloggen under viser bare det første. For det andre, se{" "}
            <a href="#kjernen" className={linkClass}>
              Filer, programmer og localhost
            </a>
            .
          </BodyLong>
          <BodyLong>
            <code className={code}>proxy.log_level</code> bestemmer hva proxyen i cplt skriver til stderr. Standard er{" "}
            <code className={code}>none</code>, men cplt hever den selv til <code className={code}>blocked</code> når en
            host-liste er aktiv. Setter du sikkerhetsnivået med nav-pilot, skriver den alltid en host-liste, så på
            strict ser du blokkeringene uten å gjøre noe.
          </BodyLong>
          <BodyLong>
            Kjører du <code className={code}>standard</code> uten egen{" "}
            <code className={code}>proxy.allowed_domains</code>, tier proxyen om alle blokkeringer: treff i cplts egen
            blokkliste, stengte porter, hoster som peker til private eller link-local IP-adresser, og oppslag som
            feiler. Vil du se dem, sett nivået selv:
          </BodyLong>
          <CodeBlock compact>{`cplt config set proxy.log_level blocked`}</CodeBlock>
          <BodyLong>
            Vil du ha en fil å lese etterpå, bruk <code className={code}>proxy.log_file</code>. Den dekker det proxyen
            avgjør, ikke det gh- og git-vakta avgjør. Seksjonen <code className={code}>[audit]</code> er fjernet, og
            cplt avviser nå <code className={code}>audit.*</code>-nøkler. <code className={code}>sandbox.audit</code> er
            noe annet og allerede på. Den viser hvilke filer agenten endret, etter at den er ferdig.
          </BodyLong>
          <BodyLong>
            Nivåene står i{" "}
            <NextLink href="/nav-pilot/referanse#sikkerhetsniva" className={linkClass}>
              referansen
            </NextLink>
            .
          </BodyLong>
          <LinkableHeading id="kjernen" size="small" level="3">
            Filer, programmer og localhost
          </LinkableHeading>
          <BodyLong>
            Stopper kjernen noe, ser du det bare i verktøyet som feilet:{" "}
            <code className={code}>Operation not permitted</code>, <code className={code}>EPERM</code> eller{" "}
            <code className={code}>connect EPERM 127.0.0.1:…</code>. Proxyloggen er tom. Spør cplt om det som feilet.
            Svarene sier om noe er tillatt eller stoppet, hvorfor, og hva som åpner det:
          </BodyLong>
          <CodeBlock compact>
            {`cplt check path ~/.gradle/gradle.properties   # får agenten lese fila?
cplt check path --write ~/.cache/verktøy       # får agenten skrive der?
cplt check net repo.adeo.no                    # slipper proxyen gjennom hosten?
cplt check exec docker                         # får agenten kjøre programmet?`}
          </CodeBlock>
          <BodyLong>
            <code className={code}>cplt check exec</code> sjekker om programmet får starte, ikke hvilke filer det leser
            etterpå. For localhost gjelder <code className={code}>allow.localhost</code>, ikke{" "}
            <code className={code}>--allow-port</code>, selv om <code className={code}>cplt check net</code> foreslår
            det. Se{" "}
            <NextLink href="/nav-pilot/guider/cplt-oppsett#stakker" className={linkClass}>
              Det stakken din trenger
            </NextLink>
            .
          </BodyLong>
          <BodyLong>
            På macOS kan du se alt kjernen stopper mens agenten jobber. Starter du cplt selv, bruk{" "}
            <code className={code}>--show-denials</code>. Linjene kommer i samme terminal som agenten:
          </BodyLong>
          <CodeBlock compact>{`cplt --show-denials`}</CodeBlock>
          <BodyLong>
            Starter du agenten med nav-pilot, kjør det samme filteret i en annen terminal mens agenten jobber:
          </BodyLong>
          <CodeBlock compact>
            {`log stream --style compact --info \
  --predicate 'eventMessage CONTAINS "Sandbox" AND eventMessage CONTAINS "deny"'`}
          </CodeBlock>
          <BodyLong>
            En linje ser slik ut:{" "}
            <code className={code}>Sandbox: cat(85111) deny(1) file-read-data /Users/deg/.npmrc</code>. Loggen tar med
            andre programmer på maskinen også, så se etter navnet på programmet som feilet. På Linux finnes ikke denne
            loggen, fordi Landlock ikke logger det den stopper.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="lokal" size="medium" level="2">
            Når den lokale modellen henger
          </LinkableHeading>
          <BodyLong>
            <code className={code}>status</code> skiller «treg» fra «død».
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot alpha local status
# står det hung: nav-pilot alpha local restart`}
          </CodeBlock>
          <BodyLong>
            mlx-lm henger seg opp på samtidige forespørsler, så serveren nav-pilot starter, tar én om gangen. Opptil
            åtte venter i kø i inntil ti minutter hver. Er køen full, eller ventetiden ute, svarer serveren 503 og sier
            at den er opptatt. Vent litt og prøv igjen. Startet serveren før du oppdaterte nav-pilot, har den ikke køen:
            kjør <code className={code}>nav-pilot alpha local restart</code>.
          </BodyLong>
          <BodyLong>
            nav-pilot avslutter en tur hvis modellen gjør det samme verktøykallet fire ganger på rad og får samme
            resultat hver gang, eller åtte ganger på rad uansett resultat, og sier fra i økten. Det er en vakt mot at
            modellen står fast, ikke en feil i koden din. Grensene endrer du med{" "}
            <code className={code}>local_loop_guard</code>.
          </BodyLong>
          <BodyLong>
            Går modellen tom for minne, for eksempel på en lang prompt, avslutter serveren seg selv. Neste økt sier{" "}
            <code className={code}>generation thread died, most likely out of memory</code>, med stien til tracebacken.
            Start den igjen med <code className={code}>nav-pilot alpha local restart</code>. Skjer det igjen, velg en
            modell med kortere kontekst.
          </BodyLong>
          <BodyLong>
            Starter du serveren på nytt midt i en økt, må du starte økten på nytt også. Den gamle er bundet til serveren
            som forsvant.
          </BodyLong>
          <BodyLong>
            Si fra med <code className={code}>nav-pilot feedback</code> om noe henger, om en endring kompilerer men er
            feil, eller om ventetiden ikke er verdt det. Dårlige erfaringer er like nyttige som gode.
          </BodyLong>
          <LinkableHeading id="lokal-start" size="small" level="3">
            Serveren vil ikke starte etter en omstart
          </LinkableHeading>
          <BodyLong>
            Minnegrensen i macOS nullstilles når maskinen starter på nytt. <code className={code}>start</code> spør da
            om å heve den med sudo, og trenger passordet ditt. Svarer du nei, eller kjører den uten terminal, som ved
            automatisk start, stopper den og skriver kommandoen du må kjøre selv:
          </BodyLong>
          <CodeBlock compact>
            {`sudo sysctl -w iogpu.wired_limit_mb=<MB>   # tallet står i feilmeldingen
nav-pilot alpha local start`}
          </CodeBlock>
          <BodyLong>
            Sier <code className={code}>start</code> at vektene ikke finnes på maskinen, kjør{" "}
            <code className={code}>nav-pilot alpha local init</code>. Den laster dem ned og starter serveren.
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
