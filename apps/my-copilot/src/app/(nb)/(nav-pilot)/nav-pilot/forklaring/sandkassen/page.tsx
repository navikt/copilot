import { BodyLong, Box, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Table, TableBody, TableRow, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, HeaderRow, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";
import { ScrollRegion } from "@/components/scroll-region";

export const metadata: Metadata = {
  title: "Sandkassen",
  description:
    "Hvorfor en agent må kjøre isolert på Nav-utstyr, hva cplt låser, og forskjellen på sikkerhetsnivåene standard og strict.",
};

const TOC: TocItem[] = [
  { id: "isolasjon-er-pakrevd", label: "Isolasjon er påkrevd" },
  { id: "ide-klienter", label: "VS Code og IntelliJ" },
  { id: "prosjektkatalogen", label: "Prosjektkatalogen" },
  { id: "sikkerhetsniva", label: "Sikkerhetsnivå i cplt" },
  { id: "autonomi", label: "Kommandoer og git" },
  { id: "strict", label: "Strict i detalj" },
  { id: "nar-strict-ikke-anbefales", label: "Strict på Linux" },
  { id: "copilot-sandkasse-linux", label: "Copilots egen sandkasse" },
];

const CPLT_REV = "https://github.com/navikt/cplt/blob/22656614c9224d82d2baade3b599f561239052f9";
const GH_SANDBOX = "https://docs.github.com/en/copilot/how-tos/cloud-and-local-sandboxes/using-local-sandboxing";

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
              <Bullets>
                <li>
                  Terminal: start agenter med <code className={code}>nav-pilot</code> (cplt).
                </li>
                <li>VS Code og IntelliJ: greit. Slå på editorens sandkasse hvis den har en.</li>
                <li>Aldri bruk Allow all, Autopilot eller global auto-godkjenning.</li>
              </Bullets>
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
          <LinkableHeading id="ide-klienter" size="medium" level="2">
            Slik gjør du det i VS Code og IntelliJ
          </LinkableHeading>
          <BodyLong>
            <strong>VS Code</strong>
          </BodyLong>
          <Bullets>
            <li>
              Sett <code className={code}>chat.agent.sandbox.enabled</code> til <code className={code}>"on"</code>{" "}
              (standard er <code className={code}>off</code>). Virker på macOS 15 eller nyere og Linux med glibc og
              bubblewrap 0.5.0+. På Windows er sandkassen eksperimentell: slå den på der den virker.
            </li>
            <li>
              Hold <code className={code}>chat.tools.global.autoApprove</code> av, og legg ikke inn brede regler med{" "}
              <code className={code}>true</code> i <code className={code}>chat.tools.terminal.autoApprove</code>.
            </li>
            <li>Ikke velg Allow all eller Autopilot.</li>
            <li>
              Sandkassen dekker terminalkommandoer, MCP-servere og språkservere. Den dekker ikke filredigering, og
              utgående nettverk er åpent som standard. Kommandoer i sandkassen kan kjøre uten å spørre.
            </li>
          </Bullets>
          <BodyLong>
            <strong>IntelliJ og andre JetBrains-IDE-er</strong>
          </BodyLong>
          <Bullets>
            <li>
              Slå på lokal sandkasse i versjoner som har den.{" "}
              <a
                href="https://github.blog/changelog/2026-10-28-github-copilot-in-jetbrains-universe-releases/"
                className={linkClass}
              >
                Allment tilgjengelig fra 28.10.2026
              </a>
              .
            </li>
            <li>
              Hold <strong>Settings &gt; GitHub Copilot &gt; Chat &gt; Auto Approve &gt; Global Auto Approve</strong>{" "}
              av, og auto-godkjenning av kommandoer og filendringer som ingen regel dekker.
            </li>
          </Bullets>
          <BodyLong>
            Kilder:{" "}
            <a href="https://code.visualstudio.com/docs/agents/run/agent-sandboxing" className={linkClass}>
              Agent sandboxing
            </a>
            ,{" "}
            <a href="https://code.visualstudio.com/docs/agents/run/approvals" className={linkClass}>
              Approvals
            </a>{" "}
            (VS Code),{" "}
            <a
              href="https://github.blog/changelog/2026-04-24-inline-agent-mode-in-preview-and-more-in-github-copilot-for-jetbrains-ides/"
              className={linkClass}
            >
              auto-godkjenning i JetBrains
            </a>{" "}
            (GitHub, 24.04.2026).
          </BodyLong>
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
              (anbefalt, og det du har hvis du ikke har valgt noe): agenten kan committe, pushe egne brancher og åpne
              pull requests. cplt stopper <code className={code}>gh pr merge</code>, push til default branch og force
              push.
            </li>
            <li>
              <strong>
                <code className={code}>strict</code>
              </strong>
              : som standard, men agenten når bare hoster på en liste, og all push er stoppet. Velg det bare hvis du vil
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
            <code className={code}>gh pr merge</code>, push til default branch og force push, uansett hva du svarer.
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
              brancher og åpne pull requests. «Commit only» stopper all push. Med{" "}
              <code className={code}>permissive</code> eller <code className={code}>full-trust</code> kommer ikke
              spørsmålet, fordi git- og gh-vaktene er av.
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
            <code className={code}>proxy.default_allowlist</code>. Da når agenten bare hostene på lista til cplt og det{" "}
            <code className={code}>proxy.allowed_domains</code> peker på. Hoster som ikke står der, blir blokkert, også
            interne hoster du tar i bruk senere. Verken <code className={code}>nav-pilot doctor</code> eller
            innstillingssiden anbefaler strict.
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
            nav-pilot skriver lista over hoster til <code className={code}>~/.nav-pilot/cplt-allowed-domains.txt</code>,
            peker <code className={code}>proxy.allowed_domains</code> dit, og setter nivået til slutt, så låsen aldri
            blir aktiv uten hostene. Har du en egen <code className={code}>proxy.allowed_domains</code>, lar nav-pilot
            den være og sier at du må legge til hostene selv. Nøkler du har satt selv, gjelder foran nivået.
          </BodyLong>
          <BodyLong>
            Fila har hele lista: Nav-hostene, cplts egen liste og pakkeregistrene. Agenten når derfor pakkeregistrene
            selv om <code className={code}>proxy.default_allowlist</code> er av. nav-pilot henter lista med{" "}
            <code className={code}>cplt config hosts</code>. Er cplt for gammel til det, skriver nav-pilot en frosset
            liste med GitHub, Copilot og pakkeregistrene som fantes da, uten dem cplt har lagt til siden. Lista til cplt
            er per agent: lista for copilot har GitHub og Copilot, den for opencode har{" "}
            <code className={code}>opencode.ai</code> og <code className={code}>models.dev</code>.
          </BodyLong>
          <BodyLong>
            <strong>Push.</strong> Strict stopper all push, så agenten kan ikke pushe en branch eller åpne en pull
            request. Velger du strict med <code className={code}>nav-pilot config setup --advanced</code>, kan du i
            neste spørsmål la agenten pushe brancher likevel. Da setter nav-pilot{" "}
            <code className={code}>git_guard.protect_default_branch_only = true</code>, og cplt stopper bare push til
            default branch og force push. Har du satt den nøkkelen fra før, gjelder den foran nivået, også når du slår
            på strict fra <code className={code}>nav-pilot config</code>. Vil du stoppe all push, velg «Commit only» i
            git-spørsmålet.
          </BodyLong>
          <BodyLong>
            <strong>Lokal modell.</strong> Den lokale modellen går gjennom en loop guard (stopper modellen når den
            gjentar seg) på <code className={code}>127.0.0.1</code>. cplt blokkerer localhost som standard, så nav-pilot
            sender porten med som <code className={code}>--allow-localhost &lt;port&gt;</code> ved hver oppstart. Én
            port slipper gjennom tvungen proxy på både macOS og Linux, så strict og lokal modell går sammen.
          </BodyLong>
          <BodyLong>
            <strong>Tilbake til standard.</strong> Bytter du fra strict til <code className={code}>standard</code>, spør
            nav-pilot (fra 29. september 2026):{" "}
            <strong>Remove the network allowlist nav-pilot set up for strict?</strong> Spørsmålet kommer bare for lista
            nav-pilot la inn selv. Svarer du nei, eller har du en egen liste, blir lista stående, og agenten når
            fortsatt bare hostene på den. Kommandoen under fjerner lista som gjelder, uansett hvem som la den inn:
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
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="copilot-sandkasse-linux" size="medium" level="2">
            Copilots egen sandkasse inni cplt på Linux
          </LinkableHeading>
          <BodyLong>
            <strong>Kort fortalt:</strong> start Copilot med <code className={code}>cplt --agent copilot</code> fra
            prosjektet, eller med <code className={code}>nav-pilot --client copilot</code>. Ikke slå på Copilots{" "}
            <code className={code}>/sandbox</code> inni cplt. cplt isolerer allerede hele agentprosessen og barna dens.
            nav-pilot trengs ikke for å starte cplt.
          </BodyLong>
          <CodeBlock compact>{`cplt --agent copilot
# Alternativt, med nav-pilot:
nav-pilot --client copilot`}</CodeBlock>

          <LinkableHeading id="bwrap-installert" size="small" level="3">
            Installert bwrap betyr ikke at en ny sandkasse kan starte
          </LinkableHeading>
          <BodyLong>
            <a href={`${GH_SANDBOX}#prerequisites`} className={linkClass}>
              GitHubs Linux-krav
            </a>{" "}
            er <code className={code}>bwrap</code> 0.5.0 eller nyere og <code className={code}>slirp4netns</code> på
            PATH. Det er verktøy som må finnes, ikke en garanti for at prosessen får opprette namespaces.
          </BodyLong>
          <BodyLong>
            cplt bruker Landlock og seccomp-BPF på Linux, med bubblewrap som et ekstra lag når det brukes.{" "}
            <a href={`${CPLT_REV}/src/sandbox_landlock.rs#L2299-L2371`} className={linkClass}>
              seccomp-filteret
            </a>{" "}
            avviser blant annet <code className={code}>unshare</code>, <code className={code}>setns</code> og{" "}
            <code className={code}>mount</code> med <code className={code}>EPERM</code>. Filteret gjelder også uten det
            ytre bubblewrap-laget. Med bubblewrap legger cplt{" "}
            <a href={`${CPLT_REV}/src/sandbox_bubblewrap.rs#L1714-L1720`} className={linkClass}>
              filteret på inne i de ferdige namespacene
            </a>
            , før agenten starter.
          </BodyLong>
          <BodyLong>
            Derfor kan bwrap være installert og virke i en vanlig terminal, men feile inni cplt med{" "}
            <code className={code}>No permissions to create a new namespace</code>. En melding fra Copilot om at
            bubblewrap mangler, beviser ikke alene at pakken mangler eller er for gammel. Ikke installer på nytt eller
            endre kjerneinnstillinger ut fra den meldingen. Ikke slå av seccomp eller namespace-begrensninger for å få
            den indre sandkassen til å starte.
          </BodyLong>

          <LinkableHeading id="cplt-ved-oppstart" size="small" level="3">
            Hva cplt gjør ved oppstart
          </LinkableHeading>
          <BodyLong>
            cplt støtter ikke Copilots indre sandkasse. For Copilot setter{" "}
            <a href={`${CPLT_REV}/src/sandbox_exec.rs#L276-L283`} className={linkClass}>
              oppstartskoden
            </a>{" "}
            <code className={code}>COPILOT_CLI_SANDBOX_SUPPORT_OVERRIDE=unsupported</code> i miljøet til barnet. Da
            hopper Copilot over støtteproben og lar sin egen sandkasse være av for sesjonen. cplt endrer ikke den
            lagrede innstillingen <code className={code}>sandbox.enabled</code>.
          </BodyLong>
          <BodyLong>
            Copilot kan da si at kommandoer kjører «unsandboxed». Det gjelder Copilots eget lag, ikke cplt rundt
            prosessen. cplt er fortsatt sikkerhetsgrensen. Du får ikke Copilots ekstra, smalere isolasjon av
            enkeltkommandoer inni den.
          </BodyLong>
          <BodyLong>
            Mekanismen ble{" "}
            <a href="https://github.com/navikt/cplt/pull/520" className={linkClass}>
              verifisert med Copilot CLI 1.0.83 i cplt PR #520
            </a>
            , merget 15. september 2026. Det er en historisk verifikasjon, ikke en test av alle nyere klientversjoner.
            Dagens kode har fortsatt mekanismen. En eksplisitt{" "}
            <code className={code}>--pass-env COPILOT_CLI_SANDBOX_SUPPORT_OVERRIDE</code> hindrer cplt i å sette
            verdien, men gir ikke prosessen lov til å opprette namespaces. Ikke bruk det mot denne feilen.
          </BodyLong>
          <BodyLong>
            <a href={`${GH_SANDBOX}#using-local-sandboxing-in-copilot-cli`} className={linkClass}>
              GitHub dokumenterer
            </a>{" "}
            at <code className={code}>/sandbox</code> åpner innstillingene, og at{" "}
            <code className={code}>/sandbox enable</code> slår på sandkassen også for senere sesjoner. En administrert
            policy kan kreve Copilots lokale sandkasse. Ikke omgå et slikt krav med flagg, miljøvariabler eller
            konfigurasjon. Krever policyen den indre sandkassen, må den som eier policyen og GitHub avklare
            kompatibiliteten før du kan starte på denne måten.
          </BodyLong>

          <LinkableHeading id="copilot-sandkasse-verifisert" size="small" level="3">
            Hva som er verifisert, og hva som er åpent
          </LinkableHeading>
          <BodyLong>
            Kildegjennomgangen 13. november 2026 brukte cplt-revisjon{" "}
            <code className={code}>22656614c9224d82d2baade3b599f561239052f9</code>.
          </BodyLong>
          <ScrollRegion label="Tabell: hva som er verifisert i Copilot-sandkassen">
            <Table size="small" className="table-stack" role="table">
              <HeaderRow stack cells={["Evidens", "Hva den viser"]} />
              <TableBody role="rowgroup">
                <TableRow role="row">
                  <TableDataCell role="cell">seccomp-filteret og oppstartskoden over</TableDataCell>
                  <TableDataCell role="cell" data-label="Hva den viser">
                    cplt avviser operasjonene som trengs, og unngår Copilots indre sandkasse ved vanlig oppstart.
                  </TableDataCell>
                </TableRow>
                <TableRow role="row">
                  <TableDataCell role="cell">
                    <a href={`${CPLT_REV}/tests/integration_linux.rs#L3582-L3601`} className={linkClass}>
                      Linux-testen <code className={code}>bwrap_seccomp_still_blocks_unshare</code>
                    </a>
                  </TableDataCell>
                  <TableDataCell role="cell" data-label="Hva den viser">
                    Testen krever at <code className={code}>unshare</code> fortsatt avvises inni det ytre
                    bubblewrap-laget.
                  </TableDataCell>
                </TableRow>
                <TableRow role="row">
                  <TableDataCell role="cell">
                    <a href={`${CPLT_REV}/tests/unit_tests.rs#L7547-L7577`} className={linkClass}>
                      Unit-test
                    </a>{" "}
                    og{" "}
                    <a href={`${CPLT_REV}/tests/e2e.rs#L1305-L1331`} className={linkClass}>
                      E2E-tester
                    </a>
                  </TableDataCell>
                  <TableDataCell role="cell" data-label="Hva den viser">
                    Overstyringen gjelder bare Copilot. E2E-testene sjekker verdien i miljøet til barnet og eksplisitt
                    pass-through.
                  </TableDataCell>
                </TableRow>
                <TableRow role="row">
                  <TableDataCell role="cell">
                    <a href="https://github.com/navikt/copilot/issues/2176" className={linkClass}>
                      Rapporten i #2176
                    </a>
                  </TableDataCell>
                  <TableDataCell role="cell" data-label="Hva den viser">
                    Brukeren rapporterer bwrap 0.11.1 og avslag på namespaces bare inni cplt. Det stemmer med policyen,
                    men forløpet er ikke gjenskapt.
                  </TableDataCell>
                </TableRow>
              </TableBody>
            </Table>
          </ScrollRegion>
          <BodyLong>
            Linux-testene er lest, ikke kjørt. Verifikasjonen skjedde på macOS uten et isolert Linux-testmiljø. Det er
            ikke kjørt bwrap på maskinen, brukt privilegerte containere eller endret kjerneinnstillinger.
          </BodyLong>
          <BodyLong>
            Gir vanlig oppstart med dagens cplt fortsatt bubblewrap-meldingen, trenger vi Linux-distribusjon og
            kjerneversjon, <code className={code}>cplt --version</code>, <code className={code}>copilot --version</code>
            , startkommandoen uten tokens og noen sladdede linjer rundt feilen. Si også om du bruker{" "}
            <code className={code}>--pass-env COPILOT_CLI_SANDBOX_SUPPORT_OVERRIDE</code>, og om klienten viser et krav
            fra en administrert policy. Det skiller gammel oppstartskode, eksplisitt pass-through og endringer i klient
            eller policy, uten at du må sende hele doctor-utskriften.
          </BodyLong>
          <BodyLong>
            Andre tilgangsfeil må undersøkes ut fra operasjonen som feiler. Dette avsnittet sier ikke at de er løst.
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
