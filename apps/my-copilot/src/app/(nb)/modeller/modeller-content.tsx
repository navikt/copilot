"use client";

// Client component because Aksel's compound components (Table.Row and friends)
// are undefined when read off a client reference in a server component.
import NextLink from "next/link";
import { Alert, BodyLong, BodyShort, Box, Heading, Link, Table, VStack } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import {
  knownCredits,
  modelName,
  passRate,
  runsBySuite,
  sourceUrl,
  type GoldenSummary,
  type Suite,
} from "@/lib/golden-baselines";
import { NAV_PILOT_MODEL_CHOICES } from "@/lib/model-policy";
import { ModelPricingTables } from "../priser/model-pricing-tables";
import { PassRateChart } from "./pass-rate-chart";

const SLACK_URL = "https://nav-it.slack.com/archives/C055TNXBM17";
const MODELLVALG_DOC = "https://github.com/navikt/copilot/blob/main/docs/modellvalg.md";

const SUITE_NAMES: Record<Suite, string> = {
  planning: "Planlegging",
  review: "Kodegjennomgang",
  norsk: "Norsk tekst",
  coding: "Koding",
  research: "Research",
};

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

const cell = { paddingBlock: "var(--ax-space-8)" };

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
              <Table.DataCell style={cell}>{choice.fallbacks.join(", ") || "Ingen"}</Table.DataCell>
              <Table.DataCell style={cell}>{choice.reason}</Table.DataCell>
            </Table.Row>
          ))}
        </Table.Body>
      </Table>
    </div>
  );
}

