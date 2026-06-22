import { getUserToken } from "@/lib/auth";
import { backendRequest, BackendApiError } from "@/lib/backend-api";
import { NextResponse } from "next/server";
import type { DailyCredits } from "@/lib/types";

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
    const credits = await backendRequest<DailyCredits[]>(
      `/api/v1/copilot/usage/user/${encodeURIComponent(username)}/daily-credits?days=30`,
      token
    );
    return NextResponse.json(credits);
  } catch (error) {
    if (error instanceof BackendApiError && error.status === 404) {
      return NextResponse.json([], { status: 200 });
    }
    console.error("[credits] Failed to fetch daily credits:", error);
    return NextResponse.json({ error: "failed_to_fetch_credits" }, { status: 500 });
  }
}
