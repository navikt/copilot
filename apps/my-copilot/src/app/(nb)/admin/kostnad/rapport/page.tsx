import type { Metadata } from "next";
import { BodyLong, BodyShort, Heading, Link, VStack } from "@navikt/ds-react";
import { fill } from "@/app/(nb)/admin/undersokelser/likert-bar";
import { PrintHeader } from "@/app/(nb)/admin/undersokelser/print-header";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import ErrorState from "@/components/error-state";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { formatDate } from "@/lib/format";
import { loadRapport, parseRapportYear } from "@/lib/rapport-load";
import { monthName } from "@/lib/team-year";
import { RAPPORT_SECTIONS } from "@/lib/rapport";
import { DocumentTitle, PrintButton, ReportNote } from "@/app/(nb)/innsikt/team/rapport/report-controls";

export const metadata: Metadata = {
  title: "Copilot i Nav: bruk og kostnad",
  description: "Utskriftsvennlig rapport om Navs bruk og kostnad for GitHub Copilot.",
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
  const maxBilled = Math.max(0, ...rows.map((r) => (typeof r.billed_usd === "number" ? r.billed_usd : 0)));
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
            {keys.map((k, i) => {
              const val = r[k];
              const barPct =
                k === "share_pct" && typeof val === "number"
                  ? Math.round(val)
                  : k === "billed_usd" && typeof val === "number" && maxBilled > 0
                    ? Math.round((val / maxBilled) * 100)
                    : null;
              return (
                <TableDataCell key={k} align={i ? "right" : "left"}>
                  {barPct != null && barPct > 0 ? (
                    <div className="flex items-center justify-end gap-2">
                      <div aria-hidden className="flex w-28 justify-end">
                        <div
                          data-testid={`bar-${k}-${n}`}
                          className={`h-3 rounded-sm${r.preliminary ? " print-stripe" : ""}`}
                          style={{ width: `${Math.min(100, barPct)}%`, ...fill("var(--ax-bg-accent-strong)") }}
                        />
                      </div>
                      <span>{cell(k, val)}</span>
                    </div>
                  ) : (
                    cell(k, val)
                  )}
                </TableDataCell>
              );
            })}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

export default async function RapportPage({ searchParams }: { searchParams: Promise<{ year?: string }> }) {
  await requireAdmin();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;
  const year = parseRapportYear((await searchParams).year);
  if (year == null) return <ErrorState message="Ugyldig år." />;

  let report;
  try {
    report = await loadRapport(year, token);
  } catch (error) {
    console.error("[rapport] Failed to build report:", error);
    return <ErrorState message="Kunne ikke hente tallene til rapporten." />;
  }

  const periodLine = `Tallene gjelder ${formatDate(report.period.from)}–${formatDate(report.period.to)}. Alle beløp er i USD.`;

  return (
    <main id="hovedinnhold" tabIndex={-1} className="print-report mx-auto max-w-4xl px-4 py-8 print:p-0">
      <DocumentTitle title={`${report.title} (til og med ${report.period.to})`} />
      <VStack gap="space-24">
        <PrintHeader title={report.title} lines={[periodLine]} />
        <VStack gap="space-8" className="print-hidden">
          <Heading level="1" size="large">
            {report.title}
          </Heading>
          <BodyShort>{periodLine}</BodyShort>
        </VStack>
        <VStack gap="space-8" className="print-hidden">
          <PrintButton />
          <Link href={`/api/admin-rapport?year=${year}`}>Last ned som JSON</Link>
        </VStack>
        <ReportNote
          label="Merknad"
          description="Vises øverst i utskriften. Står det ingenting her, blir feltet utelatt."
        />
        {RAPPORT_SECTIONS.map((id) => {
          const sec = report.sections[id];
          return (
            <section key={id} aria-labelledby={`s-${id}`}>
              <VStack gap="space-24">
                <Heading id={`s-${id}`} level="2" size="small">
                  {sec.title}
                </Heading>
                <BodyLong>{sec.summary}</BodyLong>
                {sec.tables
                  .filter((t) => t.rows.length > 0)
                  .map((t) => (
                    <VStack key={t.id} gap="space-8" className="print:break-inside-avoid">
                      <Heading level="3" size="xsmall">
                        {t.caption}
                      </Heading>
                      <div className={`team-report-table overflow-x-auto ${t.id === "familier" ? "wrap" : ""}`}>
                        <DataTable rows={t.rows} />
                      </div>
                    </VStack>
                  ))}
                {sec.notes.length > 0 && (
                  <ul className="list-disc space-y-1 pl-6">
                    {sec.notes.map((n) => (
                      <li key={n}>
                        <BodyShort size="small">{n}</BodyShort>
                      </li>
                    ))}
                  </ul>
                )}
              </VStack>
            </section>
          );
        })}
      </VStack>
    </main>
  );
}
