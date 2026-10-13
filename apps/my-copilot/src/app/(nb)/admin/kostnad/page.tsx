import type { Metadata } from "next";
import { Suspense, cache, type ReactNode } from "react";
import { SectionSkeleton } from "@/components/section-skeleton";
import type { CostEvent } from "@/lib/cost-events";
import { Alert, BodyShort, Link, List, VStack } from "@navikt/ds-react";
import { ListItem } from "@navikt/ds-react/List";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { InsightPage, InsightSection } from "@/components/insight-page";
import { MonthInsightsSection } from "@/components/cost-anomalies";
import ErrorState from "@/components/error-state";
import { ChartFigure, NumbersFold } from "@/components/charts/chart-figure";
import { EventChart } from "@/components/charts/EventChart";
import { getUserToken, requireAdmin } from "@/lib/auth";
import { getAdminMonthlyOverview, getCostForecast } from "@/lib/cached-bigquery";
import { adminMonthlyCaveats, adminMonthlyCoverage, adminMonthlyRows, rebuiltSeatMonths } from "@/lib/admin-monthly";
import { costEvents, eventAnnotations, eventSummary } from "@/lib/cost-events";
import { formatDate, formatNumber, formatUSD } from "@/lib/format";
import { monthName } from "@/lib/team-year";
import { monthLabel } from "@/lib/trends";
import type { AdminMonth } from "@/lib/types";
import { CostForecastSection, ForecastMethod } from "@/components/cost-forecast";
import { CostTrendsSection } from "@/components/cost-trends";
import { HighConsumersSection } from "@/components/high-consumers";
import { BudgetDaysSection } from "@/components/budget-days-section";
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
  [11, "Estimert fakturert hittil"],
  [9, "Lisenskostnad (anslag)"],
  [10, "Totalt inkl. lisenser"],
];

