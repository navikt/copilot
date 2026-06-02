import type { EnterpriseMetrics } from "./types";
import {
  getDateRange,
  getAggregatedMetrics,
  getPRMetrics,
  getCLIMetrics,
  getTopLanguages,
  getEditorStats,
  getModelUsageMetrics,
  buildTrendData,
  buildModelChartData,
  getGenerationModeSummary,
  buildGenerationModeTrendData,
} from "./data-utils";

// Minimal fixture with required fields
function makeDay(overrides: Partial<EnterpriseMetrics> = {}): EnterpriseMetrics {
  return {
    day: "2026-04-01",
    enterprise_id: "test",
    daily_active_users: 100,
    weekly_active_users: 200,
    monthly_active_users: 300,
    monthly_active_chat_users: 80,
    monthly_active_agent_users: 40,
    daily_active_cli_users: 10,
    code_acceptance_activity_count: 500,
    code_generation_activity_count: 1000,
    loc_added_sum: 2000,
    loc_deleted_sum: 300,
    loc_suggested_to_add_sum: 4000,
    loc_suggested_to_delete_sum: 600,
    user_initiated_interaction_count: 150,
    ...overrides,
  };
}

const twoDays: EnterpriseMetrics[] = [
  makeDay({
    day: "2026-04-01",
    daily_active_users: 90,
    code_generation_activity_count: 400,
    code_acceptance_activity_count: 200,
  }),
  makeDay({
    day: "2026-04-02",
    daily_active_users: 110,
    code_generation_activity_count: 600,
    code_acceptance_activity_count: 300,
  }),
];

// --- getDateRange ---

describe("getDateRange", () => {
  it("returns null for empty array", () => {
    expect(getDateRange([])).toBeNull();
  });

  it("returns start and end dates", () => {
    expect(getDateRange(twoDays)).toEqual({ start: "2026-04-01", end: "2026-04-02" });
  });

  it("handles single-day array", () => {
    const result = getDateRange([makeDay()]);
    expect(result).toEqual({ start: "2026-04-01", end: "2026-04-01" });
  });
});

// --- getAggregatedMetrics ---

describe("getAggregatedMetrics", () => {
  it("returns null for empty array", () => {
    expect(getAggregatedMetrics([])).toBeNull();
  });

  it("sums metrics across days", () => {
    const result = getAggregatedMetrics(twoDays)!;
    expect(result.totalGenerations).toBe(1000);
    expect(result.totalAcceptances).toBe(500);
  });

  it("uses latest day for active user counts", () => {
    const result = getAggregatedMetrics(twoDays)!;
    expect(result.dailyActiveUsers).toBe(110);
  });

  it("calculates acceptance rate", () => {
    const result = getAggregatedMetrics(twoDays)!;
    expect(result.overallAcceptanceRate).toBe(50); // 500/1000
  });

  it("returns 0 acceptance rate when no generations", () => {
    const result = getAggregatedMetrics([
      makeDay({ code_generation_activity_count: 0, code_acceptance_activity_count: 0 }),
    ])!;
    expect(result.overallAcceptanceRate).toBe(0);
  });
});

// --- getPRMetrics ---

describe("getPRMetrics", () => {
  it("returns null for empty array", () => {
    expect(getPRMetrics([])).toBeNull();
  });

  it("returns null when no pull_requests data", () => {
    expect(getPRMetrics([makeDay()])).toBeNull();
  });

  it("aggregates PR metrics across days", () => {
    const usage = [
      makeDay({
        pull_requests: {
          total_created: 10,
          total_merged: 8,
          total_reviewed: 5,
          total_reviewed_by_copilot: 2,
          total_created_by_copilot: 3,
          total_merged_created_by_copilot: 2,
          total_merged_reviewed_by_copilot: 1,
          total_suggestions: 20,
          total_copilot_suggestions: 10,
          total_applied_suggestions: 15,
          total_copilot_applied_suggestions: 8,
          median_minutes_to_merge: 60,
          median_minutes_to_merge_copilot_authored: 30,
          median_minutes_to_merge_copilot_reviewed: 45,
        },
      }),
      makeDay({
        day: "2026-04-02",
        pull_requests: {
          total_created: 5,
          total_merged: 4,
          total_reviewed: 3,
          total_reviewed_by_copilot: 1,
          total_created_by_copilot: 1,
          total_merged_created_by_copilot: 1,
          total_merged_reviewed_by_copilot: 0,
          total_suggestions: 10,
          total_copilot_suggestions: 5,
          total_applied_suggestions: 7,
          total_copilot_applied_suggestions: 3,
          median_minutes_to_merge: 45,
          median_minutes_to_merge_copilot_authored: 20,
          median_minutes_to_merge_copilot_reviewed: 30,
        },
      }),
    ];
    const result = getPRMetrics(usage)!;
    expect(result.totalCreated).toBe(15);
    expect(result.totalMerged).toBe(12);
    // Median comes from latest day
    expect(result.medianMinutesToMerge).toBe(45);
    expect(result.medianMinutesToMergeCopilotAuthored).toBe(20);
  });
});

