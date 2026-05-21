import { BigQuery } from "@google-cloud/bigquery";
import { loadBigQueryConfig, tableRef, viewRef, type BigQueryConfig } from "./bigquery-config";
import type {
  AdoptionSummary,
  CustomizationDetail,
  CustomizationUsage,
  EnterpriseMetrics,
  LanguageAdoption,
  MonthlyModelUsage,
  MonthlyTrend,
  TeamAdoption,
  TeamUsageSummary,
  UserMetricsSummary,
  WeeklyTrend,
} from "./types";

/**
 * Serialize BigQuery row to plain object.
 * BigQuery returns special objects for DATE, TIMESTAMP, etc. that cannot be
 * passed from Server Components to Client Components. This converts them to
 * plain JSON-serializable values.
 */
function serializeBigQueryRow<T>(row: Record<string, unknown>): T {
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(row)) {
    if (value && typeof value === "object" && "value" in value) {
      // BigQuery DATE/TIMESTAMP objects have a `value` property
      result[key] = (value as { value: string }).value;
    } else {
      result[key] = value;
    }
  }
  return result as T;
}

/**
 * BigQuery client abstraction for Copilot data access.
 *
 * This class provides a clean interface for querying Copilot metrics and adoption data,
 * with proper dependency injection for testability.
 */
export class CopilotBigQueryClient {
  private readonly bigquery: BigQuery;
  private readonly config: BigQueryConfig;

  constructor(config: BigQueryConfig, bigquery?: BigQuery) {
    this.config = config;
    this.bigquery = bigquery ?? new BigQuery({ projectId: config.projectId });
  }

  /**
   * Get the metrics table reference.
   */
  private metricsTableRef(): string {
    return tableRef(this.config.projectId, this.config.metricsDataset, this.config.metricsTable);
  }

  /**
   * Get an adoption view reference.
   */
  private adoptionViewRef(viewName: string): string {
    return viewRef(this.config.projectId, this.config.adoptionDataset, viewName);
  }

  /**
   * Execute a query and return typed results.
   * Results are serialized to plain objects for Server→Client component compatibility.
   */
  private async query<T>(sql: string, params?: Record<string, unknown>): Promise<T[]> {
    const [rows] = await this.bigquery.query({
      query: sql,
      params,
    });
    return (rows as Record<string, unknown>[]).map((row) => serializeBigQueryRow<T>(row));
  }

  /**
   * Get daily Copilot usage metrics.
   * Uses partition pruning on `day` column for cost efficiency.
   * @param days - Optional number of days to limit results to (default: 365)
   */
  async getDailyMetrics(days?: number): Promise<EnterpriseMetrics[]> {
    const ref = this.metricsTableRef();
    // Always limit to avoid full table scans; entity metrics are ~1 row/day so cost is low
    const effectiveDays = days ?? 365;
    const query = `
      SELECT raw_record
      FROM ${ref}
      WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
        AND scope = 'enterprise'
      ORDER BY day ASC
    `;

    try {
      const rows = await this.query<{ raw_record: string }>(query, { days: effectiveDays });
      return rows.map((row) => (typeof row.raw_record === "string" ? JSON.parse(row.raw_record) : row.raw_record));
    } catch (err) {
      console.error("[bigquery] getDailyMetrics failed:", err);
      throw err;
    }
  }

  /**
   * Get the latest adoption summary.
   */
  async getAdoptionSummary(): Promise<AdoptionSummary | null> {
    const query = `
      SELECT * FROM ${this.adoptionViewRef("v_adoption_summary")}
      ORDER BY scan_date DESC
      LIMIT 1
    `;

    try {
      const rows = await this.query<AdoptionSummary>(query);
      return rows.length > 0 ? rows[0] : null;
    } catch (err) {
      console.error("[bigquery] getAdoptionSummary failed:", err);
      throw err;
    }
  }

