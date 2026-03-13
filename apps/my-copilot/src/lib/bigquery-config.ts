/**
 * BigQuery configuration for Copilot data sources.
 *
 * Supports two datasets:
 * - Metrics: Daily Copilot usage data (from copilot-metrics naisjob)
 * - Adoption: Repository AI customization scan data (from copilot-adoption naisjob)
 */

export interface BigQueryConfig {
  projectId: string;
  metricsDataset: string;
  metricsTable: string;
  adoptionDataset: string;
}

/**
 * Load BigQuery configuration from environment variables.
 * Uses sensible defaults for dataset/table names.
 */
export function loadBigQueryConfig(): BigQueryConfig {
  const projectId = process.env.GCP_TEAM_PROJECT_ID;

  if (!projectId) {
    throw new Error("GCP_TEAM_PROJECT_ID environment variable is required");
  }

  return {
    projectId,
    metricsDataset: process.env.COPILOT_METRICS_DATASET || "copilot_metrics",
    metricsTable: process.env.COPILOT_METRICS_TABLE || "usage_metrics",
    adoptionDataset: process.env.COPILOT_ADOPTION_DATASET || "copilot_adoption",
  };
}

/**
 * Create a fully-qualified BigQuery table reference.
 */
export function tableRef(projectId: string, dataset: string, table: string): string {
  return `\`${projectId}.${dataset}.${table}\``;
}

/**
 * Create a fully-qualified BigQuery view reference.
 */
export function viewRef(projectId: string, dataset: string, view: string): string {
  return `\`${projectId}.${dataset}.${view}\``;
}
