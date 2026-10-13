import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Alert, BodyShort, Heading, Link, Tag, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import ErrorState from "@/components/error-state";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { formatDate, formatNumber } from "@/lib/format";
import { getSurveyResults, type VersionResult } from "@/lib/survey";

export const metadata: Metadata = {
  title: "Resultater",
  description: "Resultater fra en brukerundersøkelse.",
  robots: { index: false, follow: false },
};

const UNDER_FIVE = "under 5";

/** One version of a question as a table: count, share of respondents and an inline bar. */
function VersionTable({ v, caption }: { v: VersionResult; caption: string }) {
  if (!v.options) {
    return (
      <BodyShort size="small">
        {caption}: {UNDER_FIVE} svar, tallene vises ikke.
      </BodyShort>
    );
  }
  return (
    <Table size="small">
      <caption className="text-left">
        <BodyShort size="small">
          {caption}: {formatNumber(v.n)} svar
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
          const share = o.count === null || v.n === 0 ? 0 : o.count / v.n;
          return (
            <TableRow key={o.value}>
              <TableDataCell>{o.value}</TableDataCell>
              <TableDataCell align="right">
                {o.suppressed || o.count === null ? UNDER_FIVE : formatNumber(o.count)}
              </TableDataCell>
              <TableDataCell>
                {o.suppressed || o.count === null ? null : (
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

function Versions({ versions }: { versions: VersionResult[] }) {
  return versions.map((v) => (
    <VersionTable key={v.version} v={v} caption={versions.length > 1 ? `Versjon ${v.version}` : "Alle"} />
  ));
}

export default async function SurveyResultsPage({ params }: { params: Promise<{ id: string }> }) {
  await requireAdmin();
  const { id } = await params;
  if (!/^[a-z0-9-]{1,64}$/.test(id)) notFound();
  const token = await getUserToken();
  if (!token) return <ErrorState title="Resultater" message="Mangler innloggingstoken" />;
  const res = await getSurveyResults(token, id);
  if (res.status === "not-found") notFound();
  if (res.status === "error") return <ErrorState title="Resultater" message="Kunne ikke hente resultatene." />;
  const s = res.data;

  return (
    <main className="max-w-7xl mx-auto px-4 py-8">
      <VStack gap="space-24">
        <Link href="/admin/undersokelser">Alle undersøkelser</Link>
        <VStack gap="space-8">
          <Heading level="1" size="xlarge">
            {s.title}
          </Heading>
          <BodyShort>
            {formatDate(s.starts)}–{formatDate(s.ends)} ·{" "}
            <Tag size="small" variant="neutral">
              {s.status}
            </Tag>{" "}
            · {s.suppressed ? UNDER_FIVE : formatNumber(s.responses)} svar
          </BodyShort>
        </VStack>
        {s.suppressed || !s.questions ? (
          <Alert variant="info">For få svar til å vise resultater ({UNDER_FIVE}).</Alert>
        ) : (
          <>
            <BodyShort size="small">
              Grupper med færre enn 5 svar vises som «{UNDER_FIVE}». Er et spørsmål endret underveis, vises hver versjon
              for seg. Fritekstsvar vises ikke her.
            </BodyShort>
            {s.questions.map((q) => (
              <section key={q.id}>
                <VStack gap="space-8">
                  <Heading level="2" size="small">
                    {q.text}
                  </Heading>
                  {q.versions ? <Versions versions={q.versions} /> : null}
                  {q.items?.map((item) => (
                    <VStack key={item.id} gap="space-4">
                      <Heading level="3" size="xsmall">
                        {item.text}
                      </Heading>
                      <Versions versions={item.versions} />
                    </VStack>
                  ))}
                </VStack>
              </section>
            ))}
          </>
        )}
      </VStack>
    </main>
  );
}
