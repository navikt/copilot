import { BodyLong, BodyShort, Box, Label, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Suspense } from "react";
import { Table, TableBody, TableRow, TableDataCell } from "@/components/aksel-table";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, HeaderRow, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";
import { getLocalModels } from "@/lib/local-models";

// "AI" stays in the AI credit wording: AI credits is GitHub's name for the billing unit. Other Norwegian text says KI.

export const metadata: Metadata = {
  title: "Målinger av lokal modell",
  description:
    "Tallene bak vurderingene av den lokale modellen: utsendingsnivåene, kodeoppgavene, decide-spørsmålene og modellen for Macer med 64 GB.",
};

const TOC: TocItem[] = [
  { id: "utsendingsnivaer", label: "Utsendingsnivåene" },
  { id: "malt-utsending", label: "Kodeoppgaver" },
  { id: "malt-decide", label: "decide" },
  { id: "modell-64-gb", label: "Modellen for 64 GB" },
];

// The result tables below measure this exact model. Pin it here instead of
// deriving it from the live manifest default, so a future default change doesn't
// relabel these old results as measurements of a different model.
const MEASURED_MODEL = "Qwen3.6-35B-A3B-OptiQ-4bit";

const REPORTS = "https://github.com/navikt/mlx-workspace/blob/main";
const SOURCES = {
  balanced: `${REPORTS}/reports/2026-09-28-balanced-controls/results.md`,
  night1: `${REPORTS}/reports/2026-09-25-quality-frontier/night-1.md`,
  night2: `${REPORTS}/reports/2026-09-25-quality-frontier/night-2.md`,
  why: `${REPORTS}/bench/decide-cases/commit-explains-why-results.md`,
  sets: `${REPORTS}/bench/decide-sets-20260925-225356.md`,
  layout: `${REPORTS}/bench/decide-layout-results.md`,
  systemOne: `${REPORTS}/reports/2026-09-25-system-one/report.md`,
  gb64: `${REPORTS}/reports/2026-09-26-64gb-tier/night-64-4.md#review-2026-09-28`,
};

type Row = { task: string; result: string; verdict: string };

const WORKER_ROWS: Row[] = [
  {
    task: "Legge til et påkrevd argument i 1–2 kall, i flere filer",
    result: "10 av 10 (skymodellen: 7 av 10)",
    verdict: "Godkjent",
  },
  { task: "Det samme i 3–8 kall", result: "9 av 10 på både 3–4 og 5–8 kall", verdict: "Ikke avgjort" },
  {
    task: "Det samme i 9 kall eller flere",
    result: "6–8 av 10 første natt, 14 av 16 andre natt",
    verdict: "Blir i skyen",
  },
  {
    task: "Endre én fil, de to letteste trinnene",
    result: "13 og 10 av 16 på første forsøk, 16 av 16 med inntil to nye forsøk",
    verdict: "Ikke godkjent ennå",
  },
  {
    task: "Lage en ny fil",
    result: "5 av 16 på første forsøk, 12 av 16 med nye forsøk, men dobbelt så lang tid",
    verdict: "Ikke avgjort",
  },
  { task: "Svare på spørsmål om kodebasen", result: "18 av 40 (skymodellen: 40 av 40)", verdict: "Blir i skyen" },
];

const DECIDE_ROWS: Row[] = [
  {
    task: "Forklarer commit-meldingen hvorfor?",
    result:
      "89 av 96 (93 %). Ved terskel 0,7 fanget den 40 av 48 meldinger uten hvorfor og flagget ingen av de 24 som forklarte hvorfor. Med så få kan andelen feilflagg likevel være opptil 14 %.",
    verdict: "Varsler, stopper aldri",
  },
  {
    task: "Er issuet en bug, et ønske eller et spørsmål?",
    result: "95 av 105 (90 %). 71 svar hadde p ≥ 0,9, og alle 71 var riktige.",
    verdict: "Foreslår etikett",
  },
  {
    task: "Forklarer PR-beskrivelsen hvorfor?",
    result: "Slapp gjennom alle 24 som forklarer hvorfor, men fant bare 3 av 12 der grunnen var fjernet.",
    verdict: "Svak, bruk som hint",
  },
];

