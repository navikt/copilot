import { NextResponse } from "next/server";
import { getCopilotUsage } from "@/lib/github";

export async function GET() {
  const { usage, error } = await getCopilotUsage("navikt");

  if (error) {
    return NextResponse.json({ error: `Failed to fetch usage data: ${error}` }, { status: 500 });
  }

  if (!usage) {
    return NextResponse.json({ error: "No usage data available" }, { status: 404 });
  }

  return NextResponse.json(usage, {
    headers: {
      "Content-Type": "application/json",
    },
  });
}
