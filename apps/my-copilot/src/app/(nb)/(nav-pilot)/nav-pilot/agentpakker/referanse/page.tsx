import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Table, TableBody, TableRow, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { DocPage, HeaderRow, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Agentpakker: felt og regler",
  description:
    "Referanse for deg som lager en agentpakke: artefakttyper, klientoppføring, tier, MCP-servere, sandkasseforslag, gjenbruk, releases og pensjonering.",
};

const TOC: TocItem[] = [
  {
    id: "artefakttyper",
    label: "Artefakttyper",
    children: [
      { id: "kjorbar-kode", label: "Kjørbar kode" },
      { id: "skript-i-en-skill", label: "Skript i en skill" },
    ],
  },
  { id: "klientoppforinga", label: "Klientoppføringa", children: [{ id: "hvilken-tier", label: "Hvilken tier?" }] },
  { id: "uten-agent", label: "Pakke uten agent" },
  { id: "mcp-servere", label: "MCP-servere" },
  { id: "sandkasse", label: "Sandkassekonfigurasjon" },
  { id: "validering", label: "Validering" },
  {
    id: "gjenbruk",
    label: "Gjenbruk av en annen pakke",
    children: [
      { id: "kollisjoner", label: "Kollisjoner" },
      { id: "hva-som-komponerer", label: "Hva som komponerer" },
      { id: "hold-basen-oppdatert", label: "Hold basen oppdatert" },
    ],
  },
  {
    id: "releases",
    label: "Releases og pensjonering",
    children: [
      { id: "nar-endringen-nar-fram", label: "Når endringen når fram" },
      { id: "stabile-releases", label: "Stabile releases" },
      { id: "pensjonering", label: "Pensjonering" },
    ],
  },
];

const ARTIFACT_TYPES = [
  {
    type: "agents",
    form: "<navn>.agent.md",
    what: "Personaer klienten kan startes som, eller underagenter andre kaller",
  },
  { type: "skills", form: "<navn>/SKILL.md", what: "Kunnskap modellen laster ved behov" },
  { type: "instructions", form: "<navn>.instructions.md", what: "Regler som aktiveres mot matchende filer" },
  { type: "prompts", form: "<navn>.prompt.md eller <navn>/", what: "Ferdige spørsmål brukeren kan kjøre" },
  {
    type: "hooks",
    form: "<navn>.py, valgfritt <navn>.hook.json",
    what: "Skript som kjører ved verktøykall. Uten sidecaren: ingen matcher, og standard tidsgrense",
  },
  { type: "extensions", form: "<navn>/extension.mjs", what: "Kode klienten laster inn" },
];

const KLIENTER = `{
  "minNavPilotVersion": "2026.08.17-062831",
  "clients": {
    "copilot": {
      "primaryAgents": ["grillmester"],
      "compatibility": ">=1.0.79,<2",
      "defaultModel": "inherit"
    },
    "opencode": {
      "primaryAgents": ["grillmester"],
      "compatibility": ">=1.18.20,<2",
      "defaultModel": "inherit"
    },
    "pi": {
      "primaryAgents": ["grillmester"]
    }
  }
}`;

const MANIFEST_UTEN_AGENT = `{
  "contractVersion": "1",
  "name": "ditt-team",
  "description": "Skillene vi deler",
  "layout": {
    "skills": "skills"
  },
  "clients": {
    "copilot": {}
  }
}`;

const MCP = `{
  "mcpServers": ["io.github.navikt/github-mcp", "io.github.navikt/aksel-mcp"]
}`;

const PROPOSE = `{
  "policies": {
    "propose": {
      "cplt": {
        "reason": "The nais-observability skill queries Mimir, Loki and Tempo. These hosts resolve to private IP addresses over naisdevice and are blocked without this waiver. The preToolUse gate reads naisdevice's agent-status.json to tell whether you are connected.",
        "proxy": {
          "allow_private_domains": [
            "mimir.nav.cloud.nais.io",
            "loki.nav.cloud.nais.io",
            "tempo.dev-gcp.nav.cloud.nais.io",
            "tempo.prod-gcp.nav.cloud.nais.io"
          ]
        },
        "allow": {
          "read": [
            "~/Library/Application Support/naisdevice/agent-status.json",
            "~/.config/naisdevice/agent-status.json"
          ]
        }
      }
    }
  },
  "minNavPilotVersion": "2026.09.14-131410"
}`;

