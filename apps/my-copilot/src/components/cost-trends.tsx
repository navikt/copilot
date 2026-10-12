import { BodyShort, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import {
  getAdminActiveUsers,
  getAdminMonthlyOverview,
  getBillingModelBreakdown,
  getBillingMonthlyTrend,
} from "@/lib/cached-bigquery";
import { formatNumber, formatPercent, formatUSD } from "@/lib/format";
import { monthLabel } from "@/lib/trends";
import { type CostEvent, eventAnnotations, eventSummary } from "@/lib/cost-events";
import { ChartFigure } from "@/components/charts/chart-figure";
import { EventChart } from "@/components/charts/EventChart";
import type { ActiveUsersMonth, AdminMonth, BillingModelBreakdown, BillingMonthlyTrend } from "@/lib/types";

/** First month with per-user billing; earlier months are left out. */
const FIRST_MONTH = "2026-01";
const TOP_MODELS = 5;
/** The most copilot-api serves; covers January 2026 until early 2029. */
const HISTORY_MONTHS = 36;

const premium = (month: string) => month < "2026-06";

/** F3: billed amount per active user and model mix per month, chart first, for the admin cost page. */
export function CostTrendsView({
  trend,
  breakdown,
  active,
  admin = [],
  currentMonth,
  events = [],
}: {
  trend: BillingMonthlyTrend[];
  breakdown: BillingModelBreakdown[];
  active: ActiveUsersMonth[];
  /** F2 months, for the licence share; empty if they could not be read. */
  admin?: AdminMonth[];
  currentMonth: string;
  events?: CostEvent[];
}) {
  const months = trend.filter((t) => t.year_month >= FIRST_MONTH);
  if (!months.length) return <BodyShort>Ingen måneder med forbruk ennå.</BodyShort>;
  const keys = months.map((t) => t.year_month);
  const markers = eventAnnotations(events, keys);
  const activeBy = new Map(active.map((a) => [a.month, a.active_users]));
  // Licence share of the total from F2: licence estimate over usage plus licences.
  const licenceShare = new Map(
    admin
      .filter((a) => a.licence_usd_estimate != null && a.total_usd)
      .map((a) => [a.month, (a.licence_usd_estimate! / a.total_usd!) * 100])
  );
  // The current month has only the days so far, so it has no amount per user.
  const perUser = months.map((t) => {
    const users = activeBy.get(t.year_month);
    return users && t.year_month !== currentMonth ? t.total_net_amount / users : null;
  });
  const label = (month: string) =>
    monthLabel(month) + (premium(month) ? "*" : "") + (month === currentMonth ? " (hittil)" : "");

  // The models with the most billed over the period get a series each; the rest are summed as «Andre».
  const rows = breakdown.filter((b) => b.year_month >= FIRST_MONTH);
  const totals = new Map<string, number>();
  for (const b of rows) totals.set(b.model, (totals.get(b.model) ?? 0) + b.net_amount);
  const top = [...totals.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, TOP_MODELS)
    .map(([m]) => m);
  const share = (month: string, model: string | null) => {
    const inMonth = rows.filter((b) => b.year_month === month);
    const sum = inMonth.reduce((s, b) => s + b.net_amount, 0);
    if (sum <= 0) return null;
    const part = inMonth
      .filter((b) => (model === null ? !top.includes(b.model) : b.model === model))
      .reduce((s, b) => s + b.net_amount, 0);
    return (part / sum) * 100;
  };
  const mix = [...top, null].map((model) => ({ model, shares: keys.map((k) => share(k, model)) }));
  const pct = (v: number | null) => (v == null ? "–" : formatPercent(v));
  const lastPerUser = perUser.findLast((v) => v != null);
  const firstPerUser = perUser.find((v) => v != null);
  const footnote = keys.some(premium) && (
    <BodyShort size="small" textColor="subtle">
      * Fakturert som premiumforespørsler, ikke KI-kreditter.
    </BodyShort>
  );

  return (
    <VStack gap="space-24">
      <ChartFigure
        title="Blir Copilot dyrere per bruker?"
        summary={
          (firstPerUser != null && lastPerUser != null
            ? `Fakturert per aktiv bruker gikk fra ${formatUSD(firstPerUser)} i ${monthLabel(keys[perUser.indexOf(firstPerUser)])} til ${formatUSD(lastPerUser)} i ${monthLabel(keys[perUser.lastIndexOf(lastPerUser)])}.`
            : "Ingen hele måneder med aktive brukere ennå.") +
          " Aktive brukere er alle som brukte Copilot minst én dag i måneden. Denne måneden er ikke med, fordi den ikke er slutt ennå." +
          eventSummary(markers)
        }
        chart={
          <EventChart
            kind="line"
            unit="usd"
            keys={keys}
            labels={keys.map(monthLabel)}
            annotations={markers}
            series={[{ label: "Fakturert per aktiv bruker", color: 0, data: perUser }]}
          />
        }
        table={
          <>
            <Table size="small">
              <TableHeader>
                <TableRow>
                  <TableHeaderCell scope="col">Måned</TableHeaderCell>
                  <TableHeaderCell scope="col" align="right">
                    Fakturert
                  </TableHeaderCell>
                  <TableHeaderCell scope="col" align="right">
                    Aktive brukere
                  </TableHeaderCell>
                  <TableHeaderCell scope="col" align="right">
                    Per aktiv bruker
                  </TableHeaderCell>
                  <TableHeaderCell scope="col" align="right">
                    Andel lisenskostnad
                  </TableHeaderCell>
                </TableRow>
              </TableHeader>
              <TableBody>
                {months.map((t, i) => {
                  const users = activeBy.get(t.year_month);
                  return (
                    <TableRow key={t.year_month}>
                      <TableHeaderCell scope="row" className="whitespace-nowrap">
                        {label(t.year_month)}
                      </TableHeaderCell>
                      <TableDataCell align="right">{formatUSD(t.total_net_amount)}</TableDataCell>
                      <TableDataCell align="right">{users ? formatNumber(users) : "–"}</TableDataCell>
                      <TableDataCell align="right">{perUser[i] != null ? formatUSD(perUser[i]) : "–"}</TableDataCell>
                      <TableDataCell align="right">{pct(licenceShare.get(t.year_month) ?? null)}</TableDataCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
            {footnote}
            <BodyShort size="small" textColor="subtle">
              Andel lisenskostnad: lisenskostnaden (anslag til listepris, uten rabatt) som andel av totalt inkl.
              lisenser.
            </BodyShort>
          </>
        }
      />

      {top.length > 0 && (
        <ChartFigure
          title="Hvilke KI-modeller går pengene til?"
          summary={
            `Andel av det fakturerte beløpet per KI-modell. De ${TOP_MODELS} modellene med størst andel siden januar vises hver for seg, resten er samlet i «Andre». ` +
            `I ${monthLabel(keys.at(-1)!)}: ${mix
              .map((m) => `${m.model ?? "Andre"} ${pct(m.shares.at(-1) ?? null)}`)
              .join(", ")}.` +
            eventSummary(markers)
          }
          chart={
            <EventChart
              kind="bar"
              stacked
              max={100}
              unit="pct"
              keys={keys}
              labels={keys.map(monthLabel)}
              annotations={markers}
              series={mix.map((m, i) => ({ label: m.model ?? "Andre", color: i, data: m.shares }))}
            />
          }
          table={
            <>
              <Table size="small">
                <TableHeader>
                  <TableRow>
                    <TableHeaderCell scope="col">Måned</TableHeaderCell>
                    {mix.map((m) => (
                      <TableHeaderCell key={m.model ?? "andre"} scope="col" align="right">
                        {m.model ?? "Andre"}
                      </TableHeaderCell>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {keys.map((k, i) => (
                    <TableRow key={k}>
                      <TableHeaderCell scope="row" className="whitespace-nowrap">
                        {label(k)}
                      </TableHeaderCell>
                      {mix.map((m) => (
                        <TableDataCell key={m.model ?? "andre"} align="right">
                          {pct(m.shares[i])}
                        </TableDataCell>
                      ))}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              {footnote}
            </>
          }
        />
      )}
    </VStack>
  );
}

export async function CostTrendsSection({ token, events }: { token: string; events?: CostEvent[] }) {
  const [{ trend, error }, { breakdown, error: breakdownError }, active, admin] = await Promise.all([
    getBillingMonthlyTrend(token, HISTORY_MONTHS),
    getBillingModelBreakdown(token, HISTORY_MONTHS),
    getAdminActiveUsers(token).catch((err) => {
      console.error("[kostnad] Failed to fetch active users:", err);
      return null;
    }),
    getAdminMonthlyOverview(new Date().getUTCFullYear(), token)
      .then((d) => d.months)
      .catch((err) => {
        console.error("[kostnad] Failed to fetch admin monthly usage:", err);
        return null;
      }),
  ]);
  const now = new Date();
  const currentMonth = `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, "0")}`;
  return (
    <InsightSection id="trender" title="Trender">
      {error ? (
        <ErrorState message="Kunne ikke hente forbruk per måned." />
      ) : (
        <>
          {(breakdownError || !active || !admin) && (
            <ErrorState
              message={`Kunne ikke hente ${[breakdownError && "fordelingen på KI-modeller", !active && "aktive brukere", !admin && "lisenskostnaden"].filter(Boolean).join(", ")}.`}
            />
          )}
          <CostTrendsView
            trend={trend}
            breakdown={breakdown}
            active={active ?? []}
            admin={admin ?? []}
            currentMonth={currentMonth}
            events={events}
          />
        </>
      )}
    </InsightSection>
  );
}
