import type {
  EnterpriseMetrics,
  AggregatedMetrics,
  FeatureAdoptionMetrics,
  PRMetrics,
  CLIMetrics,
  LanguageData,
  EditorData,
  ModelData,
  DailyTrend,
  AdoptionTrendData,
  LanguageChartData,
  EditorChartData,
  FeatureChartData,
  LinesOfCodeChartData,
  ModelChartData,
} from "./types";

const FEATURE_LABELS: Record<string, string> = {
  code_completion: "Kodeforslag",
  chat_panel_agent_mode: "Agent-modus",
  chat_panel_ask_mode: "Chat (spør)",
  agent_edit: "Agent-redigering",
  chat_panel_custom_mode: "Egendefinert modus",
  chat_inline: "Inline chat",
};

const EXCLUDED_FEATURES = new Set(["chat_panel_unknown_mode"]);

export const calculateAcceptanceRate = (accepted: number, generated: number): number => {
  return generated > 0 ? Math.round((accepted / generated) * 100) : 0;
};

export const getDateRange = (usage: EnterpriseMetrics[]): { start: string; end: string } | null => {
  if (!usage || usage.length === 0) return null;
  return { start: usage[0].day, end: usage[usage.length - 1].day };
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

export const getFeatureAdoption = (usage: EnterpriseMetrics[]): FeatureAdoptionMetrics | null => {
  if (!usage || usage.length === 0) return null;

  const featureMap = new Map<string, { acceptances: number; generations: number; interactions: number }>();
  for (const day of usage) {
    for (const f of day.totals_by_feature || []) {
      if (EXCLUDED_FEATURES.has(f.feature)) continue;
      const existing = featureMap.get(f.feature) || { acceptances: 0, generations: 0, interactions: 0 };
      existing.acceptances += f.code_acceptance_activity_count || 0;
      existing.generations += f.code_generation_activity_count || 0;
      existing.interactions += f.user_initiated_interaction_count || 0;
      featureMap.set(f.feature, existing);
    }
  }

  const features = Array.from(featureMap.entries()).map(([name, data]) => ({
    name,
    label: FEATURE_LABELS[name] || name,
    acceptances: data.acceptances,
    generations: data.generations,
    interactions: data.interactions,
  }));

  features.sort((a, b) => b.generations - a.generations);

  const latest = usage[usage.length - 1];
  return {
    features,
    totalActiveUsers: latest.daily_active_users || 0,
  };
};

export const getPRMetrics = (usage: EnterpriseMetrics[]): PRMetrics | null => {
  if (!usage || usage.length === 0) return null;

  const result: PRMetrics = {
    totalCreated: 0, totalMerged: 0, totalReviewed: 0,
    totalReviewedByCopilot: 0, totalCreatedByCopilot: 0, totalMergedCreatedByCopilot: 0,
    medianMinutesToMerge: 0, medianMinutesToMergeCopilotAuthored: 0,
    totalSuggestions: 0, totalCopilotSuggestions: 0,
    totalAppliedSuggestions: 0, totalCopilotAppliedSuggestions: 0,
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
    result.totalSuggestions += pr.total_suggestions || 0;
    result.totalCopilotSuggestions += pr.total_copilot_suggestions || 0;
    result.totalAppliedSuggestions += pr.total_applied_suggestions || 0;
    result.totalCopilotAppliedSuggestions += pr.total_copilot_applied_suggestions || 0;
  }
  if (!hasPR) return null;

  const latest = usage[usage.length - 1];
  result.medianMinutesToMerge = latest.pull_requests?.median_minutes_to_merge || 0;
  result.medianMinutesToMergeCopilotAuthored = latest.pull_requests?.median_minutes_to_merge_copilot_authored || 0;

  return result;
};

export const getCLIMetrics = (usage: EnterpriseMetrics[]): CLIMetrics | null => {
  if (!usage || usage.length === 0) return null;

  let promptCount = 0, requestCount = 0, sessionCount = 0;
  let outputTokensSum = 0, promptTokensSum = 0;
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

export const getTopLanguages = (usage: EnterpriseMetrics[], limit: number = 10): LanguageData[] => {
  if (!usage || usage.length === 0) return [];

  const langMap = new Map<string, { acceptances: number; generations: number }>();
  for (const day of usage) {
    for (const lf of day.totals_by_language_feature || []) {
      if (lf.language === "others") continue;
      const existing = langMap.get(lf.language) || { acceptances: 0, generations: 0 };
      existing.acceptances += lf.code_acceptance_activity_count || 0;
      existing.generations += lf.code_generation_activity_count || 0;
      langMap.set(lf.language, existing);
    }
  }

  return Array.from(langMap.entries())
    .map(([name, data]) => ({
      name,
      acceptances: data.acceptances,
      generations: data.generations,
      acceptanceRate: calculateAcceptanceRate(data.acceptances, data.generations),
    }))
    .sort((a, b) => b.generations - a.generations)
    .slice(0, limit);
};

export const getEditorStats = (usage: EnterpriseMetrics[]): EditorData[] => {
  if (!usage || usage.length === 0) return [];

  const ideMap = new Map<string, { acceptances: number; generations: number; interactions: number }>();
  let cliRequests = 0, cliSessions = 0;

  for (const day of usage) {
    for (const ide of day.totals_by_ide || []) {
      const existing = ideMap.get(ide.ide) || { acceptances: 0, generations: 0, interactions: 0 };
      existing.acceptances += ide.code_acceptance_activity_count || 0;
      existing.generations += ide.code_generation_activity_count || 0;
      existing.interactions += ide.user_initiated_interaction_count || 0;
      ideMap.set(ide.ide, existing);
    }
    const cli = day.totals_by_cli;
    if (cli) {
      cliRequests += cli.request_count || 0;
      cliSessions += cli.session_count || 0;
    }
  }

  const editors: EditorData[] = Array.from(ideMap.entries()).map(([name, data]) => ({
    name,
    acceptances: data.acceptances,
    generations: data.generations,
    acceptanceRate: calculateAcceptanceRate(data.acceptances, data.generations),
    interactions: data.interactions,
  }));

  if (cliRequests > 0) {
    editors.push({
      name: "Copilot CLI",
      acceptances: 0,
      generations: cliRequests,
      acceptanceRate: 0,
      interactions: cliSessions,
    });
  }

  return editors.sort((a, b) => b.generations - a.generations);
};

export const getModelUsageMetrics = (usage: EnterpriseMetrics[]): ModelData[] => {
  if (!usage || usage.length === 0) return [];

  const modelMap = new Map<string, { generations: number; features: Set<string> }>();

  for (const day of usage) {
    for (const mf of day.totals_by_model_feature || []) {
      if (mf.model === "others") continue;
      const existing = modelMap.get(mf.model) || { generations: 0, features: new Set<string>() };
      existing.generations += mf.code_generation_activity_count || 0;
      existing.features.add(FEATURE_LABELS[mf.feature] || mf.feature);
      modelMap.set(mf.model, existing);
    }
  }

  return Array.from(modelMap.entries())
    .map(([name, data]) => ({
      name,
      generations: data.generations,
      features: Array.from(data.features),
    }))
    .sort((a, b) => b.generations - a.generations);
};

// Chart data builders — produce lean serializable objects for client components

export const buildTrendData = (usage: EnterpriseMetrics[]): DailyTrend[] => {
  return usage.map((day) => {
    const features = day.totals_by_feature || [];
    const codeCompletion = features.find((f) => f.feature === "code_completion");
    const chatFeatures = features.filter((f) =>
      f.feature.startsWith("chat_panel") || f.feature === "chat_inline"
    );
    const agentFeatures = features.filter((f) =>
      f.feature === "chat_panel_agent_mode" || f.feature === "agent_edit"
    );

    return {
      day: day.day,
      dailyActiveUsers: day.daily_active_users || 0,
      codeCompletionUsers: codeCompletion?.code_generation_activity_count || 0,
      chatUsers: chatFeatures.reduce((s, f) => s + (f.user_initiated_interaction_count || 0), 0),
      agentUsers: agentFeatures.reduce((s, f) => s + (f.code_generation_activity_count || 0), 0),
    };
  });
};

export const buildAdoptionTrendData = (usage: EnterpriseMetrics[]): AdoptionTrendData => {
  return {
    days: usage.map((d) => d.day),
    chatUsers: usage.map((d) => d.monthly_active_chat_users || 0),
    agentUsers: usage.map((d) => d.monthly_active_agent_users || 0),
    cliUsers: usage.map((d) => d.daily_active_cli_users || 0),
  };
};

export const buildLanguageChartData = (usage: EnterpriseMetrics[], limit: number = 8): LanguageChartData => {
  const topLangs = getTopLanguages(usage, limit);
  const topNames = topLangs.map((l) => l.name);

  const days = usage.map((d) => d.day);
  const languages = topNames.map((name) => ({
    name,
    values: usage.map((day) => {
      const entries = (day.totals_by_language_feature || []).filter((lf) => lf.language === name);
      return entries.reduce((s, e) => s + (e.code_generation_activity_count || 0), 0);
    }),
  }));

  return { days, languages };
};

export const buildEditorChartData = (usage: EnterpriseMetrics[]): EditorChartData => {
  const allEditors = new Set<string>();
  for (const day of usage) {
    for (const ide of day.totals_by_ide || []) {
      allEditors.add(ide.ide);
    }
  }

  const hasCLI = usage.some((d) => d.totals_by_cli && d.totals_by_cli.request_count > 0);

  const days = usage.map((d) => d.day);
  const editors = Array.from(allEditors).map((name) => ({
    name,
    values: usage.map((day) => {
      const ide = (day.totals_by_ide || []).find((i) => i.ide === name);
      return ide?.code_generation_activity_count || 0;
    }),
  }));

  if (hasCLI) {
    editors.push({
      name: "Copilot CLI",
      values: usage.map((day) => day.totals_by_cli?.request_count || 0),
    });
  }

  return { days, editors };
};

export const buildFeatureChartData = (usage: EnterpriseMetrics[]): FeatureChartData => {
  const featureMap = new Map<string, { acceptances: number; generations: number; interactions: number }>();
  for (const day of usage) {
    for (const f of day.totals_by_feature || []) {
      if (f.feature === "others" || EXCLUDED_FEATURES.has(f.feature)) continue;
      const existing = featureMap.get(f.feature) || { acceptances: 0, generations: 0, interactions: 0 };
      existing.acceptances += f.code_acceptance_activity_count || 0;
      existing.generations += f.code_generation_activity_count || 0;
      existing.interactions += f.user_initiated_interaction_count || 0;
      featureMap.set(f.feature, existing);
    }
  }

  const sorted = Array.from(featureMap.entries()).sort((a, b) => b[1].generations - a[1].generations);

  return {
    labels: sorted.map(([name]) => FEATURE_LABELS[name] || name),
    acceptances: sorted.map(([, d]) => d.acceptances),
    generations: sorted.map(([, d]) => d.generations),
    interactions: sorted.map(([, d]) => d.interactions),
  };
};

export const buildLinesOfCodeData = (usage: EnterpriseMetrics[]): LinesOfCodeChartData => {
  return {
    days: usage.map((d) => d.day),
    suggested: usage.map((d) => d.loc_suggested_to_add_sum || 0),
    accepted: usage.map((d) => d.loc_added_sum || 0),
    deletionsSuggested: usage.map((d) => d.loc_suggested_to_delete_sum || 0),
    deletionsAccepted: usage.map((d) => d.loc_deleted_sum || 0),
  };
};

export const buildModelChartData = (usage: EnterpriseMetrics[], limit: number = 8): ModelChartData[] => {
  if (!usage || usage.length === 0) return [];

  const modelMap = new Map<string, number>();
  for (const day of usage) {
    for (const mf of day.totals_by_model_feature || []) {
      if (mf.model === "others") continue;
      modelMap.set(mf.model, (modelMap.get(mf.model) || 0) + (mf.code_generation_activity_count || 0));
    }
  }

  return Array.from(modelMap.entries())
    .map(([name, generations]) => ({ name, generations }))
    .sort((a, b) => b.generations - a.generations)
    .slice(0, limit);
};
