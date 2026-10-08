import { VStack } from "@navikt/ds-react";
import { Table, TableBody, TableRow, TableDataCell } from "@/components/aksel-table";
import type { Bar, ClassVerdict, LocalModel, RejectedModel } from "@/lib/local-models";
import { HeaderRow } from "@/components/nav-pilot/doc-page";
import type { ResultRow } from "@/lib/local-model-results";

// The local-model tables on /nav-pilot/referanse and
// /nav-pilot/forklaring/lokal-modell. The rows come from the manifest in
// navikt/mlx-workspace when the page runs (src/lib/local-models.ts), with
// src/lib/local-models.json as the fallback. All numbers come from there. Only
// the Norwegian descriptions live here, without numbers. A model without one
// shows the manifest's English role with a visible note.
const LOCAL_MODEL_TEXT: Record<string, string> = {
  "qwen3.6-35b-a3b-optiq":
    "Rask og forutsigbar, og svarer på sekunder. Det eneste hovedagenten kan delegere hit uten forbehold, er en mekanisk endring over flere filer.",
  "qwen3.8-27b-optiq-4bit":
    "Mye tregere enn standardmodellen. Bruker 8 bit på de mest følsomme lagene og 4 bit på resten. I siste måling nådde ingen oppgaver tidsgrensen. Det gjorde den vanlige 4-bit-versjonen den erstatter.",
  "qwen3.8-27b-8bit-mlx":
    "Den tregeste. Løste litt flere oppgaver enn standardmodellen i siste måling, men bruker mange ganger så lang tid. Leser lange prompter i små steg for å bruke mindre minne, og det steget kjenner bare nyere nav-pilot til.",
  "qwen3.6-35b-a3b-8bit":
    "Standardmodellen i 8 bit, for Macer med 64 GB minne eller mer. Du må velge den selv. Ingen oppgavetyper er godkjent for den ennå, så hovedagenten delegerer ingenting til den.",
};

const TASK_CLASS_LABEL: Record<string, string> = {
  "read-qa": "svar og forklaringer om kode",
  "edit-single": "endring i én fil",
  "edit-multi-mechanical": "mekanisk endring over flere filer",
  "create-file": "ny fil",
  debug: "feilsøking",
};

const subtle = { color: "var(--ax-text-neutral-subtle)" };
const formatShortDate = (iso: string) =>
  new Date(`${iso}T12:00:00Z`).toLocaleDateString("nb-NO", {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "Europe/Oslo",
  });
const kTokens = (n: number) => `${Math.round(n / 1024)}k`;
const classLabel = (id: string) => TASK_CLASS_LABEL[id] ?? id;
const modelName = (m: LocalModel) => m.model.split("/").pop();

function trustedClasses(m: LocalModel) {
  return Object.entries(m.classes).flatMap(([id, c]) => [
    ...(c.delegate === "trusted" ? [`${classLabel(id)} (delegering)`] : []),
    ...(c.local === "trusted" ? [`${classLabel(id)} (hele økten lokalt)`] : []),
  ]);
}

function cloudClasses(m: LocalModel) {
  return Object.entries(m.classes)
    .filter(([, c]) => c.delegate !== "trusted" && c.local !== "trusted")
    .map(([id]) => classLabel(id));
}

function LocalModelText({ m }: { m: LocalModel }) {
  const text = LOCAL_MODEL_TEXT[m.id];
  if (text) return text;
  return (
    <>
      {m.role} <span className="text-xs italic">(norsk beskrivelse mangler)</span>
    </>
  );
}

// `stack` turns rows into cards below 640px (.table-stack in globals.css). The
// explicit roles keep the table semantics that display: block drops. A stacked
// table has room for the model id on one line; the doc pages wrap it at the hyphens.
const stackProps = (stack?: boolean) =>
  stack ? { className: "table-stack w-full", role: "table" } : { style: { minWidth: "40rem" } };

export function LocalModelsTable({ models, stack }: { models: LocalModel[]; stack?: boolean }) {
  const r = (role: string) => (stack ? role : undefined);
  return (
    <div className="overflow-x-auto">
      <Table size="small" {...stackProps(stack)}>
        <HeaderRow stack={stack} cells={["Modell", "Minst minne", "Kontekst / svar", "nav-pilot"]} />
        <TableBody role={r("rowgroup")}>
          {[...models]
            .sort((x, y) => x.min_ram_gb - y.min_ram_gb)
            .map((m) => (
              <TableRow key={m.id} role={r("row")}>
                <TableDataCell role={r("cell")}>
                  <VStack gap="space-4">
                    <span>
                      <strong>{modelName(m)}</strong>{" "}
                      <span className="text-sm" style={subtle}>
                        {m.default ? "standard" : "valgfri"}
                      </span>
                    </span>
                    <span className="text-sm" style={subtle}>
                      <LocalModelText m={m} />
                    </span>
                  </VStack>
                </TableDataCell>
                <TableDataCell role={r("cell")} data-label="Minst minne" className="whitespace-nowrap">
                  {m.min_ram_gb} GB
                  <div className="text-sm" style={subtle}>
                    vekter {m.weights_gb} GB
                  </div>
                </TableDataCell>
                <TableDataCell role={r("cell")} data-label="Kontekst / svar" className="whitespace-nowrap">
                  {kTokens(m.context)} / {kTokens(m.output)}
                </TableDataCell>
                <TableDataCell role={r("cell")} data-label="nav-pilot" className="whitespace-nowrap">
                  {m.min_nav_pilot ? (
                    // Build ids are YYYY.MM.DD-HHMMSS-sha. The date is what a reader needs; the id is in the tooltip.
                    <span title={`≥ ${m.min_nav_pilot}`}>
                      fra {formatShortDate(m.min_nav_pilot.slice(0, 10).replaceAll(".", "-"))}
                    </span>
                  ) : (
                    "alle"
                  )}
                </TableDataCell>
              </TableRow>
            ))}
        </TableBody>
      </Table>
    </div>
  );
}

