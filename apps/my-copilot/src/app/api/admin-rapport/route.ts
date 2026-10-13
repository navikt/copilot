import { getUser, getUserToken, isAdmin } from "@/lib/auth";
import { loadRapport, parseRapportYear } from "@/lib/rapport-load";
import { NextResponse } from "next/server";

export async function GET(request: Request) {
  // Like the page: everyone but admins gets 404.
  if (!isAdmin(await getUser(false))) {
    return NextResponse.json({ error: "Not found" }, { status: 404 });
  }
  const token = await getUserToken();
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  const year = parseRapportYear(new URL(request.url).searchParams.get("year"));
  if (year == null) {
    return NextResponse.json({ error: "invalid year" }, { status: 400 });
  }
  try {
    const report = await loadRapport(year, token);
    return new NextResponse(JSON.stringify(report, null, 2) + "\n", {
      headers: {
        "Content-Type": "application/json; charset=utf-8",
        "Content-Disposition": `attachment; filename="copilot-rapport-${year}.json"`,
      },
    });
  } catch (error) {
    console.error("[admin-rapport] Failed to build report:", error);
    return NextResponse.json({ error: "failed_to_fetch_usage" }, { status: 500 });
  }
}
