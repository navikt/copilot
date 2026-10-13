import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Alert, BodyShort, Box, Heading, Link, Tag, VStack } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import ErrorState from "@/components/error-state";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { formatDate, formatNumber } from "@/lib/format";
import { getSurveyPreview, getSurveyResults, type SurveyQuestion, type VersionResult } from "@/lib/survey";
import { likert, type Segment } from "./likert";

export const metadata: Metadata = {
  title: "Resultater",
  description: "Resultater fra en brukerundersøkelse.",
  robots: { index: false, follow: false },
};

const UNDER_FIVE = "under 5";

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

// Diverging scale: red for disagree, gray neutral, green for agree; [fill, ink].
const FILLS = [
  ["var(--ax-bg-danger-strong)", "var(--ax-text-danger-contrast)"],
  ["var(--ax-bg-danger-moderate-pressed)", "var(--ax-text-neutral)"],
  ["var(--ax-bg-neutral-moderate-pressed)", "var(--ax-text-neutral)"],
  ["var(--ax-bg-success-moderate-pressed)", "var(--ax-text-neutral)"],
  ["var(--ax-bg-success-strong)", "var(--ax-text-success-contrast)"],
];
const pct = (share: number) => `${Math.round(share * 100)} %`;

/**
 * One statement's bar: negatives left of the zero line, neutral split across it,
 * positives right. Decorative: the numbers are in the table under «Vis tall».
 */
function LikertBar({ v, min, max, labels }: { v: VersionResult; min: number; max: number; labels: string[] }) {
  const { segments, mean } = likert(v, min, max);
  const mid = (min + max) / 2;
  const seg = (s: Segment, width: number, label = true) => {
    const i = Math.round(((s.value - min) / (max - min || 1)) * (FILLS.length - 1));
    const text = `${pct(s.share)} (${formatNumber(s.count)})`;
    return (
      <div
        key={s.value}
        data-testid={label ? `segment-${s.value}` : undefined}
        title={`${labels[s.value - min] ?? s.value}: ${text}`}
        className="@container h-7 overflow-hidden whitespace-nowrap text-xs leading-7 text-center"
        style={{ width: `${width * 100}%`, background: FILLS[i][0], color: FILLS[i][1] }}
      >
        {/* Narrow segments show their number in the tooltip and the table only. */}
        {label ? <span className="hidden @min-[4.5rem]:inline">{text}</span> : null}
      </div>
    );
  };
  const shown = segments.filter((s) => s.count);
  return (
    <div aria-hidden className="grid grid-cols-3 items-center gap-x-0">
      <div className="flex justify-end gap-[2px] border-r-2" style={{ borderColor: "var(--ax-border-neutral-strong)" }}>
        {shown.filter((s) => s.value <= mid).map((s) => seg(s, s.value === mid ? s.share / 2 : s.share))}
      </div>
      <div className="flex gap-[2px]">
        {shown
          .filter((s) => s.value >= mid)
          .map((s) => (s.value === mid ? seg(s, s.share / 2, false) : seg(s, s.share)))}
      </div>
      <Box paddingInline="space-12 space-0" className="flex items-center gap-2 text-xs">
        {mean === null ? null : <span>Snitt {mean.toFixed(1).replace(".", ",")}</span>}
      </Box>
    </div>
  );
}

function Legend({ labels }: { labels: string[] }) {
  return (
    <div aria-hidden className="flex flex-wrap gap-x-4 gap-y-1 text-xs">
      {labels.map((l, i) => (
        <span key={l} className="flex items-center gap-1">
          <span
            className="inline-block h-3 w-3 rounded-sm"
            style={{ background: FILLS[Math.round((i / (labels.length - 1 || 1)) * (FILLS.length - 1))][0] }}
          />
          {l}
        </span>
      ))}
    </div>
  );
}

/** One version of a question as a table: count, share of respondents and an inline bar. */
function VersionTable({ v, caption, label }: { v: VersionResult; caption: string; label: Label }) {
  const n = v.n;
  // Old backends sent null for counts below five; nothing is hidden now.
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
                      style={{ width: `${Math.round(share * 100)}%`, background: "var(--ax-bg-accent-strong)" }}
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

export default async function SurveyResultsPage({ params }: { params: Promise<{ id: string }> }) {
  await requireAdmin();
  const { id } = await params;
  if (!/^[a-z0-9-]{1,64}$/.test(id)) notFound();
  const token = await getUserToken();
  if (!token) return <ErrorState title="Resultater" message="Mangler innloggingstoken" />;
  const [res, def] = await Promise.all([getSurveyResults(token, id), getSurveyPreview(id)]);
  if (res.status === "not-found") notFound();
  if (res.status === "error") return <ErrorState title="Resultater" message="Kunne ikke hente resultatene." />;
  const s = res.data;

  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero title={s.title} description="Resultater fra brukerundersøkelsen." />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-24">
            <Link href="/admin/undersokelser">Alle undersøkelser</Link>
            <BodyShort>
              {formatDate(s.starts)}–{formatDate(s.ends)} ·{" "}
              <Tag size="small" variant="neutral">
                {s.status}
              </Tag>{" "}
              · {s.suppressed ? UNDER_FIVE : formatNumber(s.responses)} svar
            </BodyShort>
            {s.suppressed || !s.questions ? (
              <Alert variant="info">For få svar til å vise resultater ({UNDER_FIVE}).</Alert>
            ) : (
              <>
                <BodyShort size="small">
                  Spørsmål som ble endret underveis, vises med én tabell per versjon. Fritekstsvar vises ikke her.
                </BodyShort>
                {s.questions.map((q) => {
                  const qdef = def?.questions.find((d) => d.id === q.id);
                  return (
                    <section key={q.id}>
                      <VStack gap="space-8">
                        <Heading level="2" size="small">
                          {q.text}
                        </Heading>
                        {(qdef?.type === "matrix" || qdef?.type === "scale") && qdef.labels ? (
                          <Legend labels={qdef.labels} />
                        ) : null}
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
                })}
              </>
            )}
          </VStack>
        </Box>
      </div>
    </main>
  );
}
