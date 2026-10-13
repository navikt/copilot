"use client";

// Client component because Aksel's compound components (Table.Row and friends)
// are undefined when read off a client reference in a server component.
import NextLink from "next/link";
import { useEffect, useState } from "react";
import {
  Alert,
  BodyLong,
  BodyShort,
  Box,
  Chips,
  Heading,
  Link,
  ReadMore,
  Select,
  Switch,
  Table,
  Tabs,
  Tag,
  VStack,
} from "@navikt/ds-react";
import { BackToTop } from "@/components/back-to-top";
import { PageHero } from "@/components/page-hero";
import { TableOfContents, type TocItem } from "@/components/table-of-contents";
import {
  creditsPerSolved,
  knownCredits,
  passAll,
  passInterval,
  modelName,
  passRate,
  runsBySuite,
  sourceUrl,
  type Effort,
  type GoldenRun,
  type GoldenSummary,
  type Suite,
} from "@/lib/golden-baselines";
import { NAV_PILOT_DEFAULT_MODEL, NAV_PILOT_MODEL_CHOICES, RECOMMENDED_DAILY_MODEL } from "@/lib/model-policy";
import { ModelPricingTables } from "../priser/model-pricing-tables";
import { PassRateChart } from "./pass-rate-chart";

const SLACK_URL = "https://nav-it.slack.com/archives/C055TNXBM17";
const MODELLVALG_DOC = "https://github.com/navikt/copilot/blob/main/docs/modellvalg.md";
const BASELINES_DIR = "https://github.com/navikt/copilot/tree/main/docs/golden-baselines";

// Tab order: the everyday tasks first, the narrow agents last.
const SUITE_NAMES: Record<Suite, string> = {
  coding: "Koding",
  norsk: "Norsk tekst",
  planning: "Planlegging",
  research: "Research",
  review: "Kodegjennomgang",
  kafka: "Kafka",
  rust: "Rust",
};
const SUITE_ORDER = Object.keys(SUITE_NAMES);
const suiteName = (suite: string) => SUITE_NAMES[suite as Suite] ?? suite;
const sortedSuites = (runs: GoldenRun[]) =>
  runsBySuite(runs).sort(([a], [b]) => SUITE_ORDER.indexOf(a) - SUITE_ORDER.indexOf(b));

const dateFormat = new Intl.DateTimeFormat("nb-NO", {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});
/** «30. september 2026»; a date the formatter cannot read is shown as it came. */
const formatDate = (iso: string) => {
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? iso : dateFormat.format(date);
};

const effortName = (effort: string) => (effort === "default" ? "standard" : effort);

/** «45 s» or «1 min 23 s». */
export const formatDuration = (seconds: number) => {
  const s = Math.round(seconds);
  return s < 60 ? `${s} s` : `${Math.floor(s / 60)} min ${s % 60} s`;
};

const suiteId = (suite: string) => `maling-${suite}`;
const H2 = "scroll-mt-16";

const cell = { paddingBlock: "var(--ax-space-8)" };

/**
 * Splits runs into the newest of each measurement and the older ones. A measurement is the same
 * model, effort and set of checks; runs from the same newest day are all kept. Pass one suite's runs.
 */
/**
 * Runs that cover only part of what another shown run of the same model and effort covered,
 * like a rerun of rv1–rv2 next to a full review. They are marked, not merged.
 */
export function partialRuns(runs: GoldenRun[]): Set<GoldenRun> {
  const ids = (run: GoldenRun) => new Set(run.checks.map((c) => c.id));
  return new Set(
    runs.filter((run) => {
      if (run.smoke) return false;
      const mine = ids(run);
      return runs.some(
        (other) =>
          other !== run &&
          !other.smoke &&
          other.model === run.model &&
          other.effort === run.effort &&
          other.checks.length > mine.size &&
          [...mine].every((id) => ids(other).has(id))
      );
    })
  );
}

export function latestRuns(runs: GoldenRun[]): { latest: GoldenRun[]; older: GoldenRun[] } {
  const key = (run: GoldenRun) =>
    [
      run.model,
      run.effort,
      !!run.smoke,
      run.checks
        .map((c) => c.id)
        .sort()
        .join(","),
    ].join("|");
  const newest = new Map<string, string>();
  for (const run of runs) if ((newest.get(key(run)) ?? "") < run.date) newest.set(key(run), run.date);
  return {
    latest: runs.filter((run) => newest.get(key(run)) === run.date),
    older: runs.filter((run) => newest.get(key(run)) !== run.date),
  };
}

