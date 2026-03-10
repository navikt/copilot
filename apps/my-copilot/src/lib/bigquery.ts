import { BigQuery } from "@google-cloud/bigquery";
import type { EnterpriseMetrics } from "./types";

const projectId = process.env.GCP_TEAM_PROJECT_ID;
const dataset = process.env.BIGQUERY_DATASET || "copilot_metrics";
const table = process.env.BIGQUERY_TABLE || "usage_metrics";

const bigquery = new BigQuery({
  projectId,
});

function fullTableRef() {
  return `\`${projectId}.${dataset}.${table}\``;
}

function parseRows(rows: Array<{ raw_record: string }>): EnterpriseMetrics[] {
  return rows.map((row) => (typeof row.raw_record === "string" ? JSON.parse(row.raw_record) : row.raw_record));
}

export async function getDailyMetrics(days?: number): Promise<EnterpriseMetrics[]> {
  const ref = fullTableRef();

  const whereClause = days != null ? `WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)` : "";

  const query = `SELECT raw_record FROM ${ref} ${whereClause} ORDER BY day ASC`;

  try {
    const [rows] = await bigquery.query({
      query,
      params: days != null ? { days } : {},
    });
    return parseRows(rows);
  } catch (err) {
    console.error("[bigquery] getDailyMetrics failed:", err);
    throw err;
  }
}
