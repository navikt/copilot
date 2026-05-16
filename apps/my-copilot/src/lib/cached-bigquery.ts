import { backendRequest } from "./backend-api";
import type {
  AdoptionData,
  AdoptionSummary,
  CustomizationDetail,
  CustomizationUsage,
  EnterpriseMetrics,
  LanguageAdoption,
  TeamAdoption,
} from "./types";

export async function getCachedBigQueryUsage(token: string): Promise<{
  usage: EnterpriseMetrics[] | null;
  error: string | null;
}> {
  try {
    const usage = await backendRequest<EnterpriseMetrics[]>("/api/v1/copilot/usage/metrics", token);
    return { usage, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedBigQueryUsage failed:", err);
    return { usage: null, error: message };
  }
}

export async function getCachedAdoptionData(token: string): Promise<{
  data: AdoptionData | null;
  error: string | null;
}> {
  try {
    const [summary, teams, languages, customizationDetails] = await Promise.all([
      backendRequest<AdoptionSummary>("/api/v1/copilot/adoption/summary", token),
      backendRequest<TeamAdoption[]>("/api/v1/copilot/adoption/teams", token),
      backendRequest<LanguageAdoption[]>("/api/v1/copilot/adoption/languages", token),
      backendRequest<CustomizationDetail[]>("/api/v1/copilot/customizations/details", token),
    ]);
    return { data: { summary, teams, languages, customizationDetails }, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedAdoptionData failed:", err);
    return { data: null, error: message };
  }
}

export async function getCachedCustomizationUsage(token: string): Promise<{
  usage: CustomizationUsage[];
  error: string | null;
}> {
  try {
    const usage = await backendRequest<CustomizationUsage[]>("/api/v1/copilot/customizations/usage", token);
    return { usage, error: null };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error("[cached-bigquery] getCachedCustomizationUsage failed:", err);
    return { usage: [], error: message };
  }
}
