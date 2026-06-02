import { NextResponse } from "next/server";
import { getCopilotUsageMetrics } from "@/lib/cached-bigquery";
import { getUser, getUserToken } from "@/lib/auth";

export async function GET() {
  const user = await getUser(false);
  if (!user) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const token = await getUserToken();
  if (!token) {
    return NextResponse.json({ error: "No authentication token" }, { status: 401 });
  }

  try {
    const { usage, error } = await getCopilotUsageMetrics(token);

    if (error) {
      return NextResponse.json({ error: "Failed to fetch usage data" }, { status: 500 });
    }

    if (!usage || usage.length === 0) {
      return NextResponse.json({ error: "No usage data available" }, { status: 404 });
    }

    return NextResponse.json(usage);
  } catch {
    return NextResponse.json({ error: "Failed to fetch usage data" }, { status: 500 });
  }
}
