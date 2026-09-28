import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { getActiveSurveys, isSkipped, scaleSteps, submitAnswers, type SurveyQuestion } from "./survey";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe("isSkipped", () => {
  const q: SurveyQuestion = { id: "why", type: "text", text: "?", skip_if: { question: "tools", answer: "none" } };
  it("follows a choice and a multi answer", () => {
    expect(isSkipped(q, { tools: "none" })).toBe(true);
    expect(isSkipped(q, { tools: ["a", "none"] })).toBe(true);
    expect(isSkipped(q, { tools: ["a"] })).toBe(false);
    expect(isSkipped({ ...q, skip_if: undefined }, { tools: "none" })).toBe(false);
  });
});

describe("scaleSteps", () => {
  it("starts at 0 when copilot-survey leaves out min", () => {
    expect(scaleSteps({ id: "s", type: "scale", text: "?", max: 3 })).toEqual([0, 1, 2, 3]);
    expect(scaleSteps({ id: "s", type: "scale", text: "?", min: 1, max: 3 })).toEqual([1, 2, 3]);
  });
});

describe("getActiveSurveys", () => {
  it("shows no survey where copilot-survey is not configured", async () => {
    vi.stubEnv("COPILOT_SURVEY_URL", "");
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    expect(await getActiveSurveys()).toEqual({ status: "ok", surveys: [] });
    expect(fetch).not.toHaveBeenCalled();
  });
  it("keeps a failing copilot-survey apart from an empty list", async () => {
    vi.stubEnv("COPILOT_SURVEY_URL", "http://copilot-survey");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("{}", { status: 503 })));
    expect(await getActiveSurveys()).toEqual({ status: "error" });
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("down")));
    expect(await getActiveSurveys()).toEqual({ status: "error" });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response('{"surveys":[]}', { status: 200 })));
    expect(await getActiveSurveys()).toEqual({ status: "ok", surveys: [] });
  });
});

describe("submitAnswers", () => {
  beforeEach(() => vi.stubEnv("COPILOT_SURVEY_URL", "http://copilot-survey"));
  it.each([
    [201, { status: "recorded" }],
    [409, { status: "duplicate" }],
    [403, { status: "no-identity" }],
    [404, { status: "closed" }],
    [503, { status: "error" }],
  ])("maps %i", async (code, want) => {
    const fetch = vi.fn().mockResolvedValue(new Response("{}", { status: code }));
    vi.stubGlobal("fetch", fetch);
    expect(await submitAnswers("token", "q4-2026", { a: 1 })).toEqual(want);
    const body = JSON.parse(fetch.mock.calls[0][1].body);
    expect(body).toEqual({
      answers: { a: 1 },
      context: { version: "web", os: "other", client: "web", local_models: false },
    });
  });
  it("maps 400 to invalid", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response('{"error":"no answers"}', { status: 400 })));
    expect(await submitAnswers("token", "q4-2026", {})).toEqual({ status: "invalid" });
  });
});
