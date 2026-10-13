import { getUser, getUserToken, isAdmin } from "@/lib/auth";
import { loadInnsyn, parseInnsynYear } from "@/lib/innsyn-load";
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
  const year = parseInnsynYear(new URL(request.url).searchParams.get("year"));
  if (year == null) {
    return NextResponse.json({ error: "invalid year" }, { status: 400 });
  }
  try {
    const answer = await loadInnsyn(year, token);
    return new NextResponse(JSON.stringify(answer, null, 2) + "\n", {
      headers: {
        "Content-Type": "application/json; charset=utf-8",
        "Content-Disposition": `attachment; filename="innsynssvar-copilot-${year}.json"`,
      },
    });
  } catch (error) {
    console.error("[admin-innsyn] Failed to build innsyn answer:", error);
    return NextResponse.json({ error: "failed_to_fetch_usage" }, { status: 500 });
  }
}
