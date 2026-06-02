import { backendRequest, BackendApiError } from "./backend-api";
import type {
  AdoptionData,
  AdoptionSummary,
  CustomizationDetail,
  CustomizationUsage,
  EnterpriseMetrics,
  LanguageAdoption,
  MonthlyBillingUsage,
  MonthlyModelUsage,
  MonthlyTrend,
  StalenessSummary,
  TeamAdoption,
  TeamUsageSummary,
  UserMetricsSummary,
  WeeklyTrend,
  AdoptionCohortDay,
} from "./types";

function getErrorMessage(label: string, err: unknown): string {
  const message = err instanceof Error ? err.message : String(err);
  console.error(`[cached-bigquery] ${label} failed:`, err);
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

export async function getCachedBigQueryUsage(token: string): Promise<{
  usage: EnterpriseMetrics[] | null;
  error: string | null;
}> {
  const result = await fetchNullable("getCachedBigQueryUsage", () =>
    backendRequest<EnterpriseMetrics[]>("/api/v1/copilot/usage/metrics", token)
  );
  return { usage: result.data, error: result.error };
}

export async function getCachedAdoptionData(token: string): Promise<{
  data: AdoptionData | null;
  error: string | null;
}> {
  return fetchNullable("getCachedAdoptionData", async () => {
    const [summary, teams, languages, customizationDetails] = await Promise.all([
      backendRequest<AdoptionSummary>("/api/v1/copilot/adoption/summary", token),
      backendRequest<TeamAdoption[]>("/api/v1/copilot/adoption/teams", token),
      backendRequest<LanguageAdoption[]>("/api/v1/copilot/adoption/languages", token),
      backendRequest<CustomizationDetail[]>("/api/v1/copilot/customizations/details", token),
    ]);

    return { summary, teams, languages, customizationDetails };
  });
}

export async function getCachedCustomizationUsage(token: string): Promise<{
  usage: CustomizationUsage[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getCachedCustomizationUsage", [] as CustomizationUsage[], () =>
    backendRequest<CustomizationUsage[]>("/api/v1/copilot/customizations/usage", token)
  );
  return { usage: result.data, error: result.error };
}

export async function getCachedStalenessData(token: string): Promise<{
  data: StalenessSummary | null;
  error: string | null;
}> {
  return fetchNullable("getCachedStalenessData", () =>
    backendRequest<StalenessSummary>("/api/v1/copilot/adoption/staleness", token)
  );
}

export async function getCachedTeamUsage(token: string): Promise<{
  teams: TeamUsageSummary[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getCachedTeamUsage", [] as TeamUsageSummary[], () =>
    backendRequest<TeamUsageSummary[]>("/api/v1/copilot/usage/team-summary", token)
  );
  return { teams: result.data, error: result.error };
}

export async function getCachedUserMetrics(
  username: string,
  token: string
): Promise<{ metrics: UserMetricsSummary | null; error: string | null }> {
  const result = await fetchNullable("getCachedUserMetrics", async () => {
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

export async function getCachedUserWeeklyTrends(
  username: string,
  token: string
): Promise<{ trends: WeeklyTrend[]; error: string | null }> {
  const result = await fetchWithFallback("getCachedUserWeeklyTrends", [] as WeeklyTrend[], () =>
    backendRequest<WeeklyTrend[]>(`/api/v1/copilot/usage/user/${encodeURIComponent(username)}/weekly`, token)
  );
  return { trends: result.data, error: result.error };
}

export async function getCachedMonthlyTrends(token: string): Promise<{
  trends: MonthlyTrend[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getCachedMonthlyTrends", [] as MonthlyTrend[], () =>
    backendRequest<MonthlyTrend[]>("/api/v1/copilot/usage/trends", token)
  );
  return { trends: result.data, error: result.error };
}

export async function getCachedMonthlyModelUsage(token: string): Promise<{
  usage: MonthlyModelUsage[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getCachedMonthlyModelUsage", [] as MonthlyModelUsage[], () =>
    backendRequest<MonthlyModelUsage[]>("/api/v1/copilot/usage/models", token)
  );
  return { usage: result.data, error: result.error };
}

export async function getCachedMonthlyBillingUsage(token: string): Promise<{
  usage: MonthlyBillingUsage[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getCachedMonthlyBillingUsage", [] as MonthlyBillingUsage[], () =>
    backendRequest<MonthlyBillingUsage[]>("/api/v1/copilot/billing/monthly", token)
  );
  return { usage: result.data, error: result.error };
}

export async function getCachedAdoptionCohorts(token: string): Promise<{
  cohorts: AdoptionCohortDay[];
  error: string | null;
}> {
  const result = await fetchWithFallback("getCachedAdoptionCohorts", [] as AdoptionCohortDay[], () =>
    backendRequest<AdoptionCohortDay[]>("/api/v1/copilot/adoption/cohorts", token)
  );
  return { cohorts: result.data, error: result.error };
}
