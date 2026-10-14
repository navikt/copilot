import { NextResponse } from "next/server";
import { getUser, getUserToken, isAdmin } from "@/lib/auth";
import { fetchSurveyExport } from "@/lib/survey";

/**
 * Streams copilot-survey's CSV of a survey's answers for the button on
 * /admin/undersokelser/[id]. A route handler, not a server action (#1804).
 * copilot-survey checks the admin group again.
 */
export async function GET(_request: Request, { params }: { params: Promise<{ id: string }> }) {
  // Like the page: everyone but admins gets 404.
  if (!isAdmin(await getUser(false))) return NextResponse.json({ error: "Not found" }, { status: 404 });
  const userToken = await getUserToken();
  if (!userToken) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  const res = await fetchSurveyExport(userToken, (await params).id);
  if (!res) return NextResponse.json({ error: "failed" }, { status: 502 });
  if (res.status === 403 || res.status === 404) return NextResponse.json({ error: "Not found" }, { status: 404 });
  if (!res.ok) {
    // 410: rows deleted after aggregation; copilot-survey's message says so.
    const body = await res.text();
    return new NextResponse(body, {
      status: res.status === 410 ? 410 : 502,
      headers: { "Content-Type": "text/plain; charset=utf-8" },
    });
  }
  const headers = new Headers({ "Cache-Control": "no-store" });
  for (const k of ["Content-Type", "Content-Disposition"]) {
    const v = res.headers.get(k);
    if (v) headers.set(k, v);
  }
  return new NextResponse(res.body, { headers });
}
