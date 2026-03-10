import { cacheLife, cacheTag } from "next/cache";
import { getDailyMetrics, getLatestDay } from "./bigquery";
import type { EnterpriseMetrics } from "./types";

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

export async function getCachedBigQueryLatestDay(): Promise<string | null> {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("bq-latest-day");

  try {
    return await getLatestDay();
  } catch (err) {
    console.error("[cached-bigquery] getCachedBigQueryLatestDay failed:", err);
    return null;
  }
}
