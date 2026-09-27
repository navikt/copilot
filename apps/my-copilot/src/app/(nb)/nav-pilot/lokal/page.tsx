import { BodyLong, BodyShort, Box, Heading, HGrid, Label, Tag, VStack } from "@navikt/ds-react";
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
    "Kjør en kodemodell på din egen Mac, eller på en server du kjører selv, for enkle oppgaver. alpha decide gir raske, typede svar i hooks og skript.",
};

const linkClass = "text-blue-600 hover:underline";
const code = "font-mono text-xs";

// The result tables and reports below measure this exact model. Pin it here instead of
// deriving it from the live manifest default, so a future default change doesn't relabel
// these old results as measurements of a different model. See SOURCES below.
const MEASURED_MODEL = "Qwen3.6-35B-A3B-OptiQ-4bit";

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

const EGEN_SERVER = `nav-pilot alpha local setup    # finner serveren, foreslår en modell og sjekker den
nav-pilot alpha local doctor   # sjekker serveren på nytt: verktøykall, logprobs, kontekst og svartid`;

type Row = { task: string; result: string; verdict: string };

const WORKER_ROWS: Row[] = [
  {
    task: "Legge til et påkrevd argument i 1–2 kall, i flere filer",
    result: "10 av 10 (skymodellen: 7 av 10)",
    verdict: "Godkjent for utsending",
  },
  {
    task: "Det samme i 3–8 kall",
    result: "9 av 10 på både 3–4 og 5–8 kall",
    verdict: "Ikke avgjort, trenger flere kjøringer",
  },
  {
    task: "Det samme i 9 kall eller flere",
    result: "6–8 av 10 første natt, 14 av 16 andre natt. Under grensen begge netter",
    verdict: "Blir i skyen",
  },
  {
    task: "Endre én fil, de to letteste trinnene",
    result: "13 og 10 av 16 på første forsøk, 16 av 16 med inntil to nye forsøk",
    verdict: "Nye forsøk hjelper, bekreftet på ett trinn, men ikke godkjent for utsending ennå",
  },
  {
    task: "Lage en ny fil",
    result: "5 av 16 på første forsøk, 12 av 16 med nye forsøk, men dobbelt så lang tid",
    verdict: "Ikke avgjort",
  },
  {
    task: "Svare på spørsmål om kodebasen",
    result: "18 av 40 (skymodellen: 40 av 40)",
    verdict: "Blir i skyen",
  },
];

