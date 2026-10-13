import { BodyShort, Detail, Link, VStack } from "@navikt/ds-react";
import { List, ListItem } from "@navikt/ds-react/List";
import { cache } from "react";
import { NumbersFold } from "@/components/charts/chart-figure";
import { InsightSection } from "@/components/insight-page";
import { backendRequest } from "@/lib/backend-api";
import {
  getAdminActiveUsers,
  getAdminBudgetDays,
  getAdminHighConsumers,
  getAdminMonthlyOverview,
  getBillingModelBreakdown,
  getBillingModelDaily,
  getBillingMonthlyTrend,
  getCostForecast,
} from "@/lib/cached-bigquery";
import { monthInsights, type Anomaly, type MonthInsight, type ModelFamilyUsers } from "@/lib/month-insights";

/** Same window as the trend charts, so the requests are shared. */
const HISTORY_MONTHS = 36;

const log = (what: string) => (error: unknown) => {
  console.error(`[kostnad] ${what} failed:`, error);
  return null;
};

/** Every source for «Månedsinnsikt». A source that fails drops the statements that need it. Shared by the status card. */
export const loadMonthInsights = cache(async (token: string): Promise<MonthInsight[]> => {
  const now = new Date();
  const year = now.getUTCFullYear();
  const prev = new Date(Date.UTC(year, now.getUTCMonth() - 1, 1)).toISOString().slice(0, 7);
  // In January and February the two last closed months reach into last year.
  const years = now.getUTCMonth() < 2 ? [year - 1, year] : [year];
  const [forecast, prevDaily, anomalies, budgetDays, trend, active, breakdown, familyUsers, high, overviews] =
    await Promise.all([
      getCostForecast(token).catch(log("Forecast")),
      getBillingModelDaily(token, prev).catch(() => ({ usage: [] })),
      backendRequest<Anomaly[]>("/api/v1/admin/billing/anomalies", token).catch(log("Anomalies")),
      getAdminBudgetDays(token).catch(log("Budget days")),
      getBillingMonthlyTrend(token, HISTORY_MONTHS).catch(() => ({ trend: [] })),
      getAdminActiveUsers(token).catch(log("Active users")),
      getBillingModelBreakdown(token, HISTORY_MONTHS).catch(() => ({ breakdown: [] })),
      backendRequest<ModelFamilyUsers[]>("/api/v1/admin/billing/model-users", token).catch(log("Model users")),
      getAdminHighConsumers(token).catch(log("High consumers")),
      Promise.all(years.map((y) => getAdminMonthlyOverview(y, token))).catch(log("Monthly overview")),
    ]);
  return monthInsights({
    forecast,
    prevDaily: prevDaily.usage,
    anomalies,
    budgetDays,
    trend: trend.trend,
    active: active ?? [],
    breakdown: breakdown.breakdown,
    familyUsers,
    high,
    months: (overviews ?? []).flatMap((o) => o.months),
  });
});

export function MonthInsightList({ insights }: { insights: MonthInsight[] }) {
  if (insights.length === 0) return <BodyShort>Ingenting å melde denne måneden.</BodyShort>;
  return (
    <List>
      {insights.map((i) => (
        <ListItem key={`${i.id}-${i.text}`}>
          <VStack gap="space-4">
            <BodyShort>
              {i.text} <Link href={i.href}>Se grafen</Link>
            </BodyShort>
            <Detail>
              <strong>Hva kan vi gjøre?</strong> {i.action}
            </Detail>
            {i.details && i.details.length > 0 && (
              <NumbersFold summary="Detaljer">
                <List size="small">
                  {i.details.map((d) => (
                    <ListItem key={d}>{d}</ListItem>
                  ))}
                </List>
              </NumbersFold>
            )}
          </VStack>
        </ListItem>
      ))}
    </List>
  );
}

export async function MonthInsightsSection({ token }: { token: string }) {
  return (
    <InsightSection id="maanedsinnsikt" title="Månedsinnsikt">
      <BodyShort>Denne måneden mot forrige, og hva vi kan gjøre med det.</BodyShort>
      <MonthInsightList insights={await loadMonthInsights(token)} />
    </InsightSection>
  );
}
