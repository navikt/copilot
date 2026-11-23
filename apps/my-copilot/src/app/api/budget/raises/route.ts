import { NextResponse } from "next/server";
import { budgetProxy } from "../proxy";

export async function POST(request: Request) {
  const key = request.headers.get("Idempotency-Key");
  if (!key) return NextResponse.json({ error: "invalid_request" }, { status: 400 });
  const body = (await request.json().catch(() => ({}))) as Record<string, unknown>;
  // The amount is never sent: the server decides it (F4).
  const raise = {
    category: body.category,
    teamId: body.teamId || undefined,
    expectedValue: body.expectedValue,
    freeText: typeof body.freeText === "string" && body.freeText.trim() ? body.freeText : undefined,
  };
  return budgetProxy("/raises", { method: "POST", headers: { "Idempotency-Key": key }, body: JSON.stringify(raise) });
}
