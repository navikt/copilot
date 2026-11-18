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
import PublishedSeriesPage from "./serie/[series]/page";
import type { SurveyComparison, SurveyResults } from "@/lib/survey";
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

  it("links to the series when two published waves share one", async () => {
    const wave = { ...results, questions: undefined, series: "utviklerundersokelsen" };
    respond(200, { surveys: [{ ...wave, id: "var-2026" }, wave] });
    render(await PublishedSurveysPage());
    expect(screen.getByRole("link", { name: "Se utviklingen over tid" })).toHaveAttribute(
      "href",
      "/innsikt/undersokelser/serie/utviklerundersokelsen"
    );
  });

  it("has no series link for a single wave", async () => {
    respond(200, { surveys: [{ ...results, questions: undefined, series: "utviklerundersokelsen" }] });
    render(await PublishedSurveysPage());
    expect(screen.queryByRole("link", { name: /over tid/ })).not.toBeInTheDocument();
  });

  it("shows the series over time with full distribution and n, without channels", async () => {
    const wave = { status: "stengt", suppressed: false, channels: null } as const;
    const comparison: SurveyComparison = {
      surveys: [
        {
          ...wave,
          id: "var-2026",
          title: "Utviklerundersøkelsen våren 2026",
          starts: "2026-03-24",
          ends: "2026-04-30",
          responses: 165,
        },
        {
          ...wave,
          id: "host-2026",
          title: "Utviklerundersøkelsen høsten 2026",
          starts: "2026-10-13",
          ends: "2026-10-27",
          responses: 20,
        },
      ],
      questions: [
        {
          id: "pastander",
          type: "matrix",
          text: "Hvor enig er du?",
          min: 1,
          max: 5,
          labels: ["Helt uenig", "Uenig", "Nøytral", "Enig", "Helt enig"],
          items: [
            {
              id: "review",
              text: "Review går fortere",
              break: true,
              waves: [1, 2].map((version, i) => ({
                versions: [
                  {
                    version,
                    n: 10 * (i + 1),
                    options: [1, 2, 3, 4, 5].map((v) => ({ value: `${v}`, count: 2 * (i + 1), suppressed: false })),
                    mean: 3,
                    agree: 0.4,
                  },
                ],
              })),
            },
            {
              id: "fart",
              text: "Jeg jobber raskere",
              break: false,
              waves: [
                { versions: [{ version: 1, n: null, options: null, hidden: true }] },
                { versions: [{ version: 1, n: 20, options: [], mean: 3, agree: 0.4 }] },
              ],
            },
          ],
        },
      ],
    };
    const fetch = respond(200, comparison);
    render(await PublishedSeriesPage({ params: Promise.resolve({ series: "utviklerundersokelsen" }) }));
    expect(fetch).toHaveBeenCalledWith(
      "http://copilot-survey/api/v1/surveys/published/compare?series=utviklerundersokelsen",
      expect.anything()
    );
    expect(screen.getByText("Spørsmålet er endret: brudd i serien")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Våren 2026, versjon 1 (n 10)" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Høsten 2026, versjon 2 (n 20)" })).toBeInTheDocument();
    expect(screen.getAllByRole("cell", { name: "4 (20 %)" })).toHaveLength(5);
    expect(screen.queryByRole("columnheader", { name: "Besvart via" })).not.toBeInTheDocument();
    expect(screen.getAllByText("Færre enn 5 svarte på denne versjonen, så tallene vises ikke.")).toHaveLength(1);
    expect(screen.queryByText(/Holdt utenfor/)).not.toBeInTheDocument();
    expect(screen.getByText(/Gruppene er derfor ikke helt like/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Utviklerundersøkelsen våren 2026" })).toHaveAttribute(
      "href",
      "/innsikt/undersokelser/var-2026"
    );
    expect(screen.getByText("Last ned sammenligning (CSV)").closest("a")).toHaveAttribute(
      "href",
      "/api/survey/compare/export?series=utviklerundersokelsen"
    );
    expect(screen.queryByRole("heading", { name: "Fritekst, kodet" })).not.toBeInTheDocument();
  });

  it("shows coded free text as counts per category, «<5» for small groups", async () => {
    const wave = { status: "stengt", suppressed: false, channels: null, starts: "", ends: "", responses: 20 } as const;
    const category = { kappa: 0.8, status: "published" as const };
    respond(200, {
      surveys: [
        { ...wave, id: "var-2026", title: "Utviklerundersøkelsen våren 2026" },
        { ...wave, id: "host-2026", title: "Utviklerundersøkelsen høsten 2026" },
      ],
      questions: [],
      freetext: [
        {
          survey_id: "host-2026",
          codebook_version: 1,
          coders: "modell og en fra verktøyteamet",
          fields: [
            {
              id: "opplevelse",
              question: "Hva er din mest minneverdige opplevelse?",
              written: 12,
              coded: 12,
              published: [
                { ...category, id: "raskere", name: "Raskere enn for hånd", count: 6, onske: 0, share: 0.5 },
                { ...category, id: "friksjon", name: "Friksjon", count: null, onske: null, share: null, small: true },
              ],
            },
          ],
        },
      ],
    } satisfies SurveyComparison);
    render(await PublishedSeriesPage({ params: Promise.resolve({ series: "utviklerundersokelsen" }) }));
    expect(screen.getByRole("heading", { name: "Fritekst, kodet" })).toBeInTheDocument();
    expect(screen.getByText(/modell og en fra verktøyteamet/)).toBeInTheDocument();
    expect(screen.getByRole("row", { name: /Raskere enn for hånd 6 50\s% 0 0,80/ })).toBeInTheDocument();
    expect(screen.getByRole("row", { name: /Friksjon <5 <5 <5 0,80/ })).toBeInTheDocument();
  });

  it("maps an unknown series or a bad slug to notFound", async () => {
    const fetch = respond(404);
    const page = (series: string) => PublishedSeriesPage({ params: Promise.resolve({ series }) });
    await expect(page("finnes-ikke")).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(page("../x")).rejects.toThrow("NEXT_NOT_FOUND");
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("says a hidden small version is under 5, without numbers", async () => {
    const [nytte, ...rest] = results.questions!;
    const hidden = { version: 3, n: null, options: null, hidden: true };
    respond(200, { ...results, questions: [{ ...nytte, versions: [...nytte.versions!, hidden] }, ...rest] });
    render(await PublishedSurveyResultsPage(params("host-2026")));
    expect(screen.getByText("Versjon 3: under 5 svar. Tallene vises ikke.")).toBeInTheDocument();
  });

  it("maps 404 or invalid id to notFound", async () => {
    respond(404);
    await expect(PublishedSurveyResultsPage(params("finnes-ikke"))).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(PublishedSurveyResultsPage(params("../secret"))).rejects.toThrow("NEXT_NOT_FOUND");
  });
});