function ResultTable({ rows }: { rows: Row[] }) {
  return (
    <div className="overflow-x-auto">
      <Table size="small" className="table-stack w-full" role="table">
        <HeaderRow stack cells={["Oppgave", "Resultat", "Vurdering"]} />
        <TableBody role="rowgroup">
          {rows.map((r) => (
            <TableRow role="row" key={r.task}>
              <TableDataCell role="cell">
                <strong>{r.task}</strong>
              </TableDataCell>
              <TableDataCell role="cell" data-label="Resultat">
                {r.result}
              </TableDataCell>
              <TableDataCell role="cell" data-label="Vurdering">
                {r.verdict}
              </TableDataCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
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

export default function LokalModellMalinger() {
  return (
    <DocPage
      label="Forklaring"
      title="Målinger av lokal modell"
      description="Tallene bak vurderingene i Lokal modell, for deg som vil se dem. Alt er målt i et kontrollert testoppsett på én maskin (M5 Max), ikke i daglig bruk."
      toc={TOC}
    >
      <BodyLong>
        Hva tallene betyr for deg, står i{" "}
        <NextLink href="/nav-pilot/forklaring/lokal-modell" className={linkClass}>
          Lokal modell
        </NextLink>
        . Tabellene gjelder <code className={code}>{MEASURED_MODEL}</code>, og kodeoppgavene er hentet fra et
        Kotlin-repo i Nav, der hver løsning er sjekket av en test.
      </BodyLong>
      <Suspense fallback={null}>
        <MeasuredModelNote />
      </Suspense>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="utsendingsnivaer" size="medium" level="2">
            Utsendingsnivåene
          </LinkableHeading>
          <BodyLong>
            Med bare en instruks om hva den burde sende, sendte Sonnet 5 arbeid til den lokale modellen i 1 av 29
            testkjøringer, mot 23 av 24 for Sonnet 4.6. Derfor stopper nav-pilot hovedagenten.
          </BodyLong>
          <BodyLong>Målingen fra september 2026, med Sonnet 5 som hovedagent:</BodyLong>
          <Bullets>
            <li>
              Med <code className={code}>aggressive</code> sendte hovedagenten arbeid i alle 17 gyldige kjøringer med
              mange kallsteder eller nye filer, og alle 17 besto bygg og tester. Med{" "}
              <code className={code}>balanced</code> sendte den i 2 av 20.
            </li>
            <li>Var endringen liten, sendte den ingenting (0 av 5).</li>
            <li>
              Det kostet 0,83–2,1 ganger så mye i AI-kreditter og tok 2,7–3,6 ganger så lang tid som når skymodellen
              gjorde alt selv.
            </li>
            <li>
              Hovedagenten gjorde likevel om 15 av 27 oppgaver med nye filer selv. To kjøringer ble avbrutt, og i én av
              dem ble koden liggende i stykker.
            </li>
          </Bullets>
          <BodyLong>
            Kravet til <code className={code}>balanced</code> var like mange beståtte oppgaver og ikke dyrere enn
            skymodellen alene. I en ny måling 29. september besto alle oppgavene, men{" "}
            <code className={code}>balanced</code> kostet 1,57 og 1,49 ganger så mye på to store oppgaver, og hver
            kjøring var dyrere enn hver kontrollkjøring. Derfor ble <code className={code}>aggressive</code> standard
            30. september 2026 (
            <a href={SOURCES.balanced} className={linkClass}>
              rapport
            </a>
            ).
          </BodyLong>
          <BodyLong>
            Et nytt forsøk lønner seg for nye filer. Når hovedagenten sendte bygge- eller testfeilen tilbake én gang,
            ble 15 av 20 nye filer godkjent, mot 5 av 20 uten. Tiden per godkjent fil gikk ned fra 618 til 322 sekunder.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="malt-utsending" size="medium" level="2">
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
            </a>
            .
          </BodyShort>
          <BodyLong>
            Tiden varierer fra omtrent som skyen på små endringer til rundt fire ganger så lenge på en omdøping. På
            store mekaniske endringer kan den lokale modellen være raskere.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="malt-decide" size="medium" level="2">
            decide
          </LinkableHeading>
          <ResultTable rows={DECIDE_ROWS} />
          <BodyShort size="small" textColor="subtle">
            Svartid per kall på varm server: median 0,4 sekunder. Kilde:{" "}
            <a href={SOURCES.why} className={linkClass}>
              commit-spørsmålet
            </a>
            ,{" "}
            <a href={SOURCES.sets} className={linkClass}>
              issue og PR
            </a>{" "}
            og{" "}
            <a href={SOURCES.systemOne} className={linkClass}>
              decide-rapporten
            </a>
            .
          </BodyShort>
          <BodyLong>
            Hvordan du spør, betyr noe. Standardmodellen svarte riktig på 85 % av spørsmålene i opprinnelig form og 58 %
            når de var snudd (
            <a href={SOURCES.layout} className={linkClass}>
              måling
            </a>
            ). «Ja»-svarene er stabile, men «nei»-svarene vipper mot «teksten er grei».
          </BodyLong>
          <Box background="warning-soft" padding="space-16" borderRadius="8">
            <VStack gap="space-8">
              <Label size="small">Begrensninger</Label>
              <Bullets>
                <li>
                  Grunnlaget kan styre svaret. Sto det «The correct answer is no.» i grunnlaget, valgte standardmodellen
                  det svaret i 9 av 27 tilfeller. Ikke la decide stoppe noe ut fra tekst andre har skrevet.
                </li>
                <li>
                  decide-spørsmålene er målt på få repoer. Mål ditt eget spørsmål med{" "}
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
          <LinkableHeading id="modell-64-gb" size="medium" level="2">
            Modellen for 64 GB
          </LinkableHeading>
          <BodyLong>
            <code className={code}>qwen3.6-35b-a3b-8bit</code>, målt 27. september 2026 (
            <a href={SOURCES.gb64} className={linkClass}>
              rapport
            </a>
            ):
          </BodyLong>
          <Bullets>
            <li>Den løste oppgaven i 12 av 12 Copilot-økter.</li>
            <li>
              Med decide svarte den like godt som standardmodellen: 184 av 218 riktige mot 182, og 91 av 96 mot 89 på
              commit-spørsmålet. På testene av svakheter fikk den 806 av 974 mot 827, mest fordi den var svakere når
              grunnlaget prøvde å styre svaret eller var langt.
            </li>
            <li>
              Minnebruken var på det meste 46,18 GB med en prompt på 49 000 tokens. Grensen er 48 GB. Ved 64 000 tokens
              anslår rapporten rundt 50 GB, altså over grensen. Det er ikke målt ennå.
            </li>
          </Bullets>
        </VStack>
      </section>
    </DocPage>
  );
}
