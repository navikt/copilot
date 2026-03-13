import { getUser } from "@/lib/auth";
import { loadBigQueryConfig, tableRef } from "@/lib/bigquery-config";
import { BigQuery } from "@google-cloud/bigquery";
import { NextResponse } from "next/server";

export async function GET(request: Request) {
  const user = await getUser(false);
  if (!user) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const config = loadBigQueryConfig();
  const bigquery = new BigQuery({ projectId: config.projectId });
  const ref = tableRef(config.projectId, config.adoptionDataset, "repo_scan");

  const { searchParams } = new URL(request.url);
  const limit = Math.min(Number(searchParams.get("limit") || "100"), 1000);
  const scanDate = searchParams.get("date");
  const hasCustomization = searchParams.get("has_customization");

  let where = "";
  const params: Record<string, unknown> = {};

  if (scanDate) {
    where += " WHERE scan_date = DATE(@scanDate)";
    params.scanDate = scanDate;
  }

  if (hasCustomization === "true") {
    where += where ? " AND" : " WHERE";
    where += " has_any_customization = true";
  }

  const countQuery = `SELECT COUNT(*) as total, COUNT(DISTINCT scan_date) as scan_dates, MIN(scan_date) as first_scan, MAX(scan_date) as last_scan FROM ${ref}${where}`;
  const sampleQuery = `SELECT * FROM ${ref}${where} ORDER BY scan_date DESC, repo ASC LIMIT @limit`;

  try {
    const [countRows] = await bigquery.query({ query: countQuery, params });
    const [sampleRows] = await bigquery.query({ query: sampleQuery, params: { ...params, limit } });

    return NextResponse.json({
      stats: countRows[0],
      sample: sampleRows,
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    return NextResponse.json({ error: message }, { status: 500 });
  }
}
