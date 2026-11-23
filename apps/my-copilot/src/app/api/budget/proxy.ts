import { getUser, getUserToken } from "@/lib/auth";
import { backendRequest, BackendApiError } from "@/lib/backend-api";
import { NextResponse } from "next/server";

/**
 * Forwards one call to copilot-api's budget raise endpoints (copilot#2704) with
 * the user's OBO token. Errors come back as `{ error: <backend code> }` with the
 * backend status, so the page can say what went wrong (not_open, cap_reached,
 * own_budget, profile_required, budget_writes_unavailable, ...).
 */
export async function budgetProxy(path: string, init: RequestInit = {}) {
  const user = await getUser(false);
  const token = await getUserToken();
  if (!user || !token) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }
  try {
    const data = await backendRequest<unknown>(`/api/v1/copilot/budget${path}`, token, { cache: "no-store", ...init });
    return NextResponse.json(data, { status: init.method === "POST" ? 201 : 200 });
  } catch (error) {
    if (error instanceof BackendApiError && error.status >= 400 && error.status < 500) {
      return NextResponse.json({ error: error.code ?? "request_failed" }, { status: error.status });
    }
    if (error instanceof BackendApiError && (error.status === 503 || error.status === 502) && error.code) {
      return NextResponse.json({ error: error.code }, { status: error.status });
    }
    console.error(`[budget] ${init.method ?? "GET"} ${path} failed:`, error);
    return NextResponse.json({ error: "failed" }, { status: 502 });
  }
}
