import type { Metadata } from "next";
import { Alert, BodyShort, HGrid, Link, List, ReadMore, VStack } from "@navikt/ds-react";
import { ListItem } from "@navikt/ds-react/List";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { InsightPage, InsightSection } from "@/components/insight-page";
import { CostAnomalies } from "@/components/cost-anomalies";
import ErrorState from "@/components/error-state";
import MetricCard from "@/components/metric-card";
import { ChartFigure } from "@/components/charts/chart-figure";
import { EventChart } from "@/components/charts/EventChart";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { getAdminMonthlyOverview, getCostForecast } from "@/lib/cached-bigquery";
import { adminMonthlyCaveats, adminMonthlyCoverage, adminMonthlyRows } from "@/lib/admin-monthly";
import { costEvents, eventAnnotations, eventSummary } from "@/lib/cost-events";
import { formatDate, formatNumber, formatUSD } from "@/lib/format";
import { monthName } from "@/lib/team-year";
import { monthLabel } from "@/lib/trends";
import type { AdminMonth } from "@/lib/types";
import { CostForecastSection, ForecastMethod } from "@/components/cost-forecast";
import { CostTrendsSection } from "@/components/cost-trends";
import { HighConsumersSection } from "@/components/high-consumers";
import { CostStatusCards } from "@/components/cost-status-cards";

// Spend and limits are internal: only the copilot team sees this page, everyone else gets 404.
export const metadata: Metadata = {
  title: "Kostnadskontroll",
  description: "Forbruk og grenser for Copilot i Nav.",
  robots: { index: false, follow: false },
};

/** Index in adminMonthlyRows and the column header. The basis column is left out; a footnote marks January–May. */
const columns: [number, string][] = [
  [0, "Måned"],
  [2, "Fakturert"],
  [3, "Fordelt på personer"],
  [4, "Ikke fordelt"],
  [5, "Ikke fordelt (%)"],
  [6, "Personer uten team"],
  [7, "Foreløpig forbruk"],
  [9, "Lisenskostnad (anslag)"],
  [10, "Totalt inkl. lisenser"],
];

