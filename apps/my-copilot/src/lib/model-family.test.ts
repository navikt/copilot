import { describe, expect, it } from "vitest";
import { modelFamily } from "./model-family";

describe("modelFamily", () => {
  it.each([
    ["Claude Opus 5.5", "claude_opus"],
    ["claude-sonnet-4.5", "claude_sonnet"],
    ["Claude Haiku 5.5", "claude_haiku"],
    ["Claude Fable 5", "claude_fable"],
    ["GPT-6 Luna", "gpt_mini"],
    ["GPT-5 mini", "gpt_mini"],
    ["gpt-4.1-nano", "gpt_mini"],
    ["GPT-6 Sol", "gpt"],
    ["GPT-5.2-Codex", "gpt"],
    ["Gemini 3 Pro", "gemini"],
  ])("%s → %s", (name, family) => {
    expect(modelFamily(name)).toBe(family);
  });

  it.each(["Grok Code Fast 1", "o3", "", "Auto"])("unknown %j → andre", (name) => {
    expect(modelFamily(name)).toBe("andre");
  });
});
