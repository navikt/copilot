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
  median_minutes_to_merge_copilot_reviewed: number;
  total_applied_suggestions: number;
  total_copilot_applied_suggestions: number;
  total_copilot_suggestions: number;
  total_created: number;
  total_created_by_copilot: number;
  total_merged: number;
  total_merged_created_by_copilot: number;
  total_merged_reviewed_by_copilot: number;
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

export interface GenerationModeTrendData {
  days: string[];
  userInitiated: number[];
  agentInitiated: number[];
}

export interface GenerationModeSummary {
  userInitiatedGenerations: number;
  agentInitiatedGenerations: number;
  userInitiatedAcceptances: number;
  agentInitiatedAcceptances: number;
  agentShare: number;
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
  totalMergedReviewedByCopilot: number;
  medianMinutesToMerge: number;
  medianMinutesToMergeCopilotAuthored: number;
  medianMinutesToMergeCopilotReviewed: number;
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

// Copilot Billing types (from GitHub API via copilot-api)
export interface CopilotBilling {
  seat_breakdown: {
    total: number;
    added_this_cycle: number;
    pending_invitation: number;
    pending_cancellation: number;
    active_this_cycle: number;
    inactive_this_cycle: number;
  };
  seat_management_setting?: string;
  ide_chat?: string;
  platform_chat?: string;
  cli?: string;
  public_code_suggestions?: string;
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
  active_repos_with_recent_commits: number;
  dormant_repos: number;
  unknown_last_commit_repos: number;
  repos_with_any_customization: number;
  repos_without_customization: number;
  adoption_rate: number;
  adoption_rate_active_only: number;
  repos_with_copilot_instructions: number;
  repos_with_agents_md: number;
  repos_with_agents: number;
  repos_with_instructions: number;
  repos_with_prompts: number;
  repos_with_skills: number;
  repos_with_mcp_config: number;
  repos_with_copilot_dir: number;
  repos_with_copilot_review_instructions: number;
  repos_with_cursorrules: number;
  repos_with_cursor_rules_dir: number;
  repos_with_claude_md: number;
  repos_with_windsurfrules: number;
  repos_with_cursorignore: number;
  repos_with_claude_settings: number;
  repos_with_copilot_setup_steps: number;
  repos_with_agentic_workflows: number;
  repos_with_agents_skills: number;
  repos_with_nav_pilot_state: number;
  repos_with_cplt_toml: number;
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
  recently_active_repos: number;
  repos_with_customizations: number;
  adoption_rate: number;
  adoption_rate_active_only: number;
  with_copilot_instructions: number;
  with_agents_md: number;
  with_agents: number;
  with_instructions: number;
  with_prompts: number;
  with_skills: number;
  with_mcp_config: number;
  with_copilot_setup_steps: number;
  with_agentic_workflows: number;
  with_agents_skills: number;
  with_nav_pilot_state: number;
  with_cplt_toml: number;
}

export interface LanguageAdoption {
  scan_date: string;
  language: string;
  total_repos: number;
  recently_active_repos: number;
  repos_with_customizations: number;
  adoption_rate: number;
  adoption_rate_active_only: number;
  with_copilot_instructions: number;
  with_agents: number;
  with_instructions: number;
  with_mcp_config: number;
}

export interface CustomizationDetail {
  category: string;
  file_name: string;
  repo_count: number;
  active_repo_count: number;
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

/**
 * Aggregated sync/staleness data per file across repos.
 * Sourced from v_staleness_summary BigQuery view.
 */
export interface StalenessFile {
  category: string;
  file_name: string;
  total_repos: number;
  in_sync_repos: number;
  out_of_sync_repos: number;
  sync_rate: number;
  recently_active_repos: number;
}

/**
 * Overall staleness summary stats.
 */
export interface StalenessSummary {
  total_files: number;
  total_file_instances: number;
  in_sync_count: number;
  out_of_sync_count: number;
  sync_rate: number;
  files: StalenessFile[];
}

/**
 * Scope for filtering adoption data by repo activity.
 * "active" = repos with commit in last 90 days.
 * "all" = all non-archived repos.
 */
export type AdoptionScope = "all" | "active";

// AI Adoption Cohort types (from user_metrics ai_adoption_phase field)
export interface AdoptionCohortDay {
  day: string;
  phase: number; // 0 = No cohort, 1 = Code first, 2 = Agent first, 3 = Multi-agent
  phase_version: string; // "v1"
  user_count: number;
  avg_generations: number;
  avg_acceptances: number;
  avg_interactions: number;
  avg_lines_added: number;
}

export interface AdoptionCohortTrendData {
  days: string[];
  phase0: number[];
  phase1: number[];
  phase2: number[];
  phase3: number[];
  total: number[];
}

// Repository contributor types
export interface Contributor {
  login: string;
  avatarUrl: string;
}
