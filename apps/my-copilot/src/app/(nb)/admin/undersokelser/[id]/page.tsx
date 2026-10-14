import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Alert, BodyShort, Box, Button, Heading, HStack, Link, Tag, VStack } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import ErrorState from "@/components/error-state";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { formatDate, formatNumber } from "@/lib/format";
import { getSurveyPreview, getSurveyResults, type SurveyQuestion, type VersionResult } from "@/lib/survey";
import { DocumentTitle, PrintButton } from "../../../innsikt/team/rapport/report-controls";
import { fill, Legend, LikertBar } from "../likert-bar";
import { PrintHeader } from "../print-header";

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
    <main id="hovedinnhold" tabIndex={-1} className="print-report survey-print">
      <DocumentTitle title={`${s.title} – resultater`} />
      <div className="print-hidden">
        <PageHero
          title={s.title}
          description="Resultater fra brukerundersøkelsen."
          actions={<PrintButton>Lagre som PDF</PrintButton>}
        />
      </div>
      <PrintHeader
        title={s.title}
        lines={[
          `${formatDate(s.starts)}–${formatDate(s.ends)} · ${s.suppressed ? UNDER_FIVE : formatNumber(s.responses)} svar`,
        ]}
      />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-24">
            <HStack gap="space-16" className="print-hidden">
              <Link href="/admin/undersokelser">Alle undersøkelser</Link>
              <Link href="/admin/undersokelser/sammenlign">Sammenlign bølger</Link>
              <Link href={`/admin/undersokelser/${id}/fritekst`}>Se fritekstsvarene</Link>
              <Button
                as="a"
                href={`/api/admin/surveys/${id}/export`}
                size="small"
                variant="secondary"
                className="print:hidden"
              >
                Last ned svar (CSV)
              </Button>
            </HStack>
            <BodyShort className="print-hidden">
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
                  Spørsmål som ble endret underveis, vises med én tabell per versjon. Fritekstsvarene har egen side og
                  er ikke med i PDF-en.
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
