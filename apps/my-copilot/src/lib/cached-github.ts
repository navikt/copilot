/**
 * Backend data fetchers for Copilot GitHub/billing data.
 *
 * Caching is owned entirely by the backend (copilot-api has a 1 h in-memory
 * cache). These functions are thin proxies — they do NOT add a second BFF
 * cache layer. Using "use cache" here was ineffective anyway because every
 * call site passes a per-user OBO token as an argument, which would create a
 * separate cache entry per user and defeat org-level caching.
 */
import type { CopilotBilling, Contributor, PremiumRequestUsage } from "./types";
import { backendRequest } from "./backend-api";

function getErrorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export async function getCopilotBilling(token: string): Promise<{
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
 * Fetch premium request usage for an organization via backend API.
 * Requires authentication token.
 */
export async function getPremiumRequestUsage(
  token: string,
  org: string,
  year?: number,
  month?: number
): Promise<{
  usage: PremiumRequestUsage | null;
  error: string | null;
}> {
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
export async function getFileContributors(
  token: string,
  owner: string,
  repo: string,
  paths: string[]
): Promise<{
  contributors: Contributor[];
  error: string | null;
}> {
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
