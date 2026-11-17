import { NextResponse } from "next/server";
import { getUser, getUserToken, isAdmin } from "@/lib/auth";
import { fetchSurveyCompareExport } from "@/lib/survey";

/** Streams copilot-survey's comparison CSV for the button on /admin/undersokelser/sammenlign. */
export async function GET(request: Request) {
  if (!isAdmin(await getUser(false))) return NextResponse.json({ error: "Not found" }, { status: 404 });
  const userToken = await getUserToken();
  if (!userToken) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  const ids = new URL(request.url).searchParams.get("ids")?.split(",") ?? [];
  const res = await fetchSurveyCompareExport(userToken, ids);
  if (!res) return NextResponse.json({ error: "failed" }, { status: 400 });
  if (res.status === 403 || res.status === 404) return NextResponse.json({ error: "Not found" }, { status: 404 });
  if (!res.ok) return NextResponse.json({ error: "failed" }, { status: 502 });
  const headers = new Headers({ "Cache-Control": "no-store" });
  for (const k of ["Content-Type", "Content-Disposition"]) {
    const v = res.headers.get(k);
    if (v) headers.set(k, v);
  }
  return new NextResponse(res.body, { headers });
}
