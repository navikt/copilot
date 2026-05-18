import { BigQuery } from "@google-cloud/bigquery";
import { loadBigQueryConfig, tableRef, viewRef, type BigQueryConfig } from "./bigquery-config";
import type {
  AdoptionSummary,
  CustomizationDetail,
  CustomizationUsage,
  EnterpriseMetrics,
  LanguageAdoption,
  TeamAdoption,
  TeamUsageSummary,
  UserMetricsSummary,
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
   * @param days - Optional number of days to limit results to
   */
  async getDailyMetrics(days?: number): Promise<EnterpriseMetrics[]> {
    const ref = this.metricsTableRef();
    const whereClause = days != null ? `WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)` : "";
    const query = `SELECT raw_record FROM ${ref} ${whereClause} ORDER BY day ASC`;

    try {
      const rows = await this.query<{ raw_record: string }>(query, days != null ? { days } : undefined);
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
   * Aggregates from the v_team_daily_summary view in the metrics dataset.
   */
  async getTeamUsageSummary(days: number = 7): Promise<TeamUsageSummary[]> {
    const ref = viewRef(this.config.projectId, this.config.metricsDataset, "v_team_daily_summary");
    const query = `
      SELECT
        team_slug,
        ROUND(AVG(active_users), 1) AS avg_active_users,
        MAX(total_users) AS total_users,
        SUM(total_acceptances) AS total_acceptances,
        SUM(total_interactions) AS total_interactions,
        SUM(total_lines_accepted) AS total_lines_accepted,
        COUNT(DISTINCT day) AS days_with_data
      FROM ${ref}
      WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
      GROUP BY team_slug
      HAVING days_with_data >= 3
      ORDER BY avg_active_users DESC
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
   */
  async getUserMetrics(userLogin: string, days: number = 7): Promise<UserMetricsSummary | null> {
    const metricsRef = tableRef(this.config.projectId, this.config.metricsDataset, "user_metrics");
    const teamsRef = tableRef(this.config.projectId, this.config.metricsDataset, "user_teams");

    const query = `
      WITH user_activity AS (
        SELECT
          JSON_VALUE(raw_record, '$.user_login') AS user_login,
          CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64) AS acceptances,
          CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64) AS interactions,
          CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64) AS lines_accepted,
          CAST(JSON_VALUE(raw_record, '$.is_active') AS BOOL) AS is_active
        FROM ${metricsRef}
        WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
          AND JSON_VALUE(raw_record, '$.user_login') = @userLogin
      ),
      user_team_list AS (
        SELECT DISTINCT JSON_VALUE(raw_record, '$.slug') AS team_slug
        FROM ${teamsRef}
        WHERE day = (SELECT MAX(day) FROM ${teamsRef} WHERE JSON_VALUE(raw_record, '$.user_login') = @userLogin)
          AND JSON_VALUE(raw_record, '$.user_login') = @userLogin
      )
      SELECT
        @userLogin AS user_login,
        COALESCE(SUM(ua.acceptances), 0) AS total_acceptances,
        COALESCE(SUM(ua.interactions), 0) AS total_interactions,
        COALESCE(SUM(ua.lines_accepted), 0) AS total_lines_accepted,
        COUNTIF(ua.is_active) AS active_days,
        COUNT(*) AS days_in_period,
        ARRAY(SELECT team_slug FROM user_team_list) AS teams
      FROM user_activity ua
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
