/**
 * User surveys, served and stored by copilot-survey (apps/copilot-survey). The
 * definitions are the same files nav-pilot's terminal survey renders
 * (apps/copilot-survey/surveys/). Answers go straight to copilot-survey with an
 * OBO token; copilot-survey refuses a second answer from the same person, from
 * the web or nav-pilot, without storing anything that links the answer to them.
 */
import { exchangeTokenFor, fetchWithTimeout, isLocalDev } from "@/lib/backend-api";

export type SurveyQuestion = {
  id: string;
  type: "scale" | "choice" | "multi" | "text" | "matrix";
  text: string;
  required?: boolean;
  min?: number;
  max?: number;
  labels?: string[];
  options?: string[];
  max_choices?: number;
  /** One more option, after options, that takes a short free text sent as `<id>.other` (otherKey). */
  other?: string;
  max_length?: number;
  skip_if?: { question: string; answer: string };
  /** matrix: statements on its scale, each answered and sent as a scale answer under its own id. */
  items?: { id: string; text: string }[];
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
  | { status: "invalid" }
  | { status: "no-identity" }
  | { status: "closed" }
  | { status: "error" };

const KNOWN_TYPES = ["scale", "choice", "multi", "text", "matrix"];

export type ActiveSurveys = { status: "ok"; surveys: Survey[] } | { status: "error" };

/**
 * The open surveys this page can show. No COPILOT_SURVEY_URL (a cluster without
 * copilot-survey) means no open survey; an unreachable or failing copilot-survey
 * is an error, so the page does not claim the survey has closed.
 */
export async function getActiveSurveys(): Promise<ActiveSurveys> {
  const url = process.env.COPILOT_SURVEY_URL;
  if (!url) return { status: "ok", surveys: [] };
  try {
    const res = await fetchWithTimeout(`${url}/api/v1/surveys/active`, { cache: "no-store" }, 5000, "timeout");
    if (!res.ok) return { status: "error" };
    const body = (await res.json()) as { surveys?: Survey[] };
    const surveys = (body.surveys ?? []).filter((s) => s.questions.every((q) => KNOWN_TYPES.includes(q.type)));
    return { status: "ok", surveys };
  } catch {
    return { status: "error" };
  }
}

/** The values of a scale or matrix question. copilot-survey leaves out min 0, as the terminal survey reads it. */
export function scaleSteps(q: SurveyQuestion): number[] {
  const steps: number[] = [];
  for (let n = q.min ?? 0; n <= (q.max ?? 5); n++) steps.push(n);
  return steps;
}

/** Where the free text of q's other option is sent. */
export const otherKey = (q: SurveyQuestion) => `${q.id}.other`;

/** The options of a choice or multi question, then its other option. */
export const optionsOf = (q: SurveyQuestion) => [...(q.options ?? []), ...(q.other ? [q.other] : [])];

/** Whether q is answered with its other option. */
export function choseOther(q: SurveyQuestion, answers: Answers): boolean {
  const a = answers[q.id];
  return !!q.other && (Array.isArray(a) ? a.includes(q.other) : a === q.other);
}

/** Whether question q is skipped by an earlier answer (skip_if). */
export function isSkipped(q: SurveyQuestion, answers: Answers): boolean {
  if (!q.skip_if) return false;
  const a = answers[q.skip_if.question];
  return Array.isArray(a) ? a.includes(q.skip_if.answer) : a === q.skip_if.answer;
}

export async function submitAnswers(userToken: string, surveyId: string, answers: Answers): Promise<SubmitResult> {
  const url = process.env.COPILOT_SURVEY_URL;
  if (!url) return { status: "error" };
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (!isLocalDev) {
    headers.Authorization = `Bearer ${await exchangeTokenFor(userToken, "copilot-survey")}`;
  }
  const res = await fetchWithTimeout(
    `${url}/api/v1/surveys/${encodeURIComponent(surveyId)}/responses`,
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
      // The form checks what copilot-survey checks, so this should not happen.
      // Log the reason; the page shows a plain message.
      const body = (await res.json().catch(() => ({}))) as { error?: string };
      console.error("survey answers refused", JSON.stringify({ survey: surveyId, reason: body.error ?? "" }));
      return { status: "invalid" };
    }
    case 404:
      return { status: "closed" };
    case 403:
      return { status: "no-identity" };
    default:
      return { status: "error" };
  }
}