// --- getCLIMetrics ---

describe("getCLIMetrics", () => {
  it("returns null for empty array", () => {
    expect(getCLIMetrics([])).toBeNull();
  });

  it("returns null when no CLI data", () => {
    expect(getCLIMetrics([makeDay()])).toBeNull();
  });

  it("aggregates CLI metrics", () => {
    const usage = [
      makeDay({
        totals_by_cli: {
          prompt_count: 10,
          request_count: 20,
          session_count: 5,
          token_usage: { avg_tokens_per_request: 100, output_tokens_sum: 1000, prompt_tokens_sum: 500 },
        },
      }),
      makeDay({
        day: "2026-04-02",
        totals_by_cli: {
          prompt_count: 15,
          request_count: 30,
          session_count: 8,
          token_usage: { avg_tokens_per_request: 100, output_tokens_sum: 2000, prompt_tokens_sum: 1000 },
        },
      }),
    ];
    const result = getCLIMetrics(usage)!;
    expect(result.promptCount).toBe(25);
    expect(result.requestCount).toBe(50);
    expect(result.sessionCount).toBe(13);
    expect(result.outputTokensSum).toBe(3000);
    expect(result.promptTokensSum).toBe(1500);
    expect(result.avgTokensPerRequest).toBe(90); // (3000+1500)/50
  });
});

// --- getTopLanguages ---

describe("getTopLanguages", () => {
  it("returns empty array for empty input", () => {
    expect(getTopLanguages([])).toEqual([]);
  });

  it("aggregates languages and sorts by generations", () => {
    const usage = [
      makeDay({
        totals_by_language_feature: [
          {
            language: "TypeScript",
            feature: "code_completion",
            code_generation_activity_count: 50,
            code_acceptance_activity_count: 25,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
          },
          {
            language: "Python",
            feature: "code_completion",
            code_generation_activity_count: 100,
            code_acceptance_activity_count: 60,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
          },
        ],
      }),
    ];
    const result = getTopLanguages(usage);
    expect(result[0].name).toBe("Python");
    expect(result[0].acceptanceRate).toBe(60); // 60/100
    expect(result[1].name).toBe("TypeScript");
  });

  it("excludes 'others'", () => {
    const usage = [
      makeDay({
        totals_by_language_feature: [
          {
            language: "others",
            feature: "code_completion",
            code_generation_activity_count: 999,
            code_acceptance_activity_count: 0,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
          },
          {
            language: "Go",
            feature: "code_completion",
            code_generation_activity_count: 10,
            code_acceptance_activity_count: 5,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
          },
        ],
      }),
    ];
    expect(getTopLanguages(usage)).toHaveLength(1);
  });

  it("respects limit parameter", () => {
    const features = Array.from({ length: 15 }, (_, i) => ({
      language: `Lang${i}`,
      feature: "code_completion",
      code_generation_activity_count: 100 - i,
      code_acceptance_activity_count: 0,
      loc_added_sum: 0,
      loc_deleted_sum: 0,
      loc_suggested_to_add_sum: 0,
      loc_suggested_to_delete_sum: 0,
    }));
    const usage = [makeDay({ totals_by_language_feature: features })];
    expect(getTopLanguages(usage, 5)).toHaveLength(5);
  });
});

// --- getEditorStats ---

describe("getEditorStats", () => {
  it("returns empty array for empty input", () => {
    expect(getEditorStats([])).toEqual([]);
  });

  it("aggregates IDE stats and includes CLI", () => {
    const usage = [
      makeDay({
        totals_by_ide: [
          {
            ide: "VS Code",
            code_generation_activity_count: 80,
            code_acceptance_activity_count: 40,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
            user_initiated_interaction_count: 20,
          },
        ],
        totals_by_cli: { prompt_count: 5, request_count: 10, session_count: 3 },
      }),
    ];
    const result = getEditorStats(usage);
    expect(result).toHaveLength(2);
    expect(result[0].name).toBe("VS Code");
    expect(result[1].name).toBe("Copilot CLI");
  });
});

// --- getModelUsageMetrics ---

