import { VStack } from "@navikt/ds-react";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
import type { LocalModel } from "@/lib/local-models";
import { code } from "@/components/nav-pilot/doc-page";

// The local-model tables on /nav-pilot/referanse and
// /nav-pilot/forklaring/lokal-modell. The rows come from the manifest in
// navikt/mlx-workspace when the page runs (src/lib/local-models.ts), with
// src/lib/local-models.json as the fallback. All numbers come from there. Only
// the Norwegian descriptions live here, without numbers. A model without one
// shows the manifest's English role with a visible note.
const LOCAL_MODEL_TEXT: Record<string, string> = {
  "qwen3.6-35b-a3b-optiq":
    "Rask og forutsigbar, og svarer på sekunder. Det eneste hovedagenten kan sende hit uten forbehold, er en mekanisk endring over flere filer.",
  "qwen3.8-27b-optiq-4bit":
    "Mye tregere enn standardmodellen. Bruker 8 bit på de mest følsomme lagene og 4 bit på resten. I siste måling nådde ingen oppgaver tidsgrensen. Det gjorde den vanlige 4-bit-versjonen den erstatter.",
  "qwen3.8-27b-8bit-mlx":
    "Den tregeste. Løste litt flere oppgaver enn standardmodellen i siste måling, men bruker mange ganger så lang tid. Leser lange prompter i små steg for å bruke mindre minne, og det steget kjenner bare nyere nav-pilot til.",
  "qwen3.6-35b-a3b-8bit":
    "Standardmodellen i 8 bit, for Macer med 64 GB minne eller mer. Du må velge den selv. Ingen oppgavetyper er godkjent for den ennå, så hovedagenten sender den ingenting.",
};

const TASK_CLASS_LABEL: Record<string, string> = {
  "read-qa": "svar og forklaringer om kode",
  "edit-single": "endring i én fil",
  "edit-multi-mechanical": "mekanisk endring over flere filer",
  "create-file": "ny fil",
  debug: "feilsøking",
};

const kTokens = (n: number) => `${Math.round(n / 1024)}k`;
const classLabel = (id: string) => TASK_CLASS_LABEL[id] ?? id;
const modelName = (m: LocalModel) => m.model.split("/").pop();

function trustedClasses(m: LocalModel) {
  return Object.entries(m.classes).flatMap(([id, c]) => [
    ...(c.delegate === "trusted" ? [`${classLabel(id)} (sendt fra hovedagenten i skyen)`] : []),
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

export function LocalModelsTable({ models }: { models: LocalModel[] }) {
  return (
    <div className="overflow-x-auto">
      <Table size="small" style={{ minWidth: "40rem" }}>
        <TableHeader>
          <TableRow>
            <TableHeaderCell scope="col">Modell</TableHeaderCell>
            <TableHeaderCell scope="col">Kontekst / svar</TableHeaderCell>
            <TableHeaderCell scope="col">Minne</TableHeaderCell>
            <TableHeaderCell scope="col">Krever nav-pilot</TableHeaderCell>
            <TableHeaderCell scope="col">Kort sagt</TableHeaderCell>
          </TableRow>
        </TableHeader>
        <TableBody>
          {models.map((m) => (
            <TableRow key={m.id}>
              <TableDataCell>
                <VStack gap="space-2">
                  <code className={code}>{modelName(m)}</code>
                  <div className="text-xs" style={{ color: "var(--ax-text-neutral-subtle)" }}>
                    {m.default ? "standard" : "valgfri"}
                  </div>
                </VStack>
              </TableDataCell>
              <TableDataCell className="whitespace-nowrap">
                {kTokens(m.context)} / {kTokens(m.output)}
              </TableDataCell>
              <TableDataCell>
                {m.min_ram_gb} GB, vektene tar {m.weights_gb} GB
              </TableDataCell>
              <TableDataCell>
                {m.min_nav_pilot ? <code className={code}>≥ {m.min_nav_pilot}</code> : "alle versjoner"}
              </TableDataCell>
              <TableDataCell>
                <LocalModelText m={m} />
              </TableDataCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

export function TrustedClassesTable({ models }: { models: LocalModel[] }) {
  return (
    <div className="overflow-x-auto">
      <Table size="small" style={{ minWidth: "40rem" }}>
        <TableHeader>
          <TableRow>
            <TableHeaderCell scope="col">Modell</TableHeaderCell>
            <TableHeaderCell scope="col">Godkjent</TableHeaderCell>
            <TableHeaderCell scope="col">Blir i skyen</TableHeaderCell>
          </TableRow>
        </TableHeader>
        <TableBody>
          {models.map((m) => {
            const trusted = trustedClasses(m);
            return (
              <TableRow key={m.id}>
                <TableDataCell>
                  <code className={code}>{modelName(m)}</code>
                </TableDataCell>
                <TableDataCell>{trusted.length ? trusted.join(", ") : "ingen oppgavetyper ennå"}</TableDataCell>
                <TableDataCell>{cloudClasses(m).join(", ")}</TableDataCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
