import { BodyLong, BodyShort, Box, VStack, HGrid, Tag } from "@navikt/ds-react";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { PageHero } from "@/components/page-hero";
import { TableOfContents, type TocItem } from "@/components/table-of-contents";
import { BackToTop } from "@/components/back-to-top";
import type { Metadata } from "next";
import NextLink from "next/link";

export const metadata: Metadata = {
  title: "Agentpakker",
  description:
    "Fire steg i rekkefølge: bruk en pakke som finnes, ta delene du trenger, bygg videre på en, og lag din egen først når ingenting av det holder.",
};

const DOC_SECTIONS: TocItem[] = [
  { id: "hva-det-er", label: "Hva det er" },
  {
    id: "bruk-en-som-finnes",
    label: "1. Bruk en som finnes",
    children: [
      { id: "pakkene-som-finnes", label: "Pakkene som finnes" },
      { id: "las-installasjonen", label: "Lås installasjonen" },
    ],
  },
  { id: "ta-delene-du-trenger", label: "2. Ta delene du trenger" },
  {
    id: "bygg-videre-pa-en",
    label: "3. Bygg videre på en",
    children: [
      { id: "kollisjoner", label: "Kollisjoner" },
      { id: "hva-som-komponerer", label: "Hva som komponerer" },
    ],
  },
  {
    id: "lag-din-egen",
    label: "4. Lag din egen",
    children: [
      { id: "struktur", label: "Struktur" },
      { id: "artefakttyper", label: "Artefakttyper" },
      { id: "kjorbar-kode", label: "Kjørbar kode" },
      { id: "manifestet", label: "Manifestet" },
      { id: "klientoppforinga", label: "Klientoppføringa" },
      { id: "uten-agent", label: "Pakke uten agent" },
      { id: "mcp-servere", label: "MCP-servere" },
      { id: "valider", label: "Valider" },
      { id: "distribuer", label: "Distribuer" },
    ],
  },
  {
    id: "vedlikehold",
    label: "Vedlikehold",
    children: [
      { id: "nar-endringen-nar-fram", label: "Når endringen når fram" },
      { id: "stabile-releases", label: "Stabile releases" },
      { id: "pensjonering", label: "Pensjonering" },
    ],
  },
  { id: "ressurser", label: "Ressurser" },
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

const PAKKER = [
  {
    repo: "navikt/copilot",
    pakke: "nav-pilot",
    tier: "Tier 1",
    what: "Nav-innholdet: agentene, ferdighetene, instruksjonene, promptene, hooks og extensions, for copilot, opencode og pi. Tier 1 vil si at nav-pilot legger filene i repoet ditt, så du ser dem i diffen.",
    install: "nav-pilot install nav-pilot --source navikt/copilot --repo",
  },
  {
    repo: "navikt/grillmester",
    pakke: "grillmester",
    tier: "Tier 2",
    what: "Team eSyfo sitt agentlag for kodeendringer som skal avklares og avgrenses før de skrives, og gjennomgås uavhengig etterpå. Agentene heter grillmester, barista, designer og doctor-who. En fokusert kontekst gir bare barista og grill-inspektor. For copilot og opencode. Tier 2 vil si at repoet bygger ferdige payload-trær selv, og at nav-pilot verifiserer dem mot en digest og pinner dem per bruker som én revisjon, ikke som filer i repoet ditt.",
    install: "nav-pilot install grillmester --source navikt/grillmester --user",
  },
  {
    repo: "nais/pilot",
    pakke: "nais-platform",
    tier: "Tier 1",
    what: "For dem som bygger Nais-plattformen: Nais API, tenants og miljøclustere, Fasit, Terraform og Loki/Mimir/Tempo. Agentene nais-platform og nais-review, ferdigheter og instruksjoner for alle tre klientene, og en preToolUse-hook som nekter en cluster- eller LGTM-kommando maskinen ikke kan betjene.",
    install: "nav-pilot install nais-platform --source nais/pilot --repo",
  },
];

const FROZEN = `- run: nav-pilot install nais-platform --frozen --force`;

const ITEMS = `{
  "contractVersion": "1",
  "source": "navikt/copilot",
  "sha": "b73a69e0000000000000000000000000000000aa",
  "items": {
    "nav-pilot": "agent",
    "klarsprak": "skill"
  }
}`;

const STRUKTUR = `ditt-repo/
├── .nav-pilot/
│   └── agentpakke.json
├── agents/
│   └── grillmester.agent.md
└── skills/`;

const MANIFEST = `{
  "contractVersion": "1",
  "name": "ditt-team",
  "description": "Hva pakka er til for",
  "layout": {
    "agents": "agents",
    "skills": "skills"
  },
  "clients": {
    "copilot": {
      "primaryAgents": ["grillmester"]
    }
  }
}`;

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
      "defaultModel": "github-copilot/auto"
    },
    "pi": {
      "primaryAgents": ["grillmester"]
    }
  }
}`;

const MANIFEST_UTEN_AGENT = `{
  "contractVersion": "1",
  "name": "ditt-team",
  "description": "Ferdighetene vi deler",
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

const VALIDER_CMD = `nav-pilot validate --source "$PWD"`;

const VALIDER_UT = `Validating: /Users/deg/ditt-repo@c3f7ca3

  ℹ manifest: .nav-pilot/agentpakke.json
  ℹ agentpakke: ditt-team (contract version 1)
  ℹ clients: copilot (tier 1)

✓ /Users/deg/ditt-repo conforms to the agentpakke contract.`;

const INSTALL_CMD = `nav-pilot install ditt-team --source navikt/ditt-repo --repo`;

const GJENBRUK = `{
  "contractVersion": "1",
  "source": "navikt/copilot",
  "sha": "b73a69e0000000000000000000000000000000aa"
}`;

const KOMPONERT_UT = `Source: navikt/ditt-repo@c3f7ca3
Reuses: navikt/copilot@b73a69e`;

const linkClass = "text-blue-600 hover:underline";

export default function Agentpakker() {
  return (
    <main>
      <PageHero
        title="Agentpakker"
        description="Ta i bruk det som finnes, før du lager noe eget."
        badge={
          <Tag variant="info" size="small" className="uppercase tracking-wide">
            Gjenbruk først
          </Tag>
        }
      />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <div className="flex gap-12">
            <aside className="hidden lg:block w-56 shrink-0">
              <div className="sticky top-6">
                <TableOfContents items={DOC_SECTIONS} />
              </div>
            </aside>

            <div className="min-w-0 flex-1">
              <VStack gap={{ xs: "space-32", md: "space-40" }}>
                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="hva-det-er" size="medium" level="2">
                      Hva det er
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      En agentpakke er et git-repo med AI-artefakter og en fil som beskriver dem. Ingenting skal
                      registreres eller godkjennes. Et repo med et gyldig manifest er en agentpakke, og den som vil ha
                      den, peker på repoet med <code className="font-mono text-xs">--source</code>.
                    </BodyLong>
                    <BodyLong textColor="subtle">
                      Agentpakker er måten et team påvirker verktøykassa uten å bygge den selv. Sida går gjennom fire
                      steg i rekkefølge: bruk en pakke som finnes, ta delene du trenger av den, bygg videre på den, og
                      lag din egen først når ingenting av det holder. Skal du bare bruke Nav-innholdet, trenger du ikke
                      denne sida:{" "}
                      <NextLink href="/nav-pilot/docs" className={linkClass}>
                        nav-pilot-dokumentasjonen
                      </NextLink>{" "}
                      dekker installasjon og bruk.
                    </BodyLong>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="bruk-en-som-finnes" size="medium" level="2">
                      1. Bruk en som finnes
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Det billigste steget er å installere en pakke noen alt vedlikeholder. nav-pilot finner ikke pakker
                      for deg: <code className="font-mono text-xs">install</code> krever at du kjenner reponavnet, og
                      det finnes ingen kommando som lister pakker (
                      <a href="https://github.com/navikt/copilot/issues/819" className={linkClass}>
                        #819
                      </a>
                      ). Lista under er ført for hånd.
                    </BodyLong>

                    <LinkableHeading id="pakkene-som-finnes" size="small" level="3">
                      Pakkene som finnes
                    </LinkableHeading>
                    <VStack gap="space-16">
                      {PAKKER.map((p) => (
                        <Box key={p.repo} background="neutral-soft" padding="space-16" borderRadius="8">
                          <VStack gap="space-8">
                            <BodyShort weight="semibold">
                              <a href={`https://github.com/${p.repo}`} className={linkClass}>
                                {p.repo}
                              </a>
                              <span className="font-normal">
                                , pakka <code className="font-mono text-xs">{p.pakke}</code>, {p.tier}
                              </span>
                            </BodyShort>
                            <BodyLong size="small" textColor="subtle">
                              {p.what}
                            </BodyLong>
                            <CodeBlock compact>{p.install}</CodeBlock>
                          </VStack>
                        </Box>
                      ))}
                    </VStack>
                    <BodyLong textColor="subtle">
                      De tre er de pakkene CI-en i navikt/copilot validerer ved hver kontraktsendring (
                      <a href="https://github.com/navikt/copilot/issues/842" className={linkClass}>
                        #842
                      </a>
                      ): en pull request som rører schemaet,{" "}
                      <code className="font-mono text-xs">internal/agentpakke</code> eller{" "}
                      <code className="font-mono text-xs">validate</code>, bygger nav-pilot fra branchen og kjører{" "}
                      <code className="font-mono text-xs">validate</code> mot hver av dem. Slutter en pakke å følge
                      kontrakten, feiler bygget vårt. Derfor står det ikke flere her: hver pakke på lista koster noe å
                      holde.
                    </BodyLong>

                    <LinkableHeading id="las-installasjonen" size="small" level="3">
                      Lås installasjonen
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Installerer du i et repo, skriver <code className="font-mono text-xs">install</code>{" "}
                      <code className="font-mono text-xs">.nav-pilot/agentpakke.lock.json</code> med kilden og
                      revisjonen. Bruker-scope har ingen erklæring, for{" "}
                      <code className="font-mono text-xs">~/.copilot</code> er ikke et repo: der ligger revisjonen i din
                      egen tilstandsfil, og gjelder bare deg. Commit erklæringa: da installerer hele teamet fra samme
                      revisjon, og <code className="font-mono text-xs">nav-pilot sync --apply</code> flytter pinnen som
                      én linje diff i en pull request.
                    </BodyLong>
                    <BodyLong textColor="subtle">
                      I CI bruker du <code className="font-mono text-xs">--frozen</code>: den installerer nøyaktig det
                      erklæringa sier, eller lar være. Den spør aldri og flytter aldri pinnen. Exit{" "}
                      <code className="font-mono text-xs">0</code>: alt gikk inn, og det som ligger der er den pinnede
                      revisjonen. Exit <code className="font-mono text-xs">1</code>: installasjonen feilet. Exit{" "}
                      <code className="font-mono text-xs">3</code>: installasjonen gikk, men pinnen i erklæringa ble
                      ikke fulgt: ingen erklæring, ingen brukbar pinne, en annen revisjon enn den erklærte, en Tier
                      2-pakke, eller en delvis install. En CI-jobb kan dermed skille «pinnen er ikke det repoet sier»
                      fra «installasjonen røk».
                    </BodyLong>
                    <CodeBlock>{FROZEN}</CodeBlock>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="ta-delene-du-trenger" size="medium" level="2">
                      2. Ta delene du trenger
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Vil du ha fire av tolv agenter fra en plattformpakke, skal du slippe å forke den.{" "}
                      <code className="font-mono text-xs">items</code> i erklæringa parer navn med artefakttype. Uten
                      feltet installeres alt pakka har. Feltet skriver du selv: nav-pilot fyller det aldri ut. Hadde den
                      ført opp alle tolv, ville hver ny agent oppstrøms blitt en merge-konflikt hos hver konsument.
                    </BodyLong>
                    <CodeBlock filename=".nav-pilot/agentpakke.lock.json">{ITEMS}</CodeBlock>
                    <BodyLong textColor="subtle">
                      Navnene sjekkes mot det pakka har. En skrivefeil stopper hele installasjonen. Du får ikke tre av
                      fire uten beskjed.
                    </BodyLong>
                    <BodyLong textColor="subtle">
                      <code className="font-mono text-xs">items</code> virker bare mot Tier 1. En Tier 1-pakke er filer
                      som kan velges hver for seg. En Tier 2-pakke er payload-trær bundet til en digest, der revisjonen
                      er enheten, ikke fila. <code className="font-mono text-xs">items</code> mot en Tier 2-pakke
                      nektes, ikke ignoreres, og feilmeldinga ber deg fjerne blokka eller be pakka publisere en
                      payload-kontekst med akkurat det teamet trenger.
                    </BodyLong>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="bygg-videre-pa-en" size="medium" level="2">
                      3. Bygg videre på en
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Vil du bygge på Nav-pakka i navikt/copilot uten å vedlikeholde en kopi av den, committer du den
                      samme erklæringa i ditt eget pakkerepo. Repoet ditt har da både et manifest og en erklæring, og
                      den som installerer pakka di får begge pakkenes innhold. Erklæringa løses på nytt ved hver{" "}
                      <code className="font-mono text-xs">install</code> og{" "}
                      <code className="font-mono text-xs">sync</code>.
                    </BodyLong>
                    <CodeBlock filename=".nav-pilot/agentpakke.lock.json">{GJENBRUK}</CodeBlock>
                    <BodyLong textColor="subtle">
                      <code className="font-mono text-xs">sha</code> er påkrevd for en kilde på formen{" "}
                      <code className="font-mono text-xs">owner/repo</code>. Uten den ville gjenbruken hentet det main
                      tilfeldigvis holdt, og to installasjoner en uke fra hverandre fått ulikt innhold. Installasjonen
                      sier hva den gjenbrukte:
                    </BodyLong>
                    <CodeBlock>{KOMPONERT_UT}</CodeBlock>
                    <BodyLong textColor="subtle">
                      Gjenbruker to pakker hverandre, finnes det ingen rekkefølge å løse dem i, og feilen ber om at
                      syklusen brytes i én av dem.
                    </BodyLong>

                    <VStack gap="space-8">
                      <LinkableHeading id="kollisjoner" size="small" level="3">
                        Kollisjoner
                      </LinkableHeading>
                      <BodyLong textColor="subtle">
                        Har begge pakkene en agent med samme navn, installeres din. Det er samme regel som{" "}
                        <code className="font-mono text-xs">overrides</code> i{" "}
                        <code className="font-mono text-xs">.github/copilot-sync.json</code>: det teamet eier selv, eier
                        de. Å skygge et artefakt er den vanlige måten å endre én ting i en pakke du ellers tar som den
                        er. Det er ikke en feil, og nav-pilot varsler ikke.
                      </BodyLong>
                    </VStack>

                    <VStack gap="space-8">
                      <LinkableHeading id="hva-som-komponerer" size="small" level="3">
                        Hva som komponerer
                      </LinkableHeading>
                      <BodyLong textColor="subtle">
                        I dag er det bare hel-pakke-installasjonen{" "}
                        <code className="font-mono text-xs">install &lt;navn&gt;</code> og{" "}
                        <code className="font-mono text-xs">sync</code> som tar med det gjenbrukte innholdet.{" "}
                        <code className="font-mono text-xs">install --all</code>, den interaktive plukkeren,{" "}
                        <code className="font-mono text-xs">install &lt;navn&gt; --type &lt;type&gt;</code> og{" "}
                        <code className="font-mono text-xs">list</code> gir bare topp-pakkas innhold, uten feil og uten
                        advarsel. <code className="font-mono text-xs">list</code> viser heller ikke det arvede
                        innholdet, så det brukeren ser stemmer med det som ble installert, og begge er ufullstendige. Si
                        det til konsumentene dine til{" "}
                        <a href="https://github.com/navikt/copilot/issues/844" className={linkClass}>
                          #844
                        </a>{" "}
                        er lukket.
                      </BodyLong>
                    </VStack>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="lag-din-egen" size="medium" level="2">
                      4. Lag din egen
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Skygging og <code className="font-mono text-xs">items</code> dekker de fleste grunnene folk har
                      til å forke en pakke. Lag din egen når innholdet ikke finnes noe sted.
                    </BodyLong>

                    <LinkableHeading id="struktur" size="small" level="3">
                      Struktur
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Katalognavnene velger du selv. <code className="font-mono text-xs">layout</code> i manifestet
                      peker på dem: heter katalogen <code className="font-mono text-xs">innhold/agenter</code> hos deg,
                      skriver du det der. Filnavnene inne i katalogene er låst, se artefakttypene under. Hver agentfil
                      åpner med YAML-frontmatter som minst har <code className="font-mono text-xs">name</code> og{" "}
                      <code className="font-mono text-xs">description</code>.
                    </BodyLong>
                    <CodeBlock>{STRUKTUR}</CodeBlock>

                    <LinkableHeading id="artefakttyper" size="small" level="3">
                      Artefakttyper
                    </LinkableHeading>
                    <Table size="small">
                      <TableHeader>
                        <TableRow>
                          <TableHeaderCell scope="col">Type</TableHeaderCell>
                          <TableHeaderCell scope="col">Form</TableHeaderCell>
                          <TableHeaderCell scope="col">Hva det er</TableHeaderCell>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {ARTIFACT_TYPES.map((t) => (
                          <TableRow key={t.type}>
                            <TableDataCell>
                              <code className="font-mono text-xs">{t.type}</code>
                            </TableDataCell>
                            <TableDataCell>
                              <code className="font-mono text-xs">{t.form}</code>
                            </TableDataCell>
                            <TableDataCell>{t.what}</TableDataCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>

                    <VStack gap="space-8">
                      <LinkableHeading id="kjorbar-kode" size="small" level="3">
                        Kjørbar kode
                      </LinkableHeading>
                      <BodyLong textColor="subtle">
                        Hooks og extensions er ikke tekst en modell leser. En hook kjører ved verktøykall, en extension
                        lastes av klienten. Den som installerer pakka di kjører koden din på maskinen sin, så si i
                        pakkas <code className="font-mono text-xs">description</code> hva den gjør.
                      </BodyLong>
                    </VStack>

                    <LinkableHeading id="manifestet" size="small" level="3">
                      Manifestet
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Minste form som validerer. <code className="font-mono text-xs">layout</code> navngir katalogene
                      pakka faktisk har, minst én av dem. <code className="font-mono text-xs">primaryAgents</code> er de
                      agentene brukeren kan starte klienten som. Resten er underagenter andre kaller. Første navn
                      startes som standard, og{" "}
                      <code className="font-mono text-xs">nav-pilot --persona &lt;navn&gt;</code> velger et annet av
                      dem. Hvert navn må ha en agentfil i <code className="font-mono text-xs">layout.agents</code>,
                      ellers avviser <code className="font-mono text-xs">validate</code> og{" "}
                      <code className="font-mono text-xs">install</code> manifestet.
                    </BodyLong>
                    <CodeBlock filename=".nav-pilot/agentpakke.json">{MANIFEST}</CodeBlock>

                    <LinkableHeading id="klientoppforinga" size="small" level="3">
                      Klientoppføringa
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Klientnøklene i dag er <code className="font-mono text-xs">copilot</code>,{" "}
                      <code className="font-mono text-xs">opencode</code> og{" "}
                      <code className="font-mono text-xs">pi</code>. En nav-pilot som ikke kjenner en nøkkel, hopper
                      over den i stedet for å avvise manifestet. En ny klient senere ugyldiggjør derfor ingen pakke som
                      alt er ute.
                    </BodyLong>
                    <CodeBlock filename=".nav-pilot/agentpakke.json">{KLIENTER}</CodeBlock>
                    <BodyLong textColor="subtle">
                      Tier utledes av formen og deklareres ikke. En klientoppføring uten{" "}
                      <code className="font-mono text-xs">payloads</code> er Tier 1: nav-pilot legger inn filene selv
                      fra stiene i <code className="font-mono text-xs">layout</code>, som da må finnes. En oppføring med{" "}
                      <code className="font-mono text-xs">payloads</code> er Tier 2: nav-pilot verifiserer og stager
                      ferdigbygde trær mot en digest, og pinner dem per bruker. En pakke kan blande de to per klient.
                      Sida her beskriver Tier 1. Hvordan du bygger payload-trær for Tier 2, står i{" "}
                      <a
                        href="https://github.com/navikt/copilot/blob/main/docs/README.agentpakke.md"
                        className={linkClass}
                      >
                        feltreferansen
                      </a>
                      .
                    </BodyLong>
                    <BodyLong textColor="subtle">
                      <code className="font-mono text-xs">compatibility</code> er et versjonsområde for klienten, ikke
                      en versjon: kommaseparerte komparatorer over semver, som{" "}
                      <code className="font-mono text-xs">&quot;&gt;=1.18.20,&lt;2&quot;</code>.{" "}
                      <code className="font-mono text-xs">&quot;1.18.20&quot;</code> alene har ingen operator og
                      avvises. Området håndheves før hver launch i begge tier (
                      <a href="https://github.com/navikt/copilot/pull/815" className={linkClass}>
                        #815
                      </a>
                      ): nav-pilot spør klienten om versjonen og nekter en versjon utenfor. Svarer ikke klienten, eller
                      er svaret uleselig, er det også fatalt. Et område nav-pilot ikke kan håndheve, er ikke håndhevet.
                    </BodyLong>
                    <BodyLong textColor="subtle">
                      <code className="font-mono text-xs">defaultModel</code> er per klient. Den literale verdien{" "}
                      <code className="font-mono text-xs">&quot;inherit&quot;</code> sender ingen{" "}
                      <code className="font-mono text-xs">--model</code>. En konkret modell-id sendes med. En modell
                      brukeren har pinnet selv, vinner over begge.{" "}
                      <code className="font-mono text-xs">minNavPilotVersion</code> ligger på pakkenivå, skrives på
                      nav-pilots releaseformat (<code className="font-mono text-xs">YYYY.MM.DD-HHMMSS</code>, eventuelt
                      med build-sha) og blokkerer eldre binærer med en melding som sier hva de skal gjøre. Et annet
                      format avvises framfor å ignoreres: nav-pilot kan ikke sammenligne det, og å godta det ville slått
                      av akkurat den gaten manifestet ba om.
                    </BodyLong>
                    <BodyLong textColor="subtle">
                      <strong>Kjørbar kode når ikke alle klientene.</strong> opencode og pi hopper over hooks, med en
                      advarsel på stderr som navngir dem. Det er koblinga som mangler, ikke evnen (
                      <a href="https://github.com/navikt/copilot/issues/709" className={linkClass}>
                        #709
                      </a>
                      ). Extensions håndteres ikke der i det hele tatt. Har pakka di en hook eller en extension noen er
                      avhengig av, er copilot den eneste klienten som får den.
                    </BodyLong>

                    <LinkableHeading id="uten-agent" size="small" level="3">
                      Pakke uten agent
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Deler dere bare ferdigheter eller instruksjoner, utelater dere{" "}
                      <code className="font-mono text-xs">primaryAgents</code> og{" "}
                      <code className="font-mono text-xs">agents</code> i{" "}
                      <code className="font-mono text-xs">layout</code>. Dere trenger ikke finne på en persona. Pakka
                      validerer, installeres og synkes som vanlig, men den kan ikke starte klienten: det finnes ingen
                      agent å gi den, og launch stopper med pakkas navn i meldinga. Start klienten selv, eller pek
                      nav-pilot på en pakke som deklarerer en agent. To pakker i samme scope er ingen utvei: et scope
                      installeres fra én kilde, og nav-pilot nekter å blande innhold fra to agentpakker i én
                      installasjon. Vil du ha den andre pakka i stedet, bytter du kilde for scopet.
                    </BodyLong>
                    <CodeBlock filename=".nav-pilot/agentpakke.json">{MANIFEST_UTEN_AGENT}</CodeBlock>

                    <LinkableHeading id="mcp-servere" size="small" level="3">
                      MCP-servere
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Valgfritt. MCP-servere styres sentralt i{" "}
                      <a href="https://mcp-registry.nav.no" className={linkClass}>
                        Navs MCP-register
                      </a>
                      . Du kan ikke definere din egen, men du kan si hvilke av registerets servere agentene og
                      ferdighetene dine forventer. nav-pilot spør registeret når den validerer og installerer: et navn
                      det ikke publiserer, er et funn. Svarer ikke registeret, blir det en advarsel i stedet, så en
                      CI-jobb uten nett ikke feiler på noe den ikke kan sjekke.
                    </BodyLong>
                    <CodeBlock filename=".nav-pilot/agentpakke.json">{MCP}</CodeBlock>
                    <BodyLong textColor="subtle">
                      Navnene skrives på registerets egen omvendt-DNS-form,{" "}
                      <code className="font-mono text-xs">&lt;namespace&gt;/&lt;navn&gt;</code>, som{" "}
                      <code className="font-mono text-xs">io.github.navikt/github-mcp</code>. Et navn uten den formen
                      avvises av schemaet før registeret spørres i det hele tatt.
                    </BodyLong>
                    <BodyLong textColor="subtle">
                      <code className="font-mono text-xs">install</code> navngir serverne pakka trenger og peker på
                      registeret. Det er alt: nav-pilot skriver ingen MCP-konfigurasjon, og brukeren slår på serveren
                      selv i klienten. Feltet ligger på pakkenivå, siden det er klientens eget oppsett som avgjør om en
                      server er tilgjengelig.
                    </BodyLong>

                    <LinkableHeading id="valider" size="small" level="3">
                      Valider
                    </LinkableHeading>
                    <CodeBlock>{VALIDER_CMD}</CodeBlock>
                    <CodeBlock>{VALIDER_UT}</CodeBlock>
                    <BodyLong textColor="subtle">
                      Advarsler (<code className="font-mono text-xs">⚠</code>) feiler ikke kommandoen. En{" "}
                      <code className="font-mono text-xs">defaultModel</code> nav-pilot ikke kjenner igjen er en av dem:
                      modellkatalogen synkes fra models.dev og henger etter en fersk modell, så et avvist manifest ville
                      tatt oftere feil enn advarselen gjør.
                    </BodyLong>
                    <BodyLong textColor="subtle">
                      Kilden må være <code className="font-mono text-xs">owner/repo</code> eller en absolutt sti.{" "}
                      <code className="font-mono text-xs">--source .</code> avvises av verdisjekken før noe forsøkes
                      hentet, så bruk <code className="font-mono text-xs">&quot;$PWD&quot;</code>, eller{" "}
                      <code className="font-mono text-xs">&quot;$GITHUB_WORKSPACE&quot;</code> i CI. Etiketten i
                      utdataene er kilden du oppga, ikke navnet i manifestet. Skjemaet ligger i{" "}
                      <code className="font-mono text-xs">cli/nav-pilot/schemas/agentpakke-v1.json</code>, så CI kan
                      linte manifestet mot det uten nav-pilot.
                    </BodyLong>

                    <LinkableHeading id="distribuer" size="small" level="3">
                      Distribuer
                    </LinkableHeading>
                    <CodeBlock>{INSTALL_CMD}</CodeBlock>
                    <BodyLong textColor="subtle">
                      Det er hele distribusjonen. Den som installerer får{" "}
                      <code className="font-mono text-xs">.nav-pilot/agentpakke.lock.json</code> i sitt eget repo, med
                      kilden og revisjonen, og committer den. Da installerer hele teamet fra samme revisjon.
                    </BodyLong>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="vedlikehold" size="medium" level="2">
                      Vedlikehold
                    </LinkableHeading>

                    <VStack gap="space-8">
                      <LinkableHeading id="nar-endringen-nar-fram" size="small" level="3">
                        Når endringen når fram
                      </LinkableHeading>
                      <BodyLong textColor="subtle">
                        Konsumentene er pinnet til revisjonen de installerte. En endring du pusher, når dem først når{" "}
                        <code className="font-mono text-xs">nav-pilot sync --apply</code> flytter pinnen hos dem, som én
                        linje diff i en pull request de leser og godkjenner. En rettelse er derfor ikke ute samme dag. Å
                        endre <code className="font-mono text-xs">name</code> i manifestet gjør eksisterende
                        installasjoner til en annen pakke, så det er ikke en omdøping du gjør i forbifarten.
                      </BodyLong>
                    </VStack>

                    <VStack gap="space-8">
                      <LinkableHeading id="stabile-releases" size="small" level="3">
                        Stabile releases
                      </LinkableHeading>
                      <BodyLong textColor="subtle">
                        Uten releases henter <code className="font-mono text-xs">sync</code> det standardgrenen holder,
                        så alt du pusher går rett ut til konsumentene. Publiserer du i stedet en GitHub Release med
                        assetet <code className="font-mono text-xs">agentpakke-release.json</code>, leser{" "}
                        <code className="font-mono text-xs">install</code> og{" "}
                        <code className="font-mono text-xs">sync</code> nyeste stabile release, og du kan jobbe videre
                        på main. Releasen må være publisert, ikke prerelease, og immutable, og taggen må binde
                        versjonen. Et repo uten slike releases fungerer nøyaktig som før, fra standardgrenen.
                      </BodyLong>
                      <BodyLong textColor="subtle">
                        <strong>Er pakka di Tier 1</strong>, altså en layout av filer, pinner den ingen revisjon: den
                        installerer filer, og abonnementet avgjør bare hvilken revisjon filene leses fra.{" "}
                        <code className="font-mono text-xs">install</code> og{" "}
                        <code className="font-mono text-xs">sync</code> uten{" "}
                        <code className="font-mono text-xs">--ref</code> leser nyeste stabile release i stedet for
                        standardgrenen, i både bruker- og repo-scope, og{" "}
                        <code className="font-mono text-xs">sync --apply</code> flytter{" "}
                        <code className="font-mono text-xs">sha</code> i erklæringa til release-SHA-en. Det finnes ikke
                        noe nedgraderingsvern her: hvert oppslag tar nyeste stabile release uten å sammenligne med det
                        som ligger på disk, så en installasjon som står foran releasen, flyttes tilbake til den ved
                        neste <code className="font-mono text-xs">sync --apply</code>. Diffen vises før{" "}
                        <code className="font-mono text-xs">--apply</code>, som for enhver annen fil.
                      </BodyLong>
                      <BodyLong textColor="subtle">
                        Tre mekanismer hører sammen med releases, og de virker i dag bare for Tier 2, fordi alle tre er
                        gatet på at scopet pinner en revisjon:{" "}
                        <code className="font-mono text-xs">nav-pilot rollback</code> tilbake til forrige revisjon på
                        maskinen, oppstartsspørsmålet om å ta en ny release, og det varige valget{" "}
                        <code className="font-mono text-xs">sync --updates auto|ask|keep</code>. En Tier 1-installasjon
                        fører opp filene den la ned, og faller derfor utenfor gaten: rollback nekter med «your user
                        scope pins none», og de to andre nås aldri. Lov derfor ikke konsumentene dine et rollback en
                        Tier 1-pakke ikke gir dem. Om gaten skal utvides, er åpent (
                        <a href="https://github.com/navikt/copilot/issues/843" className={linkClass}>
                          #843
                        </a>
                        ).
                      </BodyLong>
                      <BodyLong textColor="subtle">
                        <strong>Er pakka di Tier 2</strong>, gjelder alle tre. Konsumenten kan rulle tilbake uten nett
                        og uten å vente på deg, den forlatte revisjonen tilbys ikke igjen mens neste release gjør det
                        (lever derfor rettelsen som en ny versjon; en revert av taggen når dem ikke), og et team som har
                        valgt <code className="font-mono text-xs">keep</code>, blir stående til de selv tar releasen.
                        Regn ikke med at alle er på nyeste versjon dagen etter. Konsumenter som alt står på
                        standardgrenen ligger som regel foran din første release, og nedgraderingsvernet tilbyr den ikke
                        til dem; nav-pilot spør dem én gang ved oppstart om å pinne releasen og følge releases videre,
                        og navngir begge revisjonene.
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
                        <code className="font-mono text-xs">.nav-pilot/retired-artifacts.json</code>. En kilde hentes
                        med <code className="font-mono text-xs">--depth 1</code>, så brukeren har ingen historikk å slå
                        opp i, og en fil som bare forsvinner blir liggende hos alle som installerte den. Generer lista
                        og commit den: <code className="font-mono text-xs">scripts/generate-retired</code> i
                        navikt/copilot er en Go-modul på rundt 200 linjer som leser hashene ut av git-loggen, og{" "}
                        <code className="font-mono text-xs">mise run retired:check</code> verifiserer i CI at lista
                        stemmer.
                      </BodyLong>
                      <BodyLong textColor="subtle">
                        Fila navngir hver pensjonerte sti sammen med innholdshashene pakka en gang publiserte, og det er
                        hashene som gjør den trygg: nav-pilot sletter en installert fil bare når bytene matcher en
                        revisjon kilden faktisk har publisert. En utvikler som har skrevet sin egen agent på den stien,
                        beholder den. At kilden ikke lenger har noe med dette navnet, er ikke tillatelse til å slette.
                      </BodyLong>
                    </VStack>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="ressurser" size="medium" level="2">
                      Ressurser
                    </LinkableHeading>
                    <HGrid gap="space-16" columns={{ xs: 1, md: 2 }}>
                      <Box background="neutral-soft" padding="space-16" borderRadius="8">
                        <VStack gap="space-4">
                          <BodyShort weight="semibold">
                            <a
                              href="https://github.com/navikt/copilot/blob/main/docs/README.agentpakke.md"
                              className={linkClass}
                            >
                              Feltreferansen
                            </a>
                          </BodyShort>
                          <BodyLong size="small" textColor="subtle">
                            Beskriver hvert felt i manifestet og erklæringa, tier, stiregler og kompatibilitet.
                          </BodyLong>
                        </VStack>
                      </Box>
                      <Box background="neutral-soft" padding="space-16" borderRadius="8">
                        <VStack gap="space-4">
                          <BodyShort weight="semibold">
                            <a
                              href="https://github.com/navikt/copilot/blob/main/cli/nav-pilot/schemas/agentpakke-v1.json"
                              className={linkClass}
                            >
                              Skjemaet
                            </a>
                          </BodyShort>
                          <BodyLong size="small" textColor="subtle">
                            Kontrakten binæren validerer mot, og den CI kan linte mot.
                          </BodyLong>
                        </VStack>
                      </Box>
                    </HGrid>
                  </VStack>
                </section>
              </VStack>
            </div>
          </div>
        </Box>
      </div>
      <BackToTop />
    </main>
  );
}
