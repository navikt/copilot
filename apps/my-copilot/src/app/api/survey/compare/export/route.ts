import { NextResponse } from "next/server";
import { getUser, getUserToken } from "@/lib/auth";
import { fetchPublishedSeriesExport } from "@/lib/survey";

/** Streams copilot-survey's published comparison CSV for the button on /innsikt/undersokelser/serie/[series]. */
export async function GET(request: Request) {
  if (!(await getUser(false))) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  const userToken = await getUserToken();
  if (!userToken) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  const series = new URL(request.url).searchParams.get("series") ?? "";
  if (!/^[a-z0-9-]{1,64}$/.test(series)) return NextResponse.json({ error: "series" }, { status: 400 });
  const res = await fetchPublishedSeriesExport(userToken, series);
  if (!res) return NextResponse.json({ error: "failed" }, { status: 502 });
  if (res.status === 403 || res.status === 404) return NextResponse.json({ error: "Not found" }, { status: 404 });
  if (!res.ok) return NextResponse.json({ error: "failed" }, { status: 502 });
  const headers = new Headers({ "Cache-Control": "no-store" });
  for (const k of ["Content-Type", "Content-Disposition"]) {
    const v = res.headers.get(k);
    if (v) headers.set(k, v);
  }
  return new NextResponse(res.body, { headers });
}
