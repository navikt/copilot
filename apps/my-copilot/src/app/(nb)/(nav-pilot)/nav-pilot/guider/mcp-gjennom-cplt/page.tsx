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
    message: "broker spec rejected: … is writable in the sandbox",
    fix: (
      <>
        cplt kjører ikke en server når agenten kan endre oppsettet til den. Flytt{" "}
        <code className={code}>XDG_CONFIG_HOME</code> ut av mappa meldingen nevner, eller fjern skrivetilgangen du har
        gitt til mappa.
      </>
    ),
  },
  {
    message: "broker spec rejected: … is a symlink",
    fix: <>Bytt ut lenken meldingen nevner med en vanlig mappe eller fil, og slå på serveren igjen.</>,
  },
  {
    message: "the MCP broker runs on macOS only for now",
    fix: <>Du bruker Linux med en cplt uten Linux-støtte. Oppgrader cplt når Linux-støtten er ute.</>,
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
            Du trenger cplt-utgaven fra 23. oktober 2026 eller nyere (på Linux en nyere, se{" "}
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
            Kolonnen LOGIN viser om du er logget inn. Står det <code className={code}>cplt broker</code> under PATH,
            kjører cplt serveren. Start en ny økt etter at du har slått den på.
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
            <li>
              Kjører du Nais MCP i flere økter samtidig, kan flere prosesser fornye en utløpt Nais-innlogging på en gang
              og av og til logge deg ut. Kjør da <code className={code}>nais login -n</code> på nytt. Feilen følges opp
              i{" "}
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
          {/* TODO ved merge: bytt «<dato>» med datoen til cplt-utgaven med navikt/cplt#838 og #841. */}
          <BodyLong>
            På Linux og WSL2 kjører cplt serveren bare når bubblewrap (<code className={code}>bwrap</code>) pakker inn
            økten, fordi det er bubblewrap som skjuler nøkkelringen for agenten. Uten bubblewrap nekter cplt. Det krever
            cplt-utgaven fra &lt;dato&gt; eller nyere. Mangler bubblewrap, installerer{" "}
            <code className={code}>cplt doctor --fix</code> det med <code className={code}>apt-get</code> eller{" "}
            <code className={code}>dnf</code>.
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
                    AppArmor stopper bubblewrap som standard. cplt nekter å starte og viser kommandoen som fikser det.
                    Du kjører den én gang.
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
                    nøkkelring, se under.
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
            cplt viser kommandoene og innholdet i fila, spør <code className={code}>Fix now? [Y/n]</code>, kjører dem
            med <code className={code}>sudo</code> og sjekker på nytt. Den spør bare i en terminal. Med{" "}
            <code className={code}>--yes</code>, uten terminal, med <code className={code}>CI</code> satt eller inne i
            sandkassen skriver den bare ut kommandoene. Det samme tilbudet får du når cplt nekter å starte serveren.
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