/** CSV amounts keep a decimal point; the page shows them the Norwegian way. */
const nok = (amount: string) =>
  amount === "" ? "" : Number(amount).toLocaleString("nb-NO", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

const billed = (m: AdminMonth) => m.basis === "net" || m.basis === "premium";

// getAdminMonthlyOverview and getCostForecast are react-cached, so the streamed sections share one call each.
// The status card and Månedsinnsikt share loadMonthInsights (React cache).
const loadMonthly = cache((token: string) => getAdminMonthlyOverview(new Date().getUTCFullYear(), token));
// For the event markers only; the forecast section shows the error.
const loadEvents = cache(async (token: string) => costEvents(await getCostForecast(token).catch(() => null)));

async function Coverage({ token }: { token: string }) {
  const data = await loadMonthly(token).catch(() => null);
  return data ? adminMonthlyCoverage(data) : null;
}

async function Overview({ token }: { token: string }) {
  const data = await loadMonthly(token).catch(() => null);
  const current = data?.months.at(-1);
  return (
    <>
      {data?.coverage.gross_stale && (
        <Alert variant="warning" size="small">
          Foreløpig forbruk er sist hentet for {formatDate(data.coverage.last_gross_day)}, mer enn to døgn siden.
          Tallene for denne måneden kan være for lave.
        </Alert>
      )}
      <CostStatusCards token={token} grossToDate={current?.basis === "gross" ? current.gross_usd : null} />
    </>
  );
}

async function Forecast({ token }: { token: string }) {
  const data = await loadMonthly(token).catch(() => null);
  return <CostForecastSection token={token} months={data?.months} />;
}

async function WithEvents({
  token,
  Section,
}: {
  token: string;
  Section: (props: { token: string; events?: CostEvent[] }) => Promise<ReactNode>;
}) {
  return <Section token={token} events={await loadEvents(token)} />;
}

async function PerMonth({ token, year }: { token: string; year: number }) {
  let data;
  try {
    data = await loadMonthly(token);
  } catch (error) {
    console.error("[kostnad] Failed to fetch admin monthly usage:", error);
    return <ErrorState message="Kunne ikke hente Navs forbruk." />;
  }
  const events = await loadEvents(token);

  // The same rows as the CSV, in month order with the total last.
  const rows = adminMonthlyRows(data);
  const current = data.months.at(-1);
  const shown = data.months.filter((m) => m.basis !== "none");
  const keys = shown.map((m) => m.month);
  const markers = eventAnnotations(events, keys);
  const rebuilt = new Set(rebuiltSeatMonths(data));

  return (
    <InsightSection id="per-maned" title="Per måned">
      <ChartFigure
        title="Hva koster Copilot per måned?"
        summary={
          `Fakturert per måned: ${shown
            .filter(billed)
            .map((m) => `${monthName(m.month)} ${formatUSD(m.enterprise_net_usd ?? 0)}`)
            .join(", ")}.` +
          (current?.basis === "gross" && current.estimated_net_usd != null
            ? ` Estimert fakturert hittil i ${monthName(current.month)} (foreløpig): ${formatUSD(current.estimated_net_usd)}.`
            : "") +
          " Lisenskostnaden er et anslag og kommer i tillegg. Slå den på i forklaringen under grafen." +
          (rebuilt.size ? " Lisensene før daglige tellinger er beregnet ut fra dagens lisenser." : "") +
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
                data: shown.map((m) =>
                  billed(m) ? (m.assigned_net_usd ?? (m.residual_net_usd == null ? m.enterprise_net_usd : null)) : null
                ),
              },
              { label: "Ikke fordelt", color: 3, data: shown.map((m) => (billed(m) ? m.residual_net_usd : null)) },
              {
                label: "Estimert fakturert hittil (foreløpig)",
                color: 5,
                data: shown.map((m) => (m.basis === "gross" ? m.estimated_net_usd : null)),
              },
              {
                label: "Lisenskostnad (anslag)",
                color: 2,
                hidden: true,
                data: shown.map((m) => m.licence_usd_estimate),
              },
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
                              ? nok(cells[c]) + (c === 9 && !total && cells[c] && rebuilt.has(m.month) ? "†" : "")
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
            {rebuilt.size > 0 && (
              <BodyShort size="small" textColor="subtle">
                † Beregnet ut fra dagens lisenser, for lavt for tidligere måneder.
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
      <NumbersFold summary="Slik leser du tallene">
        <List size="small">
          {adminMonthlyCaveats(data).map((c) => (
            <ListItem key={c}>{c}</ListItem>
          ))}
          <ListItem>
            Antall lisenser per måned:{" "}
            {data.months
              .filter((m) => m.seats != null)
              .map((m) => `${monthName(m.month)} ${formatNumber(m.seats!)}${rebuilt.has(m.month) ? "†" : ""}`)
              .join(", ")}
            .{rebuilt.size > 0 && " † Beregnet ut fra dagens lisenser, for lavt for tidligere måneder."}
          </ListItem>
        </List>
      </NumbersFold>
    </InsightSection>
  );
}

// The shell renders at once; each section streams in behind a skeleton of its own shape.
export default async function CostPage() {
  await requireAdmin();
  const token = await getUserToken();
  if (!token) return <ErrorState message="Mangler innloggingstoken" />;
  const year = new Date().getUTCFullYear();

  return (
    <InsightPage
      title="Kostnadskontroll"
      description="Forbruk og grenser for Copilot i Nav."
      intro="Siden er bare for team copilot. Alle beløp er i USD."
      updated={async () => (await loadMonthly(token)).coverage.last_gross_day || null}
      source={
        <VStack gap="space-8">
          <span>
            Tallene kommer fra copilot-api (<code>admin/billing/monthly</code>): Navs faktura per måned, fordelt på
            personer og team på samme måte som på teamsidene.{" "}
            <Suspense fallback={null}>
              <Coverage token={token} />
            </Suspense>
          </span>
          <NumbersFold summary="Slik regner vi ut prognosen">
            <ForecastMethod />
          </NumbersFold>
        </VStack>
      }
    >
      {/* One row of status cards; «oversikt» was the old second card row, kept for links. */}
      <VStack id="oversikt" gap="space-16">
        <Suspense fallback={<SectionSkeleton cards={4} chart={false} />}>
          <Overview token={token} />
        </Suspense>
      </VStack>
      <Suspense fallback={<SectionSkeleton rows={2} />}>
        <PerMonth token={token} year={year} />
      </Suspense>
      <Suspense fallback={<SectionSkeleton cards={3} />}>
        <Forecast token={token} />
      </Suspense>
      <Suspense fallback={<SectionSkeleton chart={false} rows={4} />}>
        <MonthInsightsSection token={token} />
      </Suspense>
      <Suspense fallback={<SectionSkeleton />}>
        <WithEvents token={token} Section={CostTrendsSection} />
      </Suspense>
      <Suspense fallback={<SectionSkeleton />}>
        <WithEvents token={token} Section={HighConsumersSection} />
      </Suspense>
      <Suspense fallback={<SectionSkeleton />}>
        <WithEvents token={token} Section={BudgetDaysSection} />
      </Suspense>
    </InsightPage>
  );
}
