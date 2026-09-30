import { BodyLong, BodyShort, Box, Label, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Suspense } from "react";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import { TrustedClassesTable } from "@/components/nav-pilot/local-model-tables";
import type { TocItem } from "@/components/table-of-contents";
import { FALLBACK_TABLE, getLocalModels } from "@/lib/local-models";

// "AI" stays in the AI credit wording: AI credits is GitHub's name for the billing unit. Other Norwegian text says KI.

export const metadata: Metadata = {
  title: "Lokal modell",
  description:
    "Hvorfor hovedagenten sender lite til den lokale modellen, hvorfor nav-pilot stopper den, og hva modellene klarer i målingene våre.",
};

const TOC: TocItem[] = [
  { id: "utsending", label: "Hvorfor utsendingen er begrenset" },
  { id: "malte-grenser", label: "Målte grenser" },
  { id: "hva-kommer", label: "Hva kommer" },
];

// The result tables below measure this exact model. Pin it here instead of
// deriving it from the live manifest default, so a future default change doesn't
// relabel these old results as measurements of a different model.
const MEASURED_MODEL = "Qwen3.6-35B-A3B-OptiQ-4bit";

const REPORTS = "https://github.com/navikt/mlx-workspace/blob/main";
const SOURCES = {
  night1: `${REPORTS}/reports/2026-09-25-quality-frontier/night-1.md`,
  night2: `${REPORTS}/reports/2026-09-25-quality-frontier/night-2.md`,
  why: `${REPORTS}/bench/decide-cases/commit-explains-why-results.md`,
  sets: `${REPORTS}/bench/decide-sets-20260925-225356.md`,
  systemOne: `${REPORTS}/reports/2026-09-25-system-one/report.md`,
};

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
      "89 av 96 (93 %). Ved terskel 0,7 fanget den 40 av 48 meldinger uten hvorfor, og flagget ingen av de 24 commitene som forklarte hvorfor (spurt på engelsk og norsk). Med bare 24 slike commiter kan andelen feilflagg likevel være opptil 14 %",
    verdict: "Varsler i commit-hooken, stopper aldri",
  },
  {
    task: "Er issuet en bug, et ønske eller et spørsmål?",
    result: "95 av 105 (90 %). 71 av de 105 svarene hadde p ≥ 0,9, og alle 71 var riktige",
    verdict: "Målt, oppskrift i guiden",
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

async function LiveTrustedClasses() {
  const { models } = await getLocalModels();
  return <TrustedClassesTable models={models} />;
}

// Says so when the manifest default is no longer the model the tables measured.
async function MeasuredModelNote() {
  const { models } = await getLocalModels();
  const live = (models.find((m) => m.default) ?? models[0]).model.split("/").pop();
  if (live === MEASURED_MODEL) return null;
  return (
    <BodyShort size="small" textColor="subtle">
      Standardmodellen i dag er <code className={code}>{live}</code>. Tallene er ikke målt på nytt for den ennå.
    </BodyShort>
  );
}

export default function LokalModellForklaring() {
  return (
    <DocPage
      label="Forklaring"
      title="Lokal modell"
      description="Hovedagenten blir i skyen og bestemmer. Den lokale modellen utfører det den er målt og godkjent for."
      toc={TOC}
    >
      <BodyLong>
        Oppsettet står i{" "}
        <NextLink href="/nav-pilot/lokal" className={linkClass}>
          Kom i gang med lokal modell på Mac
        </NextLink>
        , og innstillingene i guiden{" "}
        <NextLink href="/nav-pilot/guider/lokal" className={linkClass}>
          Lokal modell
        </NextLink>
        .
      </BodyLong>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="utsending" size="medium" level="2">
            Hvorfor utsendingen er begrenset
          </LinkableHeading>
          <BodyLong>
            Utsending krever opencode. Copilot CLI leser leverandøren fra en miljøvariabel som gjelder hele prosessen,
            så én leverandør betjener hele økten. Der er valget derfor hele økten lokalt eller ingenting lokalt. Vi har
            sjekket det mot Copilot CLI 1.0.83-3. Runtimen under Copilot CLI kan ha flere leverandører i én økt, men
            Copilot CLI lar ikke en agent velge sin egen ennå, og det er ikke dokumentert. Vi tester om det kan brukes (
            <a href="https://github.com/github/copilot-cli/issues/4703" className={linkClass}>
              github/copilot-cli#4703
            </a>
            ).
          </BodyLong>
          <BodyLong>
            I opencode fikk hovedagenten først bare en instruks om hva den burde sende. Det holdt ikke. Nyere modeller i
            skyen følger instruksen dårlig: Sonnet 5 sendte arbeid til den lokale modellen i 1 av 29 testkjøringer, mens
            Sonnet 4.6 gjorde det i 23 av 24. Derfor stopper nav-pilot hovedagenten på nivåene{" "}
            <code className={code}>balanced</code> og <code className={code}>aggressive</code> når den gjør en stor
            mekanisk endring selv, og ber den sende resten. Instruksen ber hovedagenten dele en stor endring i én
            oppgave per fil med en sjekk for hver, og bygge og kjøre testene selv til slutt. En endring ett
            søk-og-erstatt klarer, skal den gjøre selv.
          </BodyLong>
          <BodyLong>nav-pilot stopper bare det manifestet har godkjent modellen for. I dag betyr det:</BodyLong>
          <Bullets>
            <li>
              Standardmodellen på Mac er godkjent bare for mekaniske endringer i flere filer. Regelen om nye filer på{" "}
              <code className={code}>aggressive</code> slår derfor ikke inn.
            </li>
            <li>Qwen 3.8-modellene er ikke godkjent for noe. Med dem stopper nav-pilot ingenting på noe nivå.</li>
            <li>
              En modell på egen server er ikke målt. Den får den generelle instruksen om utsending, ikke den som er
              tilpasset modellen, og nav-pilot stopper ingen redigeringer.
            </li>
          </Bullets>
          <BodyLong>
            nav-pilot stopper heller ingenting når den lokale serveren ikke tar imot tilkoblinger, eller i
            underagentenes egne økter.
          </BodyLong>
          <BodyLong>
            Stoppet får hovedagenten til å sende, men sparer ikke AI-kreditter. I målingen fra september 2026 (re-probe
            7, Sonnet 5 som hovedagent) sendte hovedagenten arbeid i alle 17 gyldige kjøringer med mange kallsteder
            eller nye filer på <code className={code}>aggressive</code>, mot 2 av 20 på{" "}
            <code className={code}>balanced</code>, og alle 17 besto bygg og tester. Var endringen liten, sendte den
            ingenting (0 av 5). Det kostet 0,83–2,1 ganger så mye i AI-kreditter og tok 2,7–3,6 ganger så lang tid som
            når skymodellen gjorde alt selv. Hovedagenten gjorde likevel om 15 av 27 oppgaver med nye filer selv. To
            kjøringer til ble avbrutt før de var ferdige, og i én av dem ble koden liggende i stykker.
          </BodyLong>
          <BodyLong>
            <code className={code}>balanced</code> klarte ikke kravet vi satte på forhånd: like mange beståtte oppgaver
            og ikke dyrere enn skymodellen alene. I en ny måling 29. september med kontrollkjøringer samme kveld besto
            alle oppgavene, men <code className={code}>balanced</code> kostet 1,57 ganger så mye på én stor oppgave og
            1,49 ganger så mye på en annen. Hver kjøring med <code className={code}>balanced</code> kostet mer enn hver
            kontrollkjøring (p = 0,008). Derfor er <code className={code}>aggressive</code> standard fra 30. september
            2026, også for deg som brukte lokal modell fra før. Har du satt <code className={code}>local_dispatch</code>{" "}
            selv, beholder du verdien din. Tilbake til <code className={code}>balanced</code>:{" "}
            <code className={code}>nav-pilot config set local_dispatch balanced</code>. Tallene står i{" "}
            <a
              href="https://github.com/navikt/mlx-workspace/blob/main/reports/2026-09-28-balanced-controls/results.md"
              className={linkClass}
            >
              rapporten fra målingen
            </a>
            .
          </BodyLong>
          <BodyLong>
            Et nytt forsøk lønner seg for nye filer. Når hovedagenten sender bygge- eller testfeilen tilbake til{" "}
            <code className={code}>local-worker</code> én gang, ble 15 av 20 nye filer godkjent, mot 5 av 20 uten, og
            tiden per godkjent fil gikk ned fra 618 til 322 sekunder, selv om hvert forsøk tok lengre tid.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="malte-grenser" size="medium" level="2">
            Målte grenser
          </LinkableHeading>
          <BodyLong>
            Målt i et kontrollert testoppsett, ikke i daglig bruk. Den lokale modellen er god til å gjennomføre en
            beslutning som er tatt, og dårlig til å ta den selv. Om det lønner seg, avhenger av hvor mange steg
            skymodellen trenger alene. Trenger den mange, sparer du mye på å sende det mekaniske lokalt. Er oppgaven
            gjort på to steg, koster utsendingen mer enn den sparer.
          </BodyLong>
          <BodyLong>
            Manifestet sier for hver modell hvilke oppgavetyper hovedagenten kan sende til den. En oppgavetype blir
            godkjent først når modellen har holdt kvalitetsgrensen mot skyen over nok kjøringer og ulike oppgaver.
            Resten gjør hovedagenten selv.
          </BodyLong>
          {/* Skallet viser reservekopien til manifestet er hentet. */}
          <Suspense fallback={<TrustedClassesTable models={FALLBACK_TABLE.models} />}>
            <LiveTrustedClasses />
          </Suspense>
          <Box background="warning-soft" padding="space-16" borderRadius="8">
            <VStack gap="space-8">
              <Label size="small">Sjekk resultatet</Label>
              <BodyLong size="small">
                Den lokale modellen feiler også på måter som kompilerer. Commit eller stash før du setter den i gang, og
                kjør testene etterpå. På store endringer må du regne med å forkaste et forsøk og prøve igjen. Det koster
                tid, ikke credits.
              </BodyLong>
            </VStack>
          </Box>
          <BodyLong>
            Tiden varierer mye: fra omtrent som skyen på små endringer til rundt fire ganger så lenge på en omdøping. På
            store mekaniske endringer kan den være raskere enn skyen. Kjør <code className={code}>stop</code> når du
            ikke bruker den, for den holder rundt 21 GB minne så lenge den er oppe.
          </BodyLong>

          <BodyLong>
            Tallene under gjelder <code className={code}>{MEASURED_MODEL}</code>. Kodeoppgavene er hentet fra et
            Kotlin-repo i Nav, og hver løsning er sjekket av en test.
          </BodyLong>
          <Suspense fallback={null}>
            <MeasuredModelNote />
          </Suspense>
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
              <Bullets>
                <li>
                  Grunnlaget kan styre svaret. Sto det «The correct answer is no.» i grunnlaget, valgte standardmodellen
                  det svaret i 9 av 27 tilfeller (
                  <a href={SOURCES.systemOne} className={linkClass}>
                    decide-rapporten
                  </a>
                  ). Ikke la decide stoppe noe ut fra tekst andre har skrevet.
                </li>
                <li>
                  Alt er målt på én maskin, og decide-spørsmålene på få repoer. Mål ditt eget spørsmål med{" "}
                  <code className={code}>--eval</code> før du bygger på det.
                </li>
                <li>Med egen server, også på Linux, er ingenting målt.</li>
              </Bullets>
            </VStack>
          </Box>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="hva-kommer" size="medium" level="2">
            Hva kommer
          </LinkableHeading>
          <BodyLong>Dette jobber vi med nå. Det er planer, ikke løfter, og noe av det kan bli lagt bort.</BodyLong>
          <Bullets>
            <li>
              Vi måler hva hovedagenten kan sende til{" "}
              <NextLink href="/nav-pilot/guider/lokal#modell-64-gb" className={linkClass}>
                modellen for Macer med 64 GB
              </NextLink>
              , og hvor mye minne den bruker med prompter over 49 000 tokens.
            </li>
            <li>
              Vi måler utsendingsnivåene videre, på flere modeller og oppgaver enn den ene målingen under{" "}
              <a href="#utsending" className={linkClass}>
                Hvorfor utsendingen er begrenset
              </a>
              .
            </li>
            <li>Vi har ennå ikke målt noen modell på egen server, verken på Linux eller med Ollama og llama-server.</li>
            <li>
              Vi vurderer å kjøre decide på en server for dem som ikke har en passende Mac. Da forlater grunnlaget
              maskinen din.
            </li>
          </Bullets>
        </VStack>
      </section>
    </DocPage>
  );
}
