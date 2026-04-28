import { describe, it, expect } from "vitest";
import {
  getModelCategory,
  getTokenRate,
  calculateTokenCost,
  estimateModelCosts,
  calculateCLICost,
  calculateCreditPool,
  calculateAll,
  MODEL_PRICING,
  PROFILES,
  BUSINESS_PLAN_COST,
  PROMO_CREDITS_PER_SEAT,
  MONTH_DAYS,
  type CalculatorInputs,
  type TokenRate,
} from "./billing-calculator";

describe("getModelCategory", () => {
  it.each([
    ["Claude Opus 4.6", "opus"],
    ["Claude Opus 4.7", "opus"],
    ["Claude Sonnet 4.6", "sonnet"],
    ["Claude Sonnet 4", "sonnet"],
    ["Claude Haiku 4.5", "haiku"],
    ["GPT-5.4", "gpt_standard"],
    ["GPT-5.2", "gpt_standard"],
    ["GPT-5.3-Codex", "gpt_standard"],
    ["GPT-5.4 mini", "gpt_mini"],
    ["GPT-5 mini", "gpt_mini"],
    ["Gemini 3 Flash", "gemini_flash"],
    ["Gemini 2.5 Pro", "gemini_pro"],
    ["Gemini 3.1 Pro", "gemini_pro"],
    ["Grok Code Fast 1", "grok"],
    ["Code Review model", "code_review"],
    ["Coding Agent model", "agent"],
  ] as const)("should classify %s as %s", (model, expected) => {
    expect(getModelCategory(model)).toBe(expected);
  });

  it("should handle Auto: prefix", () => {
    expect(getModelCategory("Auto: Claude Sonnet 4.6")).toBe("sonnet");
    expect(getModelCategory("Auto: GPT-5.4 mini")).toBe("gpt_mini");
    expect(getModelCategory("Auto: Coding Agent model")).toBe("agent");
  });

  it("should fall back to gpt_standard for unknown models", () => {
    expect(getModelCategory("Unknown Model X")).toBe("gpt_standard");
  });
});

describe("getTokenRate", () => {
  it("should return exact rate for known model", () => {
    const rate = getTokenRate("Claude Opus 4.6");
    expect(rate).toEqual({ input: 5.0, cached: 0.5, output: 25.0 });
  });

  it("should return same rate for Auto: variant", () => {
    expect(getTokenRate("Auto: Claude Opus 4.6")).toEqual(getTokenRate("Claude Opus 4.6"));
  });

  it("should return Sonnet-tier fallback for unknown model", () => {
    expect(getTokenRate("Unknown Model X")).toEqual({ input: 3.0, cached: 0.3, output: 15.0 });
  });

  it("should have pricing for all major models", () => {
    const expectedModels = [
      "Claude Haiku 4.5",
      "Claude Sonnet 4.6",
      "Claude Opus 4.6",
      "GPT-5.4",
      "GPT-5.3-Codex",
      "GPT-5.4 mini",
      "Gemini 3 Flash",
      "Grok Code Fast 1",
    ];
    for (const model of expectedModels) {
      expect(MODEL_PRICING[model]).toBeDefined();
    }
  });
});

describe("calculateTokenCost", () => {
  const rate: TokenRate = { input: 2.0, cached: 0.5, output: 8.0 };

  it("should return zero for zero tokens", () => {
    expect(calculateTokenCost(rate, 0, 0, 0.8)).toBe(0);
  });

  it("should calculate cost with no cache", () => {
    // 1M input at $2/M + 1M output at $8/M = $10
    expect(calculateTokenCost(rate, 1_000_000, 1_000_000, 0)).toBe(10);
  });

  it("should calculate cost with full cache", () => {
    // 1M cached input at $0.50/M + 1M output at $8/M = $8.50
    expect(calculateTokenCost(rate, 1_000_000, 1_000_000, 1)).toBe(8.5);
  });

  it("should calculate cost with partial cache", () => {
    // 1M input, 500K output, 25% cache
    // fresh: 750K * $2/M = $1.50
    // cached: 250K * $0.50/M = $0.125
    // output: 500K * $8/M = $4.00
    // total = $5.625
    expect(calculateTokenCost(rate, 1_000_000, 500_000, 0.25)).toBeCloseTo(5.625);
  });

  it("should handle zero output tokens", () => {
    // Only input cost: 1M at $2/M = $2
    expect(calculateTokenCost(rate, 1_000_000, 0, 0)).toBe(2);
  });

  it("should handle zero input tokens with output", () => {
    // Only output cost: 1M at $8/M = $8
    expect(calculateTokenCost(rate, 0, 1_000_000, 0.8)).toBe(8);
  });
});

