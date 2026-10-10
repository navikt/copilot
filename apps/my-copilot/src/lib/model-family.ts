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
  "claude_opus" | "claude_sonnet" | "claude_haiku" | "claude_fable" | "gpt_mini" | "gpt" | "gemini" | "andre";

/** Display order and Norwegian labels. */
export const FAMILY_LABELS: Record<ModelFamily, string> = {
  claude_opus: "Claude Opus",
  claude_sonnet: "Claude Sonnet",
  claude_haiku: "Claude Haiku",
  claude_fable: "Claude Fable",
  gpt: "GPT",
  gpt_mini: "GPT mini (mini, nano, Luna)",
  gemini: "Gemini",
  andre: "Andre",
};

/** «Claude Opus 5.5» and «claude-opus-5.5» give the same family. Unknown names give «andre». */
export function modelFamily(model: string): ModelFamily {
  const name = model
    .toLowerCase()
    .trim()
    .replace(/[\s_]+/g, "-");
  return FAMILIES.find(([pattern]) => pattern.test(name))?.[1] ?? "andre";
}
