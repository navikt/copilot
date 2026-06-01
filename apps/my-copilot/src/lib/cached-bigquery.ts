import { backendRequest } from "./backend-api";
import type {
  AdoptionData,
  AdoptionSummary,
  CustomizationDetail,
  CustomizationUsage,
  EnterpriseMetrics,
  LanguageAdoption,
  StalenessSummary,
  TeamAdoption,
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
