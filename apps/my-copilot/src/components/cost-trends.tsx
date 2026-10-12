import { BodyShort, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { getAdminActiveUsers, getBillingModelBreakdown, getBillingMonthlyTrend } from "@/lib/cached-bigquery";
import { formatNumber, formatPercent, formatUSD } from "@/lib/format";
import { monthLabel } from "@/lib/trends";
import type { ActiveUsersMonth, BillingModelBreakdown, BillingMonthlyTrend } from "@/lib/types";

/** First month with per-user billing; earlier months are left out. */
const FIRST_MONTH = "2026-01";
const TOP_MODELS = 5;

const basis = (month: string, current: string) =>
  month === current ? "Hittil" : month < "2026-06" ? "Premium" : "Netto";

/** F3: net, active users, net per active user and model mix per month for the admin cost page. */
export function CostTrendsView({
  trend,
  breakdown,
  active,
  currentMonth,
}: {
  trend: BillingMonthlyTrend[];
  breakdown: BillingModelBreakdown[];
  active: ActiveUsersMonth[];
  currentMonth: string;
}) {
  const months = trend.filter((t) => t.year_month >= FIRST_MONTH);
  if (!months.length) return <BodyShort>Ingen måneder med forbruk ennå.</BodyShort>;
  const activeBy = new Map(active.map((a) => [a.month, a.active_users]));

  // The models with the most net over the period get a column each; the rest are summed as «Andre».
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
    if (sum <= 0) return "–";
    const part = inMonth
      .filter((b) => (model === null ? !top.includes(b.model) : b.model === model))
      .reduce((s, b) => s + b.net_amount, 0);
    return formatPercent((part / sum) * 100);
  };

  return (
    <VStack gap="space-16">
      <div className="min-w-0 overflow-x-auto">
        <Table size="small">
          <TableHeader>
            <TableRow>
              <TableHeaderCell scope="col">Måned</TableHeaderCell>
              <TableHeaderCell scope="col">Grunnlag</TableHeaderCell>
              <TableHeaderCell scope="col" align="right">
                Forbruk
              </TableHeaderCell>
              <TableHeaderCell scope="col" align="right">
                Aktive brukere
              </TableHeaderCell>
              <TableHeaderCell scope="col" align="right">
                Per aktiv bruker
              </TableHeaderCell>
            </TableRow>
          </TableHeader>
          <TableBody>
            {months.map((t) => {
              const users = activeBy.get(t.year_month);
              return (
                <TableRow key={t.year_month}>
                  <TableHeaderCell scope="row" className="whitespace-nowrap">
                    {monthLabel(t.year_month)}
                  </TableHeaderCell>
                  <TableDataCell>{basis(t.year_month, currentMonth)}</TableDataCell>
                  <TableDataCell align="right">{formatUSD(t.total_net_amount)}</TableDataCell>
                  <TableDataCell align="right">{users ? formatNumber(users) : "–"}</TableDataCell>
                  <TableDataCell align="right">{users ? formatUSD(t.total_net_amount / users) : "–"}</TableDataCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>
      <BodyShort size="small" textColor="subtle">
        Aktive brukere er alle som brukte Copilot minst én dag i måneden. Forbruk per aktiv bruker er forbruket delt på
        dem. Januar til mai er premiumforespørsler. Denne måneden er bare dagene hittil.
      </BodyShort>

      {top.length > 0 && (
        <div className="min-w-0 overflow-x-auto">
          <Table size="small">
            <TableHeader>
              <TableRow>
                <TableHeaderCell scope="col">Måned</TableHeaderCell>
                {top.map((m) => (
                  <TableHeaderCell key={m} scope="col" align="right">
                    {m}
                  </TableHeaderCell>
                ))}
                <TableHeaderCell scope="col" align="right">
                  Andre
                </TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              {months.map((t) => (
                <TableRow key={t.year_month}>
                  <TableHeaderCell scope="row" className="whitespace-nowrap">
                    {monthLabel(t.year_month)}
                  </TableHeaderCell>
                  {top.map((m) => (
                    <TableDataCell key={m} align="right">
                      {share(t.year_month, m)}
                    </TableDataCell>
                  ))}
                  <TableDataCell align="right">{share(t.year_month, null)}</TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      <BodyShort size="small" textColor="subtle">
        Andel av forbruket per modell. De {TOP_MODELS} modellene med mest forbruk siden januar har egen kolonne.
      </BodyShort>
    </VStack>
  );
}

export async function CostTrendsSection({ token }: { token: string }) {
  const [{ trend, error }, { breakdown }, active] = await Promise.all([
    getBillingMonthlyTrend(token),
    getBillingModelBreakdown(token, 12),
    getAdminActiveUsers(token).catch((err) => {
      console.error("[kostnad] Failed to fetch active users:", err);
      return [] as ActiveUsersMonth[];
    }),
  ]);
  const now = new Date();
  const currentMonth = `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, "0")}`;
  return (
    <InsightSection id="trender" title="Forbruk per måned">
      {error ? (
        <ErrorState message="Kunne ikke hente forbruk per måned." />
      ) : (
        <CostTrendsView trend={trend} breakdown={breakdown} active={active} currentMonth={currentMonth} />
      )}
    </InsightSection>
  );
}
