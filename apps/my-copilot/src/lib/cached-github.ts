import { cacheLife, cacheTag } from "next/cache";
import { getPremiumRequestUsage } from "./github";
import { getFileContributors } from "./contributors";
import { backendRequest } from "./backend-api";
import { getUserToken } from "./auth";

export async function getCachedCopilotBilling(org: string) {
  "use cache";
  cacheLife({ stale: 3600 });
  cacheTag("billing-navikt");

  try {
    const token = await getUserToken();
    if (!token) {
      throw new Error("No authentication token available");
    }

    const billing = await backendRequest("/api/v1/copilot/billing", token);
    return { billing, error: null };
  } catch (error) {
    return { billing: {}, error: error instanceof Error ? error.message : String(error) };
  }
}

/**
 * Cached version of getPremiumRequestUsage
 * Premium usage data for current month updates frequently
 */
export async function getCachedPremiumRequestUsage(org: string, year?: number, month?: number) {
  "use cache";
  cacheLife({
    stale: 300, // 5 minutes until considered stale
    revalidate: 900, // 15 minutes until revalidated
    expire: 3600, // 1 hour until expired
  });
  cacheTag("premium-usage", org, `${year}-${month}`);

  return await getPremiumRequestUsage(org, year, month);
}

/**
 * Cached version of getFileContributors.
 * Contributors change infrequently — cache aggressively.
 */
export async function getCachedFileContributors(owner: string, repo: string, paths: string[]) {
  "use cache";
  cacheLife({
    stale: 7200, // 2 hours until considered stale
    revalidate: 86400, // 24 hours until revalidated
    expire: 604800, // 7 days until expired
  });
  cacheTag("contributors", ...paths);

  return await getFileContributors(owner, repo, paths);
}
