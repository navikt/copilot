/**
 * Backend data fetchers for Copilot analytics.
 *
 * Caching is owned entirely by the backend (copilot-api has a 1 h in-memory
 * cache). These functions are thin proxies — they do NOT add a second BFF
 * cache layer. The "cached-bigquery" file name is kept for now to avoid a
 * large import-path churn while this file is the subject of a rename refactor.
 */
import { backendRequest, BackendApiError } from "./backend-api";
import type {
  AdoptionData,
  AdoptionSummary,
  CustomizationDetail,
  CustomizationUsage,
  EnterpriseMetrics,
  LanguageAdoption,
  MonthlyBillingUsage,
  BillingModelDailyCost,
  BillingModelForecast,
  MonthlyTrend,
  StalenessSummary,
  TeamAdoption,
  TeamUsageSummary,
  UserMetricsSummary,
  WeeklyTrend,
  AdoptionCohortDay,
  BillingMonthlyTrend,
  BillingModelBreakdown,
  DailySummary,
  UsageDistribution,
} from "./types";

function getErrorMessage(label: string, err: unknown): string {
  const message = err instanceof Error ? err.message : String(err);
  console.error(`[copilot-data] ${label} failed:`, err);
  return message;
}

async function fetchNullable<T>(
  label: string,
  fetcher: () => Promise<T>
): Promise<{ data: T | null; error: string | null }> {
  try {
    const data = await fetcher();
    return { data, error: null };
  } catch (err) {
    return { data: null, error: getErrorMessage(label, err) };
  }
}

async function fetchWithFallback<T>(
  label: string,
  fallback: T,
  fetcher: () => Promise<T>
): Promise<{ data: T; error: string | null }> {
  try {
    const data = await fetcher();
    return { data, error: null };
  } catch (err) {
    return { data: fallback, error: getErrorMessage(label, err) };
  }
}

export async function getCopilotUsageMetrics(token: string): Promise<{
  usage: EnterpriseMetrics[] | null;
  error: string | null;
}> {
  const result = await fetchNullable("getCopilotUsageMetrics", () =>
    backendRequest<EnterpriseMetrics[]>("/api/v1/copilot/usage/metrics", token)
  );
  return { usage: result.data, error: result.error };
}

export async function getAdoptionData(token: string): Promise<{
  data: AdoptionData | null;
  error: string | null;
}> {
  return fetchNullable("getAdoptionData", async () => {
    const [summary, teams, languages, customizationDetails] = await Promise.all([
      backendRequest<AdoptionSummary>("/api/v1/copilot/adoption/summary", token),
      backendRequest<TeamAdoption[]>("/api/v1/copilot/adoption/teams", token),
      backendRequest<LanguageAdoption[]>("/api/v1/copilot/adoption/languages", token),
      backendRequest<CustomizationDetail[]>("/api/v1/copilot/customizations/details", token),
    ]);

    return { summary, teams, languages, customizationDetails };
  });
}

export async function getCustomizationUsage(token: string): Promise<{
  usage: CustomizationUsage[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getCustomizationUsage", [] as CustomizationUsage[], () =>
    backendRequest<CustomizationUsage[]>("/api/v1/copilot/customizations/usage", token)
  );
  return { usage: result.data, error: result.error };
}

export async function getStalenessData(token: string): Promise<{
  data: StalenessSummary | null;
  error: string | null;
}> {
  return fetchNullable("getStalenessData", () =>
    backendRequest<StalenessSummary>("/api/v1/copilot/adoption/staleness", token)
  );
}

export async function getTeamUsage(token: string): Promise<{
  teams: TeamUsageSummary[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getTeamUsage", [] as TeamUsageSummary[], () =>
    backendRequest<TeamUsageSummary[]>("/api/v1/copilot/usage/team-summary", token)
  );
  return { teams: result.data, error: result.error };
}

export async function getUserMetrics(
  username: string,
  token: string
): Promise<{ metrics: UserMetricsSummary | null; error: string | null }> {
  const result = await fetchNullable("getUserMetrics", async () => {
    try {
      return await backendRequest<UserMetricsSummary>(
        `/api/v1/copilot/usage/user/${encodeURIComponent(username)}`,
        token
      );
    } catch (err) {
      // A 404 is a valid state: the user is linked to a GitHub account but has
      // no Copilot usage in the period. Treat as "no metrics", not an error.
      if (err instanceof BackendApiError && err.status === 404) {
        return null;
      }
      throw err;
    }
  });
  return { metrics: result.data, error: result.error };
}

