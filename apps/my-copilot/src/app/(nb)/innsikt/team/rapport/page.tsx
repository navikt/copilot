import type { Metadata } from "next";
import { BodyShort, Heading, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import ErrorState from "@/components/error-state";
import { getUser, getUserToken } from "@/lib/auth";
import { BackendApiError } from "@/lib/backend-api";
import { getTeamYearOverview } from "@/lib/cached-bigquery";
import { formatDate } from "@/lib/format";
import { hasNoUsageAmounts, monthName, teamYearCaveats, teamYearRows } from "@/lib/team-year";
import { DocumentTitle, PrintButton, ReportNote } from "./report-controls";

export const metadata: Metadata = {
  title: "Kostnadsrapport for team",
  description: "Utskriftsvennlig rapport over et teams Copilot-kostnad hittil i år.",
};

/** CSV column index, screen header and the shorter print header, so the columns fit on A4. */
const allColumns: [number, string, string][] = [
  [0, "Måned", "Måned"],
  [1, "Grunnlag", "Grunnlag"],
  [3, "Netto", "Netto"],
  [4, "Brutto", "Brutto"],
  [5, "Netto uten bruk", "Netto u. bruk"],
  [7, "Lisenser (anslag)", "Lisenser*"],
  [8, "Totalt med lisenser", "Totalt"],
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

  const row = (cells: string[], i: number) => {
    const total = i === data.months.length;
    const preliminary = !total && data.months[i].basis === "gross";
    return (
      <TableRow key={cells[0]} className={preliminary ? "text-text-subtle italic" : undefined}>
        {columns.map(([c], col) => (
          <TableDataCell key={c} align={col < 2 ? "left" : "right"} style={total ? { fontWeight: 600 } : undefined}>
            {col === 0 && !total ? monthName(cells[c]) : col < 2 ? cells[c] : nok(cells[c])}
          </TableDataCell>
        ))}
      </TableRow>
    );
  };

  return (
    <main id="hovedinnhold" tabIndex={-1} className="team-report mx-auto max-w-4xl px-4 py-8 print:p-0">
      <DocumentTitle
        title={`Copilot-kostnad ${data.team_slug} ${data.year}${last_usage_day ? ` (til og med ${last_usage_day})` : ""}`}
      />
      <VStack gap="space-24">
        <VStack gap="space-8">
          <Heading level="1" size="large">
            Copilot-kostnad for {data.team_slug}, {data.year}
          </Heading>
          <BodyShort>
            Tallene går til og med {last_usage_day ? formatDate(last_usage_day) : "ukjent dato"}. Alle beløp er i USD.
          </BodyShort>
        </VStack>
        <PrintButton />
        <ReportNote
          label="Merknad"
          description="Vises øverst i utskriften. Står det ingenting her, blir feltet utelatt."
        />
        <div className="team-report-table overflow-x-auto">
          <Table size="small">
            <TableHeader>
              <TableRow>
                {columns.map(([, screen, print], i) => (
                  <TableHeaderCell key={screen} scope="col" align={i < 2 ? "left" : "right"}>
                    <span className="print-hidden">{screen}</span>
                    <span className="print-only">{print}</span>
                  </TableHeaderCell>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>{rows.map(row)}</TableBody>
          </Table>
        </div>
        <BodyShort size="small" className="print-only">
          * Lisenser er et anslag. «Totalt» er totalt med lisenser.
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
