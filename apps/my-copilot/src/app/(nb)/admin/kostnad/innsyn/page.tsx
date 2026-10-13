import type { Metadata } from "next";
import { BodyLong, BodyShort, Heading, Link, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import ErrorState from "@/components/error-state";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { formatDate } from "@/lib/format";
import { loadInnsyn, parseInnsynYear } from "@/lib/innsyn-load";
import { monthName } from "@/lib/team-year";
import { DocumentTitle, PrintButton, ReportNote } from "@/app/(nb)/innsikt/team/rapport/report-controls";

export const metadata: Metadata = {
  title: "Innsynssvar om KI-bruk",
  description: "Utskriftsvennlig innsynssvar om Navs bruk av GitHub Copilot.",
};

const headers: Record<string, string> = {
  month: "Måned",
  billed_usd: "Fakturert (USD)",
  preliminary: "Foreløpig",
  usd: "Lisenskostnad (USD, anslag)",
  calculated: "Tilbakeregnet",
  premium_requests: "Premiumforespørsler",
  ai_credits: "KI-kreditter",
  copilot_cli_in: "Copilot CLI, inn",
  copilot_cli_out: "Copilot CLI, ut",
  copilot_app_in: "Copilot-appen, inn",
  copilot_app_out: "Copilot-appen, ut",
  users: "Brukere",
  kind: "Grunnlag",
  family: "Modellfamilie",
  share_pct: "Andel av fakturert beløp (%)",
  models: "Modeller",
};

function cell(key: string, value: unknown): string {
  if (value == null) return "–";
  if (typeof value === "boolean") return value ? "ja" : "";
  if (Array.isArray(value)) return value.join(", ");
  if (key === "month" && typeof value === "string") return `${monthName(value)} ${value.slice(0, 4)}`;
  if (typeof value === "number")
    return value.toLocaleString("nb-NO", { maximumFractionDigits: key.endsWith("usd") || key === "usd" ? 2 : 1 });
  return String(value);
}

function DataTable({ rows }: { rows: Record<string, unknown>[] }) {
  if (!rows.length) return null;
  const keys = Object.keys(rows[0]);
  return (
    <Table size="small">
      <TableHeader>
        <TableRow>
          {keys.map((k, i) => (
            <TableHeaderCell key={k} scope="col" align={i ? "right" : "left"}>
              {headers[k] ?? k}
            </TableHeaderCell>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((r, n) => (
          <TableRow key={n}>
            {keys.map((k, i) => (
              <TableDataCell key={k} align={i ? "right" : "left"}>
                {cell(k, r[k])}
              </TableDataCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

/** Arrays of rows become tables; nested objects (licence) are walked once. */
function tables(data: Record<string, unknown> | null): Record<string, unknown>[][] {
  if (!data) return [];
  return Object.values(data).flatMap((v) =>
    Array.isArray(v) && typeof v[0] === "object"
      ? [v as Record<string, unknown>[]]
      : v && typeof v === "object" && !Array.isArray(v)
        ? tables(v as Record<string, unknown>)
        : []
  );
}

export default async function InnsynPage({ searchParams }: { searchParams: Promise<{ year?: string }> }) {
  await requireAdmin();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;
  const year = parseInnsynYear((await searchParams).year);
  if (year == null) return <ErrorState message="Ugyldig år." />;

  let answer;
  try {
    answer = await loadInnsyn(year, token);
  } catch (error) {
    console.error("[innsyn] Failed to build innsyn answer:", error);
    return <ErrorState message="Kunne ikke hente tallene til innsynssvaret." />;
  }

  return (
    <main id="hovedinnhold" tabIndex={-1} className="team-report mx-auto max-w-4xl px-4 py-8 print:p-0">
      <DocumentTitle title={`Innsynssvar KI-bruk Nav ${year} (til og med ${answer.period.to})`} />
      <VStack gap="space-24">
        <VStack gap="space-8">
          <Heading level="1" size="large">
            Innsynssvar: Navs bruk av GitHub Copilot i {year}
          </Heading>
          <BodyShort>
            Tallene gjelder {formatDate(answer.period.from)}–{formatDate(answer.period.to)}. Alle beløp er i USD.
          </BodyShort>
        </VStack>
        <VStack gap="space-8" className="print-hidden">
          <PrintButton />
          <Link href={`/api/admin-innsyn?year=${year}`}>Last ned som JSON</Link>
        </VStack>
        <ReportNote
          label="Merknad"
          description="Vises øverst i utskriften. Står det ingenting her, blir feltet utelatt."
        />
        {answer.questions.map((q) => {
          const id = q.id;
          return (
            <section key={id} aria-labelledby={`q-${id}`}>
              <Heading id={`q-${id}`} level="2" size="small" spacing>
                {id}) {q.question}
              </Heading>
              <BodyLong spacing>{q.answer}</BodyLong>
              {tables(q.data).map((rows, i) => (
                <div key={i} className="team-report-table overflow-x-auto">
                  <DataTable rows={rows} />
                </div>
              ))}
              {q.notes.length > 0 && (
                <ul className="list-disc space-y-1 pl-6">
                  {q.notes.map((n) => (
                    <li key={n}>
                      <BodyShort size="small">{n}</BodyShort>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          );
        })}
        <section aria-labelledby="slik-har-vi-regnet">
          <Heading id="slik-har-vi-regnet" level="2" size="small" spacing>
            Slik har vi regnet
          </Heading>
          <ul className="list-disc space-y-1 pl-6">
            {answer.method.map((m) => (
              <li key={m}>
                <BodyShort size="small">{m}</BodyShort>
              </li>
            ))}
          </ul>
        </section>
      </VStack>
    </main>
  );
}
