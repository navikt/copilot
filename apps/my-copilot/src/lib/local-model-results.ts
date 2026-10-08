// Measured results for the local model, shown on /innsikt/lokale-modeller.
// Each table carries the date it was measured and a link to the report in
// navikt/mlx-workspace. All rows measure MEASURED_MODEL.

export const MEASURED_MODEL = "Qwen3.6-35B-A3B-OptiQ-4bit";

export const MLX_WORKSPACE = "https://github.com/navikt/mlx-workspace/blob/main";

export const SOURCES = {
  balanced: `${MLX_WORKSPACE}/reports/2026-09-28-balanced-controls/results.md`,
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
};

export type ResultRow = { task: string; result: string; verdict: string };
export type ResultSet = { measured: string; source: string; rows: ResultRow[] };

export const DELEGATION_MEASURED = { measured: "2026-09-29", source: SOURCES.balanced };
export const GB64_MEASURED = { measured: "2026-09-27", source: SOURCES.gb64 };

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
      task: "Lage en ny fil",
      result: "5 av 16 på første forsøk, 12 av 16 med nye forsøk, men dobbelt så lang tid",
      verdict: "Ikke avgjort",
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
  ],
};

/** "2026-09-26" → "26. september 2026". */
export function formatDate(iso: string): string {
  return new Date(`${iso.slice(0, 10)}T12:00:00Z`).toLocaleDateString("nb-NO", {
    day: "numeric",
    month: "long",
    year: "numeric",
    timeZone: "Europe/Oslo",
  });
}
