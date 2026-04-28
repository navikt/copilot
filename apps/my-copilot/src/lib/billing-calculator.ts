/**
 * Billing calculator for estimating costs under GitHub Copilot's
 * AI Credits billing model (effective June 1, 2026).
 *
 * Token pricing sourced from:
 * https://docs.github.com/copilot/reference/copilot-billing/models-and-pricing
 */

// ---------------------------------------------------------------------------
// Token pricing per 1M tokens
// ---------------------------------------------------------------------------

export interface TokenRate {
  input: number;
  cached: number;
  output: number;
}

export const MODEL_PRICING: Record<string, TokenRate> = {
  // Anthropic Claude
  "Claude Haiku 4.5": { input: 1.0, cached: 0.1, output: 5.0 },
  "Claude Sonnet 4": { input: 3.0, cached: 0.3, output: 15.0 },
  "Claude Sonnet 4.5": { input: 3.0, cached: 0.3, output: 15.0 },
  "Claude Sonnet 4.6": { input: 3.0, cached: 0.3, output: 15.0 },
  "Claude Opus 4.5": { input: 5.0, cached: 0.5, output: 25.0 },
  "Claude Opus 4.6": { input: 5.0, cached: 0.5, output: 25.0 },
  "Claude Opus 4.7": { input: 5.0, cached: 0.5, output: 25.0 },
  // OpenAI GPT
  "GPT-4.1": { input: 2.0, cached: 0.5, output: 8.0 },
  "GPT-5.1": { input: 2.0, cached: 0.5, output: 8.0 },
  "GPT-5.2": { input: 1.75, cached: 0.175, output: 14.0 },
  "GPT-5.2-Codex": { input: 1.75, cached: 0.175, output: 14.0 },
  "GPT-5.3-Codex": { input: 1.75, cached: 0.175, output: 14.0 },
  "GPT-5.4": { input: 2.5, cached: 0.25, output: 15.0 },
  "GPT-5.4 mini": { input: 0.75, cached: 0.075, output: 4.5 },
  "GPT-5 mini": { input: 0.75, cached: 0.075, output: 4.5 },
  // Google Gemini
  "Gemini 2.5 Pro": { input: 1.25, cached: 0.31, output: 10.0 },
  "Gemini 3 Flash": { input: 0.15, cached: 0.02, output: 0.6 },
  "Gemini 3.1 Pro": { input: 1.25, cached: 0.31, output: 10.0 },
  // xAI
  "Grok Code Fast 1": { input: 0.5, cached: 0.05, output: 2.0 },
  // Copilot internal models (estimate: Sonnet-tier pricing)
  "Code Review model": { input: 3.0, cached: 0.3, output: 15.0 },
  "Coding Agent model": { input: 3.0, cached: 0.3, output: 15.0 },
};

// Add "Auto:" prefixed variants with same pricing
for (const [name, rate] of Object.entries({ ...MODEL_PRICING })) {
  MODEL_PRICING[`Auto: ${name}`] = rate;
}

// ---------------------------------------------------------------------------
// Token estimation profiles
// ---------------------------------------------------------------------------

export interface TokenProfile {
  inputTokens: number;
  outputTokens: number;
}

export type ModelCategory =
  | "opus"
  | "sonnet"
  | "haiku"
  | "gpt_standard"
  | "gpt_mini"
  | "gemini_flash"
  | "gemini_pro"
  | "grok"
  | "code_review"
  | "agent";

export type ProfileName = "conservative" | "moderate" | "heavy";

export const PROFILE_LABELS: Record<ProfileName, string> = {
  conservative: "Forsiktig",
  moderate: "Moderat",
  heavy: "Tung",
};

