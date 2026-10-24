import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import type { ReactNode } from "react";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "MCP gjennom cplt",
  description:
    "Slå på Nais MCP, GitHub og Figma under cplt. cplt kjører serverne på maskinen din, så innloggingen din aldri kommer inn i sandkassen.",
};

const TOC: TocItem[] = [
  { id: "hva-er-det", label: "Hva det er" },
  { id: "nais", label: "Nais MCP" },
  { id: "github", label: "GitHub i OpenCode" },
  { id: "figma", label: "Figma" },
  { id: "begrensninger", label: "Begrensninger" },
  { id: "plattformer", label: "Plattformer" },
  { id: "feilsoking", label: "Feilsøking" },
];

const PROBLEMS: { message: string; fix: ReactNode }[] = [
  {
    message: "this cplt cannot run MCP servers on the host, so nothing was written",
    fix: (
      <>
        cplt er for gammel. Kjør <code className={code}>brew upgrade cplt</code>, og slå på serveren igjen.
      </>
    ),
  },
  {
    message: "runs only under cplt, which is not installed, so nothing was written",
    fix: (
      <>
        Installer cplt med <code className={code}>brew install navikt/tap/cplt</code>, og slå på serveren igjen.
      </>
    ),
  },
  {
    message: "not logged in on this machine. Fix: nais login -n",
    fix: (
      <>
        Kjør <code className={code}>nais login -n</code> utenfor sandkassen, og start en ny økt.
      </>
    ),
  },
  {
    message: "nais … is older than 7.0.2",
    fix: (
      <>
        Serveren virker, men oppgrader med <code className={code}>brew upgrade nais</code>. Fra 7.0.2 sjekker Nais CLI
        hver GraphQL-operasjon agenten sender.
      </>
    ),
  },
  {
    message: "… is writable in the sandbox",
    fix: (
      <>
        cplt kjører ikke en server når agenten kan endre oppsettet til den. Flytt{" "}
        <code className={code}>XDG_CONFIG_HOME</code> ut av mappa meldingen nevner, eller fjern skrivetilgangen du har
        gitt til mappa.
      </>
    ),
  },
  {
    message: "… is a symlink",
    fix: <>Bytt ut lenken meldingen nevner med en vanlig mappe eller fil, og slå på serveren igjen.</>,
  },
  {
    message: "cplt's MCP broker is off here: the MCP broker needs Bubblewrap on Linux, and Bubblewrap is not usable",
    fix: (
      <>
        Kjør <code className={code}>cplt doctor --fix</code>. Den installerer bubblewrap eller fikser AppArmor, se{" "}
        <a href="#plattformer" className={linkClass}>
          Plattformer
        </a>
        .
      </>
    ),
  },
  {
    message: "cplt's MCP broker is off here: …",
    fix: (
      <>
        cplt kan ikke skjule alle D-Bus-endepunktene for agenten. Kjør <code className={code}>cplt doctor</code> for å
        se hva som stopper det.
      </>
    ),
  },
  {
    message: "--allow-gpg-signing needs sockets in the runtime dir, which the MCP broker hides from the sandbox",
    fix: (
      <>
        Gjelder også <code className={code}>--allow-rootless-docker</code>. Fjern flagget for å bruke serveren, eller
        behold det og start med <code className={code}>--no-mcp-broker io.github.nais/nais-mcp</code>.
      </>
    ),
  },
  {
    message: "cplt mcp status did not answer here",
    fix: (
      <>
        Prøv igjen. Står det <code className={code}>timed out</code>, brukte cplt for lang tid. Ellers kjører du{" "}
        <code className={code}>nav-pilot mcp list</code> fra repoet ditt.
      </>
    ),
  },
  {
    message: "cplt's MCP broker is off",
    fix: (
      <>
        Du har slått det av. Slå det på med <code className={code}>cplt config set sandbox.mcp_broker true</code>.
      </>
    ),
  },
  {
    message: "blocked by policy (i Copilot CLI)",
    fix: (
      <>
        Meldingen kommer fra Copilot CLI når serveren ikke står i Navs MCP-register. For{" "}
        <code className={code}>io.github.nais/nais-mcp</code> skal den ikke komme. Meld fra i{" "}
        <a href="https://github.com/navikt/cplt/issues" className={linkClass}>
          navikt/cplt
        </a>{" "}
        og legg ved utskriften fra <code className={code}>nav-pilot mcp list</code>.
      </>
    ),
  },
];

