import { cacheLife, cacheTag } from "next/cache";
import {
  getAdoptionSummary,
  getCustomizationDetails,
  getCustomizationUsage,
  getDailyMetrics,
  getLanguageAdoption,
  getTeamAdoption,
  getTeamUsageSummary,
  getUserMetrics,
} from "./bigquery";
import type {
  AdoptionData,
  CustomizationUsage,
  EnterpriseMetrics,
  TeamUsageSummary,
  UserMetricsSummary,
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
    const metrics = await getUserMetrics(userLogin, 7);
    return { metrics, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedUserMetrics failed:", err);
    return { metrics: null, error: message };
  }
}