export const PROFILES: Record<ProfileName, Record<ModelCategory, TokenProfile>> = {
  conservative: {
    opus: { inputTokens: 8_000, outputTokens: 2_000 },
    sonnet: { inputTokens: 5_000, outputTokens: 1_500 },
    haiku: { inputTokens: 3_000, outputTokens: 800 },
    gpt_standard: { inputTokens: 5_000, outputTokens: 1_500 },
    gpt_mini: { inputTokens: 2_000, outputTokens: 500 },
    gemini_flash: { inputTokens: 2_000, outputTokens: 500 },
    gemini_pro: { inputTokens: 5_000, outputTokens: 1_500 },
    grok: { inputTokens: 3_000, outputTokens: 800 },
    code_review: { inputTokens: 15_000, outputTokens: 3_000 },
    agent: { inputTokens: 20_000, outputTokens: 4_000 },
  },
  moderate: {
    opus: { inputTokens: 20_000, outputTokens: 4_000 },
    sonnet: { inputTokens: 12_000, outputTokens: 3_000 },
    haiku: { inputTokens: 6_000, outputTokens: 1_500 },
    gpt_standard: { inputTokens: 12_000, outputTokens: 3_000 },
    gpt_mini: { inputTokens: 4_000, outputTokens: 1_000 },
    gemini_flash: { inputTokens: 4_000, outputTokens: 1_000 },
    gemini_pro: { inputTokens: 12_000, outputTokens: 3_000 },
    grok: { inputTokens: 6_000, outputTokens: 1_500 },
    code_review: { inputTokens: 30_000, outputTokens: 5_000 },
    agent: { inputTokens: 50_000, outputTokens: 8_000 },
  },
  heavy: {
    opus: { inputTokens: 50_000, outputTokens: 8_000 },
    sonnet: { inputTokens: 30_000, outputTokens: 6_000 },
    haiku: { inputTokens: 12_000, outputTokens: 3_000 },
    gpt_standard: { inputTokens: 30_000, outputTokens: 6_000 },
    gpt_mini: { inputTokens: 8_000, outputTokens: 2_000 },
    gemini_flash: { inputTokens: 8_000, outputTokens: 2_000 },
    gemini_pro: { inputTokens: 30_000, outputTokens: 6_000 },
    grok: { inputTokens: 12_000, outputTokens: 3_000 },
    code_review: { inputTokens: 60_000, outputTokens: 10_000 },
    agent: { inputTokens: 100_000, outputTokens: 15_000 },
  },
};

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface ModelPremiumData {
  model: string;
  requests: number;
  grossAmount: number;
}

export interface CLIData {
  inputTokens: number;
  outputTokens: number;
  sessions: number;
  requests: number;
}

export interface CalculatorInputs {
  seats: number;
  creditsPerSeat: number;
  cacheRate: number; // 0–1
  profile: ProfileName;
  dataPeriodDays: number;
  models: ModelPremiumData[];
  cli: CLIData;
  cliModel: string;
}

// Copilot CLI model is unknown — use weighted average of actual model usage as best estimate.
export const DEFAULT_CLI_MODEL = "weighted";

export interface ModelEstimate {
  model: string;
  requests: number;
  currentGrossCost: number;
  category: ModelCategory;
  estInputTokens: number;
  estOutputTokens: number;
  newCost: number;
}

export interface CLIEstimate {
  inputTokens: number;
  outputTokens: number;
  cachedInputTokens: number;
  freshInputTokens: number;
  cost: number;
}

export interface CreditPool {
  monthlyCredits: number;
  monthlyCreditsPromo: number;
  estimatedMonthlyCost: number;
  surplusStandard: number;
  surplusPromo: number;
}

export interface ScenarioResult {
  profile: ProfileName;
  label: string;
  totalModelCost: number;
  cliCost: number;
  totalCost: number;
  monthlyCost: number;
  monthlyCredits: number;
  surplus: number;
}

