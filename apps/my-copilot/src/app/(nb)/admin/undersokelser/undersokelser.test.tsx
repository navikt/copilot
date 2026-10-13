import { render, screen } from "@testing-library/react";
import { vi } from "vitest";

vi.mock("next/headers", () => ({ headers: vi.fn() }));
vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
  redirect: vi.fn(),
  usePathname: () => "/admin/undersokelser",
}));

import SurveysPage from "./page";
import SurveyResultsPage from "./[id]/page";
import type { SurveyResults } from "@/lib/survey";
import fixture from "./results-fixture.json";
import { likert } from "./[id]/likert";

const group = "11111111-2222-3333-4444-555555555555";
const params = (id: string) => ({ params: Promise.resolve({ id }) });

// Shaped like aggregate() in copilot-survey (#1733) from 142 made-up answers. Since
// 2026-10-13 every count is real; only a survey under five answers is hidden.
const results = fixture as unknown as SurveyResults;

// The definition getSurveyPreview reads from main: labels for the matrix.
const definition = {
  id: "host-2026",
  title: results.title,
  ends: results.ends,
  questions: [
    { id: "nytte", version: 2, type: "choice", text: "Hvor nyttig er Copilot i arbeidet ditt?", options: [] },
    { id: "verktoy", version: 1, type: "multi", text: "Hvilke verktøy bruker du?", options: [] },
    {
      id: "pastander",
      version: 1,
      type: "matrix",
      text: "Hvor enig er du i påstandene?",
      min: 1,
      max: 5,
      labels: ["Helt uenig", "Uenig", "Nøytral", "Enig", "Helt enig"],
      items: [
        { id: "raskere", version: 1, text: "Jeg jobber raskere" },
        { id: "kvalitet", version: 1, text: "Koden blir bedre" },
      ],
    },
  ],
};

function respond(status: number, body: unknown = {}) {
  const fetch = vi.fn(async (url: string) =>
    url.startsWith("https://raw.githubusercontent.com/")
      ? new Response(JSON.stringify(definition))
      : new Response(JSON.stringify(body), { status })
  );
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
    expect(screen.getByText("142")).toBeInTheDocument();
  });

  it("shows real counts, shares and scale labels, small ones too", async () => {
    const fetch = respond(200, results);
    render(await SurveyResultsPage(params("host-2026")));
    expect(fetch).toHaveBeenCalledWith(
      "http://copilot-survey/api/v1/admin/surveys/host-2026/results",
      expect.anything()
    );
    expect(screen.getByRole("heading", { name: "Hvor nyttig er Copilot i arbeidet ditt?" })).toBeInTheDocument();
    expect(screen.getAllByRole("row", { name: /Svært nyttig/ })[0]).toHaveTextContent("61");
    expect(screen.getAllByRole("row", { name: /Svært nyttig/ })[0]).toHaveTextContent("51 %");
    expect(screen.getByRole("row", { name: /Ikke nyttig/ })).toHaveTextContent("4");
    expect(screen.getByText("Versjon 2: 3 svar")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Jeg jobber raskere" })).toBeInTheDocument();
    expect(screen.getAllByRole("row", { name: /Helt uenig/ })[0]).toHaveTextContent("4");
    expect(screen.getAllByRole("row", { name: /Helt enig/ })[0]).toHaveTextContent("42");
    expect(screen.queryByText(/skjult|tallene vises ikke|færre enn fem/i)).not.toBeInTheDocument();
  });

  it("draws matrix statements as diverging bars with a mean for each", async () => {
    respond(200, results);
    render(await SurveyResultsPage(params("host-2026")));
    // Jeg jobber raskere: n 130, Helt uenig 4, Uenig 9, Nøytral 24 split over the zero line, Enig 51, Helt enig 42.
    expect(screen.getAllByTestId("segment-1")[0]).toHaveStyle({ width: `${(4 / 130) * 100}%` });
    expect(screen.getAllByTestId("segment-3")[0]).toHaveStyle({ width: `${(24 / 130 / 2) * 100}%` });
    expect(screen.getAllByTestId("segment-4")[0]).toHaveTextContent("39 % (51)");
    // Koden blir bedre (n 4) gets its bar too.
    expect(screen.getAllByTestId("segment-4")).toHaveLength(2);
    expect(screen.getAllByText(/^Snitt/).map((e) => e.textContent)).toEqual(["Snitt 3,9", "Snitt 3,8"]);
  });

  it("gives the mean of a Likert version", () => {
    const v = {
      version: 1,
      n: 10,
      options: [1, 2, 3, 4, 5].map((x) => ({ value: `${x}`, count: 2, suppressed: false })),
    };
    expect(likert(v, 1, 5)).toMatchObject({ mean: 3 });
  });

  it("shows raw values when the definition is missing", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) =>
        url.startsWith("https://raw.githubusercontent.com/")
          ? new Response("", { status: 404 })
          : new Response(JSON.stringify(results))
      )
    );
    render(await SurveyResultsPage(params("host-2026")));
    expect(screen.getByRole("cell", { name: "5" })).toBeInTheDocument();
  });

  it("shows an info alert when the whole survey has under five answers", async () => {
    respond(200, { ...results, responses: 0, suppressed: true, questions: null });
    render(await SurveyResultsPage(params("host-2026")));
    expect(screen.getByText("For få svar til å vise resultater (under 5).")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});
