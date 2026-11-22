import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Table, TableBody, TableRow, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, HeaderRow, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Kjente begrensninger i cplt",
  description:
    "Det cplt stenger med vilje: nøkler og hemmeligheter, sky-CLI-er, Docker, nettverk og git-operasjoner. Med det du kan gjøre i stedet.",
};

const TOC: TocItem[] = [
  { id: "filer", label: "Filer og hemmeligheter" },
  { id: "verktoy", label: "Verktøy" },
  { id: "nettverk", label: "Nettverk" },
  { id: "git", label: "Git" },
  { id: "nais", label: "nais-CLI" },
];

const SOCKET = "$HOME/Library/Application Support/naisdevice/agent.sock";

const NAIS_ROWS: [string, string][] = [
  ["nais --version, nais --help", "Virker."],
  ["nais device status", "unable to connect to naisdevice; make sure naisdevice is running"],
  ["nais status, nais app …, nais secret …", "ERROR: Please run nais login --nais to (re-)authenticate."],
  [
    "nais login (uten flagg), nais kubeconfig, nais postgres …",
    "Feiler. De går via gcloud, og ~/.config/gcloud er stengt.",
  ],
];

const FAQ = "/nav-pilot/guider/cplt-feilmeldinger";
const KNOWN_IMPACTS = "https://github.com/navikt/cplt/blob/main/docs/known-impacts.md";

