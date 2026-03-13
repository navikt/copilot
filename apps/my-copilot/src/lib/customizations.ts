import type { AnyCustomization, Domain } from "./customization-types";
import manifest from "./copilot-manifest.json";

export function getAllCustomizations(): AnyCustomization[] {
  return manifest.items as AnyCustomization[];
}

/**
 * Get set of official file names from the manifest.
 * For skills, extracts the directory name (e.g., "observability-setup") since
 * BQ stores skill directory names, not "SKILL.md".
 * For other types, extracts the file basename (e.g., "nais-platform.agent.md").
 */
export function getOfficialFileNames(): Set<string> {
  const items = manifest.items as AnyCustomization[];
  const names = new Set<string>();
  for (const item of items) {
    const parts = item.filePath.split("/");
    if (item.filePath.includes(".github/skills/") && parts.length >= 2) {
      // Skills: use directory name (e.g., "observability-setup" from ".github/skills/observability-setup/SKILL.md")
      names.add(parts[parts.length - 2]);
    } else {
      const basename = parts.pop();
      if (basename) names.add(basename);
    }
  }
  return names;
}

export function getCountsByDomain(items: AnyCustomization[]): Record<Domain, number> {
  const counts: Record<Domain, number> = {
    platform: 0,
    frontend: 0,
    backend: 0,
    auth: 0,
    observability: 0,
    general: 0,
    testing: 0,
    design: 0,
  };
  for (const item of items) {
    counts[item.domain]++;
  }
  return counts;
}
