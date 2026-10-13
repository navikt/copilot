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
import { familyShares, monthLabel } from "@/lib/trends";
import type { ModelFamily } from "@/lib/model-family";
import { type CostEvent, eventAnnotations, eventSummary } from "@/lib/cost-events";
import { ChartFigure } from "@/components/charts/chart-figure";
import { EventChart } from "@/components/charts/EventChart";
import type { ActiveUsersMonth, AdminMonth, BillingModelBreakdown, BillingMonthlyTrend } from "@/lib/types";

/** First month with per-user billing; earlier months are left out. */
const FIRST_MONTH = "2026-01";
/** The most copilot-api serves; covers January 2026 until early 2029. */
const HISTORY_MONTHS = 36;

/** Fixed palette index per family (axSeries has 8): «Andre» is grey; Haiku, Fable and Gemini are all small and share one. */
const FAMILY_COLOR: Record<ModelFamily, number> = {
  claude_opus: 0,
  claude_sonnet: 1,
  claude_haiku: 2,
  claude_fable: 2,
  kodegjennomgang: 3,
  gpt_mini: 4,
  andre: 5,
  gemini: 2,
  auto: 6,
  gpt: 7,
};

const premium = (month: string) => month < "2026-06";

/** F3: billed amount per billed user and model mix per month, chart first, for the admin cost page. */
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
  // The current month is a partial point: estimated billed so far over users active so far.
  const perUser = months.map((t) => {
    const users = activeBy.get(t.year_month);
    return users ? t.total_net_amount / users : null;
  });
  // The trend in the summary uses whole, billed months only.
  const billedMonths = new Set(active.filter((a) => a.billed).map((a) => a.month));
  const full = perUser.map((v, i) => (billedMonths.has(keys[i]) ? v : null));
  const label = (month: string) =>
    monthLabel(month) + (premium(month) ? "*" : "") + (month === currentMonth ? " (hittil)" : "");

  // One series per model family, so new versions land in their family and «Andre» stays small.
  const families = familyShares(
    breakdown.filter((b) => b.year_month >= FIRST_MONTH),
    keys,
    true
  ).series;
  const mix = families.map((f) => ({ model: f.label, shares: f.shares }));
  const pct = (v: number | null) => (v == null ? "–" : formatPercent(v));
  const lastPerUser = full.findLast((v) => v != null);
  const firstPerUser = full.find((v) => v != null);
  const partial = keys.includes(currentMonth) && perUser[keys.indexOf(currentMonth)] != null;
  const footnote = keys.some(premium) && (
    <BodyShort size="small" textColor="subtle">
      * Fakturert som premiumforespørsler, ikke KI-kreditter.
    </BodyShort>
  );

  return (
    <VStack gap="space-24">
      <ChartFigure
        title="Hvor mye koster Copilot per person?"
        summary={
          (firstPerUser != null && lastPerUser != null
            ? `Fakturert per fakturert bruker gikk fra ${formatUSD(firstPerUser)} i ${monthLabel(keys[full.indexOf(firstPerUser)])} til ${formatUSD(lastPerUser)} i ${monthLabel(keys[full.lastIndexOf(lastPerUser)])}.`
            : "Ingen fakturerte måneder ennå.") +
          " Fakturerte brukere er alle som står på fakturaen for måneden." +
          (partial ? ` ${monthLabel(currentMonth)} er et foreløpig punkt (hittil), se tabellen.` : "") +
          eventSummary(markers)
        }
        chart={
          <EventChart
            kind="line"
            unit="usd"
            keys={keys}
            labels={keys.map((k) => monthLabel(k) + (k === currentMonth ? " (hittil)" : ""))}
            annotations={markers}
            series={[{ label: "Fakturert per fakturert bruker", color: 0, data: perUser }]}
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
                    Fakturerte brukere
                  </TableHeaderCell>
                  <TableHeaderCell scope="col" align="right">
                    Per fakturert bruker
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
            {partial && (
              <BodyShort size="small" textColor="subtle">
                Hittil: fakturaen for {monthLabel(currentMonth)} er ikke kommet. Raden viser estimert fakturert hittil
                delt på brukere med Copilot-aktivitet hittil i måneden. Den er foreløpig og kan ikke sammenlignes med
                hele måneder.
              </BodyShort>
            )}
            <BodyShort size="small" textColor="subtle">
              Andel lisenskostnad: lisenskostnaden (anslag til listepris, uten rabatt) som andel av totalt inkl.
              lisenser.
            </BodyShort>
          </>
        }
      />

      {mix.length > 0 && (
        <ChartFigure
          title="Hvilke KI-modeller går pengene til?"
          summary={
            `Andel av det fakturerte beløpet per modellfamilie. Automatisk modellvalg er samlet i «Auto». Kodegjennomgang er modellen Copilot bruker til å gå gjennom pull requests. ` +
            `I ${monthLabel(keys.at(-1)!)}: ${mix
              .map((m) => `${m.model} ${pct(m.shares.at(-1) ?? null)}`)
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
              series={families.map((f) => ({
                label: f.label,
                // Colour by family, so a family keeps its colour whichever families a period has.
                color: FAMILY_COLOR[f.family],
                data: f.shares,
                pointLabels: f.models.map((m) =>
                  m.length && f.family !== "kodegjennomgang"
                    ? `${f.label} (${m
                        .slice(0, 3)
                        .map((x) => x.replace(/^Auto: /, ""))
                        .join(", ")})`
                    : null
                ),
              }))}
            />
          }
          table={
            <>
              <Table size="small">
                <TableHeader>
                  <TableRow>
                    <TableHeaderCell scope="col">Måned</TableHeaderCell>
                    {mix.map((m) => (
                      <TableHeaderCell key={m.model} scope="col" align="right">
                        {m.model}
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
                        <TableDataCell key={m.model} align="right">
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
