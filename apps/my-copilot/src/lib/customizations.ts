import type { AnyCustomization, Domain } from "./customization-types";
import manifest from "./copilot-manifest.json";

export function getAllCustomizations(): AnyCustomization[] {
  return manifest.items as AnyCustomization[];
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
