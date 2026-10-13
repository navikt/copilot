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
  /** Bumped on any change to text, options or scale; results are split by it. */
  version?: number;
  type: "scale" | "choice" | "multi" | "text" | "matrix";
  text: string;
  required?: boolean;
  min?: number;
  max?: number;
  labels?: string[];
  options?: string[];
  max_choices?: number;
  exclusive?: string;
  /** One more option, after options, that takes a short free text sent as `<id>.other` (otherKey). */
  other?: string;
  max_length?: number;
  skip_if?: { question: string; answer: string };
  /** matrix: statements on its scale, each answered and sent as a scale answer under its own id. */
  items?: { id: string; version?: number; text: string }[];
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
// ponytail: copilot-survey serves only open surveys, so the page reads the
// window around them from here. Update it when the next survey is scheduled;
// a schedule endpoint in copilot-survey would replace it.
export const SURVEY_SCHEDULE = { starts: "2026-10-13", ends: "2026-10-27" };

/** Where today (UTC, as copilot-survey counts) falls against a window: before it, after it, or neither. */
export function surveyWindow(
  today: string = new Date().toISOString().slice(0, 10),
  schedule: { starts: string; ends: string } = SURVEY_SCHEDULE
): "before" | "after" | undefined {
  if (today < schedule.starts) return "before";
  if (today > schedule.ends) return "after";
  return undefined;
}

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

/**
 * A survey definition as committed on main, for a preview before it opens
 * (?forhandsvis=<id>, admins only). A preview sends no answers.
 */
export async function getSurveyPreview(id: string): Promise<Survey | undefined> {
  if (!/^[a-z0-9-]{1,64}$/.test(id)) return undefined;
  try {
    const res = await fetchWithTimeout(
      `https://raw.githubusercontent.com/navikt/copilot/main/apps/copilot-survey/surveys/${id}.json`,
      { cache: "no-store" },
      5000,
      "timeout"
    );
    if (!res.ok) return undefined;
    const s = (await res.json()) as Survey;
    return s.questions?.every((q) => KNOWN_TYPES.includes(q.type)) ? s : undefined;
  } catch {
    return undefined;
  }
}

/**
 * The titles of the open surveys, for SurveyNotice. Pages pass the promise
 * without awaiting it, so a slow copilot-survey never holds them up, and only
 * the titles reach the client.
 */
export const openSurveyTitles = (): Promise<string[]> =>
  getActiveSurveys().then((a) => (a.status === "ok" ? a.surveys.map((s) => s.title) : []));

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

/** A multi answer after a change: the exclusive option (like "I use none")
 * stands alone, so picking it drops the rest and picking another drops it. */
export function exclusivePick(q: SurveyQuestion, prev: string[], next: string[]): string[] {
  const added = next.find((v) => !prev.includes(v));
  if (!q.exclusive || added === undefined) return next;
  return added === q.exclusive ? [added] : next.filter((v) => v !== q.exclusive);
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

/**
 * Whether the user has answered the survey, by the same check that refuses a
 * second answer. undefined on anything but a 200: the page then shows the form,
 * and copilot-survey still refuses a duplicate with 409.
 */
export async function getSurveyStatus(userToken: string, surveyId: string): Promise<boolean | undefined> {
  const url = process.env.COPILOT_SURVEY_URL;
  if (!url) return undefined;
  try {
    const headers: Record<string, string> = {};
    if (!isLocalDev) headers.Authorization = `Bearer ${await exchangeTokenFor(userToken, "copilot-survey")}`;
    const res = await fetchWithTimeout(
      `${url}/api/v1/surveys/${encodeURIComponent(surveyId)}/status`,
      { headers, cache: "no-store" },
      5000,
      "timeout"
    );
    if (res.status !== 200) return undefined;
    const body = (await res.json()) as { answered?: unknown };
    return typeof body.answered === "boolean" ? body.answered : undefined;
  } catch {
    return undefined;
  }
}

/** A survey as copilot-survey's admin API lists it (GET /api/v1/admin/surveys). */
export type SurveySummary = {
  id: string;
  title: string;
  starts: string;
  ends: string;
  status: "kommende" | "åpen" | "stengt";
  responses: number;
};

/** One version of a question. n and options are null when under five answered it; an option's count is null when hidden. */
export type VersionResult = {
  version: number;
  n: number | null;
  options: { value: string; count: number | null; suppressed: boolean }[] | null;
};

export type QuestionResult = {
  id: string;
  type: SurveyQuestion["type"];
  text: string;
  versions?: VersionResult[];
  items?: { id: string; text: string; versions: VersionResult[] }[];
};

/** questions is null when the whole survey has fewer than five answers (suppressed). */
export type SurveyResults = SurveySummary & { suppressed: boolean; questions: QuestionResult[] | null };

export type AdminResult<T> = { status: "ok"; data: T } | { status: "not-found" } | { status: "error" };

/** GET an admin endpoint of copilot-survey for the user. 403 and 404 both mean not-found, so a non-admin learns nothing. */
async function adminGet<T>(userToken: string, path: string): Promise<AdminResult<T>> {
  const url = process.env.COPILOT_SURVEY_URL;
  if (!url) return { status: "error" };
  try {
    const headers: Record<string, string> = {};
    if (!isLocalDev) headers.Authorization = `Bearer ${await exchangeTokenFor(userToken, "copilot-survey")}`;
    const res = await fetchWithTimeout(`${url}${path}`, { headers, cache: "no-store" }, 10000, "timeout");
    if (res.status === 403 || res.status === 404) return { status: "not-found" };
    if (!res.ok) return { status: "error" };
    return { status: "ok", data: (await res.json()) as T };
  } catch {
    return { status: "error" };
  }
}

export const getAdminSurveys = (userToken: string) =>
  adminGet<{ surveys: SurveySummary[] }>(userToken, "/api/v1/admin/surveys");

export async function getSurveyResults(userToken: string, id: string): Promise<AdminResult<SurveyResults>> {
  if (!/^[a-z0-9-]{1,64}$/.test(id)) return { status: "not-found" };
  return adminGet<SurveyResults>(userToken, `/api/v1/admin/surveys/${id}/results`);
}