describe("estimateModelCosts", () => {
  it("should estimate costs based on profile and category", () => {
    const models = [{ model: "Claude Sonnet 4.6", requests: 10, grossAmount: 100 }];
    const result = estimateModelCosts(models, "moderate", 0.8);

    expect(result).toHaveLength(1);
    const est = result[0];
    expect(est.category).toBe("sonnet");
    expect(est.requests).toBe(10);
    expect(est.currentGrossCost).toBe(100);

    // moderate sonnet: 12_000 input, 3_000 output per request
    expect(est.estInputTokens).toBe(10 * 12_000);
    expect(est.estOutputTokens).toBe(10 * 3_000);
    expect(est.newCost).toBeGreaterThan(0);

    // Verify cost calculation matches manual computation
    const rate = getTokenRate("Claude Sonnet 4.6");
    const expectedCost = calculateTokenCost(rate, est.estInputTokens, est.estOutputTokens, 0.8);
    expect(est.newCost).toBeCloseTo(expectedCost);
  });

  it("should handle unknown model without throwing", () => {
    const models = [{ model: "Unknown Model", requests: 5, grossAmount: 50 }];
    const result = estimateModelCosts(models, "moderate", 0.8);

    expect(result).toHaveLength(1);
    expect(result[0].category).toBe("gpt_standard");
    expect(result[0].newCost).toBeGreaterThan(0);
  });

  it("should return zero cost for zero requests", () => {
    const models = [{ model: "Claude Opus 4.6", requests: 0, grossAmount: 0 }];
    const result = estimateModelCosts(models, "moderate", 0.8);

    expect(result[0].estInputTokens).toBe(0);
    expect(result[0].estOutputTokens).toBe(0);
    expect(result[0].newCost).toBe(0);
  });

  it("should return empty array for empty models", () => {
    expect(estimateModelCosts([], "moderate", 0.8)).toEqual([]);
  });

  it("should produce higher costs with heavy profile than conservative", () => {
    const models = [{ model: "Claude Opus 4.6", requests: 100, grossAmount: 1000 }];
    const conservative = estimateModelCosts(models, "conservative", 0.8);
    const heavy = estimateModelCosts(models, "heavy", 0.8);

    expect(heavy[0].newCost).toBeGreaterThan(conservative[0].newCost);
    expect(heavy[0].estInputTokens).toBeGreaterThan(conservative[0].estInputTokens);
  });
});

describe("calculateCLICost", () => {
  it("should use default Opus 4.6 model", () => {
    const cli = { inputTokens: 1_000_000, outputTokens: 500_000, sessions: 100, requests: 1000 };
    const result = calculateCLICost(cli, 0.8);

    const opusRate = getTokenRate("Claude Opus 4.6");
    const expectedCost = calculateTokenCost(opusRate, 1_000_000, 500_000, 0.8);
    expect(result.cost).toBeCloseTo(expectedCost);
  });

  it("should split cached and fresh input correctly", () => {
    const cli = { inputTokens: 1000, outputTokens: 0, sessions: 1, requests: 1 };
    const result = calculateCLICost(cli, 0.8);

    expect(result.cachedInputTokens).toBeCloseTo(800);
    expect(result.freshInputTokens).toBeCloseTo(200);
  });

  it("should return zero for zero usage", () => {
    const cli = { inputTokens: 0, outputTokens: 0, sessions: 0, requests: 0 };
    const result = calculateCLICost(cli, 0.8);

    expect(result.cost).toBe(0);
    expect(result.cachedInputTokens).toBe(0);
    expect(result.freshInputTokens).toBe(0);
  });
});

describe("calculateCreditPool", () => {
  it("should calculate positive surplus", () => {
    const result = calculateCreditPool(10, 20, 150);

    expect(result.monthlyCredits).toBe(200);
    expect(result.surplusStandard).toBe(50);
  });

  it("should calculate exact break-even", () => {
    const result = calculateCreditPool(10, 20, 200);

    expect(result.surplusStandard).toBe(0);
  });

  it("should calculate deficit as negative surplus", () => {
    const result = calculateCreditPool(10, 20, 250);

    expect(result.surplusStandard).toBe(-50);
  });

  it("should use promo constant for promo credits", () => {
    const result = calculateCreditPool(10, BUSINESS_PLAN_COST, 100);

    expect(result.monthlyCreditsPromo).toBe(10 * PROMO_CREDITS_PER_SEAT);
  });

  it("should calculate both standard and promo surplus", () => {
    const result = calculateCreditPool(100, 19, 2500);

    expect(result.surplusStandard).toBe(100 * 19 - 2500);
    expect(result.surplusPromo).toBe(100 * PROMO_CREDITS_PER_SEAT - 2500);
  });
});