/** CSV amounts keep a decimal point; the page shows them the Norwegian way. */
const nok = (amount: string) =>
  amount === "" ? "" : Number(amount).toLocaleString("nb-NO", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

const billed = (m: AdminMonth) => m.basis === "net" || m.basis === "premium";

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
  // For the event markers only; the forecast section shows the error.
  const events = costEvents(await getCostForecast(token).catch(() => null));

  // The same rows as the CSV, in month order with the total last.
  const rows = adminMonthlyRows(data);
  const { last_gross_day, gross_stale, last_net_month } = data.coverage;
  const lastNet = data.months.find((m) => m.month === last_net_month);
  const current = data.months.at(-1);
  const shown = data.months.filter((m) => m.basis !== "none");
  const keys = shown.map((m) => m.month);
  const markers = eventAnnotations(events, keys);

  return (
    <InsightPage
      title="Kostnadskontroll"
      description="Forbruk og grenser for Copilot i Nav."
      intro="Siden er bare for team copilot. Alle beløp er i USD."
      updated={async () => last_gross_day || null}
      source={
        <VStack gap="space-8">
          <span>
            Tallene kommer fra copilot-api (<code>billing/admin-monthly</code>): Navs faktura per måned, fordelt på
            personer og team på samme måte som på teamsidene. {adminMonthlyCoverage(data)}
          </span>
          <ReadMore header="Slik regner prognosen" size="small">
            <ForecastMethod />
          </ReadMore>
        </VStack>
      }
    >
      <CostStatusCards token={token} />
      <InsightSection id="oversikt" title={`Oversikt ${year}`}>
        {gross_stale && (
          <Alert variant="warning" size="small">
            Foreløpig forbruk er sist hentet for {formatDate(last_gross_day)}, mer enn to døgn siden. Tallene for denne
            måneden kan være for lave.
          </Alert>
        )}
        <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
          {lastNet?.enterprise_net_usd != null && (
            <MetricCard
              value={formatUSD(lastNet.enterprise_net_usd)}
              label={`Fakturert ${monthName(lastNet.month)}`}
              helpTitle="Fakturert"
              helpText="Beløpet Nav betaler etter rabatt, for siste måned med faktura."
              subtitle={lastNet.basis === "premium" ? "Premiumforespørsler" : undefined}
            />
          )}
          {current?.gross_usd != null && (
            <MetricCard
              value={formatUSD(current.gross_usd)}
              label={`Forbruk hittil i ${monthName(current.month)}`}
              helpTitle="Foreløpig forbruk"
              helpText="Listepris for inneværende måned. Det byttes ut med fakturert beløp når fakturaen kommer, rundt den 3. i måneden etter."
              subtitle="Foreløpig forbruk"
            />
          )}
          {lastNet?.residual_pct != null && (
            <MetricCard
              value={`${lastNet.residual_pct.toLocaleString("nb-NO", { maximumFractionDigits: 2 })} %`}
              label={`Ikke fordelt ${monthName(lastNet.month)}`}
              helpTitle="Ikke fordelt"
              helpText="Andelen av det fakturerte beløpet som ikke kan knyttes til en person."
            />
          )}
          {current?.seats != null && (
            <MetricCard
              value={formatNumber(current.seats)}
              label="Lisenser"
              subtitle="Antall"
              helpTitle="Lisenser"
              helpText="Antall lisenser denne måneden."
            />
          )}
        </HGrid>
      </InsightSection>

      <InsightSection id="per-maned" title="Per måned">
        <ChartFigure
          title="Hva koster Copilot per måned?"
          summary={
            `Fakturert per måned: ${shown
              .filter(billed)
              .map((m) => `${monthName(m.month)} ${formatUSD(m.enterprise_net_usd ?? 0)}`)
              .join(", ")}.` +
            (current?.basis === "gross" && current.gross_usd != null
              ? ` Foreløpig forbruk i ${monthName(current.month)}: ${formatUSD(current.gross_usd)}.`
              : "") +
            " Lisenskostnaden er et anslag og kommer i tillegg." +
            eventSummary(markers)
          }
          chart={
            <EventChart
              kind="bar"
              stacked
              unit="usd"
              keys={keys}
              labels={keys.map(monthLabel)}
              annotations={markers}
              series={[
                {
                  label: "Fordelt på personer",
                  color: 0,
                  data: shown.map((m) => (billed(m) ? (m.assigned_net_usd ?? m.enterprise_net_usd) : null)),
                },
                { label: "Ikke fordelt", color: 3, data: shown.map((m) => (billed(m) ? m.residual_net_usd : null)) },
                {
                  label: "Foreløpig forbruk",
                  color: 5,
                  data: shown.map((m) => (m.basis === "gross" ? m.gross_usd : null)),
                },
                { label: "Lisenskostnad (anslag)", color: 2, data: shown.map((m) => m.licence_usd_estimate) },
              ]}
            />
          }
          table={
            <>
              <Table size="small">
                <TableHeader>
                  <TableRow>
                    {columns.map(([c, label], i) => (
                      <TableHeaderCell key={c} scope="col" align={i ? "right" : "left"}>
                        {label}
                      </TableHeaderCell>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.map((cells, r) => {
                    const total = r === data.months.length;
                    const m = data.months[r];
                    const preliminary = !total && m.basis === "gross";
                    return (
                      <TableRow key={cells[0]} className={preliminary ? "text-text-subtle italic" : undefined}>
                        {columns.map(([c], i) => (
                          <TableDataCell
                            key={c}
                            align={i ? "right" : "left"}
                            style={total ? { fontWeight: 600 } : undefined}
                          >
                            {i === 0 && !total
                              ? monthName(cells[c]) +
                                (m.basis === "premium" ? "*" : "") +
                                (preliminary ? " (foreløpig)" : "")
                              : i
                                ? nok(cells[c])
                                : cells[c]}
                          </TableDataCell>
                        ))}
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
              {data.months.some((m) => m.basis === "premium") && (
                <BodyShort size="small" textColor="subtle">
                  * Fakturert som premiumforespørsler, ikke KI-kreditter.
                </BodyShort>
              )}
            </>
          }
        />
        <BodyShort>
          <Link href={`/api/admin-monthly?year=${year}`} download>
            Last ned CSV
          </Link>
        </BodyShort>
        <ReadMore header="Slik leser du tabellen" size="small">
          <List size="small">
            {adminMonthlyCaveats(data).map((c) => (
              <ListItem key={c}>{c}</ListItem>
            ))}
            <ListItem>
              Antall lisenser per måned:{" "}
              {data.months
                .filter((m) => m.seats != null)
                .map((m) => `${monthName(m.month)} ${formatNumber(m.seats!)}`)
                .join(", ")}
              .
            </ListItem>
          </List>
        </ReadMore>
      </InsightSection>
      <CostForecastSection token={token} months={data.months} />
      <CostAnomalies />
      <CostTrendsSection token={token} events={events} />
      <HighConsumersSection token={token} events={events} />
    </InsightPage>
  );
}
