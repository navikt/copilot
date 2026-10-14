import { NextResponse } from "next/server";
import { getUserToken } from "@/lib/auth";
import { submitAnswers, type Answers, type SubmitResult } from "@/lib/survey";

const reply = (result: SubmitResult, status = 200) => NextResponse.json(result, { status });

/**
 * The survey form posts here, not to a server action: an action id dies with
 * each deploy, so a page loaded before one could not send. Always JSON, so the
 * form can tell this answer from a login redirect or a proxy error page.
 */
export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const token = await getUserToken();
  if (!token) return reply({ status: "error" }, 401);
  const { id } = await params;
  let answers: Answers;
  try {
    answers = await request.json();
  } catch {
    return reply({ status: "invalid" }, 400);
  }
  if (!answers || typeof answers !== "object" || Array.isArray(answers)) return reply({ status: "invalid" }, 400);
  try {
    return reply(await submitAnswers(token, id, answers));
  } catch (err) {
    console.error("survey submit failed", err instanceof Error ? err.message : String(err));
    return reply({ status: "error" });
  }
}
