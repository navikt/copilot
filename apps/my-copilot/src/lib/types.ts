// Enterprise Metrics API types (from BigQuery raw_record)
export interface EnterpriseMetrics {
  day: string;
  enterprise_id: string;
  daily_active_users: number;
  weekly_active_users: number;
  monthly_active_users: number;
  monthly_active_chat_users: number;
  monthly_active_agent_users: number;
  daily_active_cli_users: number;
  code_acceptance_activity_count: number;
  code_generation_activity_count: number;
  loc_added_sum: number;
  loc_deleted_sum: number;
  loc_suggested_to_add_sum: number;
  loc_suggested_to_delete_sum: number;
  user_initiated_interaction_count: number;
  pull_requests?: EnterprisePullRequests;
  totals_by_cli?: EnterpriseCLITotals;
  totals_by_feature?: EnterpriseFeatureTotal[];
  totals_by_ide?: EnterpriseIDETotal[];
  totals_by_language_feature?: EnterpriseLanguageFeatureTotal[];
  totals_by_language_model?: EnterpriseLanguageModelTotal[];
  totals_by_model_feature?: EnterpriseModelFeatureTotal[];
}

export interface EnterprisePullRequests {
  median_minutes_to_merge: number;
  median_minutes_to_merge_copilot_authored: number;
  total_applied_suggestions: number;
  total_copilot_applied_suggestions: number;
  total_copilot_suggestions: number;
  total_created: number;
  total_created_by_copilot: number;
  total_merged: number;
  total_merged_created_by_copilot: number;
  total_reviewed: number;
  total_reviewed_by_copilot: number;
  total_suggestions: number;
}

export interface EnterpriseCLITotals {
  prompt_count: number;
  request_count: number;
  session_count: number;
  token_usage?: {
    avg_tokens_per_request: number;
    output_tokens_sum: number;
    prompt_tokens_sum: number;
  };
}

interface EnterpriseActivityBase {
  code_acceptance_activity_count: number;
  code_generation_activity_count: number;
  loc_added_sum: number;
  loc_deleted_sum: number;
  loc_suggested_to_add_sum: number;
  loc_suggested_to_delete_sum: number;
}

export interface EnterpriseFeatureTotal extends EnterpriseActivityBase {
  feature: string;
  user_initiated_interaction_count: number;
}

export interface EnterpriseIDETotal extends EnterpriseActivityBase {
  ide: string;
  user_initiated_interaction_count: number;
}

export interface EnterpriseLanguageFeatureTotal extends EnterpriseActivityBase {
  language: string;
  feature: string;
}

export interface EnterpriseLanguageModelTotal extends EnterpriseActivityBase {
  language: string;
  model: string;
}

export interface EnterpriseModelFeatureTotal extends EnterpriseActivityBase {
  model: string;
  feature: string;
  user_initiated_interaction_count: number;
}

// Lean chart data types (serialized to client)
export interface DailyTrend {
  day: string;
  dailyActiveUsers: number;
  codeCompletionUsers: number;
  chatUsers: number;
  agentUsers: number;
}

export interface AdoptionTrendData {
  days: string[];
  chatUsers: number[];
  agentUsers: number[];
  cliUsers: number[];
}

export interface LanguageChartData {
  days: string[];
  languages: { name: string; values: number[] }[];
}

export interface EditorChartData {
  days: string[];
  editors: { name: string; values: number[] }[];
}

export interface FeatureChartData {
  labels: string[];
  acceptances: number[];
  generations: number[];
  interactions: number[];
}

export interface LinesOfCodeChartData {
  days: string[];
  suggested: number[];
  accepted: number[];
  deletionsSuggested: number[];
  deletionsAccepted: number[];
}

export interface ModelChartData {
  name: string;
  generations: number;
}

// Processed aggregation types
export interface LanguageData {
  name: string;
  acceptances: number;
  generations: number;
  acceptanceRate: number;
}

export interface EditorData {
  name: string;
  acceptances: number;
  generations: number;
  acceptanceRate: number;
  interactions: number;
}

export interface ModelData {
  name: string;
  generations: number;
  features: string[];
}

