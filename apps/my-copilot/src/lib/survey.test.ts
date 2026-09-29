import { readFileSync } from "node:fs";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  choseOther,
  getActiveSurveys,
  isSkipped,
  optionsOf,
  scaleSteps,
  submitAnswers,
  type Survey,
  type SurveyQuestion,
} from "./survey";

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

describe("other", () => {
  const q: SurveyQuestion = { id: "tools", type: "multi", text: "?", options: ["a", "b"], other: "Annet" };
  it("comes after the options, and is chosen like one", () => {
    expect(optionsOf(q)).toEqual(["a", "b", "Annet"]);
    expect(optionsOf({ ...q, other: undefined })).toEqual(["a", "b"]);
    expect(choseOther(q, { tools: ["a", "Annet"] })).toBe(true);
    expect(choseOther(q, { tools: ["a"] })).toBe(false);
    expect(choseOther({ ...q, type: "choice" }, { tools: "Annet" })).toBe(true);
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

describe("types follow copilot-survey's schema.json", () => {
  const schema = JSON.parse(
    readFileSync(path.resolve(__dirname, "../../../copilot-survey/surveys/schema.json"), "utf8")
  ) as { properties: object; $defs: { question: { properties: object }; item: { properties: object } } };
  // Not needed to render or send: copilot-survey serves only open surveys, min_cli_version is for nav-pilot, and the rest is for analysis.
  const ignored = ["series", "active", "nudge", "min_cli_version", "starts", "version", "construct", "reverse"];
  const fields = (props: object) =>
    Object.keys(props)
      .filter((k) => !ignored.includes(k))
      .sort();
  // Every field of the types, and only those: satisfies fails the type check otherwise.
  const question = {
    id: "",
    type: "scale",
    text: "",
    required: false,
    min: 0,
    max: 0,
    labels: [],
    options: [],
    max_choices: 0,
    other: "",
    max_length: 0,
    skip_if: { question: "", answer: "" },
    items: [],
  } satisfies Required<SurveyQuestion>;
  const item = { id: "", text: "" } satisfies Required<NonNullable<SurveyQuestion["items"]>[number]>;
  const survey = { id: "", title: "", intro: "", ends: "", questions: [] } satisfies Required<Survey>;

  it("reads every field the web needs, and none the schema lacks", () => {
    expect(Object.keys(question).sort()).toEqual(fields(schema.$defs.question.properties));
    expect(Object.keys(survey).sort()).toEqual(fields(schema.properties));
    expect(Object.keys(item).sort()).toEqual(fields(schema.$defs.item.properties));
  });
});