export async function getUserWeeklyTrends(
  username: string,
  token: string
): Promise<{ trends: WeeklyTrend[]; error: string | null }> {
  const result = await fetchWithFallback("getUserWeeklyTrends", [] as WeeklyTrend[], () =>
    backendRequest<WeeklyTrend[]>(`/api/v1/copilot/usage/user/${encodeURIComponent(username)}/weekly`, token)
  );
  return { trends: result.data, error: result.error };
}

export async function getMonthlyTrends(token: string): Promise<{
  trends: MonthlyTrend[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getMonthlyTrends", [] as MonthlyTrend[], () =>
    backendRequest<MonthlyTrend[]>("/api/v1/copilot/usage/trends", token)
  );
  return { trends: result.data, error: result.error };
}

export async function getMonthlyBillingUsage(token: string): Promise<{
  usage: MonthlyBillingUsage[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getMonthlyBillingUsage", [] as MonthlyBillingUsage[], () =>
    backendRequest<MonthlyBillingUsage[]>("/api/v1/copilot/billing/monthly", token)
  );
  return { usage: result.data, error: result.error };
}

export async function getAdoptionCohorts(token: string): Promise<{
  cohorts: AdoptionCohortDay[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getAdoptionCohorts", [] as AdoptionCohortDay[], () =>
    backendRequest<AdoptionCohortDay[]>("/api/v1/copilot/adoption/cohorts", token)
  );
  return { cohorts: result.data, error: result.error };
}

export async function getBillingModelDaily(
  token: string,
  month?: string
): Promise<{
  usage: BillingModelDailyCost[];
  error: string | null;
}> {
  const query = month ? `?month=${encodeURIComponent(month)}` : "";
  const result = await fetchNullable("getBillingModelDaily", () =>
    backendRequest<BillingModelDailyCost[] | null>(`/api/v1/copilot/billing/model-daily${query}`, token)
  );
  return { usage: Array.isArray(result.data) ? result.data : [], error: result.error };
}

export async function getBillingModelForecast(
  token: string,
  month?: string
): Promise<{
  forecast: BillingModelForecast | null;
  error: string | null;
}> {
  const query = month ? `?month=${encodeURIComponent(month)}` : "";
  const result = await fetchNullable("getBillingModelForecast", () =>
    backendRequest<BillingModelForecast>(`/api/v1/copilot/billing/model-forecast${query}`, token)
  );
  return { forecast: result.data, error: result.error };
}

export async function getBillingMonthlyTrend(token: string): Promise<{
  trend: BillingMonthlyTrend[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getBillingMonthlyTrend", [] as BillingMonthlyTrend[], () =>
    backendRequest<BillingMonthlyTrend[]>("/api/v1/copilot/billing/monthly-trend", token)
  );
  return { trend: result.data, error: result.error };
}

export async function getBillingModelBreakdown(token: string): Promise<{
  breakdown: BillingModelBreakdown[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getBillingModelBreakdown", [] as BillingModelBreakdown[], () =>
    backendRequest<BillingModelBreakdown[]>("/api/v1/copilot/billing/model-breakdown", token)
  );
  return { breakdown: result.data, error: result.error };
}

export async function getDailySummary(token: string): Promise<{
  summary: DailySummary | null;
  error: string | null;
}> {
  const result = await fetchNullable("getDailySummary", () =>
    backendRequest<DailySummary>("/api/v1/copilot/usage/daily-summary", token)
  );
  return { summary: result.data, error: result.error };
}

export async function getUsageDistribution(
  token: string,
  month?: string
): Promise<{
  distribution: UsageDistribution | null;
  error: string | null;
}> {
  const query = month ? `?month=${encodeURIComponent(month)}` : "";
  const result = await fetchNullable("getUsageDistribution", () =>
    backendRequest<UsageDistribution>(`/api/v1/copilot/usage/distribution${query}`, token)
  );
  return { distribution: result.data, error: result.error };
}