describe("getModelUsageMetrics", () => {
  it("returns empty array for empty input", () => {
    expect(getModelUsageMetrics([])).toEqual([]);
  });

  it("aggregates model usage and tracks features", () => {
    const usage = [
      makeDay({
        totals_by_model_feature: [
          {
            model: "gpt-4o",
            feature: "code_completion",
            code_generation_activity_count: 50,
            code_acceptance_activity_count: 0,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
            user_initiated_interaction_count: 0,
          },
          {
            model: "gpt-4o",
            feature: "chat_panel_agent_mode",
            code_generation_activity_count: 30,
            code_acceptance_activity_count: 0,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
            user_initiated_interaction_count: 0,
          },
          {
            model: "claude-sonnet",
            feature: "code_completion",
            code_generation_activity_count: 20,
            code_acceptance_activity_count: 0,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
            user_initiated_interaction_count: 0,
          },
        ],
      }),
    ];
    const result = getModelUsageMetrics(usage);
    expect(result[0].name).toBe("gpt-4o");
    expect(result[0].generations).toBe(80);
    expect(result[0].features).toContain("Kodeforslag");
    expect(result[0].features).toContain("Agent-modus");
  });
});

// --- buildTrendData ---

describe("buildTrendData", () => {
  it("maps days to trend entries", () => {
    const usage = [
      makeDay({
        day: "2026-04-01",
        daily_active_users: 90,
        totals_by_feature: [
          {
            feature: "code_completion",
            code_generation_activity_count: 50,
            code_acceptance_activity_count: 0,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
            user_initiated_interaction_count: 0,
          },
          {
            feature: "agent_edit",
            code_generation_activity_count: 20,
            code_acceptance_activity_count: 0,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
            user_initiated_interaction_count: 0,
          },
        ],
      }),
    ];
    const result = buildTrendData(usage);
    expect(result).toHaveLength(1);
    expect(result[0].dailyActiveUsers).toBe(90);
    expect(result[0].codeCompletionUsers).toBe(50);
    expect(result[0].agentUsers).toBe(20);
  });
});

// --- buildModelChartData ---

describe("buildModelChartData", () => {
  it("returns empty for empty input", () => {
    expect(buildModelChartData([])).toEqual([]);
  });

  it("aggregates and limits models", () => {
    const models = Array.from({ length: 12 }, (_, i) => ({
      model: `model-${i}`,
      feature: "code_completion",
      code_generation_activity_count: 100 - i,
      code_acceptance_activity_count: 0,
      loc_added_sum: 0,
      loc_deleted_sum: 0,
      loc_suggested_to_add_sum: 0,
      loc_suggested_to_delete_sum: 0,
      user_initiated_interaction_count: 0,
    }));
    const usage = [makeDay({ totals_by_model_feature: models })];
    expect(buildModelChartData(usage, 5)).toHaveLength(5);
  });
});

// --- getGenerationModeSummary ---

describe("getGenerationModeSummary", () => {
  it("returns null for empty array", () => {
    expect(getGenerationModeSummary([])).toBeNull();
  });

  it("returns null when all generation counts are zero", () => {
    const usage = [makeDay({ totals_by_feature: [] })];
    expect(getGenerationModeSummary(usage)).toBeNull();
  });

  it("classifies user vs agent initiated features", () => {
    const usage = [
      makeDay({
        totals_by_feature: [
          {
            feature: "code_completion",
            code_generation_activity_count: 60,
            code_acceptance_activity_count: 30,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
            user_initiated_interaction_count: 0,
          },
          {
            feature: "agent_edit",
            code_generation_activity_count: 40,
            code_acceptance_activity_count: 20,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
            user_initiated_interaction_count: 0,
          },
        ],
      }),
    ];
    const result = getGenerationModeSummary(usage)!;
    expect(result.userInitiatedGenerations).toBe(60);
    expect(result.agentInitiatedGenerations).toBe(40);
    expect(result.agentShare).toBe(40); // 40/100
  });
});

// --- buildGenerationModeTrendData ---

describe("buildGenerationModeTrendData", () => {
  it("separates user and agent initiated per day", () => {
    const usage = [
      makeDay({
        totals_by_feature: [
          {
            feature: "code_completion",
            code_generation_activity_count: 70,
            code_acceptance_activity_count: 0,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
            user_initiated_interaction_count: 0,
          },
          {
            feature: "chat_panel_agent_mode",
            code_generation_activity_count: 30,
            code_acceptance_activity_count: 0,
            loc_added_sum: 0,
            loc_deleted_sum: 0,
            loc_suggested_to_add_sum: 0,
            loc_suggested_to_delete_sum: 0,
            user_initiated_interaction_count: 0,
          },
        ],
      }),
    ];
    const result = buildGenerationModeTrendData(usage);
    expect(result.userInitiated[0]).toBe(70);
    expect(result.agentInitiated[0]).toBe(30);
  });
});