describe("calculateAll", () => {
  const baseInputs: CalculatorInputs = {
    seats: 100,
    creditsPerSeat: BUSINESS_PLAN_COST,
    cacheRate: 0.8,
    profile: "moderate",
    dataPeriodDays: 30,
    models: [
      { model: "Claude Opus 4.6", requests: 50, grossAmount: 500 },
      { model: "Claude Sonnet 4.6", requests: 100, grossAmount: 300 },
    ],
    cli: { inputTokens: 10_000_000, outputTokens: 1_000_000, sessions: 100, requests: 1000 },
    cliModel: "Claude Opus 4.6",
  };

  it("should aggregate totalCurrentGrossCost from model grossAmounts", () => {
    const result = calculateAll(baseInputs);
    expect(result.totalCurrentGrossCost).toBe(800);
  });

  it("should calculate totalNewCost as model costs + CLI cost", () => {
    const result = calculateAll(baseInputs);
    const modelTotal = result.modelEstimates.reduce((s, m) => s + m.newCost, 0);
    expect(result.totalNewCost).toBeCloseTo(modelTotal + result.cliEstimate.cost);
  });

  it("should scale to monthly cost based on dataPeriodDays", () => {
    const result15 = calculateAll({ ...baseInputs, dataPeriodDays: 15 });
    // 15-day data scaled to 30 days = 2× multiplier
    expect(result15.monthlyCost).toBeCloseTo(result15.totalNewCost * 2);
  });

  it("should not scale when dataPeriodDays equals MONTH_DAYS", () => {
    const result = calculateAll({ ...baseInputs, dataPeriodDays: MONTH_DAYS });
    expect(result.monthlyCost).toBeCloseTo(result.totalNewCost);
  });

  it("should return all three scenarios", () => {
    const result = calculateAll(baseInputs);

    expect(result.scenarios).toHaveLength(3);
    expect(result.scenarios.map((s) => s.profile)).toEqual(["conservative", "moderate", "heavy"]);
  });

  it("should have consistent scenario surplus calculation", () => {
    const result = calculateAll(baseInputs);

    for (const scenario of result.scenarios) {
      expect(scenario.surplus).toBeCloseTo(scenario.monthlyCredits - scenario.monthlyCost);
    }
  });

  it("should order scenarios by increasing cost", () => {
    const result = calculateAll(baseInputs);

    expect(result.scenarios[0].monthlyCost).toBeLessThan(result.scenarios[1].monthlyCost);
    expect(result.scenarios[1].monthlyCost).toBeLessThan(result.scenarios[2].monthlyCost);
  });

  it("should handle empty models gracefully", () => {
    const result = calculateAll({ ...baseInputs, models: [] });

    expect(result.modelEstimates).toEqual([]);
    expect(result.totalNewCost).toBe(result.cliEstimate.cost);
    expect(result.totalCurrentGrossCost).toBe(0);
  });

  it("should produce Infinity for dataPeriodDays = 0", () => {
    // Documents current behavior — no input validation
    const result = calculateAll({ ...baseInputs, dataPeriodDays: 0 });
    expect(result.monthlyCost).toBe(Infinity);
  });
});

describe("profiles", () => {
  it("should have consistent token profiles across all categories", () => {
    for (const profileName of ["conservative", "moderate", "heavy"] as const) {
      const profile = PROFILES[profileName];
      for (const [category, tokens] of Object.entries(profile)) {
        expect(tokens.inputTokens).toBeGreaterThanOrEqual(0);
        expect(tokens.outputTokens).toBeGreaterThanOrEqual(0);
        // Output tokens should never exceed input tokens in a profile
        expect(tokens.outputTokens).toBeLessThanOrEqual(tokens.inputTokens);
      }
    }
  });

  it("should have increasing token counts from conservative to heavy", () => {
    const categories = Object.keys(PROFILES.conservative) as Array<keyof (typeof PROFILES)["conservative"]>;
    for (const category of categories) {
      expect(PROFILES.moderate[category].inputTokens).toBeGreaterThan(PROFILES.conservative[category].inputTokens);
      expect(PROFILES.heavy[category].inputTokens).toBeGreaterThan(PROFILES.moderate[category].inputTokens);
    }
  });
});
