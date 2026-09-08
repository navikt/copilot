import { Heading, BodyLong, BodyShort, Box, VStack, HGrid, Tag } from "@navikt/ds-react";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { PageHero } from "@/components/page-hero";
import { TableOfContents, type TocItem } from "@/components/table-of-contents";
import { BackToTop } from "@/components/back-to-top";
import { PackageIcon, LayersIcon, ArrowsCirclepathIcon, ShieldLockIcon, TrashIcon } from "@navikt/aksel-icons";
import type { Metadata } from "next";
import NextLink from "next/link";

export const metadata: Metadata = {
  title: "Lag en agentpakke",
  description:
    "Distribuer teamets egne agenter, skills og instruksjoner som en agentpakke andre team kan installere med nav-pilot.",
};

const DOC_SECTIONS: TocItem[] = [
  {
    id: "hva-er-en-agentpakke",
    label: "Hva det er",
    children: [
      { id: "hvem-er-dette-for", label: "Hvem det er for" },
      { id: "ingen-registrering", label: "Ingen registrering" },
    ],
  },
  {
    id: "lag-en",
    label: "Lag en",
    children: [
      { id: "struktur", label: "Struktur" },
      { id: "manifestet", label: "Manifestet" },
      { id: "valider", label: "Valider" },
      { id: "distribuer", label: "Distribuer" },
    ],
  },
  {
    id: "artefakttyper",
    label: "Artefakttyper",
    children: [{ id: "kjorbar-kode", label: "Kjørbar kode" }],
  },
  {
    id: "gjenbruk",
    label: "Gjenbruk",
    children: [
      { id: "kollisjoner", label: "Kollisjoner" },
      { id: "bindingstidspunkt", label: "Bindingstidspunkt" },
    ],
  },
  {
    id: "vedlikehold",
    label: "Vedlikehold",
    children: [
      { id: "pinner-og-sync", label: "Pinner og sync" },
      { id: "pensjonering", label: "Pensjonering" },
    ],
  },
  { id: "videre", label: "Videre" },
];

const ARTIFACT_TYPES = [
  { type: "agents", what: "Personaer klienten kan startes som, eller underagenter andre kaller", exec: false },
  { type: "skills", what: "Kunnskap modellen laster ved behov", exec: false },
  { type: "instructions", what: "Regler som aktiveres mot matchende filer", exec: false },
  { type: "prompts", what: "Ferdige spørsmål brukeren kan kjøre", exec: false },
  { type: "hooks", what: "Skript som kjører ved verktøykall", exec: true },
  { type: "extensions", what: "Kode klienten laster inn", exec: true },
];

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

const STRUKTUR = `ditt-repo/
├── .nav-pilot/
│   └── agentpakke.json
├── agents/
│   └── grillmester.agent.md
└── skills/`;

const VALIDER = `$ nav-pilot validate --source .

Validating: ditt-team@c3f7ca3

  ℹ manifest: .nav-pilot/agentpakke.json
  ℹ agentpakke: ditt-team (contract version 1)
  ℹ clients: copilot (tier 1)

✓ . conforms to the agentpakke contract.`;

const GJENBRUK = `{
  "contractVersion": "1",
  "source": "navikt/copilot",
  "sha": "b73a69e0000000000000000000000000000000aa"
}`;

const KOMPONERT_INSTALL = `$ nav-pilot install ditt-team --source navikt/ditt-repo --repo

Source: ditt-team@c3f7ca3
Reuses: navikt/copilot@b73a69e
  3 agents: eget, felles, grillmester`;

const subtle = { color: "#475569" };