export interface CalculatorResult {
  modelEstimates: ModelEstimate[];
  cliEstimate: CLIEstimate;
  creditPool: CreditPool;
  scenarios: ScenarioResult[];
  totalCurrentGrossCost: number;
  totalNewCost: number;
  monthlyCost: number;
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

export const BUSINESS_PLAN_COST = 19;
export const PROMO_CREDITS_PER_SEAT = 30;
export const DEFAULT_CACHE_RATE = 0.8;
export const DEFAULT_DATA_PERIOD_DAYS = 28;
export const MONTH_DAYS = 30;

// ---------------------------------------------------------------------------
// Calculation functions
// ---------------------------------------------------------------------------

export function getModelCategory(modelName: string): ModelCategory {
  const name = modelName.toLowerCase().replace("auto: ", "");
  if (name.includes("opus")) return "opus";
  if (name.includes("sonnet")) return "sonnet";
  if (name.includes("haiku")) return "haiku";
  if (name.includes("code review")) return "code_review";
  if (name.includes("coding agent")) return "agent";
  if (name.includes("grok")) return "grok";
  if (name.includes("gemini") && name.includes("flash")) return "gemini_flash";
  if (name.includes("gemini")) return "gemini_pro";
  if (name.includes("mini")) return "gpt_mini";
  return "gpt_standard";
}

export function getTokenRate(modelName: string): TokenRate {
  const rate = MODEL_PRICING[modelName];
  if (rate) return rate;
  // Fallback: look up without "Auto: " prefix or try category-based default
  const stripped = modelName.replace(/^Auto: /, "");
  if (MODEL_PRICING[stripped]) return MODEL_PRICING[stripped];
  // Default to Sonnet-tier pricing
  return { input: 3.0, cached: 0.3, output: 15.0 };
}

export function calculateTokenCost(
  rate: TokenRate,
  inputTokens: number,
  outputTokens: number,
  cacheRate: number
): number {
  const cachedInput = inputTokens * cacheRate;
  const freshInput = inputTokens * (1.0 - cacheRate);
  return (
    (freshInput / 1_000_000) * rate.input +
    (cachedInput / 1_000_000) * rate.cached +
    (outputTokens / 1_000_000) * rate.output
  );
}

export function estimateModelCosts(
  models: ModelPremiumData[],
  profile: ProfileName,
  cacheRate: number
): ModelEstimate[] {
  const profileData = PROFILES[profile];
  return models.map((m) => {
    const category = getModelCategory(m.model);
    const tp = profileData[category];
    const estInput = m.requests * tp.inputTokens;
    const estOutput = m.requests * tp.outputTokens;
    const rate = getTokenRate(m.model);
    const cost = calculateTokenCost(rate, estInput, estOutput, cacheRate);
    return {
      model: m.model,
      requests: m.requests,
      currentGrossCost: m.grossAmount,
      category,
      estInputTokens: estInput,
      estOutputTokens: estOutput,
      newCost: cost,
    };
  });
}

export function getWeightedTokenRate(models: ModelPremiumData[]): TokenRate {
  const totalRequests = models.reduce((sum, m) => sum + m.requests, 0);
  if (totalRequests === 0) return getTokenRate("GPT-4.1");

  let input = 0,
    cached = 0,
    output = 0;
  for (const m of models) {
    const rate = getTokenRate(m.model);
    const weight = m.requests / totalRequests;
    input += rate.input * weight;
    cached += rate.cached * weight;
    output += rate.output * weight;
  }
  return { input, cached, output };
}

export function calculateCLICost(
  cli: CLIData,
  cacheRate: number,
  modelOrRate: string | TokenRate = "GPT-4.1"
): CLIEstimate {
  const rate = typeof modelOrRate === "string" ? getTokenRate(modelOrRate) : modelOrRate;
  const cost = calculateTokenCost(rate, cli.inputTokens, cli.outputTokens, cacheRate);
  return {
    inputTokens: cli.inputTokens,
    outputTokens: cli.outputTokens,
    cachedInputTokens: cli.inputTokens * cacheRate,
    freshInputTokens: cli.inputTokens * (1.0 - cacheRate),
    cost,
  };
}

export function calculateCreditPool(seats: number, creditsPerSeat: number, estimatedMonthlyCost: number): CreditPool {
  const monthlyCredits = seats * creditsPerSeat;
  const monthlyCreditsPromo = seats * PROMO_CREDITS_PER_SEAT;
  return {
    monthlyCredits,
    monthlyCreditsPromo,
    estimatedMonthlyCost,
    surplusStandard: monthlyCredits - estimatedMonthlyCost,
    surplusPromo: monthlyCreditsPromo - estimatedMonthlyCost,
  };
}

export function calculateAll(inputs: CalculatorInputs): CalculatorResult {
  const { seats, creditsPerSeat, cacheRate, profile, dataPeriodDays, models, cli, cliModel } = inputs;
  const monthScale = MONTH_DAYS / dataPeriodDays;

  // Current profile estimates
  const modelEstimates = estimateModelCosts(models, profile, cacheRate);
  const cliRate = cliModel === "weighted" ? getWeightedTokenRate(models) : cliModel;
  const cliEstimate = calculateCLICost(cli, cacheRate, cliRate);

  const totalModelCost = modelEstimates.reduce((s, m) => s + m.newCost, 0);
  // CLI tokens are already included in model request counts — not additive
  const totalNewCost = totalModelCost;
  const monthlyCost = totalNewCost * monthScale;
  const totalCurrentGrossCost = models.reduce((s, m) => s + m.grossAmount, 0);

  const creditPool = calculateCreditPool(seats, creditsPerSeat, monthlyCost);

  // All three scenarios for comparison
  const scenarios: ScenarioResult[] = (["conservative", "moderate", "heavy"] as ProfileName[]).map((p) => {
    const est = estimateModelCosts(models, p, cacheRate);
    const modelCost = est.reduce((s, m) => s + m.newCost, 0);
    const monthly = modelCost * monthScale;
    const credits = seats * creditsPerSeat;
    return {
      profile: p,
      label: PROFILE_LABELS[p],
      totalModelCost: modelCost,
      cliCost: cliEstimate.cost,
      totalCost: modelCost,
      monthlyCost: monthly,
      monthlyCredits: credits,
      surplus: credits - monthly,
    };
  });

  return {
    modelEstimates,
    cliEstimate,
    creditPool,
    scenarios,
    totalCurrentGrossCost,
    totalNewCost,
    monthlyCost,
  };
}
