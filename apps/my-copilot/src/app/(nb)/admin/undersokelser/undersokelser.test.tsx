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

// Written by aggregate() in copilot-survey (#1733) from 142 made-up answers, so
// the suppression is the backend's own, not one this test assumes.
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

/** Tables where each person gives one answer: with n shown, a single hidden cell would be n minus the rest. */
function derivableTables(r: SurveyResults): string[] {
  const bad: string[] = [];
  for (const q of r.questions ?? []) {
    if (q.type === "multi") continue;
    const tables = [
      ...(q.versions ?? []).map((v) => [q.id, v] as const),
      ...(q.items ?? []).flatMap((i) => i.versions.map((v) => [i.id, v] as const)),
    ];
    for (const [id, v] of tables) {
      if (v.options?.filter((o) => o.count === null).length === 1) bad.push(`${id} v${v.version}`);
    }
  }
  return bad;
}

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

  it("gets a fixture from the backend where no hidden count can be worked out", () => {
    expect(derivableTables(results)).toEqual([]);
    // Control: the check catches a table with one hidden cell.
    const leaky = structuredClone(results);
    leaky.questions![0].versions![0].options = [
      { value: "x", count: null, suppressed: true },
      { value: "y", count: 108, suppressed: false },
      { value: "z", count: 12, suppressed: false },
    ];
    expect(derivableTables(leaky)).toEqual(["nytte v1"]);
    // The chart adds nothing: its only hidden number is n minus the shown counts.
    for (const item of results.questions!.find((q) => q.id === "pastander")!.items!) {
      for (const v of item.versions) {
        if (v.n === null) continue;
        const { segments, hidden } = likert(v, 1, 5);
        const shown = segments.reduce((a, s) => a + (s.count ?? 0), 0);
        expect(hidden?.count).toBe(v.n - shown);
        expect(segments.filter((s) => s.count === null).map((s) => s.share)).toEqual([0, 0]);
      }
    }
  });

  it("shows counts, shares, hidden cells and scale labels", async () => {
    const fetch = respond(200, results);
    render(await SurveyResultsPage(params("host-2026")));
    expect(fetch).toHaveBeenCalledWith(
      "http://copilot-survey/api/v1/admin/surveys/host-2026/results",
      expect.anything()
    );
    expect(screen.getByRole("heading", { name: "Hvor nyttig er Copilot i arbeidet ditt?" })).toBeInTheDocument();
    expect(screen.getByRole("row", { name: /Svært nyttig/ })).toHaveTextContent("61");
    expect(screen.getByRole("row", { name: /Svært nyttig/ })).toHaveTextContent("51 %");
    // Lite nyttig (12) is hidden with Ikke nyttig (4), or 4 would be 120 minus the rest.
    expect(screen.getByRole("row", { name: /Lite nyttig/ })).toHaveTextContent("skjult");
    expect(screen.getByRole("row", { name: /Ikke nyttig/ })).toHaveTextContent("skjult");
    expect(screen.getByText(/^Versjon 2: tallene vises ikke\./)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Jeg jobber raskere" })).toBeInTheDocument();
    expect(screen.getByRole("row", { name: /Helt uenig/ })).toHaveTextContent("skjult");
    expect(screen.getByRole("row", { name: /Helt enig/ })).toHaveTextContent("42");
    expect(screen.queryByRole("cell", { name: "5" })).not.toBeInTheDocument();
    expect(screen.getByText(/^Alle: tallene vises ikke\./)).toBeInTheDocument();
  });

  it("draws matrix statements as diverging bars, hidden cells merged and no mean", async () => {
    respond(200, results);
    render(await SurveyResultsPage(params("host-2026")));
    // Jeg jobber raskere: n 130, Nøytral 24 split over the zero line, Enig 51, Helt enig 42.
    expect(screen.getByTestId("segment-3")).toHaveStyle({ width: `${(24 / 130 / 2) * 100}%` });
    expect(screen.getByTestId("segment-4")).toHaveStyle({ width: `${(51 / 130) * 100}%` });
    expect(screen.getByTestId("segment-4")).toHaveTextContent("39 % (51)");
    // Helt uenig and Uenig are hidden: one segment of 130 − 117 = 13, never their own widths.
    expect(screen.queryByTestId("segment-1")).not.toBeInTheDocument();
    expect(screen.queryByTestId("segment-2")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("skjult")).toHaveLength(1);
    expect(screen.getByTestId("skjult")).toHaveStyle({ width: `${(13 / 130) * 100}%` });
    expect(screen.queryByText(/^Snitt/)).not.toBeInTheDocument();
    // Koden blir bedre has n null: text, no bar.
    expect(screen.getAllByTestId("segment-4")).toHaveLength(1);
  });

  it("gives the mean only when no cell is hidden", () => {
    const v = {
      version: 1,
      n: 10,
      options: [1, 2, 3, 4, 5].map((x) => ({ value: `${x}`, count: 2, suppressed: false })),
    };
    expect(likert(v, 1, 5)).toMatchObject({ hidden: null, mean: 3 });
    v.options[0] = { value: "1", count: null as unknown as number, suppressed: true };
    v.options[1] = { value: "2", count: null as unknown as number, suppressed: true };
    expect(likert(v, 1, 5)).toMatchObject({ hidden: { count: 4 }, mean: null });
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
