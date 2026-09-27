import { afterEach, describe, expect, it, vi } from "vitest";
import { isSkipped, submitAnswers, type SurveyQuestion } from "./survey";

describe("isSkipped", () => {
  const q: SurveyQuestion = { id: "why", type: "text", text: "?", skip_if: { question: "tools", answer: "none" } };
  it("follows a choice and a multi answer", () => {
    expect(isSkipped(q, { tools: "none" })).toBe(true);
    expect(isSkipped(q, { tools: ["a", "none"] })).toBe(true);
    expect(isSkipped(q, { tools: ["a"] })).toBe(false);
    expect(isSkipped({ ...q, skip_if: undefined }, { tools: "none" })).toBe(false);
  });
});

describe("submitAnswers", () => {
  afterEach(() => vi.unstubAllGlobals());
  it.each([
    [201, { status: "recorded" }],
    [409, { status: "duplicate" }],
    [403, { status: "no-identity" }],
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
  it("passes on the reason for a 400", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response('{"error":"no answers"}', { status: 400 })));
    expect(await submitAnswers("token", "q4-2026", {})).toEqual({ status: "invalid", message: "no answers" });
  });
});
