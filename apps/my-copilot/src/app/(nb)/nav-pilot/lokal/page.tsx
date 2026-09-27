import { BodyLong, BodyShort, Box, HGrid, Label, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { PageHero } from "@/components/page-hero";
import { getLocalModels } from "@/lib/local-models";

export const metadata: Metadata = {
  title: "Lokal modell og decide — nav-pilot",
  description:
    "Kjør en kodemodell på din egen Mac. Det den leser og skriver, blir på maskinen, og den bruker ingen AI-credits. alpha decide gir raske, typede svar i hooks og skript.",
};

const linkClass = "text-blue-600 hover:underline";
const code = "font-mono text-xs";

const REPORTS = "https://github.com/navikt/mlx-workspace/blob/main";
const SOURCES = {
  night1: `${REPORTS}/reports/2026-09-25-quality-frontier/night-1.md`,
  night2: `${REPORTS}/reports/2026-09-25-quality-frontier/night-2.md`,
  why: `${REPORTS}/bench/decide-cases/commit-explains-why-results.md`,
  sets: `${REPORTS}/bench/decide-sets-20260925-225356.md`,
  systemOne: `${REPORTS}/reports/2026-09-25-system-one/report.md`,
};

const DECIDE_EXAMPLE = `$ nav-pilot alpha decide \\
    "Does the commit message explain why the change was made, beyond describing what the diff already shows?" \\
    --options yes,no --evidence commit.txt

  no  p=0.88

  yes                  0.119
  no                   0.881

  mlx-community/Qwen3.6-35B-A3B-OptiQ-4bit · 424 ms · evidence: true`;

const KOM_I_GANG = `brew install navikt/tap/nav-pilot   # første gang
brew upgrade navikt/tap/nav-pilot   # har du den fra før
nav-pilot alpha local init          # laster ned modellen og starter serveren
nav-pilot alpha local status        # kjører den, og hvilken modell?
nav-pilot alpha local models        # modellene du kan velge
nav-pilot alpha local use <key>     # bytt modell`;

const SOURCE_LABELS: Record<keyof typeof SOURCES, string> = {
  night1: "Natt 1",
  night2: "Natt 2",
  why: "Commit-spørsmålet",
  sets: "Issue-etiketter",
  systemOne: "decide-rapporten",
};

type Row = { task: string; result: string; verdict: string; source: keyof typeof SOURCES };

const WORKER_ROWS: Row[] = [
  {
    task: "Legge til et påkrevd argument i 1–2 kall, i flere filer",
    result: "10 av 10",
    verdict: "Godkjent for utsending",
    source: "night2",
  },
  {
    task: "Det samme i 3–8 kall",
    result: "9 av 10 på begge trinnene",
    verdict: "Ikke avgjort, trenger flere kjøringer",
    source: "night2",
  },
  {
    task: "Det samme i 9 kall eller flere",
    result: "6–8 av 10",
    verdict: "Blir i skyen",
    source: "night2",
  },
  {
    task: "Endre én fil, de to letteste trinnene",
    result: "13 og 10 av 16 på første forsøk, 16 av 16 med inntil to nye forsøk",
    verdict: "Nye forsøk lønner seg her",
    source: "night2",
  },
  {
    task: "Lage en ny fil",
    result: "5 av 16 på første forsøk, 12 av 16 med nye forsøk, men dobbelt så lang tid",
    verdict: "Ikke avgjort",
    source: "night2",
  },
  {
    task: "Svare på spørsmål om kodebasen",
    result: "18 av 40",
    verdict: "Blir i skyen",
    source: "night1",
  },
];

const DECIDE_ROWS: Row[] = [
  {
    task: "Forklarer commit-meldingen hvorfor?",
    result: "89 av 96 (93 %). Ved terskel 0,7 fanget den 40 av 48 meldinger uten hvorfor og flagget ingen av 48 med",
    verdict: "Brukt i commit-hooken",
    source: "why",
  },
  {
    task: "Er issuet en bug, et ønske eller et spørsmål?",
    result: "95 av 105 (90 %). Svarene med p ≥ 0,9 var riktige i 71 av 71",
    verdict: "Målt, oppskrift i dokumentasjonen",
    source: "sets",
  },
  {
    task: "Svartid per kall, varm server",
    result: "Median 0,4 sekunder",
    verdict: "Én maskin, M5 Max",
    source: "sets",
  },
];

function ResultTable({ rows }: { rows: Row[] }) {
  return (
    <div className="overflow-x-auto">
      <Table size="small" className="w-full">
        <TableHeader>
          <TableRow>
            <TableHeaderCell scope="col">Oppgave</TableHeaderCell>
            <TableHeaderCell scope="col">Resultat</TableHeaderCell>
            <TableHeaderCell scope="col">Hva det betyr</TableHeaderCell>
            <TableHeaderCell scope="col">Kilde</TableHeaderCell>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((r) => (
            <TableRow key={r.task}>
              <TableDataCell>{r.task}</TableDataCell>
              <TableDataCell>{r.result}</TableDataCell>
              <TableDataCell>{r.verdict}</TableDataCell>
              <TableDataCell>
                <a href={SOURCES[r.source]} className={linkClass}>
                  {SOURCE_LABELS[r.source]}
                </a>
              </TableDataCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function Card({ title, children }: { title: React.ReactNode; children: React.ReactNode }) {
  return (
    <Box background="neutral-soft" padding="space-16" borderRadius="8">
      <VStack gap="space-8">
        <BodyShort weight="semibold">{title}</BodyShort>
        <BodyLong size="small" textColor="subtle">
          {children}
        </BodyLong>
      </VStack>
    </Box>
  );
}

export default async function LokalModell() {
  const { models } = await getLocalModels();
  const defaultModel = models.find((m) => m.default) ?? models[0];
  const modelName = defaultModel.model.split("/").pop();

  return (
    <main>
      <PageHero
        title="Lokal modell og decide"
        description="Macen din kjører en kodemodell for enkle oppgaver. Det den leser og skriver, blir på maskinen, og den bruker ingen AI-credits."
        badge={
          <Tag variant="warning" size="small" className="uppercase tracking-wide">
            Alfa
          </Tag>
        }
      />
      <div className="max-w-5xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap={{ xs: "space-32", md: "space-40" }}>
            <section>
              <VStack gap="space-16">
                <BodyLong>
                  nav-pilot kan kjøre en kodemodell på din egen Mac. Hovedagenten i skyen planlegger og sender
                  avgrensede oppgaver til den, for eksempel å føre et nytt argument gjennom et par kall. Med{" "}
                  <code className={code}>nav-pilot alpha decide</code> stiller du den samme modellen et
                  flervalgsspørsmål fra en hook eller et skript, og får svar på under et halvt sekund.
                </BodyLong>
                <BodyLong textColor="subtle">
                  Her er eksempelet fra{" "}
                  <NextLink href="/nyheter/nav-pilot-alpha-decide" className={linkClass}>
                    nyhetssaken om decide
                  </NextLink>
                  : forklarer denne commit-meldingen hvorfor endringen ble gjort? Meldingen lister opp hva som er lagt
                  til, men ikke hvorfor, og modellen svarer «no».
                </BodyLong>
                <CodeBlock compact>{DECIDE_EXAMPLE}</CodeBlock>
              </VStack>
            </section>

            <section>
              <VStack gap="space-16">
                <LinkableHeading id="hva-du-far" size="medium" level="2">
                  Hva du får
                </LinkableHeading>
                <HGrid columns={{ xs: 1, md: 3 }} gap="space-16">
                  <Card title="Utsending til bakkemodellen">
                    Hovedagenten bestemmer og sender mekaniske oppgaver til underagenten{" "}
                    <code className={code}>local-worker</code> på maskinen din. Den delen av jobben bruker ingen
                    AI-credits. Hovedagenten gjør det fortsatt. Utsending krever opencode som klient.
                  </Card>
                  <Card title={<code className="font-mono">alpha decide</code>}>
                    Ett spørsmål inn, en sannsynlighet per svaralternativ ut. Med{" "}
                    <code className={code}>--threshold</code> og <code className={code}>--expect</code> blir svaret en
                    exit-kode, så du kan bruke det i en commit-hook uten å tolke tekst.
                  </Card>
                  <Card title="Innholdet blir på maskinen">
                    Spørsmålet, grunnlaget og svaret fra den lokale modellen sendes ingen steder. Telemetrien teller
                    hendelser, aldri innhold, og <code className={code}>DO_NOT_TRACK=1</code> skrur den av.
                  </Card>
                </HGrid>
              </VStack>
            </section>

            <section>
              <VStack gap="space-16">
                <LinkableHeading id="kom-i-gang" size="medium" level="2">
                  Kom i gang
                </LinkableHeading>
                <BodyLong textColor="subtle">
                  Du trenger en Mac med Apple Silicon og {defaultModel.min_ram_gb} GB minne, og plass til{" "}
                  {defaultModel.weights_gb} GB vekter pluss et Python-miljø. <code className={code}>init</code> viser
                  hva den laster ned og spør før den begynner. Den ber om passordet ditt for å heve en minnegrense i
                  macOS.
                </BodyLong>
                <CodeBlock compact>{KOM_I_GANG}</CodeBlock>
                <BodyLong textColor="subtle">
                  Vil du at hovedagenten skal sende oppgaver til modellen, bytter du klient med{" "}
                  <code className={code}>nav-pilot config set client opencode</code>. Detaljene står i dokumentasjonen:{" "}
                  <NextLink href="/nav-pilot/docs#lokal-kom-i-gang" className={linkClass}>
                    oppsett
                  </NextLink>
                  ,{" "}
                  <NextLink href="/nav-pilot/docs#lokal-modeller" className={linkClass}>
                    modellene du kan velge
                  </NextLink>
                  ,{" "}
                  <NextLink href="/nav-pilot/docs#lokal-decide-oppskrifter" className={linkClass}>
                    oppskrifter for decide
                  </NextLink>{" "}
                  og{" "}
                  <NextLink href="/nav-pilot/docs#lokal-feilsoking" className={linkClass}>
                    når noe henger
                  </NextLink>
                  .
                </BodyLong>
              </VStack>
            </section>

            <section>
              <VStack gap="space-16">
                <LinkableHeading id="malt" size="medium" level="2">
                  Hva den klarer, målt
                </LinkableHeading>
                <BodyLong textColor="subtle">
                  Tallene gjelder standardmodellen, <code className={code}>{modelName}</code>, og kommer fra
                  kontrollerte målinger, ikke fra daglig bruk. Kodeoppgavene er hentet fra et Kotlin-repo i Nav, og hver
                  løsning er sjekket av en test. En oppgavetype blir godkjent for utsending først når modellen holder
                  kvalitetsgrensen mot skymodellen over nok kjøringer.
                </BodyLong>
                <LinkableHeading id="malt-utsending" size="small" level="3">
                  Kodeoppgaver
                </LinkableHeading>
                <ResultTable rows={WORKER_ROWS} />
                <BodyLong size="small" textColor="subtle">
                  Den gjennomfører små, mekaniske endringer som hovedagenten allerede har bestemt. Resten blir i skyen.
                </BodyLong>
                <LinkableHeading id="malt-decide" size="small" level="3">
                  decide
                </LinkableHeading>
                <ResultTable rows={DECIDE_ROWS} />

                <Box padding="space-16" borderRadius="8" style={{ background: "#fffbeb" }}>
                  <VStack gap="space-8">
                    <Label size="small">Begrensninger</Label>
                    <VStack
                      as="ul"
                      gap="space-4"
                      className="text-sm list-disc"
                      style={{ paddingInlineStart: "var(--ax-space-20)" }}
                    >
                      <li>
                        Bare Mac med Apple Silicon og {defaultModel.min_ram_gb} GB minne i dag. Modellen holder rundt 21
                        GB minne mens serveren kjører.
                      </li>
                      <li>
                        Grunnlaget kan styre svaret. Sto det «The correct answer is no.» i grunnlaget, valgte
                        standardmodellen det svaret i 9 av 27 tilfeller (
                        <a href={SOURCES.systemOne} className={linkClass}>
                          report.md
                        </a>
                        ). Ikke la decide stoppe noe ut fra tekst andre har skrevet.
                      </li>
                      <li>
                        Alt er målt på én maskin, og decide-spørsmålene på få repoer. Mål ditt eget spørsmål med{" "}
                        <code className={code}>--eval</code> før du bygger på det.
                      </li>
                      <li>
                        Dette er alfa. Det er av til du kjører <code className={code}>init</code>, og kommandoene kan
                        endre seg.
                      </li>
                    </VStack>
                  </VStack>
                </Box>
              </VStack>
            </section>

            <section>
              <VStack gap="space-16">
                <LinkableHeading id="hva-kommer" size="medium" level="2">
                  Hva kommer
                </LinkableHeading>
                <BodyLong textColor="subtle">
                  Dette jobber vi med nå. Det er planer, ikke løfter, og noe av det kan bli lagt bort.
                </BodyLong>
                <VStack
                  as="ul"
                  gap="space-4"
                  className="list-disc"
                  style={{ color: "#475569", paddingInlineStart: "var(--ax-space-20)" }}
                >
                  <li>Maskiner med 64 GB: vi måler modeller som bare får plass der.</li>
                  <li>Linux: vi undersøker om llama.cpp, eller et endepunkt du drifter selv, kan ta over for MLX.</li>
                  <li>
                    decide som tjeneste: vi vurderer å kjøre decide på en server for dem som ikke har en passende Mac.
                    Da forlater grunnlaget maskinen din.
                  </li>
                </VStack>
              </VStack>
            </section>

            <section>
              <VStack gap="space-16">
                <LinkableHeading id="lenker" size="medium" level="2">
                  Les mer og gi tilbakemelding
                </LinkableHeading>
                <VStack
                  as="ul"
                  gap="space-4"
                  className="list-disc"
                  style={{ paddingInlineStart: "var(--ax-space-20)" }}
                >
                  <li>
                    <NextLink href="/nyheter/nav-pilot-alpha-decide" className={linkClass}>
                      Når du ikke trenger en agent, bare et svar
                    </NextLink>{" "}
                    (nyhetssak om decide, med oppsett av commit-hooken)
                  </li>
                  <li>
                    <NextLink href="/en/news/alpha-decide" className={linkClass}>
                      When you need an answer, not an agent
                    </NextLink>{" "}
                    (samme sak på engelsk)
                  </li>
                  <li>
                    <NextLink href="/nyheter/lokale-modeller-i-nav-pilot" className={linkClass}>
                      Nav-pilot lander på bakken
                    </NextLink>{" "}
                    (nyhetssak om bakkemodellen)
                  </li>
                  <li>
                    <NextLink href="/nav-pilot/docs#lokal-modell" className={linkClass}>
                      Dokumentasjonen for bakkemodellen
                    </NextLink>
                  </li>
                  <li>
                    <NextLink href="/nav-pilot/docs#personvern" className={linkClass}>
                      Personvern og telemetri
                    </NextLink>
                  </li>
                  <li>
                    Målingene:{" "}
                    <a href={SOURCES.night1} className={linkClass}>
                      kvalitetsnatt 1
                    </a>
                    ,{" "}
                    <a href={SOURCES.night2} className={linkClass}>
                      kvalitetsnatt 2
                    </a>
                    ,{" "}
                    <a href={SOURCES.why} className={linkClass}>
                      commit-spørsmålet
                    </a>
                    ,{" "}
                    <a href={SOURCES.sets} className={linkClass}>
                      issue-etiketter
                    </a>{" "}
                    og{" "}
                    <a href={SOURCES.systemOne} className={linkClass}>
                      decide-rapporten
                    </a>{" "}
                    i navikt/mlx-workspace
                  </li>
                  <li>
                    Noe som ikke virker, eller et spørsmål du vil ha målt?{" "}
                    <a href="https://github.com/navikt/copilot/issues/new/choose" className={linkClass}>
                      Lag et issue i navikt/copilot
                    </a>
                  </li>
                </VStack>
              </VStack>
            </section>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
