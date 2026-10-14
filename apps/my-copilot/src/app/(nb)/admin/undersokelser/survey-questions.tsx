import { BodyShort, Heading, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { formatNumber } from "@/lib/format";
import type { QuestionResult, Survey, SurveyQuestion, VersionResult } from "@/lib/survey";
import { fill, Legend, LikertBar } from "./likert-bar";

/**
 * The scale labels of a question (Helt uenig … Helt enig), for the version the
 * definition on main describes. Other versions, and questions without labels,
 * show the raw value.
 */
function labeller(def: SurveyQuestion | undefined, current: number | undefined) {
  return (version: number, value: string) =>
    (def?.labels && version === current && def.labels[Number(value) - (def.min ?? 0)]) || value;
}

type Label = (version: number, value: string) => string;

/** One version of a question as a table: count, share of respondents and an inline bar. */
function VersionTable({ v, caption, label }: { v: VersionResult; caption: string; label: Label }) {
  const n = v.n;
  if (!v.options || n === null) return null;
  return (
    <Table size="small">
      <caption className="text-left">
        <BodyShort size="small">
          {caption}: {formatNumber(n)} svar
        </BodyShort>
      </caption>
      <TableHeader>
        <TableRow>
          <TableHeaderCell scope="col">Svar</TableHeaderCell>
          <TableHeaderCell scope="col" align="right">
            Antall
          </TableHeaderCell>
          <TableHeaderCell scope="col" className="w-1/2">
            Andel av dem som svarte
          </TableHeaderCell>
        </TableRow>
      </TableHeader>
      <TableBody>
        {v.options.map((o) => {
          const share = o.count === null || n === 0 ? 0 : o.count / n;
          return (
            <TableRow key={o.value}>
              <TableDataCell>{label(v.version, o.value)}</TableDataCell>
              <TableDataCell align="right">{o.count === null ? "–" : formatNumber(o.count)}</TableDataCell>
              <TableDataCell>
                {o.count === null ? null : (
                  <div className="flex items-center gap-2">
                    <div
                      aria-hidden
                      className="h-3 rounded-sm"
                      style={{ width: `${Math.round(share * 100)}%`, ...fill("var(--ax-bg-accent-strong)") }}
                    />
                    <span>{Math.round(share * 100)} %</span>
                  </div>
                )}
              </TableDataCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}

type Scale = { version: number; min: number; max: number; labels: string[] };

/** Scale and matrix questions with labels on main get a Likert bar for the version main describes. */
function scaleOf(def: SurveyQuestion | undefined, version: number | undefined): Scale | undefined {
  if (!def?.labels || version === undefined || (def.type !== "scale" && def.type !== "matrix")) return undefined;
  const min = def.min ?? 0;
  return { version, min, max: def.max ?? min + def.labels.length - 1, labels: def.labels };
}

function Versions({ versions, label, scale }: { versions: VersionResult[]; label: Label; scale?: Scale }) {
  return versions.map((v) => {
    const caption = versions.length > 1 ? `Versjon ${v.version}` : "Alle";
    const table = <VersionTable key={v.version} v={v} label={label} caption={caption} />;
    if (!scale || v.version !== scale.version || !v.options || v.n === null) return table;
    return (
      <VStack key={v.version} gap="space-4">
        <LikertBar v={v} min={scale.min} max={scale.max} labels={scale.labels} />
        <details>
          <summary className="cursor-pointer text-sm">Vis tall</summary>
          {table}
        </details>
      </VStack>
    );
  });
}

export function SurveyQuestions({ questions, def }: { questions: QuestionResult[] | null; def: Survey | undefined }) {
  if (!questions) return null;
  return questions.map((q) => {
    const qdef = def?.questions.find((d) => d.id === q.id);
    return (
      <section key={q.id}>
        <VStack gap="space-8">
          <Heading level="2" size="small">
            {q.text}
          </Heading>
          {(qdef?.type === "matrix" || qdef?.type === "scale") && qdef.labels ? <Legend labels={qdef.labels} /> : null}
          {q.versions ? (
            <Versions
              versions={q.versions}
              label={labeller(qdef, qdef?.version)}
              scale={scaleOf(qdef, qdef?.version)}
            />
          ) : null}
          {q.items?.map((item) => (
            <VStack key={item.id} gap="space-4">
              <Heading level="3" size="xsmall">
                {item.text}
              </Heading>
              <Versions
                versions={item.versions}
                label={labeller(qdef, qdef?.items?.find((d) => d.id === item.id)?.version)}
                scale={scaleOf(qdef, qdef?.items?.find((d) => d.id === item.id)?.version)}
              />
            </VStack>
          ))}
        </VStack>
      </section>
    );
  });
}
