import { getUser, getUserToken } from "@/lib/auth";
import { backendRequest, BackendApiError } from "@/lib/backend-api";
import { NextResponse } from "next/server";

export interface GlobalBudgetResponse {
  budgetAmount: number;
  consumedAmount: number | null;
}

export async function GET() {
  const user = await getUser(false);
  const token = await getUserToken();

  if (!user || !token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const budget = await backendRequest<GlobalBudgetResponse>("/api/v1/copilot/budget/global", token);
    return NextResponse.json(budget, {
      headers: { "Cache-Control": "public, max-age=1800, s-maxage=1800" },
    });
  } catch (error) {
    if (error instanceof BackendApiError && error.status === 404) {
      return NextResponse.json({ error: "no_budget" }, { status: 404 });
    }
    console.error("[budget/global] Failed to fetch global budget:", error);
    return NextResponse.json({ error: "failed_to_fetch_budget" }, { status: 500 });
  }
}
