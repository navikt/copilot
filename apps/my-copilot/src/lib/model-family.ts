// Model name → family, for charts that group billing rows by model.
// Same patterns and keys as the public statistics schema in #1523, so the two agree.
// First match wins, so narrow patterns come first.
const FAMILIES: [RegExp, ModelFamily][] = [
  [/^claude-opus/, "claude_opus"],
  [/^claude-sonnet/, "claude_sonnet"],
  [/^claude-haiku/, "claude_haiku"],
  [/^claude-fable/, "claude_fable"],
  [/^gpt-.*(mini|nano|luna)/, "gpt_mini"],
  [/^gpt-/, "gpt"],
  [/^gemini/, "gemini"],
];

export type ModelFamily =
  | "claude_opus"
  | "claude_sonnet"
  | "claude_haiku"
  | "claude_fable"
  | "gpt_mini"
  | "gpt"
  | "gemini"
  | "kodegjennomgang"
  | "auto"
  | "andre";

/** Display order and Norwegian labels. */
export const FAMILY_LABELS: Record<ModelFamily, string> = {
  claude_opus: "Claude Opus",
  claude_sonnet: "Claude Sonnet",
  claude_haiku: "Claude Haiku",
  claude_fable: "Claude Fable",
  gpt: "GPT",
  gpt_mini: "GPT mini (mini, nano, Luna)",
  gemini: "Gemini",
  kodegjennomgang: "Kodegjennomgang",
  auto: "Auto",
  andre: "Andre",
};

/**
 * «Claude Opus 5.5» and «claude-opus-5.5» give the same family. Every «Auto: X» is one family, «auto», in every chart
 * built on familyShares (owner decision 12 October 2026). Unknown names give «andre». With costPage (the admin cost page only),
 * Copilot's code review model is its own family.
 */
export function modelFamily(model: string, costPage = false): ModelFamily {
  const name = model
    .trim()
    .toLowerCase()
    .replace(/[\s_]+/g, "-");
  if (name.startsWith("auto:")) return "auto";
  if (costPage && name === "code-review-model") return "kodegjennomgang";
  return FAMILIES.find(([pattern]) => pattern.test(name))?.[1] ?? "andre";
}