  /**
   * Get team adoption data for the latest scan.
   */
  async getTeamAdoption(): Promise<TeamAdoption[]> {
    const viewName = "v_team_adoption";
    const query = `
      SELECT * FROM ${this.adoptionViewRef(viewName)}
      WHERE scan_date = (SELECT MAX(scan_date) FROM ${this.adoptionViewRef(viewName)})
      ORDER BY repos_with_customizations DESC
    `;

    try {
      return await this.query<TeamAdoption>(query);
    } catch (err) {
      console.error("[bigquery] getTeamAdoption failed:", err);
      throw err;
    }
  }

  /**
   * Get top customization files (agents, skills, instructions, prompts) for the latest scan.
   */
  async getCustomizationDetails(): Promise<CustomizationDetail[]> {
    const viewName = "v_customization_details";
    const query = `
      SELECT category, file_name,
        COUNT(DISTINCT repo) AS repo_count,
        COUNTIF(is_recently_active) AS active_repo_count
      FROM ${this.adoptionViewRef(viewName)}
      WHERE scan_date = (SELECT MAX(scan_date) FROM ${this.adoptionViewRef(viewName)})
      GROUP BY category, file_name
      ORDER BY repo_count DESC
    `;

    try {
      return await this.query<CustomizationDetail>(query);
    } catch (err) {
      console.error("[bigquery] getCustomizationDetails failed:", err);
      throw err;
    }
  }

  /**
   * Get customization usage with sample repo names for catalog enrichment.
   */
  async getCustomizationUsage(): Promise<CustomizationUsage[]> {
    const viewName = "v_customization_details";
    const query = `
      SELECT
        category,
        file_name,
        COUNT(DISTINCT repo) AS repo_count,
        ARRAY_AGG(DISTINCT repo ORDER BY repo LIMIT 5) AS sample_repos
      FROM ${this.adoptionViewRef(viewName)}
      WHERE scan_date = (SELECT MAX(scan_date) FROM ${this.adoptionViewRef(viewName)})
      GROUP BY category, file_name
      ORDER BY repo_count DESC
    `;

    try {
      return await this.query<CustomizationUsage>(query);
    } catch (err) {
      console.error("[bigquery] getCustomizationUsage failed:", err);
      throw err;
    }
  }

  /**
   * Get language adoption data for the latest scan.
   */
  async getLanguageAdoption(): Promise<LanguageAdoption[]> {
    const viewName = "v_language_adoption";
    const query = `
      SELECT * FROM ${this.adoptionViewRef(viewName)}
      WHERE scan_date = (SELECT MAX(scan_date) FROM ${this.adoptionViewRef(viewName)})
      ORDER BY total_repos DESC
    `;

    try {
      return await this.query<LanguageAdoption>(query);
    } catch (err) {
      console.error("[bigquery] getLanguageAdoption failed:", err);
      throw err;
    }
  }

