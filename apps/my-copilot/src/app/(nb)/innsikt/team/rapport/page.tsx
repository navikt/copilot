import type { Metadata } from "next";
import { BodyShort, Heading, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import ErrorState from "@/components/error-state";
import { getUser, getUserToken } from "@/lib/auth";
import { BackendApiError } from "@/lib/backend-api";
import { getTeamYearOverview } from "@/lib/cached-bigquery";
import { formatDate } from "@/lib/format";
import { teamYearCaveats, teamYearRows } from "@/lib/team-year";
import { PrintButton, ReportNote } from "./report-controls";

export const metadata: Metadata = {
  title: "Kostnadsrapport for team",
  description: "Utskriftsvennlig rapport over et teams Copilot-kostnad hittil i år.",
};

/** CSV columns shown in the report: måned, grunnlag, netto, brutto, netto uten bruk, lisenser, totalt. */
const columns = [0, 1, 3, 4, 5, 7, 8];
const headers = ["Måned", "Grunnlag", "Netto", "Brutto", "Netto uten bruk", "Lisenser (anslag)", "Totalt med lisenser"];

/** CSV amounts keep a decimal point; the report shows them the Norwegian way. */
const nok = (amount: string) =>
  amount === "" ? "" : Number(amount).toLocaleString("nb-NO", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

const monthName = (month: string) =>
  /^\d{4}-\d{2}$/.test(month)
    ? new Date(`${month}-01T00:00:00Z`).toLocaleString("nb-NO", { month: "long", timeZone: "UTC" })
    : month;

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

  // The same rows as the CSV. The preliminary gross months go below the total they are not part of.
  const rows = teamYearRows(data);
  const total = rows.pop()!;
  const isGross = (i: number) => data.months[i].basis === "gross";
  const billed = rows.filter((_, i) => !isGross(i));
  const gross = rows.filter((_, i) => isGross(i));
  const { last_usage_day } = data.coverage;
  // The last caveat is «Tallene går til og med …», which the report states at the top.
  const caveats = teamYearCaveats(data).slice(0, -1);

  const row = (cells: string[], key: string, bold = false) => (
    <TableRow key={key}>
      {columns.map((c, i) => (
        <TableDataCell key={c} align={i < 2 ? "left" : "right"} style={bold ? { fontWeight: 600 } : undefined}>
          {i === 0 ? monthName(cells[c]) : i < 2 ? cells[c] : nok(cells[c])}
        </TableDataCell>
      ))}
    </TableRow>
  );

  return (
    <main id="hovedinnhold" tabIndex={-1} className="team-report mx-auto max-w-4xl px-4 py-8 print:p-0">
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
                {headers.map((h, i) => (
                  <TableHeaderCell key={h} scope="col" align={i < 2 ? "left" : "right"}>
                    {h}
                  </TableHeaderCell>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {billed.map((cells) => row(cells, cells[0]))}
              {row(total, "total", true)}
              {gross.map((cells) => row(cells, cells[0]))}
            </TableBody>
          </Table>
        </div>
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
