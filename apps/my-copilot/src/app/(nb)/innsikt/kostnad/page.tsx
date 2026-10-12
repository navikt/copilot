import type { Metadata } from "next";
import { Alert, BodyShort, HGrid, Link, List } from "@navikt/ds-react";
import { ListItem } from "@navikt/ds-react/List";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { InsightPage, InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import MetricCard from "@/components/metric-card";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { getAdminMonthlyOverview } from "@/lib/cached-bigquery";
import { adminMonthlyCaveats, adminMonthlyRows } from "@/lib/admin-monthly";
import { formatDate, formatNumber, formatUSD } from "@/lib/format";
import { monthName } from "@/lib/team-year";

// Spend and limits are internal: only the copilot team sees this page, everyone else gets 404.
export const metadata: Metadata = {
  title: "Kostnad",
  description: "Forbruk og grenser for Copilot i Nav.",
  robots: { index: false, follow: false },
};

/** Index in adminMonthlyRows and the column header. */
const columns: [number, string][] = [
  [0, "Måned"],
  [1, "Grunnlag"],
  [2, "Netto"],
  [3, "Fordelt på personer"],
  [4, "Restbeløp"],
  [5, "Restbeløp (%)"],
  [6, "Uten team"],
  [7, "Brutto"],
  [8, "Lisenser"],
  [9, "Lisenser (anslag)"],
  [10, "Totalt med lisenser"],
];

/** CSV amounts keep a decimal point; the page shows them the Norwegian way. */
const nok = (amount: string) =>
  amount === "" ? "" : Number(amount).toLocaleString("nb-NO", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

export default async function CostPage() {
  await requireAdmin();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;
  const year = new Date().getUTCFullYear();

  let data;
  try {
    data = await getAdminMonthlyOverview(year, token);
  } catch (error) {
    console.error("[kostnad] Failed to fetch admin monthly usage:", error);
    return <ErrorState message="Kunne ikke hente Navs forbruk." />;
  }

  // The same rows as the CSV, in month order with the total last.
  const rows = adminMonthlyRows(data);
  const { last_gross_day, gross_stale, last_net_month } = data.coverage;
  const lastNet = data.months.find((m) => m.month === last_net_month);
  const current = data.months.at(-1);

  return (
    <InsightPage
      title="Kostnad"
      description="Forbruk og grenser for Copilot i Nav."
      intro="Siden er bare for team copilot. Alle beløp er i USD."
      updated={async () => last_gross_day || null}
      source={
        <>
          Tallene kommer fra copilot-api (<code>billing/admin-monthly</code>): Navs faktura per måned, fordelt på
          personer og team på samme måte som på teamsidene.
        </>
      }
    >
      <InsightSection id="oversikt" title={`Oversikt ${year}`}>
        {gross_stale && (
          <Alert variant="warning" size="small">
            Brutto er sist hentet for {formatDate(last_gross_day)}, mer enn to døgn siden. Tallene for denne måneden kan
            være for lave.
          </Alert>
        )}
        <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
          {lastNet?.enterprise_net_usd != null && (
            <MetricCard
              value={formatUSD(lastNet.enterprise_net_usd)}
              label={`Netto ${monthName(lastNet.month)}`}
              helpTitle="Netto"
              helpText="Navs faktura etter fradrag for siste måned som er lest inn."
              subtitle={lastNet.basis === "premium" ? "Premiumforespørsler" : "Netto"}
            />
          )}
          {current?.gross_usd != null && (
            <MetricCard
              value={formatUSD(current.gross_usd)}
              label={`Hittil i ${monthName(current.month)}`}
              helpTitle="Brutto"
              helpText="Beløpene er før fradrag. De erstattes av netto når fakturaen for måneden er lest inn."
              subtitle="Brutto (foreløpig)"
            />
          )}
          {lastNet?.residual_pct != null && (
            <MetricCard
              value={`${lastNet.residual_pct.toLocaleString("nb-NO", { maximumFractionDigits: 2 })} %`}
              label={`Restbeløp ${monthName(lastNet.month)}`}
              helpTitle="Restbeløp"
              helpText="Andelen av netto som ikke kan knyttes til en person."
            />
          )}
          {current?.seats != null && (
            <MetricCard
              value={formatNumber(current.seats)}
              label="Lisenser"
              helpTitle="Lisenser"
              helpText="Antall lisenser som gjelder denne måneden."
            />
          )}
        </HGrid>
        <BodyShort size="small" textColor="subtle">
          Siste dag med brutto: {last_gross_day ? formatDate(last_gross_day) : "ukjent"}. Siste måned med netto:{" "}
          {last_net_month ? monthName(last_net_month) : "ingen"}.
        </BodyShort>
      </InsightSection>

      <InsightSection id="per-maned" title="Per måned">
        <div className="overflow-x-auto">
          <Table size="small">
            <TableHeader>
              <TableRow>
                {columns.map(([c, label], i) => (
                  <TableHeaderCell key={c} scope="col" align={i < 2 ? "left" : "right"}>
                    {label}
                  </TableHeaderCell>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((cells, r) => {
                const total = r === data.months.length;
                const preliminary = !total && data.months[r].basis === "gross";
                return (
                  <TableRow key={cells[0]} className={preliminary ? "text-text-subtle italic" : undefined}>
                    {columns.map(([c], i) => (
                      <TableDataCell
                        key={c}
                        align={i < 2 ? "left" : "right"}
                        style={total ? { fontWeight: 600 } : undefined}
                      >
                        {i === 0 && !total
                          ? monthName(cells[c])
                          : i < 2
                            ? cells[c]
                            : c === 8
                              ? cells[c] && formatNumber(Number(cells[c]))
                              : nok(cells[c])}
                      </TableDataCell>
                    ))}
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
        <BodyShort>
          <Link href={`/api/admin-monthly?year=${year}`} download>
            Last ned CSV
          </Link>
        </BodyShort>
        <List size="small">
          {adminMonthlyCaveats(data).map((c) => (
            <ListItem key={c}>{c}</ListItem>
          ))}
        </List>
      </InsightSection>
    </InsightPage>
  );
}