  /**
   * Get team-level Copilot usage summary for the last N days.
   * Queries user_teams and user_metrics tables directly (no view dependency).
   * Uses partition pruning on `day` and cluster pruning on `scope`.
   */
  async getTeamUsageSummary(days: number = 7): Promise<TeamUsageSummary[]> {
    const teamsRef = tableRef(this.config.projectId, this.config.metricsDataset, "user_teams");
    const metricsRef = tableRef(this.config.projectId, this.config.metricsDataset, "user_metrics");
    const query = `
      WITH latest_teams AS (
        SELECT
          JSON_VALUE(raw_record, '$.user_id') AS user_id,
          JSON_VALUE(raw_record, '$.slug') AS team_slug
        FROM ${teamsRef}
        WHERE day = (SELECT MAX(day) FROM ${teamsRef} WHERE scope = 'enterprise')
          AND scope = 'enterprise'
        GROUP BY user_id, team_slug
      ),
      metrics AS (
        SELECT
          JSON_VALUE(raw_record, '$.user_id') AS user_id,
          day,
          SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64) AS generations,
          SAFE_CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64) AS acceptances,
          SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64) AS interactions,
          SAFE_CAST(JSON_VALUE(raw_record, '$.loc_suggested_to_add_sum') AS INT64) AS lines_suggested,
          SAFE_CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64) AS lines_accepted,
          SAFE_CAST(JSON_VALUE(raw_record, '$.used_agent') AS BOOL) AS used_agent,
          raw_record
        FROM ${metricsRef}
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
          AND scope = 'enterprise'
      ),
      team_metrics AS (
        SELECT
          t.team_slug,
          m.user_id,
          m.day,
          COALESCE(m.generations, 0) AS generations,
          COALESCE(m.acceptances, 0) AS acceptances,
          COALESCE(m.interactions, 0) AS interactions,
          COALESCE(m.lines_suggested, 0) AS lines_suggested,
          COALESCE(m.lines_accepted, 0) AS lines_accepted,
          COALESCE(m.used_agent, FALSE) AS used_agent,
          m.raw_record
        FROM latest_teams t
        INNER JOIN metrics m ON t.user_id = m.user_id
      ),
      team_model_usage AS (
        SELECT
          tm.team_slug,
          JSON_VALUE(mf, '$.model') AS model,
          SUM(SAFE_CAST(JSON_VALUE(mf, '$.user_initiated_interaction_count') AS INT64)) AS interactions
        FROM team_metrics tm,
          UNNEST(JSON_QUERY_ARRAY(tm.raw_record, '$.totals_by_model_feature')) AS mf
        WHERE JSON_VALUE(mf, '$.model') IS NOT NULL
          AND JSON_VALUE(mf, '$.model') != 'others'
        GROUP BY tm.team_slug, model
        HAVING interactions > 0
      ),
      team_model_ranked AS (
        SELECT
          team_slug,
          model,
          interactions,
          ROW_NUMBER() OVER (PARTITION BY team_slug ORDER BY interactions DESC) AS rn
        FROM team_model_usage
      ),
      team_models_agg AS (
        SELECT
          team_slug,
          ARRAY_AGG(STRUCT(model, interactions) ORDER BY interactions DESC) AS top_models
        FROM team_model_ranked
        WHERE rn <= 3
        GROUP BY team_slug
      ),
      team_summary AS (
        SELECT
          tm.team_slug,
          COUNT(DISTINCT CASE WHEN tm.acceptances + tm.interactions > 0 THEN tm.user_id END) AS avg_active_users,
          COUNT(DISTINCT tm.user_id) AS total_users,
          SUM(tm.generations) AS total_generations,
          SUM(tm.acceptances) AS total_acceptances,
          SUM(tm.interactions) AS total_interactions,
          SUM(tm.lines_suggested) AS total_lines_suggested,
          SUM(tm.lines_accepted) AS total_lines_accepted,
          COUNT(DISTINCT CASE WHEN tm.used_agent THEN tm.user_id END) AS agent_users,
          COUNT(DISTINCT tm.day) AS days_with_data
        FROM team_metrics tm
        GROUP BY tm.team_slug
      )
      SELECT
        ts.*,
        tma.top_models
      FROM team_summary ts
      LEFT JOIN team_models_agg tma ON tma.team_slug = ts.team_slug
      ORDER BY ts.avg_active_users DESC
    `;

    try {
      return await this.query<TeamUsageSummary>(query, { days });
    } catch (err) {
      console.error("[bigquery] getTeamUsageSummary failed:", err);
      throw err;
    }
  }

