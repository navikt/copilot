import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/headers", () => ({ headers: vi.fn() }));
vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
  redirect: vi.fn(),
  usePathname: () => "/innsikt/undersokelser",
}));

import PublishedSurveysPage from "./page";
import PublishedSurveyResultsPage from "./[id]/page";
import type { SurveyResults } from "@/lib/survey";
import fixture from "../../admin/undersokelser/results-fixture.json";

const params = (id: string) => ({ params: Promise.resolve({ id }) });

const results: SurveyResults = {
  ...(fixture as unknown as SurveyResults),
  status: "stengt",
  published: true,
  channels: { web: 98, copilot: 44 },
};

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

describe("/innsikt/undersokelser", () => {
  beforeEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "");
    vi.stubEnv("DEV_USER_GROUPS", "other");
    vi.stubEnv("COPILOT_SURVEY_URL", "http://copilot-survey");
  });

  it("allows any signed-in Nav user without admin group", async () => {
    const fetch = respond(200, { surveys: [{ ...results, questions: undefined }] });
    render(await PublishedSurveysPage());
    expect(fetch).toHaveBeenCalledWith("http://copilot-survey/api/v1/surveys/published", expect.anything());
    expect(screen.getByRole("link", { name: results.title })).toHaveAttribute(
      "href",
      "/innsikt/undersokelser/host-2026"
    );
    expect(screen.getByText("142")).toBeInTheDocument();
  });

  it("shows an empty state when no survey is published yet", async () => {
    respond(200, { surveys: [] });
    render(await PublishedSurveysPage());
    expect(screen.getByText(/Ingen undersøkelser er publisert ennå/)).toBeInTheDocument();
  });

  it("renders published results with methodology section, channels, and no freetext or CSV links", async () => {
    const fetch = respond(200, results);
    render(await PublishedSurveyResultsPage(params("host-2026")));
    expect(fetch).toHaveBeenCalledWith("http://copilot-survey/api/v1/surveys/host-2026/results", expect.anything());
    expect(screen.getByRole("heading", { name: "Metode og datagrunnlag" })).toBeInTheDocument();
    expect(screen.getByText(/Nettside \(knapperad\): 98, Terminal, nav-pilot \(copilot\): 44/)).toBeInTheDocument();
    expect(screen.getByText(/Deltakelsen var frivillig/)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Hvor nyttig er Copilot i arbeidet ditt?" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /fritekst/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /CSV/i })).not.toBeInTheDocument();
    expect(screen.getByText(/#ki-produktutvikling/)).toBeInTheDocument();
  });

  it("maps 404 or invalid id to notFound", async () => {
    respond(404);
    await expect(PublishedSurveyResultsPage(params("finnes-ikke"))).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(PublishedSurveyResultsPage(params("../secret"))).rejects.toThrow("NEXT_NOT_FOUND");
  });
});