export default function CpltBegrensninger() {
  return (
    <DocPage
      label="Referanse"
      upgrade
      title="Kjente begrensninger i cplt"
      description="Det cplt stenger med vilje, og hva du gjør i stedet. Alt gjelder når nav-pilot starter agenten i cplt."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="filer" size="medium" level="2">
            Filer og hemmeligheter
          </LinkableHeading>
          <Bullets>
            <li>
              <code className={code}>~/.ssh</code>, <code className={code}>~/.gnupg</code>,{" "}
              <code className={code}>~/.aws</code>, <code className={code}>~/.azure</code>,{" "}
              <code className={code}>~/.kube</code> og <code className={code}>~/.config/gcloud</code> er stengt. Du kan
              åpne én fil om gangen for lesing, ikke hele mappa:
            </li>
          </Bullets>
          <CodeBlock
            compact
          >{`cplt config set allow.read ~/.config/gcloud/application_default_credentials.json`}</CodeBlock>
          <Bullets>
            <li>
              macOS: <code className={code}>.env</code>-filer og nøkkelfiler i prosjektet er stengt. På Linux er de
              åpne, se{" "}
              <NextLink href={`${FAQ}#env-filer`} className={linkClass}>
                Feil i sandkassen
              </NextLink>
              .
            </li>
            <li>
              Miljøvariabler som kan være hemmeligheter, blir fjernet. Send inn dem du trenger med{" "}
              <code className={code}>sandbox.pass_env</code>.
            </li>
            <li>
              På macOS er <code className={code}>~/Library/Keychains</code> stengt når cplt sender inn gh-tokenet, og på
              Linux og i WSL fjerner cplt <code className={code}>DBUS_SESSION_BUS_ADDRESS</code> og stenger
              nøkkelringfiler. For Copilot på Linux leser cplt i stedet den lagrede innloggingen fra nøkkelringen på
              maskinen før sandkassen starter, og sender den inn som <code className={code}>COPILOT_GITHUB_TOKEN</code>.
              Logg inn utenfor cplt, ikke med <code className={code}>/login</code> inne i sesjonen, se{" "}
              <NextLink href={`${FAQ}#keyring`} className={linkClass}>
                System vault not available
              </NextLink>
              .
            </li>
            <li>
              macOS: <code className={code}>~/Desktop</code> og <code className={code}>~/Documents</code> er stengt av
              macOS, også uten cplt. Kopier filen inn i prosjektet, eller gi terminalen full disktilgang.
            </li>
          </Bullets>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="verktoy" size="medium" level="2">
            Verktøy
          </LinkableHeading>
          <Bullets>
            <li>
              <code className={code}>gcloud</code>, <code className={code}>aws</code>, <code className={code}>az</code>{" "}
              og <code className={code}>kubectl</code> med dine påloggingsdata virker ikke. Kjør dem selv, utenfor cplt.
            </li>
            <li>
              Docker og Testcontainers er stengt, fordi Docker gir tilgang til hele maskinen. Se{" "}
              <NextLink href="/nav-pilot/guider/cplt-gradle#testcontainers" className={linkClass}>
                Testcontainers
              </NextLink>{" "}
              for hvordan du åpner.
            </li>
            <li>
              <code className={code}>ps</code>, <code className={code}>top</code> og <code className={code}>sudo</code>{" "}
              virker ikke på macOS, se{" "}
              <NextLink href={`${FAQ}#setuid`} className={linkClass}>
                setuid
              </NextLink>
              .
            </li>
            <li>
              Globale installasjoner med npm og pnpm feiler. Kjør dem utenfor cplt, se{" "}
              <NextLink href="/nav-pilot/guider/cplt-node#globale" className={linkClass}>
                Globale installasjoner
              </NextLink>
              .
            </li>
          </Bullets>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="nettverk" size="medium" level="2">
            Nettverk
          </LinkableHeading>
          <Bullets>
            <li>Bare port 443 er åpen ut.</li>
            <li>Proxyen stenger hoster med privat IP-adresse.</li>
            <li>macOS: localhost er stengt. På Linux når agenten en lokal tjeneste på en åpen port.</li>
            <li>
              Proxyen ser bare trafikk som bruker den. Uten tvungen proxy kan et program koble seg direkte til port 443,
              og da gjelder ingen tillatelsesliste.
            </li>
          </Bullets>
          <BodyLong>
            Hvordan du åpner, står i{" "}
            <NextLink href="/nav-pilot/guider/cplt-nettverk" className={linkClass}>
              Nettverk i sandkassen
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="git" size="medium" level="2">
            Git
          </LinkableHeading>
          <Bullets>
            <li>
              Push til default branch, force push og <code className={code}>gh pr merge</code> er stoppet.
            </li>
            <li>
              macOS: <code className={code}>git config</code>, <code className={code}>git remote set-url</code>,{" "}
              <code className={code}>git clone</code>, <code className={code}>git init</code>,{" "}
              <code className={code}>git worktree remove</code> og git-hooks må kjøres utenfor cplt. Nye worktrees inne
              i sesjonen lages med <code className={code}>cplt worktree add &lt;navn&gt; &lt;ref&gt;</code> når{" "}
              <code className={code}>sandbox.allow_git_worktrees</code> er slått på, se{" "}
              <NextLink href="/nav-pilot/guider/worktrees#subagenter" className={linkClass}>
                Worktrees til subagenter
              </NextLink>
              .
            </li>
            <li>SSH-nøkler er stengt. Bruk HTTPS.</li>
            <li>Commitene til agenten er usignerte.</li>
          </Bullets>
          <BodyLong>
            Se{" "}
            <NextLink href="/nav-pilot/guider/cplt-git" className={linkClass}>
              Git og GitHub i sandkassen
            </NextLink>
            . Hele lista, med detaljer per plattform, står i{" "}
            <a href={KNOWN_IMPACTS} className={linkClass}>
              known-impacts.md
            </a>{" "}
            i cplt.
          </BodyLong>
        </VStack>
      </section>
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="nais" size="medium" level="2">
            nais-CLI
          </LinkableHeading>
          <BodyLong>
            <strong>Kort fortalt:</strong> <code className={code}>nais</code> kan virke i cplt, men ikke skru det på.{" "}
            <code className={code}>nais</code> er laget for å skrive ekte hemmeligheter til stdout, og i en agent-sesjon
            er stdout modellens kontekstvindu. Kjør oppsettet før du starter agenten:
          </BodyLong>
          <CodeBlock>{`nais device status          # skal si "Connected"
nais login
./hentEnv.sh                # eller: nais app env <app> -e dev-gcp -t <team> -o json > .env
cplt                        # start agenten etterpå`}</CodeBlock>
          <BodyLong>
            <code className={code}>.env</code> ligger da i prosjektkatalogen, som agenten har full tilgang til. Docker
            Compose og appen leser den som vanlig. Agenten trenger aldri <code className={code}>nais</code>.
          </BodyLong>

          <LinkableHeading id="nais-i-sesjonen" size="small" level="3">
            Hva som skjer i en agent-sesjon
          </LinkableHeading>
          <div className="overflow-x-auto">
            <Table size="small" className="table-stack" role="table">
              <HeaderRow stack cells={["Kommando", "I en agent-sesjon (cplt)"]} />
              <TableBody role="rowgroup">
                {NAIS_ROWS.map(([cmd, result]) => (
                  <TableRow role="row" key={cmd}>
                    <TableDataCell role="cell">
                      <code className={code}>{cmd}</code>
                    </TableDataCell>
                    <TableDataCell role="cell" data-label="I en agent-sesjon (cplt)">
                      {result}
                    </TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <BodyLong>
            Feilmeldingen fra <code className={code}>nais device status</code> villeder: naisdevice kjører. Sandkassen
            nekter bare tilkoblingen til agentens unix-socket. <code className={code}>cplt exec</code> kjører med en
            strammere profil enn en ekte agent-sesjon og har ikke tilgang til nøkkelringen. Ikke bruk{" "}
            <code className={code}>cplt exec</code> til å finne ut hva agenten din kan.
          </BodyLong>

          <LinkableHeading id="nais-hvorfor" size="small" level="3">
            Hvorfor det stopper
          </LinkableHeading>
          <Bullets>
            <li>
              <strong>Påloggingen ligger utenfor det agenten ser.</strong> <code className={code}>nais</code> lagrer
              tokenet kryptert i{" "}
              <code className={code}>~/Library/Application Support/nais/nais-credentials.json.enc</code> (macOS) eller{" "}
              <code className={code}>~/.config/nais/…</code> (Linux). cplt gir agenten bare noen få kataloger i
              hjemmeområdet, og denne er ikke blant dem. Nøkkelen som dekrypterer, ligger i nøkkelringen (
              <code className={code}>cli.nais.io</code> / <code className={code}>nais-user</code>) og er tilgjengelig i
              en agent-sesjon. Det er fila som mangler, ikke nøkkelen.
            </li>
            <li>
              <strong>naisdevice-socketen er stengt.</strong> <code className={code}>nais device …</code> snakker gRPC
              over <code className={code}>~/Library/Application Support/naisdevice/agent.sock</code>. Sandkassen nekter{" "}
              <code className={code}>connect()</code> til sockets den ikke har fått eksplisitt.
            </li>
            <li>
              <code className={code}>~/.config/gcloud</code> er stengt, som <code className={code}>~/.aws</code>,{" "}
              <code className={code}>~/.azure</code>, <code className={code}>~/.kube</code> og{" "}
              <code className={code}>~/.ssh</code>. Alt i <code className={code}>nais</code> som går via{" "}
              <code className={code}>gcloud</code>, er stengt uansett hva du ellers åpner.
            </li>
          </Bullets>

          <LinkableHeading id="nais-hemmeligheter" size="small" level="3">
            Ikke gi agenten tilgang til hemmeligheter
          </LinkableHeading>
          <BodyLong>To tillatelser holder for alt som går mot Nais-APIet:</BodyLong>
          <CodeBlock>{`cplt \\
  --allow-read   "$HOME/Library/Application Support/nais/nais-credentials.json.enc" \\
  --allow-socket "${SOCKET}"`}</CodeBlock>
          <BodyLong>
            <strong>Ikke gjør dette.</strong> Nettopp fordi det virker, er valget ditt og ikke sandkassens.{" "}
            <code className={code}>nais secret get --with-values</code> og <code className={code}>nais app env</code>{" "}
            skriver ekte, produksjonsnære hemmeligheter til stdout. Det havner i transkripsjonen, blir liggende og
            sendes med på hvert påfølgende modellkall. En hemmelighet som har vært innom et kontekstvindu, må regnes som
            lekket. Med de to tillatelsene kan agenten lese alle teamets hemmeligheter. Det er akkurat det stengingen av{" "}
            <code className={code}>~/.config/gcloud</code> og <code className={code}>~/.aws</code> skal hindre.
          </BodyLong>
          <BodyLong>Trenger du bare VPN-status for feilsøking, gi bare socketen:</BodyLong>
          <CodeBlock compact>{`cplt --allow-socket "${SOCKET}"`}</CodeBlock>
          <BodyLong>
            Det gir <code className={code}>nais device status</code> og ingenting annet: ingen innlogging, og{" "}
            <code className={code}>nais app env</code> feiler fortsatt.
          </BodyLong>

          <LinkableHeading id="nais-pilot-port" size="small" level="3">
            Er porten i nais/pilot på?
          </LinkableHeading>
          <BodyLong>
            Agentpakka <code className={code}>nais/pilot</code> har en preToolUse-port som nekter cluster-kommandoer mot
            feil tenant. Porten må se hvilken tenant naisdevice står på. Inne i cplt ser den som regel ingenting, fordi
            sandkassen nekter agent-socketen, og da slipper kommandoene gjennom uten at noe sier fra.
          </BodyLong>
          <BodyLong>
            <code className={code}>nav-pilot doctor</code> viser om naisdevice er installert, om du er tilkoblet,
            hvilken tenant som er aktiv (og hva porten kaller den), og om porten ser tilstanden inne i sandkassen.
            Seksjonen vises bare når du har en agentpakke fra <code className={code}>nais</code>. doctor leser aldri ut
            sesjonsnøkkelen i svaret fra <code className={code}>nais device status --output json</code>.
          </BodyLong>
          <BodyLong>
            Statusfila porten leser i sandkassen, <code className={code}>agent-status.json</code>, kommer med{" "}
            <a href="https://github.com/nais/device/issues/564" className={linkClass}>
              nais/device#564
            </a>{" "}
            og er ikke sluppet ennå. Til da mangler den på alle maskiner, og doctor sier det uten å kalle det en feil.
            Når fila finnes, gir du cplt lesetilgang én gang:
          </BodyLong>
          <CodeBlock
            compact
          >{`cplt config set allow.read "$HOME/Library/Application Support/naisdevice/agent-status.json"`}</CodeBlock>
          <BodyLong>
            Det er lesetilgang til en fil med tilkoblingstilstand og tenantnavn, ikke til socketen og ikke til
            påloggingen.
          </BodyLong>

          <LinkableHeading id="nais-smating" size="small" level="3">
            Småting
          </LinkableHeading>
          <Bullets>
            <li>
              <code className={code}>nais login --nais</code> inne i sandkassen krever også skrivetilgang til katalogen
              med påloggingen og en callback-lytter på <code className={code}>localhost:8865</code>. Logg heller inn
              utenfor. Med bare lesetilgang feiler en tokenfornyelse etter hvert.
            </li>
            <li>
              Sett <code className={code}>DO_NOT_TRACK=1</code> hvis du ikke vil at <code className={code}>nais</code>{" "}
              sender telemetri til <code className={code}>collector-internet.nav.cloud.nais.io</code> ved hver kjøring.
            </li>
            <li>
              Med <code className={code}>--preset strict</code> styres utgående trafikk av en tillatelsesliste, og{" "}
              <code className={code}>auth.nais.io</code> og <code className={code}>console.nav.cloud.nais.io</code> står
              ikke på den.
            </li>
            <li>
              naisdevice-agenten lytter også på en TCP-port på localhost, men porten er tilfeldig ved hver omstart.
              Derfor går det ikke med <code className={code}>--allow-port</code>.
            </li>
          </Bullets>
        </VStack>
      </section>
    </DocPage>
  );
}
