import { cacheLife, cacheTag } from "next/cache";
import {
  getAdoptionSummary,
  getCustomizationDetails,
  getCustomizationUsage,
  getDailyMetrics,
  getLanguageAdoption,
  getMonthlyBillingUsage,
  getMonthlyModelUsage,
  getMonthlyTrends,
  getStalenessData,
  getTeamAdoption,
  getTeamUsageSummary,
  getUserMetrics,
  getUserWeeklyTrends,
} from "./bigquery";
import type {
  AdoptionData,
  CustomizationUsage,
  EnterpriseMetrics,
  MonthlyBillingUsage,
  MonthlyModelUsage,
  MonthlyTrend,
  StalenessSummary,
  TeamUsageSummary,
  UserMetricsSummary,
  WeeklyTrend,
} from "./types";

export async function getCachedBigQueryUsage(): Promise<{
  usage: EnterpriseMetrics[] | null;
  error: string | null;
}> {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("bq-usage");

  try {
    const usage = await getDailyMetrics();
    return { usage, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedBigQueryUsage failed:", err);
    return { usage: null, error: message };
  }
}

export async function getCachedAdoptionData(): Promise<{
  data: AdoptionData | null;
  error: string | null;
}> {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("bq-adoption");

  try {
    const [summary, teams, languages, customizationDetails] = await Promise.all([
      getAdoptionSummary(),
      getTeamAdoption(),
      getLanguageAdoption(),
      getCustomizationDetails(),
    ]);
    return { data: { summary, teams, languages, customizationDetails }, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedAdoptionData failed:", err);
    return { data: null, error: message };
  }
}

export async function getCachedCustomizationUsage(): Promise<{
  usage: CustomizationUsage[];
  error: string | null;
}> {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("bq-customization-usage");

  try {
    const usage = await getCustomizationUsage();
    return { usage, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedCustomizationUsage failed:", err);
    return { usage: [], error: message };
  }
}

export async function getCachedStalenessData(): Promise<{
  data: StalenessSummary | null;
  error: string | null;
}> {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("bq-staleness");

  try {
    const files = await getStalenessData();
    const totalInstances = files.reduce((sum, f) => sum + f.total_repos, 0);
    const inSyncCount = files.reduce((sum, f) => sum + f.in_sync_repos, 0);
    const outOfSyncCount = files.reduce((sum, f) => sum + f.out_of_sync_repos, 0);

    const summary: StalenessSummary = {
      total_files: files.length,
      total_file_instances: totalInstances,
      in_sync_count: inSyncCount,
      out_of_sync_count: outOfSyncCount,
      sync_rate: totalInstances > 0 ? inSyncCount / totalInstances : 0,
      files,
    };

    return { data: summary, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedStalenessData failed:", err);
    return { data: null, error: message };
  }
}

export async function getCachedTeamUsage(): Promise<{
  teams: TeamUsageSummary[];
  error: string | null;
}> {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("bq-team-usage");

  try {
    const teams = await getTeamUsageSummary(7);
    return { teams, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedTeamUsage failed:", err);
    return { teams: [], error: message };
  }
}

export async function getCachedUserMetrics(userLogin: string): Promise<{
  metrics: UserMetricsSummary | null;
  error: string | null;
}> {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("bq-user-metrics", userLogin);

  try {
    const metrics = await getUserMetrics(userLogin, 30);
    return { metrics, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedUserMetrics failed:", err);
    return { metrics: null, error: message };
  }
}

export async function getCachedMonthlyTrends(): Promise<{
  trends: MonthlyTrend[];
  error: string | null;
}> {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("bq-monthly-trends");

  try {
    const trends = await getMonthlyTrends(12);
    return { trends, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedMonthlyTrends failed:", err);
    return { trends: [], error: message };
  }
}

export async function getCachedUserWeeklyTrends(userLogin: string): Promise<{
  trends: WeeklyTrend[];
  error: string | null;
}> {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("bq-user-weekly-trends", userLogin);

  try {
    const trends = await getUserWeeklyTrends(userLogin, 12);
    return { trends, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedUserWeeklyTrends failed:", err);
    return { trends: [], error: message };
  }
}

export async function getCachedMonthlyModelUsage(): Promise<{
  usage: MonthlyModelUsage[];
  error: string | null;
}> {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("bq-monthly-model-usage");

  try {
    const usage = await getMonthlyModelUsage(12);
    return { usage, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedMonthlyModelUsage failed:", err);
    return { usage: [], error: message };
  }
}

export async function getCachedMonthlyBillingUsage(): Promise<{
  usage: MonthlyBillingUsage[];
  error: string | null;
}> {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("bq-monthly-billing-usage");

  try {
    const usage = await getMonthlyBillingUsage(12);
    return { usage, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedMonthlyBillingUsage failed:", err);
    return { usage: [], error: message };
  }
}
