import { NextResponse } from "next/server";
import { getDailyMetrics } from "@/lib/bigquery";

export async function GET() {
  try {
    const usage = await getDailyMetrics();

    if (!usage || usage.length === 0) {
      return NextResponse.json({ error: "No usage data available" }, { status: 404 });
    }

    return NextResponse.json(usage);
  } catch (err) {
    return NextResponse.json(
      { error: `Failed to fetch usage data: ${err instanceof Error ? err.message : String(err)}` },
      { status: 500 }
    );
  }
}