function Measurements({ summary }: { summary: GoldenSummary | null }) {
  if (!summary) {
    return (
      <Alert variant="info" size="small">
        Ingen målinger publisert ennå.
      </Alert>
    );
  }
  // Colour follows the model across every chart on the page.
  // Only plotted runs take a palette slot, so the fixed order is not spent on models no chart shows.
  const models = [
    ...new Set(
      summary.runs.filter((run) => !run.smoke && knownCredits(run) !== null).map((run) => modelName(run.model))
    ),
  ];

  return (
    <VStack gap="space-24">
      <BodyLong>
        Tallet n viser hvor mange ganger vi kjørte samme oppgave. Få kjøringer holder til å finne tydelige feil, men
        ikke til å rangere modellene. Sist oppdatert {formatDate(summary.generated)}.
      </BodyLong>
      {runsBySuite(summary.runs).map(([suite, runs]) => {
        // Smoke runs check the harness, not the model: table only, after the real runs.
        const plotted = runs.filter((run) => !run.smoke && knownCredits(run) !== null);
        const unverified = runs.some((run) => !run.smoke && run.model_verified === false);
        const smoke = runs.some((run) => run.smoke);
        const incomplete = runs.some((run) => !run.smoke && run.model_verified !== false && knownCredits(run) === null);
        return (
          <VStack gap="space-12" key={suite}>
            <Heading size="small" level="3">
              {SUITE_NAMES[suite] ?? suite}
            </Heading>
            {plotted.length > 0 && (
              <PassRateChart
                title={`${SUITE_NAMES[suite] ?? suite}: andel bestått mot median credits`}
                models={models}
                points={plotted.map((run) => ({
                  model: modelName(run.model),
                  effort: run.effort,
                  credits: knownCredits(run)!,
                  passPercent: passRate(run) * 100,
                  n: run.n,
                }))}
              />
            )}
            <div className="w-full overflow-x-auto">
              <Table size="small">
                <Table.Header>
                  <Table.Row>
                    <Table.HeaderCell scope="col">Modell</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Effort</Table.HeaderCell>
                    <Table.HeaderCell scope="col" align="right">
                      Bestått
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
                    <Table.HeaderCell scope="col">Dato</Table.HeaderCell>
                    <Table.HeaderCell scope="col">CLI</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Kilde</Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {[...runs]
                    .sort((a, b) => Number(!!a.smoke) - Number(!!b.smoke))
                    .map((run) => {
                      const credits = knownCredits(run);
                      return (
                        <Table.Row key={`${run.model}-${run.effort}-${run.date}-${run.source}`}>
                          <Table.DataCell>
                            <span className="whitespace-nowrap">
                              {modelName(run.model)}
                              {run.smoke && " (benchmark)"}
                              {run.model_verified === false && " (ikke bekreftet)"}
                            </span>
                            {!!run.subagent_models?.length && (
                              <BodyShort size="small" style={{ color: "var(--ax-text-neutral-subtle)" }}>
                                Subagenter brukte også: {run.subagent_models.map(modelName).join(", ")}
                              </BodyShort>
                            )}
                          </Table.DataCell>
                          <Table.DataCell>
                            {effortName(run.effort)}
                            {run.ran_at && run.ran_at !== run.effort && ` (kjørte på ${effortName(run.ran_at)})`}
                          </Table.DataCell>
                          <Table.DataCell align="right">{Math.round(passRate(run) * 100)} %</Table.DataCell>
                          <Table.DataCell align="right">
                            {credits === null ? "–" : credits.toLocaleString("nb-NO")}
                          </Table.DataCell>
                          <Table.DataCell align="right">{Math.round(run.wall_seconds.median)} s</Table.DataCell>
                          <Table.DataCell align="right">{run.n}</Table.DataCell>
                          <Table.DataCell className="whitespace-nowrap">{formatDate(run.date)}</Table.DataCell>
                          <Table.DataCell>{run.cli_version}</Table.DataCell>
                          <Table.DataCell>
                            <Link href={sourceUrl(run.source)} target="_blank" rel="noopener noreferrer">
                              {run.source.split("/").pop()}
                            </Link>
                          </Table.DataCell>
                        </Table.Row>
                      );
                    })}
                </Table.Body>
              </Table>
            </div>
            {incomplete && (
              <BodyShort size="small">
                Strek (–) betyr at forbruket ikke ble registrert for alle kall. Kjøringen er derfor ikke med i
                diagrammet.
              </BodyShort>
            )}
            {smoke && (
              <BodyShort size="small">
                En benchmark med én kjøring sjekker at testoppsettet virker. Den sier ikke noe om modellen og er ikke
                med i diagrammet.
              </BodyShort>
            )}
            {unverified && (
              <BodyShort size="small">
                «Ikke bekreftet» betyr at kjøringen manglet forbruksdata, så vi vet ikke sikkert hvilken modell som
                svarte. Kjøringen er derfor ikke med i diagrammet.
              </BodyShort>
            )}
          </VStack>
        );
      })}
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

function Effort() {
  return (
    <VStack gap="space-12">
      <BodyLong>
        Effort styrer hvor mye arbeid modellen legger i svaret: hvor mye den resonnerer, hvor mange verktøykall den gjør
        og hvor langt den svarer. Mer er ikke alltid bedre. På enkle oppgaver bruker modellene ofte mange flere tokens
        uten å bli mer treffsikre, og for lang resonnering kan gi dårligere svar. På lange kodeoppgaver løfter høyere
        effort andelen bestått noe, men gevinsten er liten øverst: fra high til max ga ett prosentpoeng til 76 prosent
        høyere kostnad. Vi har ikke målt effort selv ennå. Rådene under bygger på kildene.
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

  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero title="Modellvalg" description="Hvilken modell agentene og promptene våre bruker, og hvorfor." />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap={{ xs: "space-32", md: "space-40" }}>
            <VStack gap="space-12">
              <Heading size="medium" level="2">
                Kort fortalt
              </Heading>
              <BodyLong>
                Vi velger modell etter hva oppgaven krever, hva den koster og hvordan modellen gjør det i våre egne
                målinger. Hvem som lager modellen, spiller ingen rolle.
              </BodyLong>
              <ul className="list-disc" style={{ paddingInlineStart: "var(--ax-space-20)" }}>
                {NAV_PILOT_MODEL_CHOICES.map((choice) => (
                  <li key={choice.purpose}>
                    <BodyShort as="span">
                      {choice.purpose}: <strong>{choice.primary}</strong>
                    </BodyShort>
                  </li>
                ))}
              </ul>
              <BodyLong>
                Bruk GPT-6 Sol med lav innsats til daglig koding og planlegging. Den besto alle sjekkene i koding, norsk
                og research 6. oktober og stoppet riktig etter fase 1 i fem av fem planleggingskjøringer. Bruk Claude
                Opus 5.5 med lav innsats til kodegjennomgang og alt som gjelder sikkerhet og personopplysninger. Av
                modellene vi har målt, fant den oftest feilene i nais.yaml, SQL og tilgangskontroll, men den koster
                omtrent dobbelt så mye. GPT-6.1 Sol er like god som GPT-6 Sol på koding, norsk og research, men svakere
                på kodegjennomgang og skriver de åpne punktene i fase 1 som påstander, ikke spørsmål. Vi anbefaler den
                derfor ikke som standard. GPT-6 Luna er billig og god til små, avgrensede rettinger og en rask
                førstegjennomgang, men ikke til sikkerhet.
              </BodyLong>
            </VStack>

            <VStack gap="space-12">
              <Heading size="medium" level="2">
                Våre valg
              </Heading>
              <BodyLong>
                Modellen står i fila til agenten eller prompten. Reservemodellen er den vi bytter til hvis hovedmodellen
                svikter. Starter <code>@nav-pilot</code> en annen agent som subagent, arver den modellen fra{" "}
                <code>@nav-pilot</code>.
              </BodyLong>
              <Choices users={users} />
            </VStack>

            <VStack gap="space-12">
              <Heading size="medium" level="2">
                Målinger
              </Heading>
              <Measurements summary={summary} />
            </VStack>

            <VStack gap="space-12">
              <Heading size="medium" level="2">
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
              <Heading size="medium" level="2">
                Innsatsnivå (effort)
              </Heading>
              <Effort />
            </VStack>

            <VStack gap="space-12">
              <Heading size="medium" level="2">
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
        </Box>
      </div>
    </main>
  );
}