  /**
   * Get personal usage metrics for a specific GitHub user.
   * Queries user_metrics table directly and joins user_teams for team membership.
   * Uses partition pruning on `day` and cluster pruning on `scope`.
   */
  async getUserMetrics(userLogin: string, days: number = 7): Promise<UserMetricsSummary | null> {
    const metricsRef = tableRef(this.config.projectId, this.config.metricsDataset, "user_metrics");
    const teamsRef = tableRef(this.config.projectId, this.config.metricsDataset, "user_teams");

    const query = `
      WITH user_activity AS (
        SELECT
          JSON_VALUE(raw_record, '$.user_login') AS user_login,
          SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64) AS generations,
          SAFE_CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64) AS acceptances,
          SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64) AS interactions,
          SAFE_CAST(JSON_VALUE(raw_record, '$.loc_suggested_to_add_sum') AS INT64) AS lines_suggested,
          SAFE_CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64) AS lines_accepted,
          SAFE_CAST(JSON_VALUE(raw_record, '$.loc_deleted_sum') AS INT64) AS lines_deleted,
          SAFE_CAST(JSON_VALUE(raw_record, '$.used_agent') AS BOOL) AS used_agent,
          SAFE_CAST(JSON_VALUE(raw_record, '$.used_chat') AS BOOL) AS used_chat,
          SAFE_CAST(JSON_VALUE(raw_record, '$.used_cli') AS BOOL) AS used_cli,
          SAFE_CAST(JSON_VALUE(raw_record, '$.used_copilot_code_review_active') AS BOOL) AS used_code_review,
          -- Chat mode breakdown
          SAFE_CAST(JSON_VALUE(raw_record, '$.chat_panel_agent_mode') AS INT64) AS chat_agent_mode,
          SAFE_CAST(JSON_VALUE(raw_record, '$.chat_panel_ask_mode') AS INT64) AS chat_ask_mode,
          SAFE_CAST(JSON_VALUE(raw_record, '$.chat_panel_edit_mode') AS INT64) AS chat_edit_mode,
          SAFE_CAST(JSON_VALUE(raw_record, '$.chat_panel_plan_mode') AS INT64) AS chat_plan_mode,
          SAFE_CAST(JSON_VALUE(raw_record, '$.chat_panel_custom_mode') AS INT64) AS chat_custom_mode,
          -- CLI metrics
          SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.request_count') AS INT64) AS cli_requests,
          SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.prompt_count') AS INT64) AS cli_prompts,
          SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.session_count') AS INT64) AS cli_sessions,
          SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.prompt_tokens_sum') AS INT64) AS cli_prompt_tokens,
          SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.output_tokens_sum') AS INT64) AS cli_output_tokens,
          raw_record
        FROM ${metricsRef}
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
          AND scope = 'enterprise'
          AND JSON_VALUE(raw_record, '$.user_login') = @userLogin
      ),
      user_team_list AS (
        SELECT DISTINCT JSON_VALUE(raw_record, '$.slug') AS team_slug
        FROM ${teamsRef}
        WHERE day = (SELECT MAX(day) FROM ${teamsRef} WHERE scope = 'enterprise')
          AND scope = 'enterprise'
          AND JSON_VALUE(raw_record, '$.user_login') = @userLogin
      ),
      model_usage AS (
        SELECT
          JSON_VALUE(mf, '$.model') AS model,
          SUM(SAFE_CAST(JSON_VALUE(mf, '$.user_initiated_interaction_count') AS INT64)) AS interactions
        FROM user_activity,
          UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_model_feature')) AS mf
        WHERE JSON_VALUE(mf, '$.model') IS NOT NULL
          AND JSON_VALUE(mf, '$.model') != 'others'
        GROUP BY model
        HAVING interactions > 0
        ORDER BY interactions DESC
        LIMIT 5
      ),
      model_agg AS (
        SELECT ARRAY_AGG(STRUCT(model, interactions) ORDER BY interactions DESC) AS top_models
        FROM model_usage
      ),
      team_agg AS (
        SELECT ARRAY_AGG(team_slug) AS teams
        FROM user_team_list
      )
      SELECT
        @userLogin AS user_login,
        COALESCE(SUM(ua.generations), 0) AS total_generations,
        COALESCE(SUM(ua.acceptances), 0) AS total_acceptances,
        COALESCE(SUM(ua.interactions), 0) AS total_interactions,
        COALESCE(SUM(ua.lines_suggested), 0) AS total_lines_suggested,
        COALESCE(SUM(ua.lines_accepted), 0) AS total_lines_accepted,
        COALESCE(SUM(ua.lines_deleted), 0) AS total_lines_deleted,
        COUNTIF(COALESCE(ua.acceptances, 0) + COALESCE(ua.interactions, 0) > 0) AS active_days,
        COUNT(*) AS days_in_period,
        COUNTIF(ua.used_agent) AS days_used_agent,
        COUNTIF(ua.used_chat) AS days_used_chat,
        COUNTIF(ua.used_cli) AS days_used_cli,
        COUNTIF(ua.used_code_review) AS days_used_code_review,
        -- Chat mode totals
        COALESCE(SUM(ua.chat_agent_mode), 0) AS chat_agent_requests,
        COALESCE(SUM(ua.chat_ask_mode), 0) AS chat_ask_requests,
        COALESCE(SUM(ua.chat_edit_mode), 0) AS chat_edit_requests,
        COALESCE(SUM(ua.chat_plan_mode), 0) AS chat_plan_requests,
        COALESCE(SUM(ua.chat_custom_mode), 0) AS chat_custom_requests,
        -- CLI totals
        COALESCE(SUM(ua.cli_requests), 0) AS cli_total_requests,
        COALESCE(SUM(ua.cli_prompts), 0) AS cli_prompts,
        COALESCE(SUM(ua.cli_sessions), 0) AS cli_sessions,
        COALESCE(SUM(ua.cli_prompt_tokens), 0) AS cli_prompt_tokens,
        COALESCE(SUM(ua.cli_output_tokens), 0) AS cli_output_tokens,
        -- Model breakdown (top 5)
        ANY_VALUE(ma.top_models) AS top_models,
        ANY_VALUE(ta.teams) AS teams
      FROM user_activity ua
      CROSS JOIN model_agg ma
      CROSS JOIN team_agg ta
    `;

    try {
      const rows = await this.query<UserMetricsSummary>(query, { days, userLogin });
      if (rows.length === 0 || rows[0].days_in_period === 0) return null;
      return rows[0];
    } catch (err) {
      console.error("[bigquery] getUserMetrics failed:", err);
      throw err;
    }
  }

