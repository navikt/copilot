import type { EnterpriseMetrics, AggregatedMetrics, PRMetrics, CLIMetrics, DailyTrend } from "./types";

// Agent features (matches v_code_generation.sql)
const AGENT_INITIATED_FEATURES = new Set([
  "agent_edit",
  "chat_panel_agent_mode",
  "chat_panel_edit_mode",
  "chat_panel_custom_mode",
]);

const calculateAcceptanceRate = (accepted: number, generated: number): number => {
  return generated > 0 ? Math.round((accepted / generated) * 100) : 0;
};

export const getAggregatedMetrics = (usage: EnterpriseMetrics[]): AggregatedMetrics | null => {
  if (!usage || usage.length === 0) return null;

  const latest = usage[usage.length - 1];

  let totalAcceptances = 0;
  let totalGenerations = 0;
  let totalLinesSuggested = 0;
  let totalLinesAccepted = 0;
  let totalLinesDeletedSuggested = 0;
  let totalLinesDeleted = 0;
  let totalInteractions = 0;

  for (const day of usage) {
    totalAcceptances += day.code_acceptance_activity_count || 0;
    totalGenerations += day.code_generation_activity_count || 0;
    totalLinesSuggested += day.loc_suggested_to_add_sum || 0;
    totalLinesAccepted += day.loc_added_sum || 0;
    totalLinesDeletedSuggested += day.loc_suggested_to_delete_sum || 0;
    totalLinesDeleted += day.loc_deleted_sum || 0;
    totalInteractions += day.user_initiated_interaction_count || 0;
  }

  return {
    dailyActiveUsers: latest.daily_active_users || 0,
    weeklyActiveUsers: latest.weekly_active_users || 0,
    monthlyActiveUsers: latest.monthly_active_users || 0,
    monthlyActiveChatUsers: latest.monthly_active_chat_users || 0,
    monthlyActiveAgentUsers: latest.monthly_active_agent_users || 0,
    dailyActiveCLIUsers: latest.daily_active_cli_users || 0,
    totalAcceptances,
    totalGenerations,
    totalLinesSuggested,
    totalLinesAccepted,
    totalLinesDeletedSuggested,
    totalLinesDeleted,
    totalInteractions,
    overallAcceptanceRate: calculateAcceptanceRate(totalAcceptances, totalGenerations),
    linesAcceptanceRate: calculateAcceptanceRate(totalLinesAccepted, totalLinesSuggested),
  };
};

export const getPRMetrics = (usage: EnterpriseMetrics[]): PRMetrics | null => {
  if (!usage || usage.length === 0) return null;

  const result: PRMetrics = {
    totalCreated: 0,
    totalMerged: 0,
    totalReviewed: 0,
    totalReviewedByCopilot: 0,
    totalCreatedByCopilot: 0,
    totalMergedCreatedByCopilot: 0,
    totalMergedReviewedByCopilot: 0,
    medianMinutesToMerge: null,
    medianMinutesToMergeCopilotAuthored: null,
    medianMinutesToMergeCopilotReviewed: null,
    totalSuggestions: 0,
    totalCopilotSuggestions: 0,
    totalAppliedSuggestions: 0,
    totalCopilotAppliedSuggestions: 0,
  };

  let hasPR = false;
  for (const day of usage) {
    const pr = day.pull_requests;
    if (!pr) continue;
    hasPR = true;
    result.totalCreated += pr.total_created || 0;
    result.totalMerged += pr.total_merged || 0;
    result.totalReviewed += pr.total_reviewed || 0;
    result.totalReviewedByCopilot += pr.total_reviewed_by_copilot || 0;
    result.totalCreatedByCopilot += pr.total_created_by_copilot || 0;
    result.totalMergedCreatedByCopilot += pr.total_merged_created_by_copilot || 0;
    result.totalMergedReviewedByCopilot += pr.total_merged_reviewed_by_copilot || 0;
    result.totalSuggestions += pr.total_suggestions || 0;
    result.totalCopilotSuggestions += pr.total_copilot_suggestions || 0;
    result.totalAppliedSuggestions += pr.total_applied_suggestions || 0;
    result.totalCopilotAppliedSuggestions += pr.total_copilot_applied_suggestions || 0;
  }
  if (!hasPR) return null;

  // Use the most recent day that has PR median data, not just the absolute last day
  // (avoids showing "0 min" during ingestion lag)
  let medianSource: (typeof usage)[number]["pull_requests"] | undefined;
  for (let i = usage.length - 1; i >= 0; i--) {
    const pr = usage[i].pull_requests;
    if (
      pr &&
      (pr.median_minutes_to_merge ||
        pr.median_minutes_to_merge_copilot_authored ||
        pr.median_minutes_to_merge_copilot_reviewed)
    ) {
      medianSource = pr;
      break;
    }
  }
  result.medianMinutesToMerge = medianSource?.median_minutes_to_merge ?? null;
  result.medianMinutesToMergeCopilotAuthored = medianSource?.median_minutes_to_merge_copilot_authored ?? null;
  result.medianMinutesToMergeCopilotReviewed = medianSource?.median_minutes_to_merge_copilot_reviewed ?? null;

  return result;
};

export const getCLIMetrics = (usage: EnterpriseMetrics[]): CLIMetrics | null => {
  if (!usage || usage.length === 0) return null;

  let promptCount = 0,
    requestCount = 0,
    sessionCount = 0;
  let outputTokensSum = 0,
    promptTokensSum = 0;
  let hasCLI = false;

  for (const day of usage) {
    const cli = day.totals_by_cli;
    if (!cli) continue;
    hasCLI = true;
    promptCount += cli.prompt_count || 0;
    requestCount += cli.request_count || 0;
    sessionCount += cli.session_count || 0;
    outputTokensSum += cli.token_usage?.output_tokens_sum || 0;
    promptTokensSum += cli.token_usage?.prompt_tokens_sum || 0;
  }
  if (!hasCLI) return null;

  const totalTokens = outputTokensSum + promptTokensSum;
  const avgTokensPerRequest = requestCount > 0 ? Math.round(totalTokens / requestCount) : 0;

  return { promptCount, requestCount, sessionCount, avgTokensPerRequest, outputTokensSum, promptTokensSum };
};

// Chart data builders — produce lean serializable objects for client components

export const buildTrendData = (usage: EnterpriseMetrics[]): DailyTrend[] => {
  return usage.map((day) => {
    const features = day.totals_by_feature || [];
    const codeCompletion = features.find((f) => f.feature === "code_completion");
    const chatFeatures = features.filter((f) => f.feature.startsWith("chat_panel") || f.feature === "chat_inline");
    const agentFeatures = features.filter((f) => AGENT_INITIATED_FEATURES.has(f.feature));

    return {
      day: day.day,
      dailyActiveUsers: day.daily_active_users || 0,
      codeCompletionUsers: codeCompletion?.code_generation_activity_count || 0,
      chatUsers: chatFeatures.reduce((s, f) => s + (f.user_initiated_interaction_count || 0), 0),
      agentUsers: agentFeatures.reduce((s, f) => s + (f.code_generation_activity_count || 0), 0),
    };
  });
};
