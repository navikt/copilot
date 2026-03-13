import type { AnyCustomization } from "./customization-types";
import type { CustomizationUsage } from "./types";

export type EnrichedCustomization = AnyCustomization & {
  usageCount: number;
  usedBy: string[];
};

/**
 * Category mapping from BQ v_customization_details categories to manifest file path patterns.
 * BQ stores: "agents" with file_name "nais-platform.agent.md"
 * Manifest stores: filePath ".github/agents/nais-platform.agent.md"
 */
const CATEGORY_TO_DIR: Record<string, string> = {
  agents: ".github/agents/",
  instructions: ".github/instructions/",
  prompts: ".github/prompts/",
  skills: ".github/skills/",
};

/**
 * Build a lookup key from a catalog item's filePath to match against BQ data.
 * Returns [category, file_name] or null if no match.
 * Skills use directory names in BQ (e.g., "observability-setup"), not "SKILL.md".
 */
function extractCategoryAndFile(filePath: string): [string, string] | null {
  for (const [category, dir] of Object.entries(CATEGORY_TO_DIR)) {
    if (filePath.includes(dir)) {
      const parts = filePath.split("/");
      if (category === "skills" && parts.length >= 2) {
        // Skills: use directory name (e.g., "observability-setup" from ".github/skills/observability-setup/SKILL.md")
        return [category, parts[parts.length - 2]];
      }
      const fileName = parts.pop();
      if (fileName) return [category, fileName];
    }
  }
  return null;
}

/**
 * Build a lookup map from BQ usage data keyed by "category:file_name".
 */
function buildUsageMap(usage: CustomizationUsage[]): Map<string, CustomizationUsage> {
  const map = new Map<string, CustomizationUsage>();
  for (const item of usage) {
    map.set(`${item.category}:${item.file_name}`, item);
  }
  return map;
}

/**
 * Enrich catalog items with usage data from BigQuery.
 * Items without matching usage data get usageCount: 0 and usedBy: [].
 */
export function enrichWithUsage(
  items: AnyCustomization[],
  usage: CustomizationUsage[],
): EnrichedCustomization[] {
  const usageMap = buildUsageMap(usage);

  return items.map((item) => {
    const match = extractCategoryAndFile(item.filePath);
    if (match) {
      const [category, fileName] = match;
      const found = usageMap.get(`${category}:${fileName}`);
      if (found) {
        return { ...item, usageCount: found.repo_count, usedBy: found.sample_repos };
      }
    }
    return { ...item, usageCount: 0, usedBy: [] };
  });
}
