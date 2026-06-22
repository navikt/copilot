import { getUserToken } from "@/lib/auth";
import { backendRequest, BackendApiError } from "@/lib/backend-api";
import { NextResponse } from "next/server";
import type { UserMetricsSummary } from "@/lib/types";

export async function GET(request: Request) {
  const token = await getUserToken();
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const { searchParams } = new URL(request.url);
  const username = searchParams.get("username");
  if (!username) {
    return NextResponse.json({ error: "username required" }, { status: 400 });
  }

  try {
    const metrics = await backendRequest<UserMetricsSummary>(
      `/api/v1/copilot/usage/user/${encodeURIComponent(username)}?days=30`,
      token
    );
    return NextResponse.json(metrics);
  } catch (error) {
    if (error instanceof BackendApiError) {
      if (error.status === 404) {
        return NextResponse.json({ error: "no_data" }, { status: 404 });
      }
      if (error.status === 503) {
        return NextResponse.json({ error: "unavailable" }, { status: 503 });
      }
    }
    console.error("[usage] Failed to fetch user metrics:", error);
    return NextResponse.json({ error: "failed_to_fetch_usage" }, { status: 500 });
  }
}
