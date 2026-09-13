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
  title: "Lag en agentpakke",
  description:
    "Distribuer teamets egne agenter, skills og instruksjoner som en agentpakke andre team kan installere med nav-pilot.",
};

const DOC_SECTIONS: TocItem[] = [
  { id: "hva-det-er", label: "Hva det er" },
  {
    id: "lag-pakka",
    label: "Lag pakka",
    children: [
      { id: "struktur", label: "Struktur" },
      { id: "artefakttyper", label: "Artefakttyper" },
      { id: "kjorbar-kode", label: "Kjørbar kode" },
      { id: "manifestet", label: "Manifestet" },
      { id: "uten-agent", label: "Pakke uten agent" },
      { id: "mcp-servere", label: "MCP-servere" },
      { id: "valider", label: "Valider" },
      { id: "distribuer", label: "Distribuer" },
    ],
  },
  {
    id: "gjenbruk",
    label: "Gjenbruk",
    children: [{ id: "kollisjoner", label: "Kollisjoner" }],
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
  { type: "hooks", form: "<navn>.py og <navn>.hook.json", what: "Skript som kjører ved verktøykall" },
  { type: "extensions", form: "<navn>/extension.mjs", what: "Kode klienten laster inn" },
];

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
        title="Lag en agentpakke"
        description="Distribuer teamets eget oppsett som noe andre team kan installere."
        badge={
          <Tag variant="info" size="small" className="uppercase tracking-wide">
            For pakkeforfattere
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
                      En agentpakke er et git-repo som sender AI-artefakter og en fil som beskriver dem. Det finnes
                      ingen registrering, ingen godkjenning og ingen sentral liste å komme inn på. Et repo med et gyldig
                      manifest er en agentpakke, og den som vil ha den, peker på repoet med{" "}
                      <code className="font-mono text-xs">--source</code>. Skal du bare bruke Nav-innholdet, trenger du
                      ikke denne sida:{" "}
                      <NextLink href="/nav-pilot/docs" className={linkClass}>
                        nav-pilot-dokumentasjonen
                      </NextLink>{" "}
                      dekker installasjon og bruk.
                    </BodyLong>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="lag-pakka" size="medium" level="2">
                      Lag pakka
                    </LinkableHeading>

                    <LinkableHeading id="struktur" size="small" level="3">
                      Struktur
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Katalognavnene er dine egne. <code className="font-mono text-xs">layout</code> i manifestet peker
                      på dem: heter katalogen <code className="font-mono text-xs">innhold/agenter</code> hos deg,
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
                      agentene brukeren kan starte klienten som; resten er underagenter andre kaller. Første navn
                      startes som standard, og{" "}
                      <code className="font-mono text-xs">nav-pilot --persona &lt;navn&gt;</code> velger et annet av
                      dem. Hvert navn må ha en agentfil i <code className="font-mono text-xs">layout.agents</code>,
                      ellers avvises manifestet av <code className="font-mono text-xs">validate</code> og{" "}
                      <code className="font-mono text-xs">install</code>.
                    </BodyLong>
                    <CodeBlock filename=".nav-pilot/agentpakke.json">{MANIFEST}</CodeBlock>

                    <LinkableHeading id="uten-agent" size="small" level="3">
                      Pakke uten agent
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Deler dere bare ferdigheter eller instruksjoner, utelater dere{" "}
                      <code className="font-mono text-xs">primaryAgents</code> og{" "}
                      <code className="font-mono text-xs">agents</code> i{" "}
                      <code className="font-mono text-xs">layout</code>. Ingen persona å finne på. Pakka validerer,
                      installeres og synkes som vanlig, men den kan ikke starte klienten: da finnes det ingen agent å gi
                      den, og launch stopper med pakkas navn i meldinga. Start klienten selv, eller bruk pakka sammen
                      med en som deklarerer en agent.
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
                      <code className="font-mono text-xs">install</code> navngir serverne pakka trenger og peker på
                      registeret. Det er alt: nav-pilot skriver ingen MCP-konfigurasjon, og å slå på en server er
                      brukerens handling i klienten. Feltet ligger på pakkenivå, siden det er klientens eget oppsett som
                      avgjør om en server er tilgjengelig.
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
                      Kilden må være en absolutt sti. <code className="font-mono text-xs">--source .</code> blir forsøkt
                      klonet som et GitHub-repo og feiler, så bruk{" "}
                      <code className="font-mono text-xs">&quot;$PWD&quot;</code>, eller{" "}
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
                    <LinkableHeading id="gjenbruk" size="medium" level="2">
                      Gjenbruk
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Vil du bygge på Nav-pakka i navikt/copilot uten å vedlikeholde en kopi av den, committer du den
                      samme erklæringa i ditt eget pakkerepo. Repoet ditt sender da både et manifest og en erklæring, og
                      den som installerer pakka di får begge pakkenes innhold. Erklæringa løses på nytt ved hver{" "}
                      <code className="font-mono text-xs">install</code> og{" "}
                      <code className="font-mono text-xs">sync</code>.
                    </BodyLong>
                    <CodeBlock filename=".nav-pilot/agentpakke.lock.json">{GJENBRUK}</CodeBlock>
                    <BodyLong textColor="subtle">
                      <code className="font-mono text-xs">sha</code> er påkrevd for en kilde på formen{" "}
                      <code className="font-mono text-xs">owner/repo</code>. Uten den ville gjenbruken hentet det main
                      tilfeldigvis holdt, og to installasjoner en uke fra hverandre fikk ulikt innhold. Installasjonen
                      sier hva den gjenbrukte:
                    </BodyLong>
                    <CodeBlock>{KOMPONERT_UT}</CodeBlock>

                    <VStack gap="space-8">
                      <LinkableHeading id="kollisjoner" size="small" level="3">
                        Kollisjoner
                      </LinkableHeading>
                      <BodyLong textColor="subtle">
                        Sender begge pakkene en agent med samme navn, installeres din. Det er samme regel som{" "}
                        <code className="font-mono text-xs">overrides</code> i{" "}
                        <code className="font-mono text-xs">.github/copilot-sync.json</code>: det teamet eier selv, eier
                        de. Å skygge et artefakt er den normale måten å endre én ting fra en pakke du ellers tar rått.
                        Det er ikke en feil, og varsles ikke.
                      </BodyLong>
                    </VStack>
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
                        versjonen.
                      </BodyLong>
                      <BodyLong textColor="subtle">
                        Konsumenter som alt står på standardgrenen, ligger som regel foran din første release.
                        Nedgraderingsvernet tilbyr den ikke til dem, så nav-pilot spør dem én gang ved oppstart om å
                        pinne releasen og følge releases videre. Spørsmålet navngir begge revisjonene og sier at din
                        release kan være eldre enn det de har. Svarer de nei, blir de stående, og ingenting flyttes uten
                        et ja.
                      </BodyLong>
                      <BodyLong textColor="subtle">
                        Går en release i stykker hos en konsument, kan de kjøre{" "}
                        <code className="font-mono text-xs">nav-pilot rollback</code> og gå tilbake til revisjonen de
                        hadde før, uten nett og uten å vente på deg. Den revisjonen blir ikke tilbudt dem igjen, men
                        neste release blir det. Lever derfor rettelsen som en ny versjon; en revert av taggen når dem
                        ikke.
                      </BodyLong>
                      <BodyLong textColor="subtle">
                        Et repo uten slike releases fungerer nøyaktig som før, fra standardgrenen. Feltene og hele
                        kontrakten står i{" "}
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
                        navikt/copilot leser hashene ut av git-loggen, og{" "}
                        <code className="font-mono text-xs">mise run retired:check</code> verifiserer i CI at lista
                        stemmer. Skriptet er rundt hundre linjer og kan kopieres.
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
