import type { Answers, SubmitResult } from "@/lib/survey";

/** SubmitResult, plus "expired": the login ran out, so the answers never reached the route. */
export type SendResult = SubmitResult | { status: "expired" };

export async function sendSurvey(surveyId: string, answers: Answers): Promise<SendResult> {
  let res: Response;
  try {
    res = await fetch(`/api/survey/${encodeURIComponent(surveyId)}/responses`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(answers),
      // A login redirect becomes an opaque response here, not the login page's HTML.
      redirect: "manual",
    });
  } catch {
    return { status: "error" };
  }
  if (res.type === "opaqueredirect" || res.status === 401) return { status: "expired" };
  if (!res.headers.get("content-type")?.includes("application/json")) return { status: "error" };
  try {
    const body = await res.json();
    return KNOWN.includes(body?.status) ? (body as SubmitResult) : { status: "error" };
  } catch {
    return { status: "error" };
  }
}

const KNOWN: unknown[] = ["recorded", "duplicate", "invalid", "no-identity", "closed", "error"];