export default function McpGjennomCplt() {
  return (
    <DocPage
      label="Guider"
      upgrade
      title="MCP gjennom cplt"
      description="Noen MCP-servere trenger innloggingen din. cplt kjører dem på maskinen din, utenfor sandkassen, så agenten får verktøyene uten å se innloggingen."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="hva-er-det" size="medium" level="2">
            Hva det er
          </LinkableHeading>
          <BodyLong>
            Nais MCP leser innloggingen din fra nøkkelringen, og GitHub-serveren trenger{" "}
            <code className={code}>gh</code>-tokenet ditt. Ingen av delene skal inn i sandkassen. Derfor kjører cplt
            disse serverne på maskinen din, utenfor sandkassen, og agenten når bare de lesende verktøyene. Det virker
            likt i Copilot CLI og OpenCode.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="nais" size="medium" level="2">
            Nais MCP
          </LinkableHeading>
          <BodyLong>
            Du trenger cplt fra 23. oktober 2026 eller nyere på macOS og cplt fra 24. oktober 2026 på Linux (se{" "}
            <a href="#plattformer" className={linkClass}>
              Plattformer
            </a>
            ), og Nais CLI 7.0.2 eller nyere. Slå på serveren:
          </BodyLong>
          <CodeBlock compact>{`nav-pilot mcp enable io.github.nais/nais-mcp`}</CodeBlock>
          <BodyLong>
            Er du ikke logget inn, spør nav-pilot om den skal kjøre <code className={code}>nais login -n</code>. Trykker
            du bare Enter, betyr det ja. Har du en eldre Nais CLI, ber nav-pilot deg oppgradere, men slår på serveren
            likevel.
          </BodyLong>
          <BodyLong>Sjekk at det virker:</BodyLong>
          <CodeBlock compact>{`nav-pilot mcp list`}</CodeBlock>
          <BodyLong>
            Kolonnen LOGIN viser <code className={code}>nais: ok</code> når du er logget inn. Andre verdier er{" "}
            <code className={code}>nais: logged out</code>, <code className={code}>nais: not installed</code> og{" "}
            <code className={code}>nais: unknown (timed out)</code>, som betyr at sjekken tok for lang tid, ikke at du
            er logget ut. Står det <code className={code}>cplt broker</code> under PATH, kjører cplt serveren. Står det{" "}
            <code className={code}>cplt (off here)</code>, kan ikke cplt kjøre den på denne maskinen, og linja under
            tabellen sier hvorfor. Kommandoen virker fra hvilken som helst mappe. Start en ny økt etter at du har slått
            serveren på.
          </BodyLong>
          <BodyLong>Slå den av igjen:</BodyLong>
          <CodeBlock compact>{`nav-pilot mcp disable io.github.nais/nais-mcp`}</CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="github" size="medium" level="2">
            GitHub i OpenCode
          </LinkableHeading>
          <BodyLong>
            I OpenCode kobler cplt GitHub-serveren gjennom sin egen proxy, med <code className={code}>gh</code>
            -innloggingen din. Du trenger ikke <code className={code}>mcp auth</code>. Sjekk at du er logget inn med{" "}
            <code className={code}>gh auth status</code>, og start en ny økt. Proxyen slipper bare gjennom lesende
            verktøy. Detaljene står i{" "}
            <NextLink href="/nav-pilot/guider/feilsoking#mcp-opencode" className={linkClass}>
              Feilsøking
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="figma" size="medium" level="2">
            Figma
          </LinkableHeading>
          <BodyLong>Figma går ikke gjennom cplt. Du logger inn i klienten. I OpenCode gjør du det én gang:</BodyLong>
          <CodeBlock compact>{`opencode mcp auth com.figma/figma-mcp`}</CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="begrensninger" size="medium" level="2">
            Begrensninger
          </LinkableHeading>
          <Bullets>
            <li>
              Bare lesende verktøy er slått på som standard. Vil du ha verktøy som skriver, slår du dem på med{" "}
              <code className={code}>--tools</code> eller <code className={code}>--all-tools</code>, og nav-pilot ber
              deg godkjenne det:
              <CodeBlock
                compact
              >{`nav-pilot mcp enable io.github.nais/nais-mcp --tools <verktøy som skriver>`}</CodeBlock>
            </li>
            <li>
              GraphQL-verktøyet i Nais MCP sender bare spørringer. cplt avviser mutasjoner og dokumenter med mer enn én
              operasjon.
            </li>
            <li>Et kall som ikke har fått svar etter 120 sekunder, avbryter cplt.</li>
            <li>
              I OpenCode feiler <code className={code}>schema_list_queries</code>. Følges opp i{" "}
              <a href="https://github.com/nais/cli/issues/817" className={linkClass}>
                nais/cli#817
              </a>
              .
            </li>
            <li>
              Kjører du Nais MCP i flere økter samtidig, kan flere prosesser fornye en utløpt Nais-innlogging på en gang
              og i sjeldne tilfeller logge deg ut. Kjør da <code className={code}>nais login -n</code> på nytt. Risikoen
              er teoretisk: vi fant den ved å lese koden til Nais CLI og har ikke sett den skje. Den følges opp i{" "}
              <a href="https://github.com/nais/cli/issues/816" className={linkClass}>
                nais/cli#816
              </a>
              .
            </li>
            <li>
              Ikke alle plattformer er støttet, se{" "}
              <a href="#plattformer" className={linkClass}>
                Plattformer
              </a>
              .
            </li>
          </Bullets>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="plattformer" size="medium" level="2">
            Plattformer
          </LinkableHeading>
          <BodyLong>
            På Linux og WSL2 kjører cplt serveren bare når bubblewrap (<code className={code}>bwrap</code>) pakker inn
            økten, fordi det er bubblewrap som skjuler nøkkelringen for agenten. Uten bubblewrap kjører ikke cplt
            serveren. Det krever cplt fra 24. oktober 2026 eller nyere. Mangler bubblewrap, installerer{" "}
            <code className={code}>cplt doctor --fix</code> det med <code className={code}>apt-get</code> (Debian og
            Ubuntu) eller <code className={code}>dnf</code> (Fedora). Starter du cplt med{" "}
            <code className={code}>--allow-gpg-signing</code> eller{" "}
            <code className={code}>--allow-rootless-docker</code>, er serveren av, fordi begge trenger sockets som cplt
            skjuler.
          </BodyLong>
          <div className="overflow-x-auto">
            <Table size="small">
              <TableHeader>
                <TableRow>
                  <TableHeaderCell scope="col">Plattform</TableHeaderCell>
                  <TableHeaderCell scope="col">Status</TableHeaderCell>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow>
                  <TableDataCell>macOS</TableDataCell>
                  <TableDataCell>Virker.</TableDataCell>
                </TableRow>
                <TableRow>
                  <TableDataCell>Ubuntu 22.04, Debian 12, Fedora</TableDataCell>
                  <TableDataCell>Virker med bubblewrap.</TableDataCell>
                </TableRow>
                <TableRow>
                  <TableDataCell>Ubuntu 24.04 og nyere</TableDataCell>
                  <TableDataCell>
                    AppArmor stopper bubblewrap som standard. Kjør <code className={code}>cplt doctor --fix</code> én
                    gang.
                  </TableDataCell>
                </TableRow>
                <TableRow>
                  <TableDataCell>Linux med bare Landlock, uten bubblewrap</TableDataCell>
                  <TableDataCell>Ikke støttet ennå.</TableDataCell>
                </TableRow>
                <TableRow>
                  <TableDataCell>WSL2</TableDataCell>
                  <TableDataCell>
                    Skal virke med bubblewrap, men er ikke testet på en ekte maskin ennå. Nais-innloggingen trenger en
                    nøkkelring, se under (
                    <a href="https://github.com/nais/cli/issues/815" className={linkClass}>
                      nais/cli#815
                    </a>
                    ).
                  </TableDataCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>
          <LinkableHeading id="ubuntu-apparmor" size="small" level="3">
            Ubuntu 24.04 og nyere
          </LinkableHeading>
          <BodyLong>
            Fiksen gir <code className={code}>bwrap</code>, og bare den, lov til å lage brukernavnerom. cplt kan gjøre
            det selv:
          </BodyLong>
          <CodeBlock compact>{`cplt doctor --fix`}</CodeBlock>
          <BodyLong>
            cplt viser kommandoene og innholdet i fila og spør{" "}
            <code className={code}>Fix now? sudo will ask for your password. [Y/n]</code>. Svarer du ja, kjører den
            kommandoene med <code className={code}>sudo</code> og sjekker på nytt. Den spør bare i en terminal. Med{" "}
            <code className={code}>cplt --yes doctor --fix</code>, uten terminal, med <code className={code}>CI</code>{" "}
            satt, inne i sandkassen eller med <code className={code}>--print-profile</code> skriver den kommandoene og{" "}
            <code className={code}>Not offering to run it (…)</code> med grunnen, og du kjører dem selv. Det samme
            tilbudet får du når du starter cplt og bubblewrap ikke virker.
          </BodyLong>
          <BodyLong>
            Vil du gjøre det for hånd og <code className={code}>/etc/apparmor.d/bwrap-userns-restrict</code> finnes,
            laster du profilen Ubuntu har lagt med:
          </BodyLong>
          <CodeBlock compact>{`sudo apparmor_parser -r /etc/apparmor.d/bwrap-userns-restrict`}</CodeBlock>
          <BodyLong>Finnes den ikke, skriver du en egen profil og laster den:</BodyLong>
          <CodeBlock compact>
            {`sudo tee /etc/apparmor.d/bwrap >/dev/null <<'EOF'
abi <abi/4.0>,
include <tunables/global>

profile bwrap /usr/bin/bwrap flags=(unconfined) {
  userns,
  include if exists <local/bwrap>
}
EOF
sudo apparmor_parser -r /etc/apparmor.d/bwrap`}
          </CodeBlock>
          <LinkableHeading id="wsl2-nokkelring" size="small" level="3">
            Nais-innlogging i WSL2
          </LinkableHeading>
          <BodyLong>
            Nais CLI lagrer innloggingen gjennom Secret Service. WSL2 uten skrivebord har ingen slik tjeneste, så
            installer gnome-keyring og start den før du kjører <code className={code}>nais login -n</code>:
          </BodyLong>
          <CodeBlock compact>
            {`sudo apt install gnome-keyring dbus-x11
eval "$(dbus-launch --sh-syntax)"
echo -n "" | gnome-keyring-daemon --unlock --components=secrets`}
          </CodeBlock>
          <BodyLong>
            <a href="https://github.com/nais/cli/issues/815" className={linkClass}>
              nais/cli#815
            </a>{" "}
            ber om at Nais CLI kan lagre innloggingen i en fil i stedet.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="feilsoking" size="medium" level="2">
            Feilsøking
          </LinkableHeading>
          <BodyLong>
            Meldingene kommer fra <code className={code}>nav-pilot mcp enable</code>,{" "}
            <code className={code}>nav-pilot mcp list</code> eller cplt ved oppstart.
          </BodyLong>
          <div className="overflow-x-auto">
            <Table size="small">
              <TableHeader>
                <TableRow>
                  <TableHeaderCell scope="col">Melding</TableHeaderCell>
                  <TableHeaderCell scope="col">Gjør dette</TableHeaderCell>
                </TableRow>
              </TableHeader>
              <TableBody>
                {PROBLEMS.map((p) => (
                  <TableRow key={p.message}>
                    <TableDataCell>
                      <code className={code}>{p.message}</code>
                    </TableDataCell>
                    <TableDataCell>{p.fix}</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </VStack>
      </section>
    </DocPage>
  );
}