function Choices({ users }: { users: Record<string, string[]> }) {
  return (
    <div className="w-full overflow-x-auto">
      <Table size="small">
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell scope="col">Oppgave</Table.HeaderCell>
            <Table.HeaderCell scope="col">Brukes av</Table.HeaderCell>
            <Table.HeaderCell scope="col">Modell</Table.HeaderCell>
            <Table.HeaderCell scope="col">Reserve</Table.HeaderCell>
            <Table.HeaderCell scope="col">Hvorfor</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {NAV_PILOT_MODEL_CHOICES.map((choice) => (
            <Table.Row key={choice.purpose}>
              <Table.DataCell style={cell}>{choice.purpose}</Table.DataCell>
              <Table.DataCell style={cell}>{users[choice.purpose]?.join(", ")}</Table.DataCell>
              <Table.DataCell style={cell} className="font-medium whitespace-nowrap">
                {choice.primary}
              </Table.DataCell>
              <Table.DataCell style={cell}>
                {/* One model per line, so a name never breaks in the middle. */}
                {choice.fallbacks.length
                  ? choice.fallbacks.map((model) => (
                      <span key={model} className="block whitespace-nowrap">
                        {model}
                      </span>
                    ))
                  : "Ingen"}
              </Table.DataCell>
              <Table.DataCell style={cell}>
                {choice.reason}
                {choice.source && (
                  <>
                    {" "}
                    <Link href={choice.source} target="_blank" rel="noopener noreferrer">
                      Se målingen
                    </Link>
                  </>
                )}
              </Table.DataCell>
            </Table.Row>
          ))}
        </Table.Body>
      </Table>
    </div>
  );
}

const choiceFor = (purpose: string) => NAV_PILOT_MODEL_CHOICES.find((c) => c.purpose.startsWith(purpose))!.primary;

/** Task → model → one line. The models come from model-policy, so this cannot drift from «Våre valg». */
const QUICK: { task: string; model: string; why: string }[] = [
  {
    task: "Daglig koding",
    model: `${RECOMMENDED_DAILY_MODEL.model} (${RECOMMENDED_DAILY_MODEL.effort})`,
    why: "Besto alle sjekkene i koding, norsk og research.",
  },
  {
    task: "@nav-pilot",
    model: NAV_PILOT_DEFAULT_MODEL,
    why: "Målt på flere planleggingsoppgaver. Agentpakken bruker den som standard.",
  },
  {
    task: "Kodegjennomgang, sikkerhet og personopplysninger",
    model: `${choiceFor("Høyrisiko")} (Low)`,
    why: "Fant oftest feilene i nais.yaml, SQL og tilgangskontroll.",
  },
  {
    task: "Små, avgrensede rettinger og research",
    model: choiceFor("Research"),
    why: "Billig. Ikke til sikkerhetsgjennomgang.",
  },
  {
    task: "Norsk tekst, Aksel og tilgjengelighet",
    model: choiceFor("Aksel, tilgjengelighet"),
    why: "Besto 24 av 25 tilgjengelighetssjekker og 20 av 20 norsksjekker.",
  },
];

/** A link to a benchmark tab. It fires hashchange itself, so it works even when the hash is already set. */
function SuiteLink({ suite, children }: { suite: string; children: string }) {
  return (
    <Link
      href={`#${suiteId(suite)}`}
      onClick={(e) => {
        e.preventDefault();
        window.history.replaceState(null, "", `#${suiteId(suite)}`);
        window.dispatchEvent(new HashChangeEvent("hashchange"));
        document.getElementById(suiteId(suite))?.scrollIntoView({ behavior: "smooth", block: "start" });
      }}
    >
      {children}
    </Link>
  );
}