  /**
   * Get org-wide monthly trends aggregated from user_metrics.
   * Uses COUNT(DISTINCT) for user counts per feature to avoid double-counting.
   * Leverages partition pruning on `day` and cluster pruning on `scope`.
   */
  async getMonthlyTrends(months: number = 12): Promise<MonthlyTrend[]> {
    const metricsRef = tableRef(this.config.projectId, this.config.metricsDataset, "user_metrics");

    const query = `
      SELECT
        FORMAT_DATE('%Y-%m', day) AS month,
        COUNT(DISTINCT day) AS days_in_month,
        -- Only count users with actual activity (not just appearing in report)
        COUNT(DISTINCT IF(
          COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64), 0)
          + COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64), 0)
          + COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.request_count') AS INT64), 0) > 0,
          JSON_VALUE(raw_record, '$.user_id'),
          NULL
        )) AS unique_users,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64)), 0) AS ide_interactions,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64)), 0) AS code_generations,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.request_count') AS INT64)), 0) AS cli_requests,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.prompt_tokens_sum') AS INT64)), 0) AS prompt_tokens,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.output_tokens_sum') AS INT64)), 0) AS output_tokens,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64)), 0) AS lines_added,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.loc_deleted_sum') AS INT64)), 0) AS lines_deleted,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64)), 0) AS acceptances,
        COUNT(DISTINCT IF(SAFE_CAST(JSON_VALUE(raw_record, '$.used_agent') AS BOOL), JSON_VALUE(raw_record, '$.user_id'), NULL)) AS agent_users,
        COUNT(DISTINCT IF(SAFE_CAST(JSON_VALUE(raw_record, '$.used_chat') AS BOOL), JSON_VALUE(raw_record, '$.user_id'), NULL)) AS chat_users,
        COUNT(DISTINCT IF(SAFE_CAST(JSON_VALUE(raw_record, '$.used_cli') AS BOOL), JSON_VALUE(raw_record, '$.user_id'), NULL)) AS cli_users
      FROM ${metricsRef}
      WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @months MONTH)
        AND scope = 'enterprise'
      GROUP BY month
      ORDER BY month
    `;

    try {
      return await this.query<MonthlyTrend>(query, { months });
    } catch (err) {
      console.error("[bigquery] getMonthlyTrends failed:", err);
      throw err;
    }
  }

