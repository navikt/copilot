import { BigQuery } from "@google-cloud/bigquery";
import { loadBigQueryConfig, tableRef, viewRef, type BigQueryConfig } from "./bigquery-config";
import type { AdoptionSummary, EnterpriseMetrics, LanguageAdoption, TeamAdoption } from "./types";

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
