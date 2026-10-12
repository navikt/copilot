import { getUser, getUserToken, isAdmin } from "@/lib/auth";
import { getAdminMonthlyOverview } from "@/lib/cached-bigquery";
import { adminMonthlyCsv } from "@/lib/admin-monthly";
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
  const year = Number(new URL(request.url).searchParams.get("year") || new Date().getUTCFullYear());
  if (!Number.isInteger(year) || year < 2026 || year > new Date().getUTCFullYear()) {
    return NextResponse.json({ error: "year required" }, { status: 400 });
  }
  try {
    const data = await getAdminMonthlyOverview(year, token);
    // The byte order mark makes Excel read æøå as UTF-8.
    return new NextResponse("﻿" + adminMonthlyCsv(data), {
      headers: {
        "Content-Type": "text/csv; charset=utf-8",
        "Content-Disposition": `attachment; filename="copilot-nav-${year}.csv"`,
      },
    });
  } catch (error) {
    console.error("[admin-monthly] Failed to fetch admin monthly usage:", error);
    return NextResponse.json({ error: "failed_to_fetch_usage" }, { status: 500 });
  }
}