const AVSLAG = `cplt config set proxy.allow_private_domains mimir.nav.cloud.nais.io
cplt config set proxy.allow_private_domains loki.nav.cloud.nais.io
cplt config set proxy.allow_private_domains tempo.dev-gcp.nav.cloud.nais.io
cplt config set proxy.allow_private_domains tempo.prod-gcp.nav.cloud.nais.io
cplt config set allow.read "~/Library/Application Support/naisdevice/agent-status.json"`;

export default function AgentpakkerReferanse() {
  return (
    <DocPage
      label="Referanse"
      title="Agentpakker: felt og regler"
      description="Detaljene du trenger når du lager og vedlikeholder en agentpakke."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="artefakttyper" size="medium" level="2">
            Artefakttyper
          </LinkableHeading>
          <div className="overflow-x-auto">
            <Table size="small" className="table-stack" role="table">
              <HeaderRow stack cells={["Type", "Form", "Hva det er"]} />
              <TableBody role="rowgroup">
                {ARTIFACT_TYPES.map((t) => (
                  <TableRow role="row" key={t.type}>
                    <TableDataCell role="cell">
                      <code className={code}>{t.type}</code>
                    </TableDataCell>
                    <TableDataCell role="cell">
                      <code className={code}>{t.form}</code>
                    </TableDataCell>
                    <TableDataCell role="cell">{t.what}</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <VStack gap="space-8">
            <LinkableHeading id="kjorbar-kode" size="small" level="3">
              Kjørbar kode
            </LinkableHeading>
            <BodyLong textColor="subtle">
              Hooks og extensions er ikke tekst en modell leser. En hook kjører ved verktøykall, en extension lastes av
              klienten. Den som installerer pakka di kjører koden din på maskinen sin, så si i pakkas{" "}
              <code className={code}>description</code> hva den gjør. Installasjonen må skje utenfor cplt: inne i
              sandkassen nekter cplt å skrive hooks, extensions og skills, og <code className={code}>install</code>{" "}
              stopper med en feil som sier det.
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <LinkableHeading id="skript-i-en-skill" size="small" level="3">
              Skript i en skill
            </LinkableHeading>
            <BodyLong textColor="subtle">
              Sender skillen din med et skript, kopieres det med resten av katalogen, men katalogen havner ulike steder
              per klient: <code className={code}>~/.copilot/skills/</code> for copilot,{" "}
              <code className={code}>skills/</code> under konfigurasjonskatalogen for opencode,{" "}
              <code className={code}>~/.nav-pilot/pi/skills</code> for pi, og <code className={code}>skills/</code> i
              payloadtreet for Tier 2. Skriver du én av stiene i teksten, er skillen feil på de andre. nav-pilot
              eksporterer derfor <code className={code}>NAV_PILOT_SKILLS_DIR</code> ved hver launch, med roten skillene
              faktisk ble lagt i for den klienten, og sender den gjennom sandkassa. Skriv{" "}
              <code className={code}>bash &quot;$NAV_PILOT_SKILLS_DIR/&lt;skill&gt;/&lt;skript&gt;&quot;</code> og den
              peker riktig overalt. La nav-pilot ingen skills ut for klienten, er variabelen usatt framfor å peke på en
              katalog som ikke finnes, så en skill kan teste på den og si fra.
            </BodyLong>
          </VStack>
        </VStack>
      </section>
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="klientoppforinga" size="medium" level="2">
            Klientoppføringa
          </LinkableHeading>
          <BodyLong textColor="subtle">
            Klientnøklene i dag er <code className={code}>copilot</code>, <code className={code}>opencode</code> og{" "}
            <code className={code}>pi</code>. En nav-pilot som ikke kjenner en nøkkel, hopper over den i stedet for å
            avvise manifestet. En ny klient senere ugyldiggjør derfor ingen pakke som alt er ute.
          </BodyLong>
          <CodeBlock filename=".nav-pilot/agentpakke.json">{KLIENTER}</CodeBlock>
          <BodyLong textColor="subtle">
            Tier utledes av formen og deklareres ikke. En klientoppføring uten <code className={code}>payloads</code> er
            Tier 1: nav-pilot legger inn filene selv fra stiene i <code className={code}>layout</code>, som da må
            finnes. En oppføring med <code className={code}>payloads</code> er Tier 2: nav-pilot verifiserer og stager
            ferdigbygde trær mot en digest, og pinner dem per bruker. En pakke kan blande de to per klient. Hvordan du
            bygger payload-trær for Tier 2, står i{" "}
            <a href="https://github.com/navikt/copilot/blob/main/docs/README.agentpakke.md" className={linkClass}>
              feltreferansen
            </a>
            .
          </BodyLong>
          <BodyLong textColor="subtle">
            <code className={code}>compatibility</code> er et versjonsområde for klienten, ikke en versjon:
            kommaseparerte komparatorer over semver, som <code className={code}>&quot;&gt;=1.18.20,&lt;2&quot;</code>.{" "}
            <code className={code}>&quot;1.18.20&quot;</code> alene har ingen operator og avvises. Området håndheves før
            hver launch i begge tier: nav-pilot spør klienten om versjonen og nekter en versjon utenfor. Svarer ikke
            klienten, eller er svaret uleselig, er det også fatalt. Et område nav-pilot ikke kan håndheve, er ikke
            håndhevet.
          </BodyLong>
          <BodyLong textColor="subtle">
            <code className={code}>owner</code> er attribusjon, ikke tilgangsstyring: kilden til en installasjon er
            repoet manifestet ble klonet fra. <code className={code}>policies.opencodePermissions</code>,{" "}
            <code className={code}>profiles</code> og <code className={code}>provenance</code> står i skjemaet, men gjør
            ingenting ennå: stiene sti-sjekkes, og nav-pilot leser dem ikke. Vent med dem.
          </BodyLong>
          <LinkableHeading id="hvilken-tier" size="small" level="3">
            Hvilken tier skal du velge?
          </LinkableHeading>
          <BodyLong textColor="subtle">
            Velg Tier 1 om du ikke har en grunn til noe annet. Innholdet er filer, de havner i repoet eller profilen og
            er synlige i en diff, konsumenter kan plukke enkeltdeler med <code className={code}>items</code>, og du
            vedlikeholder ingen byggekjede: nav-pilot legger inn filene fra <code className={code}>layout</code>.
          </BodyLong>
          <BodyLong textColor="subtle">
            Velg Tier 2 når pakka di er et ferdig bygget oppsett som skal leveres som én enhet, og ikke plukkes fra. Da
            får du digestverifisering, én revisjon per bruker framfor filer i repoet, og de tre mekanismene under{" "}
            <a href="#stabile-releases" className={linkClass}>
              Stabile releases
            </a>{" "}
            som i dag bare virker der. Prisen er at du bygger payload-trærne selv og holder dem i takt med kontrakten.
            nav-pilot har ingen kommando som bygger dem (
            <a href="https://github.com/navikt/copilot/issues/840" className={linkClass}>
              #840
            </a>
            ).
          </BodyLong>
          <BodyLong textColor="subtle">
            <code className={code}>defaultModel</code> er per klient. Den literale verdien{" "}
            <code className={code}>&quot;inherit&quot;</code> sender ingen <code className={code}>--model</code>. En
            konkret modell-id sendes med. En modell brukeren har pinnet selv, vinner over begge.{" "}
            <code className={code}>minNavPilotVersion</code> ligger på pakkenivå, skrives på nav-pilots releaseformat (
            <code className={code}>YYYY.MM.DD-HHMMSS</code>, eventuelt med build-sha) og blokkerer eldre binærer med en
            melding som sier hva de skal gjøre. Et annet format avvises framfor å ignoreres: nav-pilot kan ikke
            sammenligne det, og å godta det ville slått av akkurat den gaten manifestet ba om. Et utviklingsbygg (
            <code className={code}>dev</code>) er unntatt gaten, så lokalt arbeid på pakka stopper ikke.
          </BodyLong>
          <BodyLong textColor="subtle">
            <strong>Kjørbar kode når ikke alle klientene.</strong> opencode og pi hopper over hooks, med en advarsel på
            stderr som navngir dem. Det er koblinga som mangler, ikke evnen. Extensions håndteres ikke der i det hele
            tatt. Har pakka di en hook eller en extension noen er avhengig av, er copilot den eneste klienten som får
            den.
          </BodyLong>
        </VStack>
      </section>
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="uten-agent" size="medium" level="2">
            Pakke uten agent
          </LinkableHeading>
          <BodyLong textColor="subtle">
            Deler dere bare skills eller instruksjoner, utelater dere <code className={code}>primaryAgents</code> og{" "}
            <code className={code}>agents</code> i <code className={code}>layout</code>. Dere trenger ikke finne på en
            persona. Pakka validerer, installeres og synkes som vanlig, men den kan ikke starte klienten: det finnes
            ingen agent å gi den, og launch stopper med pakkas navn i meldinga. Start klienten selv, eller pek nav-pilot
            på en pakke som deklarerer en agent. To pakker i samme scope er ingen utvei: et scope installeres fra én
            kilde, og nav-pilot nekter å blande innhold fra to agentpakker i én installasjon. Vil du ha den andre pakka
            i stedet, bytter du kilde for scopet.
          </BodyLong>
          <CodeBlock filename=".nav-pilot/agentpakke.json">{MANIFEST_UTEN_AGENT}</CodeBlock>
        </VStack>
      </section>
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="mcp-servere" size="medium" level="2">
            MCP-servere
          </LinkableHeading>
          <BodyLong textColor="subtle">
            Valgfritt. MCP-servere styres sentralt i{" "}
            <a href="https://mcp-registry.nav.no" className={linkClass}>
              Navs MCP-register
            </a>
            . Du kan ikke definere din egen, men du kan si hvilke av registerets servere agentene og skillene dine
            forventer. nav-pilot spør registeret når den validerer og installerer: et navn det ikke publiserer, er et
            funn. Svarer ikke registeret, blir det en advarsel i stedet, så en CI-jobb uten nett ikke feiler på noe den
            ikke kan sjekke.
          </BodyLong>
          <CodeBlock filename=".nav-pilot/agentpakke.json">{MCP}</CodeBlock>
          <BodyLong textColor="subtle">
            Navnene skrives på registerets egen omvendt-DNS-form,{" "}
            <code className={code}>&lt;namespace&gt;/&lt;navn&gt;</code>, som{" "}
            <code className={code}>io.github.navikt/github-mcp</code>. Et navn uten den formen avvises av schemaet før
            registeret spørres i det hele tatt.
          </BodyLong>
          <BodyLong textColor="subtle">
            <code className={code}>install</code> navngir serverne pakka trenger, peker på registeret og viser
            kommandoen som slår dem på: <code className={code}>nav-pilot mcp enable &lt;navn&gt; …</code>. Selve
            installasjonen skriver ingen MCP-konfigurasjon. Det gjør du med den kommandoen. Feltet ligger på pakkenivå,
            siden det er klientens eget oppsett som avgjør om en server er tilgjengelig.
          </BodyLong>
        </VStack>
      </section>
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="sandkasse" size="medium" level="2">
            Sandkassekonfigurasjon
          </LinkableHeading>
          <BodyLong textColor="subtle">
            Trenger en skill noe av sandkassa cplt setter rundt klienten, sier pakka det i{" "}
            <code className={code}>policies.propose</code>, i stedet for å la brukeren møte feilen midt i arbeidet.
            Nais-pakka spør Mimir, Loki og Tempo under <code className={code}>nav.cloud.nais.io</code>. Navnene slår opp
            til private adresser over naisdevice, og cplt avviser dem (
            <code className={code}>403 Private target blocked by cplt</code>) til brukeren har gitt et unntak. Pakka
            navngir hver host. Et suffiks som <code className={code}>cloud.nais.io</code> ville gitt unntak for alle
            hoster under alle organisasjoner på Nais, også dem skillen aldri spør. Et forslag konfigurerer ingenting av
            seg selv: det blir et launch-flagg først når brukeren har sagt ja.
          </BodyLong>
          <CodeBlock filename=".nav-pilot/agentpakke.json">{PROPOSE}</CodeBlock>
          <BodyLong textColor="subtle">
            I v1 får pakka foreslå to ting: <code className={code}>proxy.allow_private_domains</code>, 1 til 32 fulle
            DNS-navn uten wildcard, port eller sti, og <code className={code}>allow.read</code>, opptil 8 navngitte
            filer. <code className={code}>reason</code> er påkrevd, høyst 400 tegn, uten linjeskift og kontrolltegn, og
            er alt brukeren har å avgjøre på: skriv hva som ryker uten unntaket. nav-pilot vasker teksten igjen når den
            skrives ut, så en pakke kan ikke lage en linje som ser ut som nav-pilots egen. Skjemaet avviser{" "}
            <code className={code}>allow.write</code>, <code className={code}>allow.exec</code>,{" "}
            <code className={code}>allow.socket</code>, <code className={code}>deny</code>,{" "}
            <code className={code}>preset</code>, <code className={code}>repo_dirs</code>,{" "}
            <code className={code}>inherit_env</code>, <code className={code}>allowed_domains</code>,{" "}
            <code className={code}>blocked_domains</code>, <code className={code}>proxy.forced</code>, guardene og hele{" "}
            <code className={code}>sandbox</code>. Under <code className={code}>proxy</code> validerer ingenting annet
            enn <code className={code}>allow_private_domains</code>. Andre nøkler på toppnivå i{" "}
            <code className={code}>cplt</code>-blokka validerer, men nav-pilot navngir dem ved install og honorerer dem
            aldri. Andre verktøynøkler enn <code className={code}>cplt</code> ignoreres.
          </BodyLong>
          <BodyLong textColor="subtle">
            <strong>En lesetilgang navngir én fil, aldri en katalog.</strong> cplt gir én regel per sti,{" "}
            <code className={code}>(allow file-read* (subpath &quot;&lt;sti&gt;&quot;))</code> på macOS og{" "}
            <code className={code}>AccessFs::ReadFile | ReadDir</code> på Linux. Det er smalere enn domeneunntaket:
            ingen skriving, ingen port, ingen utgående trafikk. Men <code className={code}>subpath</code> på en katalog
            er alt under den, og <code className={code}>~/Library/Application Support/naisdevice</code> er ett tegn unna
            å dele ut <code className={code}>private.key</code>. Derfor krever skjemaet en sti som begynner med{" "}
            <code className={code}>~/</code> og ender i et navn med punktum og filendelse, avviser{" "}
            <code className={code}>nav-pilot validate</code> både <code className={code}>..</code> og alt som ligger på
            eller under cplts <code className={code}>DENIED_DOTFILES</code>, <code className={code}>DENIED_FILES</code>{" "}
            og <code className={code}>DENIED_HOME_SUBPATHS</code>, og stat-er launchen stien og slipper en katalog. Fila
            finnes ofte ikke ennå når manifestet valideres, så det siste laget er det eneste som kan se hva stien
            faktisk er.
          </BodyLong>
          <BodyLong textColor="subtle">
            Stien skrives <code className={code}>~/</code>-relativt, den formen cplt selv forstår. nav-pilot utvider{" "}
            <code className={code}>~</code> ved launch og sender den absolutte stien, mens samtykkeposten tar vare på{" "}
            <code className={code}>~/</code>
            -formen brukeren så. naisdevice legger tilstanden under{" "}
            <code className={code}>~/Library/Application Support/naisdevice/</code> på macOS og{" "}
            <code className={code}>~/.config/naisdevice/</code> på Linux, så pakka navngir begge og launchen sender bare
            den som finnes. <code className={code}>$XDG_CONFIG_HOME</code> utvides ikke: manifestet har én utvidelse.
          </BodyLong>
          <BodyLong textColor="subtle">
            Brukeren svarer i terminalen ved <code className={code}>install</code>, og ved{" "}
            <code className={code}>sync --apply</code> når blokka er endret. Svaret lagres per scope i{" "}
            <code className={code}>~/.nav-pilot/pakke-consent.json</code>, nøklet på en hash av hele blokka: endrer du{" "}
            <code className={code}>reason</code> eller legger til en host, kommer spørsmålet tilbake med det som endret
            seg. Blokka er ett spørsmål: et domene og en lesetilgang i samme blokk vises sammen og besvares én gang. Et
            nei installerer pakka likevel. Brukeren får vite hva som ryker, og kommandoene som åpner det for hånd:
          </BodyLong>
          <CodeBlock compact>{AVSLAG}</CodeBlock>
          <BodyLong textColor="subtle">
            Uten terminal, og med <code className={code}>--json</code>, godkjennes ingenting og noteres ingenting. Er
            cplt ikke installert ennå, spør ikke nav-pilot. Spørsmålet kommer ved neste{" "}
            <code className={code}>install</code> eller <code className={code}>sync --apply</code> når cplt er på plass.
            Etter et nei viser <code className={code}>nav-pilot doctor</code> avslaget som informasjon, med kommandoene
            over, ikke som en advarsel. <code className={code}>nav-pilot uninstall</code> sletter svaret. Et ja blir{" "}
            <code className={code}>--allow-private-domain &lt;host&gt;</code> og{" "}
            <code className={code}>--allow-read &lt;absolutt sti&gt;</code> på cplt-kommandolinja, for launcher fra
            scopet som svarte, og skrives ut ved hver launch. nav-pilot rører ikke cplt-konfigurasjonen. Unntaket løfter
            bare DNS-rebinding-vernet for de navnene: tillatelses- og blokklista gjelder fortsatt, ingen port åpnes,
            ingenting kjøres.
          </BodyLong>
          <BodyLong textColor="subtle">
            Sett <code className={code}>minNavPilotVersion</code> til en nav-pilot-release fra 14. september 2026 eller
            nyere, og til releasen som innførte <code className={code}>allow.read</code> om du bruker den. En eldre
            nav-pilot ignorerer blokka som et ukjent felt, og brukeren får feilen uten forklaring. En nav-pilot fra før{" "}
            <code className={code}>allow.read</code> gjør noe strengere: skjemaet følger binæren, så manifestet avvises
            i sin helhet. Brukeren trenger dessuten cplt fra 14. september 2026 eller nyere: under det lagres svaret,
            men unntaket anvendes ikke, og launchen sier hvorfor.
          </BodyLong>
        </VStack>
      </section>
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="validering" size="medium" level="2">
            Validering
          </LinkableHeading>
          <BodyLong textColor="subtle">
            Advarsler (<code className={code}>⚠</code>) feiler ikke kommandoen. En{" "}
            <code className={code}>defaultModel</code> nav-pilot ikke kjenner igjen er en av dem: modellkatalogen synkes
            fra models.dev og henger etter en fersk modell, så et avvist manifest ville tatt oftere feil enn advarselen
            gjør.
          </BodyLong>
          <BodyLong textColor="subtle">
            Kilden må være <code className={code}>owner/repo</code> eller en absolutt sti.{" "}
            <code className={code}>--source .</code> avvises av verdisjekken før noe forsøkes hentet, så bruk{" "}
            <code className={code}>&quot;$PWD&quot;</code>, eller{" "}
            <code className={code}>&quot;$GITHUB_WORKSPACE&quot;</code> i CI. Etiketten i utdataene er kilden du oppga,
            ikke navnet i manifestet. Skjemaet ligger i{" "}
            <code className={code}>cli/nav-pilot/schemas/agentpakke-v1.json</code>, så CI kan linte manifestet mot det
            uten nav-pilot.
          </BodyLong>
          <BodyLong textColor="subtle">
            Utelater du <code className={code}>--source</code>, velger nav-pilot kilde i denne rekkefølgen:{" "}
            <code className={code}>--source</code>, så <code className={code}>source</code>-nøkkelen i konfigurasjonen
            din, så <code className={code}>navikt/copilot</code>. Lokal autogjenkjenning gjelder bare en
            navikt/copilot-checkout, ikke en vanlig agentpakke. Det er derfor en <code className={code}>validate</code>{" "}
            uten <code className={code}>--source</code> i pakkerepoet ditt validerer Nav-pakka og melder alt grønt: den
            så aldri på din.
          </BodyLong>
        </VStack>
      </section>
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="gjenbruk" size="medium" level="2">
            Gjenbruk av en annen pakke
          </LinkableHeading>
          <BodyLong textColor="subtle">
            Oppsettet står under{" "}
            <NextLink href="/nav-pilot/agentpakker#bygg-videre-pa-en" className={linkClass}>
              Bygg videre på en
            </NextLink>
            . Her står reglene for hvordan de to pakkene settes sammen.
          </BodyLong>
          <BodyLong textColor="subtle">
            Gjenbruker to pakker hverandre, finnes det ingen rekkefølge å løse dem i, og feilen ber om at syklusen
            brytes i én av dem.
          </BodyLong>
          <VStack gap="space-8">
            <LinkableHeading id="kollisjoner" size="small" level="3">
              Kollisjoner
            </LinkableHeading>
            <BodyLong textColor="subtle">
              Har begge pakkene en agent med samme navn, installeres din. Det er samme regel som{" "}
              <code className={code}>overrides</code> i <code className={code}>.github/copilot-sync.json</code>: det
              teamet eier selv, eier de. Å skygge et artefakt er den vanlige måten å endre én ting i en pakke du ellers
              tar som den er. Det er ikke en feil, og nav-pilot varsler ikke.
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <LinkableHeading id="hva-som-komponerer" size="small" level="3">
              Hva som komponerer
            </LinkableHeading>
            <BodyLong textColor="subtle">
              Alle installasjonsveiene tar med det gjenbrukte innholdet:{" "}
              <code className={code}>install &lt;navn&gt;</code>, <code className={code}>install --all</code>, den
              interaktive plukkeren, <code className={code}>install &lt;navn&gt; --type &lt;type&gt;</code> og{" "}
              <code className={code}>sync</code>, og <code className={code}>list</code> viser det. En konsument kan
              navngi et arvet artefakt i <code className={code}>items</code>. <code className={code}>validate</code>{" "}
              komponerer med vilje ikke: den sjekker hva repoet ditt selv sender, så en base kan ikke gjøre en pakke
              gyldig som ikke er det.
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <LinkableHeading id="hold-basen-oppdatert" size="small" level="3">
              Hold basen oppdatert
            </LinkableHeading>
            <BodyLong textColor="subtle">
              Pinnen flytter seg ikke av seg selv, og modellene i agentenes frontmatter blir stående sammen med den.{" "}
              <code className={code}>sync</code> og <code className={code}>doctor</code> sier fra med én linje når basen
              har flyttet seg, men flytter ikke pinnen. Du flytter den med{" "}
              <code className={code}>nav-pilot pakke bump-base</code> i pakkerepoet ditt. Kommandoen skriver ut hvilke
              agenter og modeller som er endret. Workflowen <code className={code}>agentpakke-base-bump.yaml</code> i
              navikt/copilot gjør det samme på en tidsplan og åpner en pull request du ser over og merger. Oppsettet
              står i{" "}
              <a
                href="https://github.com/navikt/copilot/blob/main/docs/README.agentpakke.md#en-pakke-som-gjenbruker-en-annen"
                className={linkClass}
              >
                agentpakke-guiden
              </a>
              .
            </BodyLong>
          </VStack>
        </VStack>
      </section>
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="releases" size="medium" level="2">
            Releases og pensjonering
          </LinkableHeading>
          <VStack gap="space-8">
            <LinkableHeading id="nar-endringen-nar-fram" size="small" level="3">
              Når endringen når fram
            </LinkableHeading>
            <BodyLong textColor="subtle">
              Konsumentene er pinnet til revisjonen de installerte. En endring du pusher, når dem først når{" "}
              <code className={code}>nav-pilot sync --apply</code> flytter pinnen hos dem, som én linje diff i en pull
              request de leser og godkjenner. En rettelse er derfor ikke ute samme dag. Å endre{" "}
              <code className={code}>name</code> i manifestet gjør eksisterende installasjoner til en annen pakke, så
              det er ikke en omdøping du gjør i forbifarten.
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <LinkableHeading id="stabile-releases" size="small" level="3">
              Stabile releases
            </LinkableHeading>
            <BodyLong textColor="subtle">
              Uten releases henter <code className={code}>sync</code> det standardgrenen holder, så alt du pusher går
              rett ut til konsumentene. Publiserer du i stedet en GitHub Release med assetet{" "}
              <code className={code}>agentpakke-release.json</code>, leser <code className={code}>install</code> og{" "}
              <code className={code}>sync</code> nyeste stabile release, og du kan jobbe videre på main. Releasen må
              være publisert, ikke prerelease, og immutable, og taggen må binde versjonen. Et repo uten slike releases
              fungerer nøyaktig som før, fra standardgrenen.
            </BodyLong>
            <BodyLong textColor="subtle">
              <strong>Er pakka di Tier 1</strong>, altså en layout av filer, pinner den ingen revisjon: den installerer
              filer, og abonnementet avgjør bare hvilken revisjon filene leses fra.{" "}
              <code className={code}>install</code> og <code className={code}>sync</code> uten{" "}
              <code className={code}>--ref</code> leser nyeste stabile release i stedet for standardgrenen, i både
              bruker- og repo-scope, og <code className={code}>sync --apply</code> flytter{" "}
              <code className={code}>sha</code> i erklæringa til release-SHA-en. Det finnes ikke noe nedgraderingsvern
              her: hvert oppslag tar nyeste stabile release uten å sammenligne med det som ligger på disk, så en
              installasjon som står foran releasen, flyttes tilbake til den ved neste{" "}
              <code className={code}>sync --apply</code>. Diffen vises før <code className={code}>--apply</code>, som
              for enhver annen fil.
            </BodyLong>
            <BodyLong textColor="subtle">
              Tre mekanismer hører sammen med releases, og de virker i dag bare for Tier 2, fordi alle tre er gatet på
              at scopet pinner en revisjon: <code className={code}>nav-pilot rollback</code> tilbake til forrige
              revisjon på maskinen, oppstartsspørsmålet om å ta en ny release, og det varige valget{" "}
              <code className={code}>sync --updates auto|ask|keep</code>. En Tier 1-installasjon fører opp filene den la
              ned, og faller derfor utenfor gaten: rollback nekter med «your user scope pins none», og de to andre nås
              aldri. Lov derfor ikke konsumentene dine et rollback en Tier 1-pakke ikke gir dem. Gaten blir ikke
              utvidet. Vil en Tier 1-konsument tilbake til en eldre revisjon, pinner de den selv med{" "}
              <code className={code}>nav-pilot sync --apply --ref &lt;sha&gt;</code>.
            </BodyLong>
            <BodyLong textColor="subtle">
              <strong>Er pakka di Tier 2</strong>, gjelder alle tre. Konsumenten kan rulle tilbake uten nett og uten å
              vente på deg, den forlatte revisjonen tilbys ikke igjen mens neste release gjør det (lever derfor
              rettelsen som en ny versjon; en revert av taggen når dem ikke), og et team som har valgt{" "}
              <code className={code}>keep</code>, blir stående til de selv tar releasen. Regn ikke med at alle er på
              nyeste versjon dagen etter. Konsumenter som alt står på standardgrenen ligger som regel foran din første
              release, og nedgraderingsvernet tilbyr den ikke til dem; nav-pilot spør dem én gang ved oppstart om å
              pinne releasen og følge releases videre, og navngir begge revisjonene.
            </BodyLong>
            <BodyLong textColor="subtle">
              Feltene og hele kontrakten står i{" "}
              <NextLink
                href="https://github.com/navikt/copilot/blob/main/docs/README.agentpakke.md#stabile-releases"
                className={linkClass}
              >
                README.agentpakke.md
              </NextLink>
              .
            </BodyLong>
          </VStack>

          <VStack gap="space-8">
            <LinkableHeading id="pensjonering" size="small" level="3">
              Pensjonering
            </LinkableHeading>
            <BodyLong textColor="subtle">
              Slett aldri et artefakt uten å føre det opp i{" "}
              <code className={code}>.nav-pilot/retired-artifacts.json</code>. En kilde hentes med{" "}
              <code className={code}>--depth 1</code>, så brukeren har ingen historikk å slå opp i, og en fil som bare
              forsvinner blir liggende hos alle som installerte den. Generer lista og commit den:{" "}
              <code className={code}>scripts/generate-retired</code> i navikt/copilot er en Go-modul på rundt 200 linjer
              som leser hashene ut av git-loggen, og <code className={code}>mise run retired:check</code> verifiserer i
              CI at lista stemmer.
            </BodyLong>
            <BodyLong textColor="subtle">
              Fila navngir hver pensjonerte sti sammen med innholdshashene pakka en gang publiserte, og det er hashene
              som gjør den trygg: nav-pilot sletter en installert fil bare når bytene matcher en revisjon kilden faktisk
              har publisert. En utvikler som har skrevet sin egen agent på den stien, beholder den. At kilden ikke
              lenger har noe med dette navnet, er ikke tillatelse til å slette.
            </BodyLong>
          </VStack>
        </VStack>
      </section>
    </DocPage>
  );
}