  /**
   * Get org-wide monthly model/token usage breakdown.
   * Returns one row per model per month, ordered by month then interactions.
   */
  async getMonthlyModelUsage(months: number = 12): Promise<MonthlyModelUsage[]> {
    const metricsRef = tableRef(this.config.projectId, this.config.metricsDataset, "user_metrics");

    // Token data lives in totals_by_cli, not per model. We aggregate CLI tokens
    // per month alongside per-model activity from totals_by_model_feature.
    // We sum ALL activity: interactions + code generations + code acceptances
    // to get a more accurate picture of actual model usage.
    const query = `
      WITH model_activity AS (
        SELECT
          FORMAT_DATE('%Y-%m', day) AS month,
          JSON_VALUE(mf, '$.model') AS model,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(mf, '$.user_initiated_interaction_count') AS INT64)), 0) AS interactions,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(mf, '$.code_generation_activity_count') AS INT64)), 0) AS generations,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(mf, '$.code_acceptance_activity_count') AS INT64)), 0) AS acceptances
        FROM ${metricsRef},
          UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_model_feature')) AS mf
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @months MONTH)
          AND scope = 'enterprise'
          AND JSON_VALUE(mf, '$.model') IS NOT NULL
          AND JSON_VALUE(mf, '$.model') != 'others'
        GROUP BY month, model
        HAVING (interactions + generations) > 0
      ),
      monthly_tokens AS (
        SELECT
          FORMAT_DATE('%Y-%m', day) AS month,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.prompt_tokens_sum') AS INT64)), 0) AS prompt_tokens,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.output_tokens_sum') AS INT64)), 0) AS output_tokens
        FROM ${metricsRef}
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @months MONTH)
          AND scope = 'enterprise'
        GROUP BY month
      )
      SELECT
        ma.month,
        ma.model,
        (ma.interactions + ma.generations + ma.acceptances) AS interactions,
        COALESCE(mt.prompt_tokens, 0) AS prompt_tokens,
        COALESCE(mt.output_tokens, 0) AS output_tokens
      FROM model_activity ma
      LEFT JOIN monthly_tokens mt ON ma.month = mt.month
      ORDER BY ma.month, interactions DESC
    `;

    try {
      return await this.query<MonthlyModelUsage>(query, { months });
    } catch (err) {
      console.error("[bigquery] getMonthlyModelUsage failed:", err);
      throw err;
    }
  }