export interface AggregatedMetrics {
  dailyActiveUsers: number;
  weeklyActiveUsers: number;
  monthlyActiveUsers: number;
  monthlyActiveChatUsers: number;
  monthlyActiveAgentUsers: number;
  dailyActiveCLIUsers: number;
  totalAcceptances: number;
  totalGenerations: number;
  totalLinesSuggested: number;
  totalLinesAccepted: number;
  totalLinesDeletedSuggested: number;
  totalLinesDeleted: number;
  totalInteractions: number;
  overallAcceptanceRate: number;
  linesAcceptanceRate: number;
}

export interface FeatureAdoptionMetrics {
  features: {
    name: string;
    label: string;
    acceptances: number;
    generations: number;
    interactions: number;
  }[];
  totalActiveUsers: number;
}

export interface PRMetrics {
  totalCreated: number;
  totalMerged: number;
  totalReviewed: number;
  totalReviewedByCopilot: number;
  totalCreatedByCopilot: number;
  totalMergedCreatedByCopilot: number;
  medianMinutesToMerge: number;
  medianMinutesToMergeCopilotAuthored: number;
  totalSuggestions: number;
  totalCopilotSuggestions: number;
  totalAppliedSuggestions: number;
  totalCopilotAppliedSuggestions: number;
}

export interface CLIMetrics {
  promptCount: number;
  requestCount: number;
  sessionCount: number;
  avgTokensPerRequest: number;
  outputTokensSum: number;
  promptTokensSum: number;
}

// Billing types
interface PremiumRequestUsageItem {
  product: string;
  sku: string;
  model: string;
  unitType: string;
  pricePerUnit: number;
  grossQuantity: number;
  grossAmount: number;
  discountQuantity: number;
  discountAmount: number;
  netQuantity: number;
  netAmount: number;
}

interface BillingTimePeriod {
  year: number;
  month?: number;
  day?: number;
}

export interface PremiumRequestUsage {
  timePeriod: BillingTimePeriod;
  organization: string;
  usageItems: PremiumRequestUsageItem[];
}

// AI Customization Adoption types (from copilot_adoption BigQuery views)
export interface AdoptionSummary {
  scan_date: string;
  total_repos: number;
  active_repos: number;
  archived_repos: number;
  repos_with_any_customization: number;
  repos_without_customization: number;
  adoption_rate: number;
  repos_with_copilot_instructions: number;
  repos_with_agents_md: number;
  repos_with_agents: number;
  repos_with_instructions: number;
  repos_with_prompts: number;
  repos_with_skills: number;
  repos_with_mcp_config: number;
  repos_with_copilot_dir: number;
  repos_with_cursorrules: number;
  repos_with_cursor_rules_dir: number;
  repos_with_claude_md: number;
  repos_with_windsurfrules: number;
  repos_with_cursorignore: number;
  repos_with_claude_settings: number;
  repos_with_any_non_copilot_ai: number;
  avg_customization_count: number;
  max_customization_count: number;
}

export interface TeamAdoption {
  scan_date: string;
  team_slug: string;
  team_name: string;
  team_repos: number;
  active_repos: number;
  repos_with_customizations: number;
  adoption_rate: number;
  with_copilot_instructions: number;
  with_agents_md: number;
  with_agents: number;
  with_instructions: number;
  with_prompts: number;
  with_skills: number;
  with_mcp_config: number;
}

export interface LanguageAdoption {
  scan_date: string;
  language: string;
  total_repos: number;
  repos_with_customizations: number;
  adoption_rate: number;
  with_copilot_instructions: number;
  with_agents: number;
  with_instructions: number;
  with_mcp_config: number;
}

export interface CustomizationDetail {
  category: string;
  file_name: string;
  repo_count: number;
}

export interface CustomizationUsage {
  category: string;
  file_name: string;
  repo_count: number;
  sample_repos: string[];
}

export interface AdoptionData {
  summary: AdoptionSummary | null;
  teams: TeamAdoption[];
  languages: LanguageAdoption[];
  customizationDetails: CustomizationDetail[];
}
