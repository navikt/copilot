import { NextResponse } from "next/server";
import { getCachedBigQueryUsage } from "@/lib/cached-bigquery";
import { getUser } from "@/lib/auth";

export async function GET() {
  const user = await getUser(false);
  if (!user) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const { usage, error } = await getCachedBigQueryUsage();

    if (error) {
      return NextResponse.json({ error }, { status: 500 });
    }

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
