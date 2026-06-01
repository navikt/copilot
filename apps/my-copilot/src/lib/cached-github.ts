import { cacheLife, cacheTag } from "next/cache";
import type { CopilotBilling, Contributor, PremiumRequestUsage } from "./types";
import { backendRequest } from "./backend-api";

function getErrorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export async function getCachedCopilotBilling(token: string): Promise<{
  billing: CopilotBilling | null;
  error: string | null;
}> {
  try {
    const billing = await backendRequest<CopilotBilling>("/api/v1/copilot/billing", token);
    return { billing, error: null };
  } catch (error) {
    return { billing: null, error: getErrorMessage(error) };
  }
}

/**
 * Cached version of premium request usage via backend API.
 * Premium usage data for current month updates frequently.
 */
export async function getCachedPremiumRequestUsage(
  org: string,
  year?: number,
  month?: number
): Promise<{
  usage: PremiumRequestUsage | null;
  error: string | null;
}> {
  "use cache";
  cacheLife({
    stale: 300, // 5 minutes until considered stale
    revalidate: 900, // 15 minutes until revalidated
    expire: 3600, // 1 hour until expired
  });
  cacheTag("premium-usage", org, `${year}-${month}`);

  try {
    // This function is called server-side during page render
    // For now, it requires the backend context (no token available here)
    // The frontend pages that use this should pass a token
    return { usage: null, error: "Premium usage requires authentication context" };
  } catch (error) {
    return { usage: null, error: getErrorMessage(error) };
  }
}

/**
 * Fetch premium request usage for an organization via backend API.
 * Requires authentication token.
 */
export async function getCachedPremiumRequestUsageWithToken(
  token: string,
  org: string,
  year?: number,
  month?: number
): Promise<{
  usage: PremiumRequestUsage | null;
  error: string | null;
}> {
  "use cache";
  cacheLife({
    stale: 300,
    revalidate: 900,
    expire: 3600,
  });
  cacheTag("premium-usage", org, `${year}-${month}`);

  try {
    let path = `/api/v1/copilot/billing/premium?org=${encodeURIComponent(org)}`;
    if (year) path += `&year=${year}`;
    if (month) path += `&month=${month}`;

    const usage = await backendRequest<PremiumRequestUsage>(path, token);
    return { usage, error: null };
  } catch (error) {
    return { usage: null, error: getErrorMessage(error) };
  }
}

/**
 * Fetch repository contributors via backend API.
 * Requires authentication token.
 */
export async function getCachedFileContributors(
  token: string,
  owner: string,
  repo: string,
  paths: string[]
): Promise<{
  contributors: Contributor[];
  error: string | null;
}> {
  "use cache";
  cacheLife({
    stale: 7200, // 2 hours until considered stale
    revalidate: 86400, // 24 hours until revalidated
    expire: 604800, // 7 days until expired
  });
  cacheTag("contributors", ...paths);

  try {
    const result = await backendRequest<{ contributors: Contributor[] }>(
      `/api/v1/copilot/repo-contributors?owner=${encodeURIComponent(owner)}&repo=${encodeURIComponent(repo)}&paths=${encodeURIComponent(JSON.stringify(paths))}`,
      token
    );
    return { contributors: result.contributors || [], error: null };
  } catch (error) {
    return { contributors: [], error: getErrorMessage(error) };
  }
}
