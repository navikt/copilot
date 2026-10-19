import type { Metadata } from "next";
import { BodyShort, Heading, VStack } from "@navikt/ds-react";
import { fill } from "@/app/(nb)/admin/undersokelser/likert-bar";
import { PrintHeader } from "@/app/(nb)/admin/undersokelser/print-header";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import ErrorState from "@/components/error-state";
import { getUser, getUserToken } from "@/lib/auth";
import { BackendApiError } from "@/lib/backend-api";
import { getTeamYearOverview } from "@/lib/cached-bigquery";
import { formatDate } from "@/lib/format";
import { hasNoUsageAmounts, monthName, teamYearCaveats, teamYearRows } from "@/lib/team-year";
import { DocumentTitle, PrintButton, ReportNote } from "@/components/report-controls";

export const metadata: Metadata = {
  title: "Kostnadsrapport for team",
  description: "Utskriftsvennlig rapport over et teams Copilot-kostnad hittil i år.",
};

/** CSV column index, screen header and the shorter print header, so the columns fit on A4. No basis column; a footnote marks January–May. */
const allColumns: [number, string, string][] = [
  [0, "Måned", "Måned"],
  [3, "Fakturert", "Fakturert"],
  [4, "Foreløpig forbruk", "Foreløpig"],
  [5, "Fakturert uten bruk", "Fakt. u. bruk"],
  [7, "Lisenskostnad (anslag)", "Lisenser**"],
  [8, "Totalt inkl. lisenser", "Totalt"],
];

/** CSV amounts keep a decimal point; the report shows them the Norwegian way. */
const nok = (amount: string) =>
  amount === "" ? "" : Number(amount).toLocaleString("nb-NO", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

export default async function TeamReportPage({
  searchParams,
}: {
  searchParams: Promise<{ team?: string; year?: string }>;
}) {
  await getUser();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;
  const { team = "", year: requestedYear } = await searchParams;
  const year = Number(requestedYear ?? new Date().getUTCFullYear());
  if (!/^[1-9][0-9]{0,19}$/.test(team) || !Number.isInteger(year)) {
    return <ErrorState message="Ugyldig team eller år." />;
  }

  let data;
  try {
    data = await getTeamYearOverview(team, year, token);
  } catch (error) {
    if (error instanceof BackendApiError && error.status === 400)
      return <ErrorState message="Ugyldig team eller år." />;
    console.error("[team-rapport] Failed to fetch team year usage:", error);
    return <ErrorState message="Kunne ikke hente teamets forbruk." />;
  }

  // The same rows as the CSV, in month order with the total last.
  const rows = teamYearRows(data);
  const columns = allColumns.filter(([c]) => c !== 5 || hasNoUsageAmounts(rows));
  const { last_usage_day } = data.coverage;
  // The last caveat is «Tallene går til og med …», which the report states at the top.
  const caveats = teamYearCaveats(data).slice(0, -1);
  const chartMonths = data.months.filter((m) => (m.total_usd ?? 0) > 0);
  const maxMonthTotal = Math.max(0, ...chartMonths.map((m) => m.total_usd ?? 0));

  const row = (cells: string[], i: number) => {
    const total = i === data.months.length;
    const preliminary = !total && data.months[i].basis === "gross";
    return (
      <TableRow key={cells[0]} className={preliminary ? "text-text-subtle italic" : undefined}>
        {columns.map(([c], col) => (
          <TableDataCell key={c} align={col ? "right" : "left"} style={total ? { fontWeight: 600 } : undefined}>
            {col === 0 && !total
              ? monthName(cells[c]) +
                (data.months[i].basis === "premium" ? "*" : "") +
                (preliminary ? " (foreløpig)" : "")
              : col
                ? nok(cells[c])
                : cells[c]}
          </TableDataCell>
        ))}
      </TableRow>
    );
  };

  const periodLine = `Tallene går til og med ${last_usage_day ? formatDate(last_usage_day) : "ukjent dato"}. Alle beløp er i USD.`;

  return (
    <main id="hovedinnhold" tabIndex={-1} className="print-report mx-auto max-w-4xl px-4 py-8 print:p-0">
      <DocumentTitle
        title={`Copilot-kostnad ${data.team_slug} ${data.year}${last_usage_day ? ` (til og med ${last_usage_day})` : ""}`}
      />
      <VStack gap="space-24">
        <PrintHeader title={`Copilot-kostnad for ${data.team_slug}, ${data.year}`} lines={[periodLine]} />
        <VStack gap="space-8" className="print-hidden">
          <Heading level="1" size="large">
            Copilot-kostnad for {data.team_slug}, {data.year}
          </Heading>
          <BodyShort>{periodLine}</BodyShort>
        </VStack>
        <PrintButton />
        <ReportNote
          label="Merknad"
          description="Vises øverst i utskriften. Står det ingenting her, blir feltet utelatt."
        />
        {chartMonths.length > 0 && maxMonthTotal > 0 && (
          <div aria-hidden className="space-y-1.5 print:break-inside-avoid" data-testid="team-month-bars">
            {chartMonths.map((m) => {
              const total = m.total_usd ?? 0;
              const preliminary = m.basis === "gross";
              const widthPct = Math.round((total / maxMonthTotal) * 100);
              return (
                <div key={m.month} className="grid grid-cols-[9rem_1fr_5.5rem] items-center gap-2 text-xs">
                  <span className={preliminary ? "text-text-subtle italic" : undefined}>
                    {monthName(m.month)}
                    {preliminary ? " (foreløpig)" : ""}
                  </span>
                  <div className="flex items-center">
                    <div
                      data-testid={`month-bar-${m.month}`}
                      className={`h-3 rounded-sm${preliminary ? " print-stripe" : ""}`}
                      style={{ width: `${widthPct}%`, ...fill("var(--ax-bg-accent-strong)") }}
                    />
                  </div>
                  <span className="text-right tabular-nums">{nok(total.toFixed(2))}</span>
                </div>
              );
            })}
          </div>
        )}
        <div className="team-report-table overflow-x-auto">
          <Table size="small">
            <TableHeader>
              <TableRow>
                {columns.map(([, screen, print], i) => (
                  <TableHeaderCell key={screen} scope="col" align={i ? "right" : "left"}>
                    <span className="print-hidden">{screen}</span>
                    <span className="print-only">{print}</span>
                  </TableHeaderCell>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>{rows.map(row)}</TableBody>
          </Table>
        </div>
        <BodyShort size="small">
          {data.months.some((m) => m.basis === "premium") && "* Fakturert som premiumforespørsler, ikke KI-kreditter. "}
          <span className="print-only">** Lisenskostnad er et anslag. «Totalt» er totalt inkl. lisenser.</span>
        </BodyShort>
        <ReportNote label="Kommentar" description="Valgfri tekst som vises under tabellen i utskriften." />
        <section aria-labelledby="slik-regner-vi">
          <Heading id="slik-regner-vi" level="2" size="small" spacing>
            Slik regner vi
          </Heading>
          <ul className="list-disc space-y-1 pl-6">
            {caveats.map((c) => (
              <li key={c}>
                <BodyShort size="small">{c}</BodyShort>
              </li>
            ))}
          </ul>
        </section>
      </VStack>
    </main>
  );
}
