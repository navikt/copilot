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
              <Table.DataCell style={cell}>{users[choice.primary]?.join(", ")}</Table.DataCell>
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
  const models = [...new Set(summary.runs.map((run) => modelName(run.model)))];

  return (
    <VStack gap="space-24">
      <BodyLong>
        Tallet n viser hvor mange ganger vi kjørte samme oppgave. Fem kjøringer holder til å finne tydelige feil, men
        ikke til å rangere modellene generelt. Sist oppdatert {summary.generated}.
      </BodyLong>
      {runsBySuite(summary.runs).map(([suite, runs]) => {
        // Smoke runs check the harness, not the model: table only, after the real runs.
        const plotted = runs.filter((run) => !run.smoke && knownCredits(run) !== null);
        const unplotted = runs.filter((run) => !run.smoke && knownCredits(run) === null).length;
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
                          <Table.DataCell className="whitespace-nowrap">
                            {modelName(run.model)}
                            {run.smoke && " (røyktest)"}
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
                          <Table.DataCell className="whitespace-nowrap">{run.date}</Table.DataCell>
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
            {unplotted > 0 && (
              <BodyShort size="small">
                – betyr at forbruket ikke ble registrert helt. Kjøringen er derfor ikke med i diagrammet.
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
  ["Medium", "Vanlig agentisk koding med tydelig omfang. Standardvalget."],
  ["High", "Endringer på tvers av moduler, sikkerhet og lange oppgaver uten tilsyn."],
];

const EFFORT_SOURCES = [
  { href: "https://arxiv.org/abs/2412.21187", label: "Chen mfl. 2024: overtenkning på enkle oppgaver" },
  { href: "https://arxiv.org/abs/2502.07266", label: "Wu mfl. 2025: når lengre resonnering gir dårligere svar" },
  { href: "https://arxiv.org/abs/2502.08235", label: "Cuadron mfl. 2025: overtenkning i agentoppgaver" },
  { href: "https://arxiv.org/abs/2609.26777", label: "SWE-Serve 2026: effort på lange kodeoppgaver" },
  {
    href: "https://platform.claude.com/docs/en/build-with-claude/effort",
    label: "Anthropic: råd om effort (leverandørens egne råd)",
  },
];

function Effort() {
  return (
    <VStack gap="space-12">
      <BodyLong>
        Effort styrer hvor mye modellen resonnerer før den svarer. Mer er ikke alltid bedre. På enkle oppgaver bruker
        modellene ofte mange flere tokens uten å bli mer treffsikre, og for lang resonnering kan gi dårligere svar. På
        lange og krevende kodeoppgaver gir høyere effort derimot bedre resultater. Vi har ikke målt effort selv ennå, så
        rådene under bygger på kildene.
      </BodyLong>
      <div className="w-full overflow-x-auto">
        <Table size="small">
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell scope="col">Effort</Table.HeaderCell>
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
                målinger. Hvilken leverandør som lager modellen, avgjør ikke.
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
                Velger du modell selv, gjelder samme regel: Bruk den billigste modellen som løser oppgaven, og velg en
                sterkere modell når oppgaven er krevende eller feil er dyre.
              </BodyLong>
            </VStack>

            <VStack gap="space-12">
              <Heading size="medium" level="2">
                Våre valg
              </Heading>
              <BodyLong>
                Modellen står i agentens eller promptens egen fil. Reservemodellen er den vi bytter til hvis
                hovedmodellen svikter. Starter <code>@nav-pilot</code> en annen agent som subagent, arver den modellen
                fra <code>@nav-pilot</code>.
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
                Listepris per million tokens for modellene over. Hele prislisten står på{" "}
                <Link as={NextLink} href="/priser">
                  Modellpriser
                </Link>
                .
              </BodyLong>
              <ModelPricingTables only={selected} />
            </VStack>

            <VStack gap="space-12">
              <Heading size="medium" level="2">
                Effort
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
