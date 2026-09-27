/**
 * User surveys, served and stored by copilot-cli (apps/copilot-cli). The
 * definitions are the same files nav-pilot's terminal survey renders
 * (apps/copilot-cli/surveys/). Answers go through copilot-cli with an OBO
 * token; copilot-cli refuses a second answer from the same person, from the
 * web or nav-pilot, without storing anything that links the answer to them.
 */
import { exchangeTokenFor, fetchWithTimeout, isLocalDev } from "@/lib/backend-api";

const COPILOT_CLI_URL = process.env.COPILOT_CLI_URL || "http://copilot-cli";

export type SurveyQuestion = {
  id: string;
  type: "scale" | "choice" | "multi" | "text";
  text: string;
  required?: boolean;
  min?: number;
  max?: number;
  labels?: string[];
  options?: string[];
  max_choices?: number;
  max_length?: number;
  skip_if?: { question: string; answer: string };
};

export type Survey = {
  id: string;
  title: string;
  intro?: string;
  ends: string;
  questions: SurveyQuestion[];
};

export type Answers = Record<string, number | string | string[]>;

export type SubmitResult =
  | { status: "recorded" }
  | { status: "duplicate" }
  | { status: "invalid"; message: string }
  | { status: "no-identity" }
  | { status: "closed" }
  | { status: "error" };

const KNOWN_TYPES = ["scale", "choice", "multi", "text"];

/** The open surveys this page can show; [] when copilot-cli has none or is out of reach. */
export async function getActiveSurveys(): Promise<Survey[]> {
  try {
    const res = await fetchWithTimeout(
      `${COPILOT_CLI_URL}/api/v1/surveys/active`,
      { cache: "no-store" },
      5000,
      "timeout"
    );
    if (!res.ok) return [];
    const body = (await res.json()) as { surveys?: Survey[] };
    return (body.surveys ?? []).filter((s) => s.questions.every((q) => KNOWN_TYPES.includes(q.type)));
  } catch {
    return [];
  }
}

/** Whether question q is skipped by an earlier answer (skip_if). */
export function isSkipped(q: SurveyQuestion, answers: Answers): boolean {
  if (!q.skip_if) return false;
  const a = answers[q.skip_if.question];
  return Array.isArray(a) ? a.includes(q.skip_if.answer) : a === q.skip_if.answer;
}

export async function submitAnswers(userToken: string, surveyId: string, answers: Answers): Promise<SubmitResult> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (!isLocalDev) {
    headers.Authorization = `Bearer ${await exchangeTokenFor(userToken, "copilot-cli")}`;
  }
  const res = await fetchWithTimeout(
    `${COPILOT_CLI_URL}/api/v1/surveys/${encodeURIComponent(surveyId)}/responses`,
    {
      method: "POST",
      headers,
      // The web sends no setup details: version and OS mean nothing here.
      body: JSON.stringify({ answers, context: { version: "web", os: "other", client: "web", local_models: false } }),
    },
    15000,
    "timeout"
  );
  switch (res.status) {
    case 201:
      return { status: "recorded" };
    case 409:
      return { status: "duplicate" };
    case 400: {
      // The form checks what copilot-cli checks, so this should not happen.
      // Log the reason; the page shows a plain message.
      const body = (await res.json().catch(() => ({}))) as { error?: string };
      console.error(`survey ${surveyId}: answers refused: ${body.error ?? ""}`);
      return { status: "invalid", message: body.error ?? "" };
    }
    case 404:
      return { status: "closed" };
    case 403:
      return { status: "no-identity" };
    default:
      return { status: "error" };
  }
}
