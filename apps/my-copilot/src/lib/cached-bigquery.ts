import { cacheLife, cacheTag } from "next/cache";
import { getAdoptionSummary, getDailyMetrics, getLanguageAdoption, getTeamAdoption } from "./bigquery";
import type { AdoptionData, EnterpriseMetrics } from "./types";

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
    const [summary, teams, languages] = await Promise.all([
      getAdoptionSummary(),
      getTeamAdoption(),
      getLanguageAdoption(),
    ]);
    return { data: { summary, teams, languages }, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedAdoptionData failed:", err);
    return { data: null, error: message };
  }
}