  /**
   * Get personal weekly trends for a specific user.
   * Uses partition pruning on `day` and cluster pruning on `scope`.
   */
  async getUserWeeklyTrends(userLogin: string, weeks: number = 12): Promise<WeeklyTrend[]> {
    const metricsRef = tableRef(this.config.projectId, this.config.metricsDataset, "user_metrics");
    const days = weeks * 7;

    const query = `
      WITH weekly_data AS (
        SELECT
          FORMAT_DATE('%G-W%V', day) AS week,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64)), 0) AS interactions,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.request_count') AS INT64)), 0) AS cli_requests,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64)), 0) AS acceptances,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64)), 0) AS lines_added,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.loc_deleted_sum') AS INT64)), 0) AS lines_deleted,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.prompt_tokens_sum') AS INT64)), 0) AS prompt_tokens,
          COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.output_tokens_sum') AS INT64)), 0) AS output_tokens,
          COUNT(*) AS active_days
        FROM ${metricsRef}
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
          AND scope = 'enterprise'
          AND JSON_VALUE(raw_record, '$.user_login') = @userLogin
        GROUP BY week
      ),
      weekly_models AS (
        SELECT
          FORMAT_DATE('%G-W%V', day) AS week,
          JSON_VALUE(mf, '$.model') AS model,
          SUM(SAFE_CAST(JSON_VALUE(mf, '$.user_initiated_interaction_count') AS INT64)) AS interactions
        FROM ${metricsRef},
          UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_model_feature')) AS mf
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
          AND scope = 'enterprise'
          AND JSON_VALUE(raw_record, '$.user_login') = @userLogin
          AND JSON_VALUE(mf, '$.model') IS NOT NULL
          AND JSON_VALUE(mf, '$.model') != 'others'
        GROUP BY week, model
        HAVING interactions > 0
      ),
      weekly_models_ranked AS (
        SELECT
          week,
          model,
          interactions,
          ROW_NUMBER() OVER (PARTITION BY week ORDER BY interactions DESC) AS rn
        FROM weekly_models
      ),
      weekly_models_agg AS (
        SELECT
          week,
          ARRAY_AGG(STRUCT(model, interactions) ORDER BY interactions DESC) AS models
        FROM weekly_models_ranked
        WHERE rn <= 5
        GROUP BY week
      )
      SELECT
        wd.week,
        wd.interactions,
        wd.cli_requests,
        wd.acceptances,
        wd.lines_added,
        wd.lines_deleted,
        wd.prompt_tokens,
        wd.output_tokens,
        wd.active_days,
        wma.models
      FROM weekly_data wd
      LEFT JOIN weekly_models_agg wma ON wma.week = wd.week
      ORDER BY wd.week
    `;

    try {
      return await this.query<WeeklyTrend>(query, { days, userLogin });
    } catch (err) {
      console.error("[bigquery] getUserWeeklyTrends failed:", err);
      throw err;
    }
  }
}

// Default client instance (lazy-loaded)
let defaultClient: CopilotBigQueryClient | null = null;

function getDefaultClient(): CopilotBigQueryClient {
  if (!defaultClient) {
    defaultClient = new CopilotBigQueryClient(loadBigQueryConfig());
  }
  return defaultClient;
}

// Export convenience functions that use the default client
export async function getDailyMetrics(days?: number): Promise<EnterpriseMetrics[]> {
  return getDefaultClient().getDailyMetrics(days);
}

export async function getAdoptionSummary(): Promise<AdoptionSummary | null> {
  return getDefaultClient().getAdoptionSummary();
}

export async function getTeamAdoption(): Promise<TeamAdoption[]> {
  return getDefaultClient().getTeamAdoption();
}

export async function getLanguageAdoption(): Promise<LanguageAdoption[]> {
  return getDefaultClient().getLanguageAdoption();
}

export async function getCustomizationDetails(): Promise<CustomizationDetail[]> {
  return getDefaultClient().getCustomizationDetails();
}

export async function getCustomizationUsage(): Promise<CustomizationUsage[]> {
  return getDefaultClient().getCustomizationUsage();
}

export async function getTeamUsageSummary(days?: number): Promise<TeamUsageSummary[]> {
  return getDefaultClient().getTeamUsageSummary(days);
}

export async function getUserMetrics(userLogin: string, days?: number): Promise<UserMetricsSummary | null> {
  return getDefaultClient().getUserMetrics(userLogin, days);
}

export async function getMonthlyTrends(months: number = 12): Promise<MonthlyTrend[]> {
  return getDefaultClient().getMonthlyTrends(months);
}

export async function getMonthlyModelUsage(months: number = 12): Promise<MonthlyModelUsage[]> {
  return getDefaultClient().getMonthlyModelUsage(months);
}

export async function getUserWeeklyTrends(userLogin: string, weeks: number = 12): Promise<WeeklyTrend[]> {
  return getDefaultClient().getUserWeeklyTrends(userLogin, weeks);
}
