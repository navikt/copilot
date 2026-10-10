// Measured results for the local model, shown on /innsikt/lokale-modeller.
// Each table carries the date it was measured and a link to the report in
// navikt/mlx-workspace. All rows measure MEASURED_MODEL.

export const MEASURED_MODEL = "Qwen3.6-35B-A3B-OptiQ-4bit";

export const MLX_WORKSPACE = "https://github.com/navikt/mlx-workspace/blob/main";

export const SOURCES = {
  reprobe7: `${MLX_WORKSPACE}/reports/2026-09-23-local-model-evaluation/pending-tasks.md#88-the-64-gb-tier-a-worker-directed-by-a-cloud-orchestrator-downloaded-profiles-in-place`,
  balanced: `${MLX_WORKSPACE}/reports/2026-09-28-balanced-controls/results.md`,
  followups1: `${MLX_WORKSPACE}/reports/2026-09-25-quality-frontier/night-followups-1.md`,
  night1: `${MLX_WORKSPACE}/reports/2026-09-25-quality-frontier/night-1.md`,
  night2: `${MLX_WORKSPACE}/reports/2026-09-25-quality-frontier/night-2.md`,
  why: `${MLX_WORKSPACE}/bench/decide-cases/commit-explains-why-results.md`,
  sets: `${MLX_WORKSPACE}/bench/decide-sets-20260925-225356.md`,
  layout: `${MLX_WORKSPACE}/bench/decide-layout-results.md`,
  systemOne: `${MLX_WORKSPACE}/reports/2026-09-25-system-one/report.md`,
  gb64: `${MLX_WORKSPACE}/reports/2026-09-26-64gb-tier/night-64-4.md#review-2026-09-28`,
  readme: `${MLX_WORKSPACE}/reports/README.md`,
  template: `${MLX_WORKSPACE}/reports/TEMPLATE.md`,
  unmeasured: `${MLX_WORKSPACE}/reports/UNMEASURED.md`,
  cfRetry2: `${MLX_WORKSPACE}/reports/2026-10-08-cf-retry2/report.md`,
  phaseC: `${MLX_WORKSPACE}/reports/2026-10-01-phase-c-rerun/report.md`,
  emm8: `${MLX_WORKSPACE}/reports/2026-09-30-emm8-delegate/report.md`,
  smallDelegate: `${MLX_WORKSPACE}/reports/2026-10-10-small-delegate/report.md`,
  prAudit: `${MLX_WORKSPACE}/reports/2026-10-09-navikt-pr-audit/report.md`,
  gapFill: `${MLX_WORKSPACE}/reports/2026-10-10-gap-fill/report.md`,
  k2Ifm: `${MLX_WORKSPACE}/reports/2026-10-10-k2-ifm/report.md`,
  newCandidates: `${MLX_WORKSPACE}/reports/2026-10-09-new-candidates/report.md`,
  costRule: "https://github.com/navikt/mlx-workspace/pull/171",
  benchmarking: `${MLX_WORKSPACE}/BENCHMARKING.md#when-we-benchmark`,
};

export type ResultRow = { task: string; result: string; verdict: string };
export type ResultSet = { measured: string; source: string; rows: ResultRow[] };

export const GB64_MEASURED = { measured: "2026-09-27", source: SOURCES.gb64 };

// Dispatch re-probe 7 (2026-09-28) and the same-day controls for balanced (2026-09-29).
export const DELEGATION_RESULTS: ResultSet = {
  measured: "2026-09-29",
  source: SOURCES.reprobe7,
  rows: [
    {
      task: "aggressive",
      result:
        "Delegerte i 17 av 17 gyldige kjøringer med mange kall eller nye filer, og alle 17 besto bygg og tester. Kostet 0,83–2,1 ganger så mye i AI-kreditter og tok 2,7–3,6 ganger så lang tid som skymodellen alene.",
      verdict: "Standard fra 30. september 2026",
    },
    {
      task: "balanced",
      result:
        "Delegerte i 2 av 20 kjøringer. I kontrollmålingen 29. september besto alle oppgavene, men hver kjøring kostet 1,4–1,6 ganger så mye som skymodellen alene og tok 1,6–2,5 ganger så lang tid.",
      verdict: "Dyrere uten å delegere",
    },
    {
      task: "Bare en instruks",
      result:
        "Sonnet 5 delegerte i 1 av 29 kjøringer, uansett hvordan instruksen var skrevet. Sonnet 4.6 delegerte i 23 av 24 med en eldre instruks.",
      verdict: "Virker ikke med Sonnet 5",
    },
  ],
};

/**
 * Cost and time per delegation level as multiples of the cloud model alone, low–high
 * across the measured cells. The prose in DELEGATION_RESULTS rounds these numbers.
 * aggressive: dispatch re-probe 7, AI credits. balanced: same-day controls, cloud cost
 * per cell (r4 1.57×, r6 1.49×, small 1.39×) and median time (2.5×, 1.6×, 1.7×).
 */
export type DelegationRange = {
  level: string;
  cost: [number, number];
  time: [number, number];
  measured: string;
  source: string;
};

export const DELEGATION_RANGES: DelegationRange[] = [
  { level: "aggressive", cost: [0.83, 2.1], time: [2.7, 3.6], measured: "2026-09-28", source: SOURCES.reprobe7 },
  { level: "balanced", cost: [1.39, 1.57], time: [1.6, 2.5], measured: "2026-09-29", source: SOURCES.balanced },
];

export const WORKER_RESULTS: ResultSet = {
  measured: "2026-09-26",
  source: SOURCES.night2,
  rows: [
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
      task: "Lage en ny fil, med retry2",
      result:
        "9. oktober 2026: 10 av 10 på hvert av de to letteste trinnene, både med standardmodellen og 8-bitsmodellen. Trinn 3 ga også 10 av 10, men er ikke avgjort ennå. Trinn 4 ga 7 og 5 av 10, og alle feilene var tidsavbrudd. Uten retry2 ga samme oppsett 26 av 40 10. oktober 2026.",
      verdict: "Godkjent lokalt på de to letteste trinnene",
    },
    { task: "Svare på spørsmål om kodebasen", result: "18 av 40 (skymodellen: 40 av 40)", verdict: "Blir i skyen" },
  ],
};

export const DECIDE_RESULTS: ResultSet = {
  measured: "2026-09-25",
  source: SOURCES.systemOne,
  rows: [
    {
      task: "Forklarer commit-meldingen hvorfor?",
      result:
        "89 av 96 (93 %). Ved terskel 0,7 fanget den 40 av 48 kall om meldinger uten hvorfor og flagget ingen av 48 kall om meldinger som forklarte hvorfor. Hver gruppe er 24 meldinger, spurt på engelsk og norsk. Med så få meldinger kan andelen feilflagg likevel være opptil 14 %.",
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
  ],
};

/** "2026-09-26" → "26. september 2026". A full timestamp keeps its time, so the Oslo date is right. */
export function formatDate(iso: string): string {
  return new Date(iso.length > 10 ? iso : `${iso}T12:00:00Z`).toLocaleDateString("nb-NO", {
    day: "numeric",
    month: "long",
    year: "numeric",
    timeZone: "Europe/Oslo",
  });
}
