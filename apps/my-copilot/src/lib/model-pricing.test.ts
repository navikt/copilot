import { describe, it, expect } from "vitest";
import { MODEL_PRICING } from "./model-pricing";

describe("promotionEndsOn", () => {
  it("merker nøyaktig radene med kampanjefotnote hos GitHub", () => {
    const promoted = MODEL_PRICING.filter((m) => m.promotionEndsOn).map((m) => m.model);
    expect(promoted).toEqual([
      "GPT-5.6 Sol (Default, ≤ 272K)",
      "GPT-5.6 Sol (Long context, 272K)",
      "Gemini 3.6 Flash (Default)",
      "Gemini 3.7 Flash (Default)",
    ]);
  });

  it("gir sluttdatoen, som er hele poenget med merket", () => {
    expect(MODEL_PRICING.find((m) => m.model === "GPT-5.6 Sol (Default, ≤ 272K)")?.promotionEndsOn).toBe("2026-09-03");
  });

  it("lar modeller uten fotnote være", () => {
    expect(MODEL_PRICING.find((m) => m.model === "GPT-5.6 Luna (Default, ≤ 200K)")?.promotionEndsOn).toBeUndefined();
    expect(MODEL_PRICING.find((m) => m.model === "Gemini 3.5 Flash (Default)")?.promotionEndsOn).toBeUndefined();
  });
});
