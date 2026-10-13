import { render, screen } from "@testing-library/react";
import { vi } from "vitest";

vi.mock("next/headers", () => ({ headers: vi.fn() }));
vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
  redirect: vi.fn(),
}));

import SurveysPage from "./page";
import SurveyResultsPage from "./[id]/page";

const group = "11111111-2222-3333-4444-555555555555";
const params = (id: string) => ({ params: Promise.resolve({ id }) });

// The shape copilot-survey's results.go writes.
const results = {
  id: "host-2026",
  title: "Utviklerundersøkelsen høsten 2026",
  starts: "2026-10-13",
  ends: "2026-10-27",
  status: "åpen",
  responses: 42,
  suppressed: false,
  questions: [
    {
      id: "nytte",
      type: "choice",
      text: "Hvor nyttig er Copilot?",
      versions: [
        {
          version: 1,
          n: 40,
          options: [
            { value: "Svært nyttig", count: 30, suppressed: false },
            { value: "Lite nyttig", count: null, suppressed: true },
          ],
        },
        { version: 2, n: 2, options: null },
      ],
    },
    {
      id: "pastander",
      type: "matrix",
      text: "Hvor enig er du?",
      items: [{ id: "raskere", text: "Jeg jobber raskere", versions: [{ version: 1, n: 3, options: null }] }],
    },
  ],
};

function respond(status: number, body: unknown = {}) {
  const fetch = vi.fn(async () => new Response(JSON.stringify(body), { status }));
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

describe("/admin/undersokelser", () => {
  beforeEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "");
    vi.stubEnv("COPILOT_ADMIN_GROUP_ID", group);
    vi.stubEnv("DEV_USER_GROUPS", `other,${group}`);
    vi.stubEnv("COPILOT_SURVEY_URL", "http://copilot-survey");
  });

  it("gives a non-admin 404 without calling copilot-survey", async () => {
    vi.stubEnv("DEV_USER_GROUPS", "other");
    const fetch = respond(200, { surveys: [] });
    await expect(SurveysPage()).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(SurveyResultsPage(params("host-2026"))).rejects.toThrow("NEXT_NOT_FOUND");
    expect(fetch).not.toHaveBeenCalled();
  });

  it("maps 403 and 404 from copilot-survey, and a bad id, to 404", async () => {
    respond(403);
    await expect(SurveysPage()).rejects.toThrow("NEXT_NOT_FOUND");
    respond(404);
    await expect(SurveyResultsPage(params("finnes-ikke"))).rejects.toThrow("NEXT_NOT_FOUND");
    const fetch = respond(200, results);
    await expect(SurveyResultsPage(params("../hemmelig"))).rejects.toThrow("NEXT_NOT_FOUND");
    expect(fetch).not.toHaveBeenCalled();
  });

  it("lists the surveys with a link to each", async () => {
    respond(200, { surveys: [{ ...results, questions: undefined }] });
    render(await SurveysPage());
    expect(screen.getByRole("link", { name: results.title })).toHaveAttribute("href", "/admin/undersokelser/host-2026");
    expect(screen.getByText("åpen")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("shows counts, shares and «under 5» for small groups", async () => {
    const fetch = respond(200, results);
    render(await SurveyResultsPage(params("host-2026")));
    expect(fetch).toHaveBeenCalledWith(
      "http://copilot-survey/api/v1/admin/surveys/host-2026/results",
      expect.anything()
    );
    expect(screen.getByRole("heading", { name: "Hvor nyttig er Copilot?" })).toBeInTheDocument();
    expect(screen.getByText("30")).toBeInTheDocument();
    expect(screen.getByText("75 %")).toBeInTheDocument();
    expect(screen.getByRole("row", { name: /Lite nyttig/ })).toHaveTextContent("under 5");
    expect(screen.getByText("Versjon 2: under 5 svar, tallene vises ikke.")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Jeg jobber raskere" })).toBeInTheDocument();
    expect(screen.getByText("Alle: under 5 svar, tallene vises ikke.")).toBeInTheDocument();
  });

  it("shows an info alert when the whole survey has under five answers", async () => {
    respond(200, { ...results, responses: 0, suppressed: true, questions: null });
    render(await SurveyResultsPage(params("host-2026")));
    expect(screen.getByText("For få svar til å vise resultater (under 5).")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});