function Section({
  id,
  icon,
  title,
  children,
}: {
  id: string;
  icon: React.ReactNode;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section id={id}>
      <VStack gap="space-16">
        <div>
          <div className="flex items-center gap-3">
            {icon}
            <LinkableHeading size="medium" level="2">
              {title}
            </LinkableHeading>
          </div>
        </div>
        {children}
      </VStack>
    </section>
  );
}

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
                <Section
                  id="hva-er-en-agentpakke"
                  icon={<PackageIcon aria-hidden fontSize="1.5rem" />}
                  title="Hva det er"
                >
                  <BodyLong style={subtle}>
                    En agentpakke er et git-repo som sender AI-artefakter og en fil som beskriver dem. Det er hele
                    greia. Teamet ditt kan distribuere sine egne agenter, skills og instruksjoner uten å forke nav-pilot
                    og uten å sende pull request til <code className="font-mono text-xs">navikt/copilot</code>.
                  </BodyLong>

                  <div id="hvem-er-dette-for">
                    <Heading size="small" level="3">
                      Hvem det er for
                    </Heading>
                    <BodyLong className="mt-2" style={subtle}>
                      Team som allerede har bygd et oppsett de er fornøyde med, og som andre spør etter. Skal du bare
                      bruke Nav-innholdet, trenger du ikke denne sida:{" "}
                      <NextLink href="/nav-pilot/docs" className="underline">
                        nav-pilot-dokumentasjonen
                      </NextLink>{" "}
                      dekker installasjon og bruk.
                    </BodyLong>
                  </div>

                  <div id="ingen-registrering">
                    <Heading size="small" level="3">
                      Ingen registrering
                    </Heading>
                    <BodyLong className="mt-2" style={subtle}>
                      Det finnes ingen godkjenning og ingen sentral liste å komme inn på. Et repo med et gyldig manifest
                      er en agentpakke. Den som vil ha den, peker på repoet med{" "}
                      <code className="font-mono text-xs">--source</code>.
                    </BodyLong>
                  </div>
                </Section>

                <Section id="lag-en" icon={<LayersIcon aria-hidden fontSize="1.5rem" />} title="Lag en">
                  <div id="struktur">
                    <Heading size="small" level="3">
                      Struktur
                    </Heading>
                    <BodyLong className="mt-2 mb-3" style={subtle}>
                      Katalognavnene er dine egne. <code className="font-mono text-xs">layout</code> i manifestet peker
                      på dem, så heter de <code className="font-mono text-xs">innhold/agenter</code> hos deg, sier du
                      det der.
                    </BodyLong>
                    <CodeBlock>{STRUKTUR}</CodeBlock>
                  </div>

                  <div id="manifestet">
                    <Heading size="small" level="3">
                      Manifestet
                    </Heading>
                    <BodyLong className="mt-2 mb-3" style={subtle}>
                      Minste form som validerer. Både <code className="font-mono text-xs">agents</code> og{" "}
                      <code className="font-mono text-xs">skills</code> må stå i{" "}
                      <code className="font-mono text-xs">layout</code>, også når den ene er tom.{" "}
                      <code className="font-mono text-xs">primaryAgents</code> er de agentene brukeren kan starte
                      klienten som; resten er underagenter andre kaller.
                    </BodyLong>
                    <CodeBlock filename=".nav-pilot/agentpakke.json">{MANIFEST}</CodeBlock>
                  </div>

                  <div id="valider">
                    <Heading size="small" level="3">
                      Valider
                    </Heading>
                    <BodyLong className="mt-2 mb-3" style={subtle}>
                      Kjør den i CI også. Skjemaet er publisert, så du kan linte mot det uten nav-pilot.
                    </BodyLong>
                    <CodeBlock>{VALIDER}</CodeBlock>
                  </div>

                  <div id="distribuer">
                    <Heading size="small" level="3">
                      Distribuer
                    </Heading>
                    <BodyLong className="mt-2" style={subtle}>
                      Det er hele distribusjonen:{" "}
                      <code className="font-mono text-xs">
                        nav-pilot install ditt-team --source navikt/ditt-repo --repo
                      </code>
                      . Den som installerer får en{" "}
                      <code className="font-mono text-xs">.nav-pilot/agentpakke.lock.json</code> i sitt eget repo, med
                      kilden og revisjonen, og committer den. Da installerer hele teamet fra samme revisjon.
                    </BodyLong>
                  </div>
                </Section>

                <Section
                  id="artefakttyper"
                  icon={<ShieldLockIcon aria-hidden fontSize="1.5rem" />}
                  title="Artefakttyper"
                >
                  <BodyLong style={subtle}>Seks typer. To av dem er kjørbar kode.</BodyLong>
                  <Table size="small">
                    <TableHeader>
                      <TableRow>
                        <TableHeaderCell>Type</TableHeaderCell>
                        <TableHeaderCell>Hva det er</TableHeaderCell>
                        <TableHeaderCell>Kjøres</TableHeaderCell>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {ARTIFACT_TYPES.map((t) => (
                        <TableRow key={t.type}>
                          <TableDataCell>
                            <code className="font-mono text-xs">{t.type}</code>
                          </TableDataCell>
                          <TableDataCell>{t.what}</TableDataCell>
                          <TableDataCell>{t.exec ? "Ja" : "Nei"}</TableDataCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>

                  <div id="kjorbar-kode">
                    <Heading size="small" level="3">
                      Kjørbar kode
                    </Heading>
                    <BodyLong className="mt-2" style={subtle}>
                      Hooks og extensions er ikke tekst en modell leser. En hook kjører ved verktøykall, en extension
                      lastes av klienten. Den som installerer pakka di kjører koden din på maskinen sin, så si i{" "}
                      <code className="font-mono text-xs">description</code> hva den gjør.
                    </BodyLong>
                  </div>
                </Section>

                <Section id="gjenbruk" icon={<ArrowsCirclepathIcon aria-hidden fontSize="1.5rem" />} title="Gjenbruk">
                  <BodyLong style={subtle}>
                    Vil du bygge på plattformpakka uten å vedlikeholde en kopi av den, committer du den samme erklæringa
                    i ditt eget pakkerepo. Repoet ditt sender da både et manifest og en erklæring, og den som
                    installerer pakka di får begge pakkenes innhold.
                  </BodyLong>
                  <CodeBlock filename=".nav-pilot/agentpakke.lock.json">{GJENBRUK}</CodeBlock>
                  <BodyLong style={subtle}>
                    <code className="font-mono text-xs">sha</code> er påkrevd for en kilde på formen{" "}
                    <code className="font-mono text-xs">owner/repo</code>. Uten den ville gjenbruken hentet det main
                    tilfeldigvis holdt, og to installasjoner en uke fra hverandre fikk ulikt innhold.
                  </BodyLong>
                  <CodeBlock>{KOMPONERT_INSTALL}</CodeBlock>

                  <div id="kollisjoner">
                    <Heading size="small" level="3">
                      Kollisjoner
                    </Heading>
                    <BodyLong className="mt-2" style={subtle}>
                      Sender begge pakkene en agent som heter det samme, vinner din. Nærmeste vinner, og det er samme
                      regel som <code className="font-mono text-xs">overrides</code> i{" "}
                      <code className="font-mono text-xs">.github/copilot-sync.json</code> ett nivå opp. Å skygge et
                      artefakt er den normale måten å endre én ting fra en pakke du ellers tar rått. Det er ikke en
                      feil, og varsles ikke.
                    </BodyLong>
                  </div>

                  <div id="bindingstidspunkt">
                    <Heading size="small" level="3">
                      Bindingstidspunkt
                    </Heading>
                    <BodyLong className="mt-2" style={subtle}>
                      Følger formen på manifestet, ikke en egen mekanisme. En layout-pakke løser erklæringa ved hver
                      install og sync. En payload-pakke løste den ved byggetid, og payloaden bærer resultatet.
                    </BodyLong>
                  </div>
                </Section>

                <Section id="vedlikehold" icon={<TrashIcon aria-hidden fontSize="1.5rem" />} title="Vedlikehold">
                  <div id="pinner-og-sync">
                    <Heading size="small" level="3">
                      Pinner og sync
                    </Heading>
                    <BodyLong className="mt-2" style={subtle}>
                      Konsumentens erklæring bærer revisjonen. <code className="font-mono text-xs">nav-pilot sync</code>{" "}
                      flytter pinnen som en linje i en pull request, så en oppdatering fra deg kommer til dem som noe de
                      kan lese og godkjenne.
                    </BodyLong>
                  </div>

                  <div id="pensjonering">
                    <Heading size="small" level="3">
                      Pensjonering
                    </Heading>
                    <BodyLong className="mt-2" style={subtle}>
                      Slett aldri et artefakt uten å føre det opp i{" "}
                      <code className="font-mono text-xs">.nav-pilot/retired-artifacts.json</code>. En kilde hentes med{" "}
                      <code className="font-mono text-xs">--depth 1</code>, så brukeren har ingen historikk å slå opp i,
                      og en fil som bare forsvinner blir liggende hos alle som installerte den.
                    </BodyLong>
                  </div>
                </Section>

                <section id="videre">
                  <VStack gap="space-16">
                    <LinkableHeading size="medium" level="2">
                      Videre
                    </LinkableHeading>
                    <HGrid gap="space-16" columns={{ xs: 1, md: 2 }}>
                      <Box background="neutral-soft" padding="space-16" borderRadius="8">
                        <BodyShort weight="semibold">Oppskrifta</BodyShort>
                        <BodyLong size="small" className="mt-1" style={subtle}>
                          <code className="font-mono text-xs">docs/lag-en-agentpakke.md</code> i navikt/copilot, steg
                          for steg med kommandoene.
                        </BodyLong>
                      </Box>
                      <Box background="neutral-soft" padding="space-16" borderRadius="8">
                        <BodyShort weight="semibold">Feltreferansen</BodyShort>
                        <BodyLong size="small" className="mt-1" style={subtle}>
                          <code className="font-mono text-xs">docs/README.agentpakke.md</code> beskriver hvert felt i
                          kontrakten, og JSON-skjemaet er publisert.
                        </BodyLong>
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