export function TrustedClassesTable({ models, stack }: { models: LocalModel[]; stack?: boolean }) {
  const r = (role: string) => (stack ? role : undefined);
  return (
    <div className="overflow-x-auto">
      <Table size="small" {...stackProps(stack)}>
        <HeaderRow stack={stack} cells={["Modell", "Godkjent", "Blir i skyen"]} />
        <TableBody role={r("rowgroup")}>
          {models.map((m) => {
            const trusted = trustedClasses(m);
            return (
              <TableRow key={m.id} role={r("row")}>
                <TableDataCell role={r("cell")}>
                  <strong className="whitespace-nowrap">{modelName(m)}</strong>
                </TableDataCell>
                <TableDataCell role={r("cell")} data-label="Godkjent">
                  {trusted.length ? trusted.join(", ") : "ingen oppgavetyper ennå"}
                </TableDataCell>
                <TableDataCell role={r("cell")} data-label="Blir i skyen">
                  {cloudClasses(m).join(", ")}
                </TableDataCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}

function CountLine({ mode, k, n, verdict }: { mode: string; k: number; n: number; verdict: string }) {
  if (verdict === "trusted") {
    return (
      <span>
        {mode} <strong>{`${k}\u00a0av\u00a0${n}`} ✓ godkjent</strong>
      </span>
    );
  }
  return <span style={subtle}>{`${mode} ${k}\u00a0av\u00a0${n}`}</span>;
}

// Only the modes with runs; a stacked card keeps the first line next to the column label.
function Counts({ c }: { c?: ClassVerdict }) {
  const lines = [
    c?.delegate_n ? <CountLine key="d" mode="delegert" k={c.delegate_k} n={c.delegate_n} verdict={c.delegate} /> : null,
    c?.local_n ? <CountLine key="l" mode="lokalt" k={c.local_k} n={c.local_n} verdict={c.local} /> : null,
  ].filter(Boolean);
  if (!lines.length) return <span style={subtle}>ikke målt</span>;
  return lines.flatMap((l, i) => (i ? [<br key={i} />, l] : [l]));
}

type CountRow = { key: string; name: React.ReactNode; classes: Record<string, ClassVerdict> };

/**
 * Passed runs of all runs per task type, for delegation and for a fully local
 * session. A row per model; stacks into cards below 640px.
 */
export function ClassCountsTable({ rows }: { rows: CountRow[] }) {
  const ids = [...new Set(rows.flatMap((r) => Object.keys(r.classes)))];
  const label = (id: string) => classLabel(id).replace(/^./, (c) => c.toUpperCase());
  return (
    <div className="overflow-x-auto">
      <Table size="small" className="table-stack w-full" role="table">
        <HeaderRow stack cells={["Modell", ...ids.map(label)]} />
        <TableBody role="rowgroup">
          {rows.map((r) => (
            <TableRow role="row" key={r.key}>
              <TableDataCell role="cell">{r.name}</TableDataCell>
              {ids.map((id) => {
                const c = r.classes[id];
                return (
                  <TableDataCell role="cell" key={id} data-label={label(id)}>
                    <Counts c={c} />
                  </TableDataCell>
                );
              })}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

export const modelCountRows = (models: LocalModel[]): CountRow[] =>
  models.map((m) => ({
    key: m.id,
    name: <strong className="whitespace-nowrap">{modelName(m)}</strong>,
    classes: m.classes,
  }));

export const rejectedCountRows = (rejected: RejectedModel[]): CountRow[] =>
  rejected.map((m) => ({
    key: m.model,
    name: (
      <VStack gap="space-4">
        <strong className="whitespace-nowrap">{m.model.split("/").pop()}</strong>
        <span className="text-sm" style={subtle}>
          {m.replaced_by ? `erstattet av ${m.replaced_by}` : "ingen oppgavetype nådde kravet"}
        </span>
      </VStack>
    ),
    classes: m.classes,
  }));

const pct = (x: number) => `${Math.round(x * 100)} %`;

/** The bar behind «godkjent», in one sentence. */
export function barText(bar: Bar) {
  return `En oppgavetype blir godkjent når modellen er målt i minst ${bar.min_runs} kjøringer på minst ${bar.min_tasks} ulike oppgaver, og vi med ${pct(bar.confidence)} sikkerhet kan si at den lykkes minst ${pct(bar.x_caught)} så ofte som skymodellen. Der en feil ikke blir oppdaget, som i svar og forklaringer, er kravet ${pct(bar.x_silent)}, og det krever så mange kjøringer uten én feil at en modell kan mangle noen få selv om alle hittil har bestått.`;
}

// Measured results per task type: task, result and verdict, stacked on narrow screens.
export function ResultTable({
  rows,
  headers = ["Oppgave", "Resultat", "Vurdering"],
}: {
  rows: ResultRow[];
  headers?: [string, string, string];
}) {
  return (
    <div className="overflow-x-auto">
      <Table size="small" className="table-stack w-full" role="table">
        <HeaderRow stack cells={headers} />
        <TableBody role="rowgroup">
          {rows.map((r) => (
            <TableRow role="row" key={r.task}>
              <TableDataCell role="cell">
                <strong>{r.task}</strong>
              </TableDataCell>
              <TableDataCell role="cell" data-label={headers[1]}>
                {r.result}
              </TableDataCell>
              <TableDataCell role="cell" data-label={headers[2]}>
                {r.verdict}
              </TableDataCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
