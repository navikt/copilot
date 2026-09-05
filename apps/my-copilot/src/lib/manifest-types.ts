export type Domain = "platform" | "frontend" | "backend" | "auth" | "observability" | "general" | "testing" | "design";

export const VALID_DOMAINS: readonly Domain[] = [
  "platform",
  "frontend",
  "backend",
  "auth",
  "observability",
  "general",
  "testing",
  "design",
] as const;

export interface UsageExample {
  prompt: string;
  scenario: string;
}

export type ExampleItem = string | UsageExample;

export function normalizeExample(example: ExampleItem): UsageExample {
  if (typeof example === "string") {
    return { prompt: example, scenario: "" };
  }
  return example;
}
