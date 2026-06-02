import { getUser, getUserToken } from "@/lib/auth";
import { backendRequest, BackendApiError } from "@/lib/backend-api";
import { NextResponse } from "next/server";

export interface BudgetResponse {
  budgetAmount: number;
  consumedAmount: number | null;
  isOverride: boolean;
  defaultBudget: number;
}

export async function GET() {
  const user = await getUser(false);
  const token = await getUserToken();

  if (!user || !token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const budget = await backendRequest<BudgetResponse>("/api/v1/copilot/budget", token);
    return NextResponse.json(budget);
  } catch (error) {
    if (error instanceof BackendApiError) {
      if (error.status === 404) {
        return NextResponse.json({ error: "no_budget" }, { status: 404 });
      }
      if (error.status === 503) {
        return NextResponse.json({ error: "unavailable" }, { status: 503 });
      }
    }
    console.error("[budget] Failed to fetch budget:", error);
    return NextResponse.json({ error: "failed_to_fetch_budget" }, { status: 500 });
  }
}