const DECIDE_ROWS: Row[] = [
  {
    task: "Forklarer commit-meldingen hvorfor?",
    result:
      "89 av 96 (93 %). Ved terskel 0,7 fanget den 40 av 48 svar om meldinger uten hvorfor, og flagget ingen av de 24 commitene som forklarte hvorfor (spurt på engelsk og norsk). Så få tilfeller gir opptil 14 % feilflagg",
    verdict: "Varsler i commit-hooken, stopper aldri",
  },
  {
    task: "Er issuet en bug, et ønske eller et spørsmål?",
    result: "95 av 105 (90 %). 71 av de 105 svarene hadde p ≥ 0,9, og alle 71 var riktige",
    verdict: "Målt, oppskrift i dokumentasjonen",
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
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((r) => (
            <TableRow key={r.task}>
              <TableDataCell>{r.task}</TableDataCell>
              <TableDataCell>{r.result}</TableDataCell>
              <TableDataCell>{r.verdict}</TableDataCell>
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
        <Heading level="3" size="xsmall">
          {title}
        </Heading>
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
  const liveModelName = defaultModel.model.split("/").pop();
  const liveModelDiffers = liveModelName !== MEASURED_MODEL;

  return (
    <main>
      <PageHero
        title="Lokal modell og decide"
        description="Macen din kjører en kodemodell for enkle oppgaver. Filene den leser og skriver, blir på maskinen, og modellen bruker ingen AI-credits."
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
                  nav-pilot kan kjøre en kodemodell på din egen Mac, eller bruke en server du kjører selv, for eksempel
                  Ollama på Linux. Hovedagenten i skyen planlegger og sender avgrensede oppgaver til den, for eksempel å
                  føre et nytt argument gjennom et par kall. Med <code className={code}>nav-pilot alpha decide</code>{" "}
                  stiller du den samme modellen et flervalgsspørsmål fra en hook eller et skript, og får svar på under
                  et halvt sekund.
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
                    AI-credits. Hovedagenten gjør det fortsatt. Utsending krever opencode som klient, og nav-pilot
                    legger inn underagenten selv. Sonnet 5 sender sjelden noe av seg selv, se{" "}
                    <a href="#utsending" className={linkClass}>
                      hvor mye hovedagenten sender
                    </a>
                    .
                  </Card>
                  <Card title={<code className="font-mono">alpha decide</code>}>
                    Ett spørsmål inn, en sannsynlighet per svaralternativ ut. Med{" "}
                    <code className={code}>--threshold</code> og <code className={code}>--expect</code> blir svaret en
                    exit-kode, så du kan bruke det i en commit-hook uten å tolke tekst.
                  </Card>
                  <Card title="Innholdet blir på maskinen">
                    Spørsmålet og grunnlaget du gir decide, forlater ikke maskinen, eller serveren du selv har pekt
                    nav-pilot på. Ved utsending ser hovedagenten i skyen oppgaven den selv skrev, og bakkemodellens
                    korte svar. nav-pilots telemetri teller hendelser, ikke innhold, og{" "}
                    <code className={code}>DO_NOT_TRACK=1</code> skrur av både den og målingene nav-pilot slår på i
                    Copilot og opencode.
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
                  macOS. Etter en omstart av maskinen spør <code className={code}>start</code> før den hever grensen
                  igjen.
                </BodyLong>
                <CodeBlock compact>{KOM_I_GANG}</CodeBlock>
                <LinkableHeading id="egen-server" size="small" level="3">
                  Linux eller egen server
                </LinkableHeading>
                <BodyLong textColor="subtle">
                  Har du Linux, eller vil du bruke Ollama, llama-server, LM Studio eller vLLM, starter du serveren selv
                  og kjører <code className={code}>setup</code>. Den finner serveren, foreslår en modell og sjekker den.
                  Mangler modellen i Ollama, eller er konteksten for liten, spør den før den laster ned eller retter
                  noe. nav-pilot godtar bare servere på localhost eller en privat IP-adresse.{" "}
                  <code className={code}>decide</code> trenger logprobs, og det gir ikke LM Studio.
                </BodyLong>
                <CodeBlock compact>{EGEN_SERVER}</CodeBlock>
                <BodyShort size="small" textColor="subtle">
                  Denne veien er alfa, og vi har ikke målt noen modell der. Tallene lenger ned gjelder ikke.
                </BodyShort>
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
                  <NextLink href="/nav-pilot/docs#lokal-egen-server" className={linkClass}>
                    egen server
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
                <LinkableHeading id="utsending" size="medium" level="2">
                  Hvor mye hovedagenten sender
                </LinkableHeading>
                <BodyLong textColor="subtle">
                  Du styrer det med <code className={code}>nav-pilot config set local_dispatch &lt;nivå&gt;</code>,
                  eller med <code className={code}>--local-dispatch &lt;nivå&gt;</code> for én økt. Nivåene er{" "}
                  <code className={code}>off</code>, <code className={code}>conservative</code>,{" "}
                  <code className={code}>balanced</code> (standard) og <code className={code}>aggressive</code>. Det
                  virker bare i opencode, fordi Copilot CLI ikke har noen underagent.
                </BodyLong>
                <BodyLong textColor="subtle">
                  Nivåene kom fordi instruksen alene ikke virket. I testene våre sendte Sonnet 5 arbeid til
                  bakkemodellen i 1 av 29 kjøringer. Sonnet 4.6 gjorde det i 23 av 24 i august. På{" "}
                  <code className={code}>balanced</code> og <code className={code}>aggressive</code> stopper nav-pilot
                  derfor hovedagenten når den gjør en stor mekanisk endring selv, for eksempel når den redigerer en
                  femte fil i samme tur, og ber den sende resten til <code className={code}>local-worker</code>.
                </BodyLong>
                <BodyLong textColor="subtle">Hva det betyr i dag, avhenger av modellen:</BodyLong>
                <VStack
                  as="ul"
                  gap="space-4"
                  className="list-disc"
                  style={{ color: "var(--ax-text-neutral-subtle)", paddingInlineStart: "var(--ax-space-20)" }}
                >
                  <li>
                    Standardmodellen på Mac er godkjent bare for mekaniske endringer i flere filer. På{" "}
                    <code className={code}>balanced</code> stopper nav-pilot hovedagenten én gang per tur, og samme
                    redigering går gjennom andre gang. På <code className={code}>aggressive</code> slipper en fil
                    gjennom først når den er sendt til <code className={code}>local-worker</code>. Nye filer stopper
                    ikke, fordi modellen ikke er godkjent for dem.
                  </li>
                  <li>
                    Qwen 3.8-modellene er ikke godkjent for noen oppgavetype. Hovedagenten blir bedt om ikke å sende
                    noe, og nav-pilot stopper ingenting.
                  </li>
                  <li>
                    Med egen server er modellen ikke målt. Hovedagenten får en generell instruks, og nav-pilot stopper
                    ingenting.
                  </li>
                </VStack>
                <BodyLong textColor="subtle">
                  Nivåene er nye, og vi har ikke målt dem. Vi vet ennå ikke om stoppet får Sonnet 5 til å sende mer,
                  eller om det sparer AI-credits. Den ene oppgaven Sonnet 5 sendte i testene, kostet omtrent det samme
                  som da den gjorde oppgaven selv, og tok nesten tre ganger så lang tid. Går stoppet i veien for deg,
                  velg <code className={code}>conservative</code>: da vurderer hovedagenten selv, og nav-pilot stopper
                  ingenting. Svarer ikke den lokale serveren, stopper nav-pilot heller ingenting. Reglene står i{" "}
                  <NextLink href="/nav-pilot/docs#lokal-utsending" className={linkClass}>
                    dokumentasjonen
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
                  Tallene gjelder <code className={code}>{MEASURED_MODEL}</code>, og kommer fra kontrollerte målinger,
                  ikke fra daglig bruk. Kodeoppgavene er hentet fra et Kotlin-repo i Nav, og hver løsning er sjekket av
                  en test. En oppgavetype blir godkjent for utsending først når modellen holder kvalitetsgrensen mot
                  skymodellen over nok kjøringer.
                </BodyLong>
                {liveModelDiffers && (
                  <BodyShort size="small" textColor="subtle">
                    Standardmodellen i dag er <code className={code}>{liveModelName}</code>. Tallene over er ikke målt
                    på nytt for den ennå.
                  </BodyShort>
                )}
                <LinkableHeading id="malt-utsending" size="small" level="3">
                  Kodeoppgaver
                </LinkableHeading>
                <ResultTable rows={WORKER_ROWS} />
                <BodyShort size="small" textColor="subtle">
                  Kilde:{" "}
                  <a href={SOURCES.night1} className={linkClass}>
                    kvalitetsnatt 1
                  </a>{" "}
                  og{" "}
                  <a href={SOURCES.night2} className={linkClass}>
                    kvalitetsnatt 2
                  </a>{" "}
                  i navikt/mlx-workspace.
                </BodyShort>
                <BodyLong size="small" textColor="subtle">
                  Den gjennomfører små, mekaniske endringer som hovedagenten allerede har bestemt. Resten blir i skyen.
                </BodyLong>
                <LinkableHeading id="malt-decide" size="small" level="3">
                  decide
                </LinkableHeading>
                <ResultTable rows={DECIDE_ROWS} />
                <BodyShort size="small" textColor="subtle">
                  Svartid per kall på varm server: median 0,4 sekunder, målt på én maskin (M5 Max). Kilde:{" "}
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
                  </a>
                  .
                </BodyShort>

                <Box background="warning-soft" padding="space-16" borderRadius="8">
                  <VStack gap="space-8">
                    <Label size="small">Begrensninger</Label>
                    <VStack
                      as="ul"
                      gap="space-4"
                      className="text-sm list-disc"
                      style={{ paddingInlineStart: "var(--ax-space-20)" }}
                    >
                      <li>
                        Modellen nav-pilot setter opp selv, krever en Mac med Apple Silicon og {defaultModel.min_ram_gb}{" "}
                        GB minne, og holder rundt 21 GB minne mens serveren kjører. Med egen server, også på Linux, er
                        ingenting målt.
                      </li>
                      <li>
                        Utsending virker bare i opencode, og Sonnet 5 sender sjelden noe av seg selv. Stoppet på{" "}
                        <code className={code}>balanced</code> er nytt og ikke målt.
                      </li>
                      <li>
                        Grunnlaget kan styre svaret. Sto det «The correct answer is no.» i grunnlaget, valgte
                        standardmodellen det svaret i 9 av 27 tilfeller (
                        <a href={SOURCES.systemOne} className={linkClass}>
                          decide-rapporten
                        </a>
                        ). Ikke la decide stoppe noe ut fra tekst andre har skrevet.
                      </li>
                      <li>
                        Alt er målt på én maskin, og decide-spørsmålene på få repoer. Mål ditt eget spørsmål med{" "}
                        <code className={code}>--eval</code> før du bygger på det.
                      </li>
                      <li>
                        Dette er alfa. Ingenting kjører før du selv kjører <code className={code}>init</code> eller{" "}
                        <code className={code}>setup</code>, og kommandoene kan endre seg.
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
                  style={{ color: "var(--ax-text-neutral-subtle)", paddingInlineStart: "var(--ax-space-20)" }}
                >
                  <li>Maskiner med 64 GB: vi måler modeller som bare får plass der.</li>
                  <li>
                    Utsendingsnivåene: vi måler hvert nivå for å se om stoppet får hovedagenten til å sende, og om det
                    sparer AI-credits.
                  </li>
                  <li>Egen server: vi har ennå ikke målt noen modell på Linux eller via Ollama og llama-server.</li>
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
