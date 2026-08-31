import { describe, it, expect } from "vitest";
import { MODEL_PRICING } from "./model-pricing";
import { promotionFor } from "./model-promotions";

describe("promotionFor", () => {
  it("merker nøyaktig radene med kampanjefotnote hos GitHub", () => {
    const promoted = MODEL_PRICING.filter((m) => promotionFor(m.model)).map((m) => m.model);
    expect(promoted).toEqual([
      "GPT-5.6 Sol (Default, ≤ 272K)",
      "GPT-5.6 Sol (Long context, 272K)",
      "Gemini 3.6 Flash (Default)",
      "Gemini 3.7 Flash (Default)",
    ]);
  });

  it("gir sluttdatoen, som er hele poenget med merket", () => {
    expect(promotionFor("GPT-5.6 Sol (Default, ≤ 272K)")?.endsOn).toBe("3. september 2026");
  });

  it("lar modeller uten fotnote være", () => {
    expect(promotionFor("GPT-5.6 Luna (Default, ≤ 200K)")).toBeUndefined();
    expect(promotionFor("Gemini 3.5 Flash (Default)")).toBeUndefined();
  });
});
