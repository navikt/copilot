import { getUserToken } from "@/lib/auth";
import { BackendApiError } from "@/lib/backend-api";
import { getTeamYearOverview } from "@/lib/cached-bigquery";
import { teamYearCsv } from "@/lib/team-year";
import { NextResponse } from "next/server";

export async function GET(request: Request) {
  const token = await getUserToken();
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const { searchParams } = new URL(request.url);
  const team = searchParams.get("team") ?? "";
  const year = Number(searchParams.get("year") ?? new Date().getUTCFullYear());
  if (!/^[1-9][0-9]{0,19}$/.test(team) || !Number.isInteger(year)) {
    return NextResponse.json({ error: "team and year required" }, { status: 400 });
  }

  try {
    const data = await getTeamYearOverview(team, year, token);
    // The byte order mark makes Excel read æøå as UTF-8.
    return new NextResponse("﻿" + teamYearCsv(data), {
      headers: {
        "Content-Type": "text/csv; charset=utf-8",
        "Content-Disposition": `attachment; filename="copilot-${data.team_slug || team}-${year}.csv"`,
      },
    });
  } catch (error) {
    if (error instanceof BackendApiError && error.status === 400) {
      return NextResponse.json({ error: "invalid_parameter" }, { status: 400 });
    }
    console.error("[team-year] Failed to fetch team year usage:", error);
    return NextResponse.json({ error: "failed_to_fetch_usage" }, { status: 500 });
  }
}
