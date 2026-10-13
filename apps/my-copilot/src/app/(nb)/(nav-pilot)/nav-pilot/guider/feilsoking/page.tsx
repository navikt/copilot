import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Feilsøking",
  description:
    "Sjekk maskinen med nav-pilot doctor, få en MCP-server til å virke, se hva cplt blokkerer, og få liv i en lokal modell som henger.",
};

const TOC: TocItem[] = [
  { id: "doctor", label: "Sjekk maskinen" },
  { id: "mcp", label: "Når en MCP-server ikke virker" },
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
nav-pilot mcp list            # MCP-servere som ikke virker, og kommandoen som retter det
nav-pilot alpha local doctor  # bare egen LLM-server (local_endpoint)`}
          </CodeBlock>
          <BodyLong>
            <code className={code}>doctor</code> endrer ingenting, og hvert problem kommer med kommandoen som løser det.
            Den avslutter med kode 0 også når den finner noe, så les linjene med «Solution».
          </BodyLong>
          <BodyLong>
            <code className={code}>alpha local doctor</code> sjekker verktøykall, logprobs, kontekstlengde og tid til
            første token på serveren du kjører selv. Kontekstsjekken kan ta minutter på en CPU. For modellen nav-pilot
            setter opp på Mac bruker du <code className={code}>nav-pilot alpha local status</code>.
          </BodyLong>
          <BodyLong>
            Finner du en feil, meld den med <code className={code}>nav-pilot feedback</code>. Den åpner et issue i
            navikt/copilot med versjon og systeminformasjon fylt inn. Ingenting går ut før du sender inn issuet.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="mcp" size="medium" level="2">
            Når en MCP-server ikke virker
          </LinkableHeading>
          <BodyLong>
            Start med <code className={code}>nav-pilot mcp list</code>. Den endrer ingenting. Øverst står det som
            hindrer serverne du har slått på, med kommandoen som retter det: et navn organisasjonens policy blokkerer,
            en host eller localhost-port cplt stopper, en pakkekjører som mangler, eller en server registeret har tatt
            ut.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot mcp list                            # problemer først, så serverne i registeret
nav-pilot mcp enable <navn>                   # slå på i Copilot CLI og opencode
nav-pilot mcp enable <navn> --client copilot  # bare i én klient
nav-pilot mcp disable <navn>                  # slå av, og steng hostene bare den trengte`}
          </CodeBlock>
          <BodyLong>
            Navnet er hele navnet i registeret eller delen etter siste skråstrek, som{" "}
            <code className={code}>figma-mcp</code>. Start en ny økt etterpå, så laster klienten serveren. Hvilke
            servere som finnes, står under{" "}
            <NextLink href="/nav-pilot/klienter#mcp-register" className={linkClass}>
              MCP-registeret
            </NextLink>
            .
          </BodyLong>
          <LinkableHeading id="mcp-verktoy" size="small" level="3">
            Verktøy som er slått av
          </LinkableHeading>
          <BodyLong>
            Registeret gir hvert verktøy en risikoklasse. <code className={code}>enable</code> slår på{" "}
            <code className={code}>read</code> og <code className={code}>write</code>. Av står{" "}
            <code className={code}>external</code>, som handler i et annet system (en issue, en PR, en fil i Figma), og{" "}
            <code className={code}>host-exec</code>, som kjører på maskinen din utenfor cplt-sandkassen. I en terminal
            viser <code className={code}>enable</code> alle verktøyene med klasse, og du velger selv. Et{" "}
            <code className={code}>host-exec</code>-verktøy må du i tillegg si ja til.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot mcp enable <navn> --tools <a,b>   # nøyaktig disse verktøyene
nav-pilot mcp enable <navn> --all-tools     # alle, også dem serveren legger til senere`}
          </CodeBlock>
          <BodyLong>
            Har du serveren fra før, lar <code className={code}>enable</code> oppføringen være som den er, med mindre du
            gir <code className={code}>--tools</code> eller <code className={code}>--all-tools</code>. Da endres bare
            verktøyene.
          </BodyLong>
          <LinkableHeading id="mcp-intellij" size="small" level="3">
            IntelliJ og verktøy utenfor sandkassen
          </LinkableHeading>
          <BodyLong>
            IntelliJ-serveren har verktøy som <code className={code}>execute_terminal_command</code>, som kjører i
            IntelliJ og ikke i sandkassen. Et slikt verktøy slår du bare på med{" "}
            <code className={code}>--allow-host-exec</code>. Er det på, advarer <code className={code}>mcp list</code>{" "}
            før du åpner localhost-porten til IntelliJ, fordi agenten da når verktøyet. Rettingen den foreslår, slår av
            verktøyene som kjører utenfor sandkassen først, og åpner porten etterpå.
          </BodyLong>
          <LinkableHeading id="mcp-github" size="small" level="3">
            GitHub-serveren leser bare
          </LinkableHeading>
          <BodyLong>
            GitHubs MCP-server får det skrivebeskyttede endepunktet. Velger du et verktøy som skriver til GitHub, som{" "}
            <code className={code}>issue_write</code>, bytter nav-pilot til det fulle endepunktet. Da kan agenten skrive
            til GitHub gjennom MCP, forbi vakta cplt har på <code className={code}>gh</code>.{" "}
            <code className={code}>mcp list</code> merker det med <code className={code}>full endpoint</code> og gir
            kommandoen som tar deg tilbake.
          </BodyLong>
          <LinkableHeading id="mcp-hoster" size="small" level="3">
            Hoster i sandkassen
          </LinkableHeading>
          <BodyLong>
            Når du starter nav-pilot i en terminal, spør den om cplt skal slippe gjennom hostene serverne dine trenger.
            Hostene hentes fra registeret, aldri fra MCP-konfigen din: serverens adresse, og{" "}
            <code className={code}>sandboxHosts</code> for andre hoster den trenger, som innloggingen til Figma. Enter
            betyr nei. Uten terminal spør nav-pilot ikke, og slipper bare gjennom hoster du alt har sagt ja til.
          </BodyLong>
          <BodyLong>
            Har du sagt nei, spør ikke nav-pilot igjen ved oppstart. Kjør{" "}
            <code className={code}>nav-pilot mcp enable &lt;navn&gt;</code> i en terminal, så får du spørsmålet på nytt.
            Med <code className={code}>mcp_hosts = off</code> spør den aldri.
          </BodyLong>
          <BodyLong>
            Satt du <code className={code}>strict</code> med nav-pilot, legger nav-pilot hostene du sa ja til i sin egen
            host-liste. Har du laget lista selv med <code className={code}>proxy.allowed_domains</code>, gjør den ikke
            det. Da viser <code className={code}>mcp list</code> at cplt stopper hosten, og du legger den til selv:
          </BodyLong>
          <CodeBlock compact>{`cplt config set allow.domains <host>`}</CodeBlock>
          <BodyLong>
            Mangler registeret en host serveren trenger, må oppføringen i registeret rettes. Se{" "}
            <NextLink href="/nav-pilot/klienter#legg-til-server" className={linkClass}>
              Få en server inn i registeret
            </NextLink>
            .
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
            . Trenger repoet mer enn standard, se{" "}
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
            cplt stopper ting på to steder. Proxyen stopper nettverkskall til hoster på internett, og skriver dem til
            stderr. Kjernen stopper filer, programmer og localhost, og det står bare i verktøyet som feilet, se{" "}
            <a href="#kjernen" className={linkClass}>
              Filer, programmer og localhost
            </a>
            .
          </BodyLong>
          <BodyLong>
            Med en host-liste, som på <code className={code}>strict</code> satt med nav-pilot, viser proxyen
            blokkeringene uten at du gjør noe. På <code className={code}>standard</code> uten egen{" "}
            <code className={code}>proxy.allowed_domains</code> tier den om dem: treff i cplts egen blokkliste, stengte
            porter, hoster som peker til private eller link-local IP-adresser, og oppslag som feiler. Vil du se dem,
            sett nivået selv:
          </BodyLong>
          <CodeBlock compact>{`cplt config set proxy.log_level blocked`}</CodeBlock>
          <BodyLong>
            Vil du ha en fil å lese etterpå, bruk <code className={code}>proxy.log_file</code>. Den dekker det proxyen
            avgjør, ikke det gh- og git-vakta avgjør. Sikkerhetsnivåene står i{" "}
            <NextLink href="/nav-pilot/referanse#sikkerhetsniva" className={linkClass}>
              referansen
            </NextLink>
            .
          </BodyLong>
          <LinkableHeading id="kjernen" size="small" level="3">
            Filer, programmer og localhost
          </LinkableHeading>
          <BodyLong>
            Meldingen er <code className={code}>Operation not permitted</code>, <code className={code}>EPERM</code>{" "}
            eller <code className={code}>connect EPERM 127.0.0.1:…</code>, og proxyloggen er tom. Spør cplt om det som
            feilet. Svaret sier om det er tillatt eller stoppet, hvorfor, og hva som åpner det:
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
            <code className={code}>--allow-port</code>, se{" "}
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
            andre programmer også, så se etter navnet på det som feilet. På Linux finnes ikke loggen, fordi Landlock
            ikke logger det den stopper.
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
            at den er opptatt. Vent litt og prøv igjen. Startet serveren før du oppgraderte nav-pilot, kjør{" "}
            <code className={code}>nav-pilot alpha local restart</code>.
          </BodyLong>
          <BodyLong>
            nav-pilot avslutter en runde, og sier fra i økten, hvis modellen gjør det samme verktøykallet fire ganger på
            rad med samme resultat, eller åtte ganger på rad uansett resultat. Det er en vakt mot at modellen står fast,
            ikke en feil i koden din. Grensene endrer du med <code className={code}>local_loop_guard</code>.
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
            feil, eller om ventetiden ikke er verdt det.
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