function KeyPoints() {
  return (
    <VStack gap="space-16">
      <BodyLong>
        Vi velger modell etter hva oppgaven krever, hva den koster og hvordan modellen gjør det i våre egne målinger.
        Hvem som lager modellen, spiller ingen rolle.
      </BodyLong>
      <div className="w-full overflow-x-auto">
        <Table size="small">
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell scope="col">Oppgave</Table.HeaderCell>
              <Table.HeaderCell scope="col">Bruk</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {QUICK.map((row) => (
              <Table.Row key={row.task}>
                <Table.HeaderCell scope="row">{row.task}</Table.HeaderCell>
                {/* Reason under the model keeps the table two columns wide on a phone. */}
                <Table.DataCell className="hyphens-auto">
                  <BodyShort weight="semibold">{row.model}</BodyShort>
                  <BodyShort size="small" textColor="subtle">
                    {row.why}
                  </BodyShort>
                </Table.DataCell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table>
      </div>
      <Alert variant="warning" size="small">
        <Heading size="xsmall" level="3" spacing>
          Vær forsiktig med personopplysninger i GPT-6.1 Sol
        </Heading>
        <BodyLong size="small">
          I en pilot med tre kjøringer 12. oktober 2026 la den to ganger fødselsnummer i en Kafka-melding uten å spørre
          om personvern først. GPT-6 Sol stoppet før den endret koden i alle fem kjøringene, og spurte i fire av dem.
          Tre kjøringer er for få til å si hvor ofte dette skjer, og anbefalingen for daglig koding står.{" "}
          <Link href={`${BASELINES_DIR}/2026-10-12-pin-pilot-sol61`} target="_blank" rel="noopener noreferrer">
            Se piloten
          </Link>
          .
        </BodyLong>
      </Alert>
      <VStack as="ul" gap="space-8" className="list-disc" style={{ paddingInlineStart: "var(--ax-space-20)" }}>
        <li>
          <BodyShort as="span">
            GPT-6 Luna og Claude Haiku 5.5 besto alle sjekkene i kodeoppgavene for under 2 credits per kjøring, mot
            rundt 25 for GPT-6.1 Sol. <SuiteLink suite="coding">Se Koding</SuiteLink>
          </BodyShort>
        </li>
        <li>
          <BodyShort as="span">
            Claude Opus 5.5 fant flest av de plantede sikkerhetsfeilene i kodegjennomgangen 7. oktober.{" "}
            <SuiteLink suite="review">Se Kodegjennomgang</SuiteLink>
          </BodyShort>
        </li>
        <li>
          <BodyShort as="span">
            I koding besto alle innsatsnivåene alle sjekkene, så høyere nivå ga bare høyere pris.{" "}
            <Link href="#innsatsniva">Se Innsatsnivå</Link>
          </BodyShort>
        </li>
      </VStack>
    </VStack>
  );
}

/** The pinned and recommended models we have measured, plus the cheapest plotted one. */
function defaultModels(summary: GoldenSummary | null): string[] {
  const measured = new Set(summary?.runs.map((run) => modelName(run.model)));
  const picked = new Set(
    [...NAV_PILOT_MODEL_CHOICES.map((c) => c.primary), RECOMMENDED_DAILY_MODEL.model].filter((m) => measured.has(m))
  );
  const cheapest = summary?.runs
    .filter((run) => !run.smoke && knownCredits(run) !== null)
    .sort((a, b) => knownCredits(a)! - knownCredits(b)!)[0];
  if (cheapest) picked.add(modelName(cheapest.model));
  return [...picked];
}

const toggle = <T,>(list: T[], item: T) => (list.includes(item) ? list.filter((x) => x !== item) : [...list, item]);

interface View {
  frontier: boolean;
  ci: boolean;
  moreColumns: boolean;
  highlight: string | null;
  xRange: [number, number];
}

function RunRows({ runs, moreColumns, partial }: { runs: GoldenRun[]; moreColumns: boolean; partial: Set<GoldenRun> }) {
  return (
    <>
      {[...runs]
        .sort((a, b) => Number(!!a.smoke) - Number(!!b.smoke))
        .map((run) => {
          const credits = knownCredits(run);
          const perSolved = creditsPerSolved(run);
          const [low, high] = passInterval(run);
          return (
            <Table.Row key={`${run.model}-${run.effort}-${run.date}-${run.source}`}>
              <Table.DataCell>
                <span className="whitespace-nowrap">
                  {modelName(run.model)}
                  {run.smoke && " (benchmark)"}
                  {run.model_verified === false && " (ikke bekreftet)"}
                </span>
                {partial.has(run) && (
                  <BodyShort size="small" textColor="subtle">
                    <Tag size="xsmall" variant="neutral">
                      delvis testsett
                    </Tag>{" "}
                    Bare {run.checks.map((c) => c.id).join(", ")}
                  </BodyShort>
                )}
                {!!run.subagent_models?.length && (
                  <BodyShort size="small" textColor="subtle">
                    Subagenter brukte også: {run.subagent_models.map(modelName).join(", ")}
                  </BodyShort>
                )}
              </Table.DataCell>
              <Table.DataCell>
                {effortName(run.effort)}
                {run.ran_at && run.ran_at !== run.effort && ` (kjørte på ${effortName(run.ran_at)})`}
              </Table.DataCell>
              <Table.DataCell align="right">{Math.round(passRate(run) * 100)} %</Table.DataCell>
              <Table.DataCell align="right" className="whitespace-nowrap">
                {Math.round(low * 100)}–{Math.round(high * 100)} %
              </Table.DataCell>
              <Table.DataCell align="right">
                {perSolved !== null
                  ? perSolved.toLocaleString("nb-NO", { maximumFractionDigits: 1 })
                  : credits !== null
                    ? "Ingen løst"
                    : "–"}
              </Table.DataCell>
              <Table.DataCell className="whitespace-nowrap">{formatDate(run.date)}</Table.DataCell>
              {moreColumns && (
                <>
                  <Table.DataCell align="right">{Math.round(passAll(run) * 100)} %</Table.DataCell>
                  <Table.DataCell align="right">
                    {credits === null ? "–" : credits.toLocaleString("nb-NO")}
                  </Table.DataCell>
                  <Table.DataCell align="right" className="whitespace-nowrap">
                    {formatDuration(run.wall_seconds.median)}
                  </Table.DataCell>
                  <Table.DataCell align="right">{run.n}</Table.DataCell>
                  <Table.DataCell>{run.cli_version}</Table.DataCell>
                  <Table.DataCell>
                    <Link href={sourceUrl(run.source)} target="_blank" rel="noopener noreferrer">
                      {run.source.split("/").pop()}
                    </Link>
                  </Table.DataCell>
                </>
              )}
            </Table.Row>
          );
        })}
    </>
  );
}

function SuitePanel({ suite, runs, view, colors }: { suite: Suite; runs: GoldenRun[]; view: View; colors: string[] }) {
  const [showOlder, setShowOlder] = useState(false);
  if (runs.length === 0) return <BodyShort>Ingen målinger passer filteret.</BodyShort>;
  const { latest, older } = latestRuns(runs);
  const partial = partialRuns(showOlder ? runs : latest);
  const tableRuns = showOlder ? [...latest, ...older] : latest;
  // Smoke runs check the harness, not the model: table only, after the real runs.
  const plotted = latest.filter((run) => !run.smoke && knownCredits(run) !== null);
  // Every real row in the table, not just the plotted ones, so an uncharted failure is not hidden.
  const saturated = plotted.length > 1 && latest.every((run) => run.smoke || passRate(run) === 1);
  const unverified = tableRuns.some((run) => !run.smoke && run.model_verified === false);
  const smoke = tableRuns.some((run) => run.smoke);
  const incomplete = tableRuns.some((run) => !run.smoke && run.model_verified !== false && knownCredits(run) === null);
  const name = suiteName(suite);
  return (
    <VStack gap="space-12">
      {saturated && (
        <BodyShort weight="semibold">
          Alle modellene besto alle sjekkene; diagrammet skiller dem bare på pris.
        </BodyShort>
      )}
      {plotted.length > 0 && (
        <PassRateChart
          title={`${name}: andel bestått mot median credits`}
          models={colors}
          frontier={view.frontier}
          ci={view.ci}
          xRange={view.xRange}
          highlight={view.highlight}
          points={plotted.map((run) => ({
            model: modelName(run.model),
            effort: run.effort,
            credits: knownCredits(run)!,
            passPercent: passRate(run) * 100,
            low: passInterval(run)[0] * 100,
            high: passInterval(run)[1] * 100,
            n: run.n,
            partial: partial.has(run),
          }))}
        />
      )}
      <div className="w-full overflow-x-auto">
        <Table size="small">
          <caption className="sr-only">{name}: målinger per modell og innsatsnivå</caption>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell scope="col">Modell</Table.HeaderCell>
              <Table.HeaderCell scope="col">Innsatsnivå</Table.HeaderCell>
              <Table.HeaderCell scope="col" align="right">
                Bestått
              </Table.HeaderCell>
              <Table.HeaderCell scope="col" align="right">
                95 %-intervall
              </Table.HeaderCell>
              <Table.HeaderCell scope="col" align="right">
                Credits per løste oppgave
              </Table.HeaderCell>
              <Table.HeaderCell scope="col">Dato</Table.HeaderCell>
              {view.moreColumns && (
                <>
                  <Table.HeaderCell scope="col" align="right">
                    Besto hver gang
                  </Table.HeaderCell>
                  <Table.HeaderCell scope="col" align="right">
                    Median credits
                  </Table.HeaderCell>
                  <Table.HeaderCell scope="col" align="right">
                    Median tid
                  </Table.HeaderCell>
                  <Table.HeaderCell scope="col" align="right">
                    n
                  </Table.HeaderCell>
                  <Table.HeaderCell scope="col">CLI</Table.HeaderCell>
                  <Table.HeaderCell scope="col">Kilde</Table.HeaderCell>
                </>
              )}
            </Table.Row>
          </Table.Header>
          <Table.Body>
            <RunRows runs={tableRuns} moreColumns={view.moreColumns} partial={partial} />
          </Table.Body>
        </Table>
      </div>
      {older.length > 0 && (
        <Switch size="small" checked={showOlder} onChange={(e) => setShowOlder(e.target.checked)}>
          {`Vis tidligere målinger (${older.length})`}
        </Switch>
      )}
      {incomplete && (
        <BodyShort size="small">
          Strek (–) betyr at forbruket ikke ble registrert for alle kall. Kjøringen er derfor ikke med i diagrammet.
        </BodyShort>
      )}
      {smoke && (
        <BodyShort size="small">
          En benchmark med én kjøring sjekker at testoppsettet virker. Den sier ikke noe om modellen og er ikke med i
          diagrammet.
        </BodyShort>
      )}
      {unverified && (
        <BodyShort size="small">
          «Ikke bekreftet» betyr at kjøringen manglet forbruksdata, så vi vet ikke sikkert hvilken modell som svarte.
          Kjøringen er derfor ikke med i diagrammet.
        </BodyShort>
      )}
    </VStack>
  );
}

function Measurements({ summary }: { summary: GoldenSummary | null }) {
  const runs = summary?.runs ?? [];
  const suites = sortedSuites(runs);
  const allModels = [...new Set(runs.map((run) => modelName(run.model)))];
  const present = new Set(runs.map((run) => run.effort));
  const allEfforts = (["low", "medium", "high", "default"] as Effort[]).filter((e) => present.has(e));
  const [shownModels, setShownModels] = useState(() => defaultModels(summary));
  const [shownEfforts, setShownEfforts] = useState(allEfforts);
  const [frontier, setFrontier] = useState(false);
  const [ci, setCi] = useState(false);
  const [moreColumns, setMoreColumns] = useState(false);
  const [highlight, setHighlight] = useState<string | null>(null);
  const [tab, setTab] = useState<string>(suites[0]?.[0] ?? "");

  // The tab lives in the hash, so the old #maling-<suite> links open their tab.
  useEffect(() => {
    const sync = () => {
      const hash = window.location.hash.slice(1);
      const suite = hash.slice("maling-".length);
      if (hash.startsWith("maling-") && suites.some(([s]) => s === suite)) setTab(suite);
    };
    sync();
    window.addEventListener("hashchange", sync);
    return () => window.removeEventListener("hashchange", sync);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- the suites are fixed for the page's life
  }, []);

  if (!summary) {
    return (
      <Alert variant="info" size="small">
        Ingen målinger publisert ennå.
      </Alert>
    );
  }
  // Colour follows the model across every chart on the page.
  // Only plotted runs take a palette slot, so the fixed order is not spent on models no chart shows.
  const plottedAll = summary.runs.filter((run) => !run.smoke && knownCredits(run) !== null);
  const colors = [...new Set(plottedAll.map((run) => modelName(run.model)))];
  const credits = plottedAll.map((run) => knownCredits(run)!);
  // One x-scale for every suite, in whole decades so the end points sit clear of the edges.
  const xRange: [number, number] = [
    10 ** Math.floor(Math.log10(Math.min(...credits))),
    10 ** Math.ceil(Math.log10(Math.max(...credits))),
  ];
  const shown = summary.runs.filter(
    (run) => shownModels.includes(modelName(run.model)) && shownEfforts.includes(run.effort)
  );
  const view: View = { frontier, ci, moreColumns, highlight, xRange };
  const selectTab = (value: string) => {
    setTab(value);
    window.history.replaceState(null, "", `#${suiteId(value)}`);
  };

  return (
    <VStack gap="space-20">
      <BodyLong>
        Vi kjører de samme oppgavene flere ganger for hver modell og hvert innsatsnivå. Få kjøringer holder til å finne
        tydelige feil, men ikke til å rangere modellene. Sist oppdatert {formatDate(summary.generated)}.
      </BodyLong>
      <ReadMore header="Slik leser du tallene" size="small">
        <VStack as="ul" gap="space-4" className="list-disc" style={{ paddingInlineStart: "var(--ax-space-20)" }}>
          <li>n er hvor mange ganger vi kjørte samme oppgave.</li>
          <li>
            95 %-intervallet viser hvor den sanne andelen bestått trolig ligger: 5 av 5 bestått gir et intervall fra 57
            til 100 %.
          </li>
          <li>Credits per løste oppgave er median credits delt på andelen bestått.</li>
          <li>«Besto hver gang» er andelen sjekker som besto i alle n kjøringene.</li>
          <li>
            Tabellene viser den nyeste målingen for hver modell og hvert innsatsnivå. Eldre målinger kan du slå på.
          </li>
          <li>
            «Delvis testsett» betyr at kjøringen bare tok noen av sjekkene som en annen kjøring av samme modell og nivå
            tok. Den vises med hul markør i diagrammet og står utenfor linjen til modellen.
          </li>
        </VStack>
      </ReadMore>

      <Box padding="space-16" borderRadius="8" borderWidth="1" borderColor="neutral-subtle" background="neutral-soft">
        <VStack gap="space-12">
          <BodyShort size="small" weight="semibold">
            Filter for alle målingene
          </BodyShort>
          <div role="group" aria-label="Modeller">
            <BodyShort size="small" textColor="subtle" style={{ marginBottom: "var(--ax-space-4)" }} aria-hidden>
              Modeller
            </BodyShort>
            <Chips size="small">
              {allModels.map((model) => (
                <Chips.Toggle
                  key={model}
                  selected={shownModels.includes(model)}
                  onClick={() => setShownModels(toggle(shownModels, model))}
                >
                  {model}
                </Chips.Toggle>
              ))}
            </Chips>
          </div>
          <div role="group" aria-label="Innsatsnivå">
            <BodyShort size="small" textColor="subtle" style={{ marginBottom: "var(--ax-space-4)" }} aria-hidden>
              Innsatsnivå
            </BodyShort>
            <Chips size="small">
              {allEfforts.map((effort) => (
                <Chips.Toggle
                  key={effort}
                  selected={shownEfforts.includes(effort)}
                  onClick={() => setShownEfforts(toggle(shownEfforts, effort))}
                >
                  {effortName(effort)}
                </Chips.Toggle>
              ))}
            </Chips>
          </div>
          <ReadMore header="Visning" size="small">
            <VStack gap="space-8">
              <Select
                label="Fremhev modell"
                size="small"
                className="max-w-xs"
                value={highlight ?? ""}
                onChange={(e) => setHighlight(e.target.value || null)}
              >
                <option value="">Ingen</option>
                {colors
                  .filter((model) => shownModels.includes(model))
                  .map((model) => (
                    <option key={model} value={model}>
                      {model}
                    </option>
                  ))}
              </Select>
              <Switch size="small" checked={frontier} onChange={(e) => setFrontier(e.target.checked)}>
                Vis beste avveining mellom bestått og pris
              </Switch>
              <Switch size="small" checked={ci} onChange={(e) => setCi(e.target.checked)}>
                Vis 95 %-intervall for alle modeller
              </Switch>
              <Switch size="small" checked={moreColumns} onChange={(e) => setMoreColumns(e.target.checked)}>
                Vis flere kolonner
              </Switch>
            </VStack>
          </ReadMore>
        </VStack>
      </Box>

      {/* Anchors for the old per-suite links; the hash selects the tab. */}
      <div>
        {suites.map(([suite]) => (
          // Only the open tab's anchor takes up layout, so the side menu marks the right suite.
          <span key={suite} id={suiteId(suite)} className={H2} hidden={suite !== tab} />
        ))}
        <Tabs value={tab} onChange={selectTab} size="small">
          <Tabs.List>
            {suites.map(([suite]) => (
              <Tabs.Tab key={suite} value={suite} label={suiteName(suite)} />
            ))}
          </Tabs.List>
          {suites.map(([suite, all]) => (
            <Tabs.Panel key={suite} value={suite} style={{ paddingTop: "var(--ax-space-16)" }}>
              {tab === suite && (
                <SuitePanel suite={suite} runs={all.filter((run) => shown.includes(run))} view={view} colors={colors} />
              )}
            </Tabs.Panel>
          ))}
        </Tabs>
      </div>
    </VStack>
  );
}

const EFFORT_ROWS = [
  ["Low", "Godt avgrensede oppgaver som er lette å kontrollere, som faste maler, søk og subagenter."],
  ["Medium", "Vanlig agentisk koding med tydelig omfang."],
  ["High", "Endringer på tvers av moduler og lange oppgaver."],
];

const EFFORT_SOURCES = [
  { href: "https://arxiv.org/abs/2412.21187", label: "Chen mfl. 2024: overtenkning på enkle oppgaver" },
  { href: "https://arxiv.org/abs/2502.07266", label: "Wu mfl. 2025: når lengre resonnering gir dårligere svar" },
  { href: "https://arxiv.org/abs/2502.08235", label: "Cuadron mfl. 2025: overtenkning i agentoppgaver" },
  { href: "https://arxiv.org/abs/2609.26777", label: "SWE-Serve 2026: effort mot kostnad på lange kodeoppgaver" },
  {
    href: "https://platform.claude.com/docs/en/build-with-claude/effort",
    label: "Anthropic: leverandørens egne råd om effort",
  },
];

function EffortSection() {
  return (
    <VStack gap="space-12">
      <BodyLong>
        Effort styrer hvor mye arbeid modellen legger i svaret: hvor mye den resonnerer, hvor mange verktøykall den gjør
        og hvor langt den svarer. Mer er ikke alltid bedre. På enkle oppgaver bruker modellene ofte mange flere tokens
        uten å bli mer treffsikre, og for lang resonnering kan gi dårligere svar. På lange kodeoppgaver løfter høyere
        effort andelen bestått noe, men gevinsten er liten øverst: fra high til max ga ett prosentpoeng til 76 prosent
        høyere kostnad.
      </BodyLong>
      <BodyLong>
        Våre egne målinger sier det samme. På kodeoppgavene besto GPT-6 Sol (30. september) og GPT-6.1 Sol (30.
        september og 12. oktober 2026) alle sjekkene både på Low og på High, men High kostet 8–9 prosent mer. På
        planlegging 30. september var High dårligere: GPT-6 Sol besto 14 av 20 sjekker på High mot 18 av 20 på Low, og
        GPT-6.1 Sol 10 av 20 mot 15 av 20. I kodegjennomgang fant Claude Opus 5.5 alle de plantede feilene på både Low
        og High, men High kostet 60 prosent mer. Rådata ligger i{" "}
        <Link href={`${BASELINES_DIR}/2026-09-30-batch1`} target="_blank" rel="noopener noreferrer">
          målingen 30. september
        </Link>{" "}
        og{" "}
        <Link href={`${BASELINES_DIR}/2026-10-12-effort-sol61`} target="_blank" rel="noopener noreferrer">
          målingen 12. oktober
        </Link>
        . Rådene under bygger på kildene og på disse målingene.
      </BodyLong>
      <div className="w-full overflow-x-auto">
        <Table size="small">
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell scope="col">Innsatsnivå (effort)</Table.HeaderCell>
              <Table.HeaderCell scope="col">Når du bør bruke det</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {EFFORT_ROWS.map(([level, when]) => (
              <Table.Row key={level}>
                <Table.HeaderCell scope="row">{level}</Table.HeaderCell>
                <Table.DataCell>{when}</Table.DataCell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table>
      </div>
      <BodyShort size="small">Kilder:</BodyShort>
      <ul className="list-disc text-sm" style={{ paddingInlineStart: "var(--ax-space-20)" }}>
        {EFFORT_SOURCES.map((source) => (
          <li key={source.href}>
            <Link href={source.href} target="_blank" rel="noopener noreferrer">
              {source.label}
            </Link>
          </li>
        ))}
      </ul>
    </VStack>
  );
}

export function ModellerContent({
  summary,
  users,
}: {
  summary: GoldenSummary | null;
  users: Record<string, string[]>;
}) {
  const selected = [...new Set(NAV_PILOT_MODEL_CHOICES.flatMap((c) => [c.primary, ...c.fallbacks]))];
  const toc: TocItem[] = [
    { id: "kort-fortalt", label: "Kort fortalt" },
    { id: "vare-valg", label: "Våre valg" },
    {
      id: "malinger",
      label: "Målinger",
      children: sortedSuites(summary?.runs ?? []).map(([suite]) => ({ id: suiteId(suite), label: suiteName(suite) })),
    },
    { id: "priser", label: "Priser" },
    { id: "innsatsniva", label: "Innsatsnivå" },
    { id: "del-erfaringer", label: "Del dine erfaringer" },
  ];

  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero title="Modellvalg" description="Hvilken modell agentene og promptene våre bruker, og hvorfor." />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <div className="flex gap-12">
            <div className="min-w-0 flex-1">
              <VStack gap={{ xs: "space-32", md: "space-40" }}>
                <div className="xl:hidden">
                  <ReadMore header="Innhold på siden">
                    <TableOfContents items={toc} title="Innhold" />
                  </ReadMore>
                </div>

                <VStack gap="space-12">
                  <Heading size="medium" level="2" id="kort-fortalt" className={H2}>
                    Kort fortalt
                  </Heading>
                  <KeyPoints />
                </VStack>

                <VStack gap="space-12">
                  <Heading size="medium" level="2" id="vare-valg" className={H2}>
                    Våre valg
                  </Heading>
                  <BodyLong>
                    Bruk {RECOMMENDED_DAILY_MODEL.model} med lav innsats til daglig koding. Den besto alle sjekkene i
                    koding, norsk og informasjonsinnhenting 6. oktober. 12. oktober vurderte vi planleggingstesten og to
                    av kodegjennomgangstestene på nytt med rettede sjekker (kjørt 11. oktober), og den besto fem av fem.
                    Til planlegging anbefaler vi fortsatt {NAV_PILOT_DEFAULT_MODEL}, som er målt på flere
                    planleggingsoppgaver, og @nav-pilot bruker den. Til kodegjennomgang og alt som gjelder sikkerhet og
                    personopplysninger anbefaler vi likevel Claude Opus 5.5 med lav innsats: GPT-6.1 Sol fant
                    sikkerhetsfeilene sjeldnere (7. oktober). Av modellene vi har målt, fant Opus oftest feilene i
                    nais.yaml, SQL og tilgangskontroll, men den koster 1,6 til 2,8 ganger så mye. GPT-6 Luna er billig
                    og god til små, avgrensede rettinger og en rask førstegjennomgang av kode der sikkerhet ikke er
                    tema, men ikke til sikkerhetsgjennomgang.
                  </BodyLong>
                  <BodyLong>
                    Tabellen viser hva agentene og promptene er pinnet til. Modellen står i fila til agenten eller
                    prompten. Reservemodellen er den vi bytter til hvis hovedmodellen svikter. Starter{" "}
                    <code>@nav-pilot</code> en annen agent som subagent, arver den modellen fra <code>@nav-pilot</code>.
                  </BodyLong>
                  <Choices users={users} />
                </VStack>

                <VStack gap="space-12">
                  <Heading size="medium" level="2" id="malinger" className={H2}>
                    Målinger
                  </Heading>
                  <Measurements summary={summary} />
                </VStack>

                <VStack gap="space-12">
                  <Heading size="medium" level="2" id="priser" className={H2}>
                    Priser
                  </Heading>
                  <BodyLong>
                    Listepris per million tokens for modellene over som Nav har aktivert. Hele prislisten står på{" "}
                    <Link as={NextLink} href="/priser">
                      Modellpriser
                    </Link>
                    .
                  </BodyLong>
                  <ModelPricingTables only={selected} />
                </VStack>

                <VStack gap="space-12">
                  <Heading size="medium" level="2" id="innsatsniva" className={H2}>
                    Innsatsnivå (effort)
                  </Heading>
                  <EffortSection />
                </VStack>

                <VStack gap="space-12">
                  <Heading size="medium" level="2" id="del-erfaringer" className={H2}>
                    Del dine erfaringer
                  </Heading>
                  <BodyLong>
                    Har du fått bedre eller dårligere resultater med en annen modell? Si fra i{" "}
                    <Link href={SLACK_URL} target="_blank" rel="noopener noreferrer">
                      #github-copilot på Slack
                    </Link>
                    . Bakgrunnen for valgene står i{" "}
                    <Link href={MODELLVALG_DOC} target="_blank" rel="noopener noreferrer">
                      modellvalg.md
                    </Link>
                    .
                  </BodyLong>
                </VStack>
              </VStack>
            </div>
            <aside className="hidden xl:block w-56 shrink-0">
              <div className="sticky top-6">
                <TableOfContents items={toc} title="Innhold på siden" />
              </div>
            </aside>
          </div>
        </Box>
      </div>
      <BackToTop />
    </main>
  );
}
