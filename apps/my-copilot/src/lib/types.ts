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

export interface PRMetrics {
  totalCreated: number;
  totalMerged: number;
  totalReviewed: number;
  totalReviewedByCopilot: number;
  totalCreatedByCopilot: number;
  totalMergedCreatedByCopilot: number;
  totalMergedReviewedByCopilot: number;
  medianMinutesToMerge: number | null;
  medianMinutesToMergeCopilotAuthored: number | null;
  medianMinutesToMergeCopilotReviewed: number | null;
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

/** Display-ready from the API: only teams with at least five active repos, rates in whole percent. */
export interface TeamAdoption {
  team_slug: string;
  team_name: string;
  active_repos: number;
  recently_active_repos: number;
  repos_with_customizations: number;
  adoption_pct: number;
  adoption_active_pct: number | null;
}

export interface TeamAdoptionOverview {
  teams: TeamAdoption[];
  total_teams: number;
  teams_with_adoption: number;
  adoption_pct: number;
  small_teams: number;
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
  teams: TeamAdoptionOverview;
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

/** Display-ready team row: n >= 5, amounts rounded to cents, change computed by the API. */
export interface TeamSpend {
  team_id: string;
  team_slug: string;
  users: number;
  amount_usd: number;
  per_user_usd: number;
  change_usd: number | null;
  highlight: boolean;
}

export type TeamComparison = "ok" | "current_month" | "previous_net_missing" | "incomplete";

export interface TeamGrossOverview {
  usage?: Record<string, { providers: string[]; categories: string[]; feature: string; language: string }>;
  month: string;
  teams: TeamSpend[];
  small_teams: number;
  last_usage_day: string;
  comparison: TeamComparison;
}

// One team, one row per month of a year, from GET /api/v1/copilot/usage/team-year.
// Amounts are null when the month has no data. No under-five threshold.
export interface TeamYearMonth {
  month: string;
  basis: "net" | "premium" | "gross" | "none";
  users: number | null;
  net_usd: number | null;
  gross_usd: number | null;
  no_usage_net_usd: number | null;
  /** Percent of the Nav-wide net bill not assigned to any user. Null for gross months and for runs published before the residual was recorded. Optional for older APIs. */
  residual_pct?: number | null;
  /** Members with a seat that month; null without team history or before the first seat snapshot. */
  seat_months: number | null;
  /** seat_months at list price by plan: 19 USD Business, 39 USD Enterprise. An estimate, not billed. */
  licence_usd_estimate: number | null;
  /** Basis amount (net plus no-usage net, or gross) plus licences; null when any part is missing. */
  total_usd: number | null;
  total_basis: "net" | "premium" | "gross" | "none";
}

export interface TeamYearOverview {
  team_id: string;
  team_slug: string;
  year: number;
  months: TeamYearMonth[];
  coverage: {
    membership_from: string;
    gross_from: string;
    last_usage_day: string;
    net_months: string[];
    seats_snapshot: string;
    team_members_from: string;
  };
}

// All of Nav, one row per month of a year, from GET /api/v1/copilot/billing/admin-monthly.
// Net months reconcile: assigned_net_usd + residual_net_usd = enterprise_net_usd.
export interface AdminMonth {
  month: string;
  basis: "net" | "premium" | "gross" | "none";
  enterprise_net_usd: number | null;
  assigned_net_usd: number | null;
  residual_net_usd: number | null;
  residual_pct: number | null;
  /** Null before team history exists. */
  no_team_net_usd: number | null;
  /** Preliminary list price for a month without net. */
  gross_usd: number | null;
  seats: number | null;
  licence_usd_estimate: number | null;
  total_usd: number | null;
  loaded_at: string;
}

export interface AdminMonthlyOverview {
  year: number;
  months: AdminMonth[];
  coverage: {
    membership_from: string;
    last_gross_day: string;
    gross_stale: boolean;
    last_net_month: string;
    seats_snapshot: string;
  };
}

export interface TeamNetOverview {
  month: string;
  teams: TeamSpend[];
  small_teams: number;
  loaded_at: string;
  comparison: TeamComparison;
}

// Per-repository Copilot PR activity, from the v_repository_usage BigQuery view
// via copilot-api's GET /api/v1/copilot/usage/repositories. Privacy-preserving:
// the view is all-time aggregated (never per-day per-repo), excludes private
// repos (repo_visibility IN ('PUBLIC','INTERNAL')) and suppresses low-activity
// repos (fewer than 5 total PRs) — see the k=5 guard mirrored from
// minUsersForDistribution. The three median fields are nullable when no
// qualifying PRs exist for that slice.
export interface RepositoryUsage {
  repo_id: number;
  repo_owner: string;
  repo_name: string;
  repo_visibility: string;
  scope_id: string;
  days_with_data: number;
  first_day: string;
  last_day: string;
  pr_total_created: number;
  pr_total_merged: number;
  pr_total_reviewed: number;
  pr_created_by_copilot: number;
  pr_reviewed_by_copilot: number;
  pr_merged_copilot_authored: number;
  pr_merged_copilot_reviewed: number;
  pr_copilot_suggestions: number;
  pr_copilot_applied_suggestions: number;
  pr_avg_median_minutes_to_merge: number | null;
  pr_avg_median_minutes_to_merge_copilot: number | null;
  pr_avg_median_minutes_to_merge_copilot_reviewed: number | null;
}

export interface DailyCredits {
  day: string;
  credits: number;
  generations: number;
  acceptances: number;
  interactions: number;
  cli_requests: number;
}

export interface UserMetricsSummary {
  user_login: string;
  total_acceptances: number;
  total_interactions: number;
  total_generations: number;
  total_lines_suggested: number;
  total_lines_accepted: number;
  total_lines_deleted: number;
  active_days: number;
  days_in_period: number;
  days_used_agent: number;
  days_used_chat: number;
  days_used_cli: number;
  days_used_code_review: number;
  // Chat mode breakdown (number of requests per mode)
  chat_agent_requests: number;
  chat_ask_requests: number;
  chat_edit_requests: number;
  chat_plan_requests: number;
  chat_custom_requests: number;
  // CLI metrics
  cli_total_requests: number;
  cli_prompts: number;
  cli_sessions: number;
  cli_prompt_tokens: number;
  cli_output_tokens: number;
  // Model usage breakdown
  top_models: Array<{ model: string; interactions: number }>;
  teams: string[];
}

export interface MonthlyTrend {
  month: string;
  days_in_month: number;
  unique_users: number;
  ide_interactions: number;
  code_generations: number;
  cli_requests: number;
  prompt_tokens: number;
  output_tokens: number;
  lines_added: number;
  lines_deleted: number;
  acceptances: number;
  agent_users: number;
  chat_users: number;
  cli_users: number;
}

export interface MonthlyBillingUsage {
  month: string;
  model: string;
  sku: string;
  gross_requests: number;
  net_requests: number;
  gross_amount: number;
  net_amount: number;
}

export interface BillingMonthlyTrend {
  year_month: string;
  total_gross_amount: number;
  total_net_amount: number;
  discount_rate_pct: number;
  distinct_models: number;
}

export interface BillingModelBreakdown {
  year_month: string;
  model: string;
  gross_amount: number;
  net_amount: number;
  pct_of_monthly_net: number;
}

/** AI Credits per active user in one month, from copilot-api. Aggregates only. */
export interface CreditsPerUserMonth {
  month: string;
  median: number;
  mean: number;
  active_users: number;
}

/** One monthly user cohort and its retention in whole percent, from copilot-api. null: the month is not complete yet. */
export interface CohortRetention {
  cohort_month: string;
  cohort_size: number;
  m1: number | null;
  m3: number | null;
  m6: number | null;
}

/**
 * One segment chart from copilot-api, display-ready: whole-percent shares per band and month that sum to 100,
 * or null in every band when the month has no data. The bands are fixed: no merging, also under five.
 */
export interface SegmentChart {
  months: string[];
  bands: { label: string; shares: (number | null)[] }[];
  /** Up minus down in percentage points (movement only). */
  net?: (number | null)[];
}

/** Anonymous monthly segments from copilot-api. */
export interface UserSegments {
  intensity: SegmentChart;
  mode: SegmentChart;
  movement: SegmentChart;
}

/** Copilot coding agent and code review PRs in one month, from repository_metrics. */
export interface CopilotPRMonth {
  month: string;
  created_by_copilot: number;
  reviewed_by_copilot: number;
  days: number;
}

export interface DailySummary {
  date: string;
  daily_active_users: number;
  weekly_active_users: number;
  monthly_active_users: number;
  monthly_active_chat_users: number;
  monthly_active_agent_users: number;
  daily_active_cli_users: number;
  pr_reviewed_by_copilot: number;
  pr_created_by_copilot: number;
  pr_merged_copilot_authored: number;
  cli_session_count: number;
  cli_request_count: number;
  pr_median_minutes_to_merge: number;
  pr_avg_minutes_to_review: number | null;
  pr_avg_review_cycles: number | null;
}

// Privacy-preserving, aggregate-only usage spread for a given month.
// Never contains per-user identifiers — see copilot-api's minUsersForDistribution.
export interface UsageHistogramBucket {
  bucket: string;
  num_users: number;
  /** 1-4 users: the API sends num_users 0 and hides the exact count. */
  suppressed?: boolean;
}

export interface UsageDistribution {
  month: string;
  num_users: number;
  total_licensed_seats: number;
  budget_credits: number;
  credits_deciles: number[] | null;
  interactions_deciles: number[] | null;
  acceptances_deciles: number[] | null;
  credits_histogram: UsageHistogramBucket[];
}

export interface BillingModelDailyCost {
  day: string;
  model: string;
  gross_requests: number;
  net_requests: number;
  gross_amount: number;
  net_amount: number;
}

export interface BillingModelForecastPoint {
  day: string;
  actual_cumulative?: number;
  projected_cumulative: number;
  is_actual: boolean;
}

export interface BillingModelForecast {
  month: string;
  days_in_month: number;
  days_elapsed: number;
  last_actual_day?: string;
  actual_mtd_net_amount: number;
  projected_daily_run_rate: number;
  projected_eom_net_amount: number;
  lower_eom_net_amount: number;
  upper_eom_net_amount: number;
  points: BillingModelForecastPoint[];
}

// Month-end forecast against the enterprise budget, from copilot-api's admin-only
// GET /api/v1/copilot/admin/cost-forecast. Internal: never show on public pages.
export interface CostForecastModel {
  model: "trend" | "per_user";
  gross_mid_usd: number;
  net_low_usd: number;
  net_mid_usd: number;
  net_high_usd: number;
  range_pct: number;
  allowance_runs_out?: string;
  backtest_error_pct: number;
  backtest_months: string[] | null;
  budget_pct: number;
  over_budget: boolean;
}

export interface CostForecast {
  month: string;
  days_elapsed: number;
  days_in_month: number;
  allowance_used_up: boolean;
  allowance_run_out_date?: string;
  net_to_date_usd: number;
  budget_status: "set" | "missing" | "error";
  budget_usd?: number;
  models: CostForecastModel[];
  /** «tidlig i måneden, stor usikkerhet» in the first seven days of the month, when the range is ±35 %. */
  uncertainty_label?: string;
  /** Cumulative net per day this month: actual, then the trend midpoint (forecast: true). */
  days?: { day: string; net_usd: number; forecast: boolean }[];
  /** Projected cumulative net per remaining day with its range, from the trend model shaped by weekday. */
  daily_forecast?: { date: string; cumulative_net_usd: number; low: number; high: number }[];
  /** Cumulative net from 1 January per month: closed months as billed, then the forecast to 31 December. */
  year_forecast?: { month: string; cumulative_net_usd: number; low: number; high: number; forecast: boolean }[];
  year_end?: { mid: number; low: number; high: number };
  /** Days the default personal limit changed, from budget_snapshots. */
  limit_changes?: { date: string; usd: number }[];
}

// AI adoption phases per ISO week, averaged by copilot-api; every phase is sent, also under five
export interface AdoptionCohortWeek {
  week: string; // Monday of the ISO week
  phase: number; // 0 = No cohort, 1 = Code first, 2 = Agent first, 3 = Multi-agent
  user_count: number;
}

export interface AdoptionCohortTrendData {
  weeks: string[];
  // null = no row from the API for that week and phase
  phase0: (number | null)[];
  phase1: (number | null)[];
  phase2: (number | null)[];
  phase3: (number | null)[];
}

// Repository contributor types
export interface Contributor {
  login: string;
  avatarUrl: string;
}

// Users per share of the spending limit, from copilot-api's GET /api/v1/copilot/usage/spend-bands.
// Only aggregates: bands with 1–4 users are merged server-side. `first` and `last` index the base bands
// 0 %, under 25, 25–50, 50–75, 75–90, 90–100 and over 100 %.
export interface SpendBand {
  first: number;
  last: number;
  label: string;
  users: number;
}

export interface SpendBandMonth {
  month: string;
  limit_usd: number;
  limit_changed: boolean;
  users: number;
  bands: SpendBand[] | null;
}

export interface SpendForecastMonth {
  month: string;
  low_usd: number;
  mid_usd: number;
  high_usd: number;
}

export interface SpendBands {
  months: SpendBandMonth[];
  current: SpendBandMonth | null;
  days: number;
  days_in_month: number;
  last_day: string;
  net_ratio: number;
  run_weight: number;
  totals: SpendForecastMonth[];
  forecast: SpendForecastMonth[];
}

/** Users with at least one Copilot usage day in the month (admin only). */
export interface ActiveUsersMonth {
  month: string;
  active_users: number;
}

/** Top top_pct % of billed users by net. users and net_share_pct are hidden (0 and null) below five people. */
export interface HighConsumerTier {
  top_pct: number;
  users: number;
  net_share_pct: number | null;
}

/** Users per share of their own personal limit used on one day, from the 1st (admin only, aggregates only). */
export interface BudgetDay {
  day: string;
  /** 0 and bands null below five users. */
  users: number;
  /** Users with GitHub's own figure; 0 below five. The rest are estimates from usage data. */
  snapshots: number;
  bands: SpendBand[] | null;
}

/** High-consumer groups and personal budget bands for one complete month (admin only, aggregates only). */
export interface HighConsumerMonth {
  month: string;
  /** "hittil" is the current month to date: budget bands only, against each user's own limit. */
  basis: "premium" | "netto" | "hittil";
  users: number;
  tiers: HighConsumerTier[];
  /** Month to date only: the day the bands are for, and users with GitHub's own figure. */
  last_day?: string;
  snapshots?: number;
  limit_usd: number;
  limit_changed: boolean;
  /** Users by gross for the month as share of the limit; null without a limit or too few users. */
  budget_bands: SpendBand[] | null;
}
