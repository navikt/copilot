import { render, screen, within } from "@testing-library/react";
import { vi } from "vitest";

vi.mock("next/headers", () => ({ headers: vi.fn() }));
vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
  redirect: vi.fn(),
  usePathname: () => "/admin/undersokelser/sammenlign",
}));

import ComparePage from "./page";
import type { SurveyComparison, WaveStat } from "@/lib/survey";

const group = "11111111-2222-3333-4444-555555555555";

const scale = (counts: number[], version = 1): WaveStat => {
  const n = counts.reduce((a, b) => a + b, 0);
  const mean = counts.reduce((a, c, i) => a + c * (i + 1), 0) / n;
  return {
    version,
    n,
    options: counts.map((c, i) => ({ value: `${i + 1}`, count: c, suppressed: false })),
    mean,
    agree: (counts[3] + counts[4]) / n,
  };
};

// Shaped like compare() in copilot-survey: spring (Forms) and autumn (web + terminal).
const comparison: SurveyComparison = {
  surveys: [
    {
      id: "q2-2026",
      title: "Utviklerundersøkelsen våren 2026",
      starts: "2026-03-24",
      ends: "2026-04-30",
      status: "stengt",
      responses: 165,
      suppressed: false,
      channels: { forms: 165 },
    },
    {
      id: "q4-2026",
      title: "Utviklerundersøkelsen høsten 2026",
      starts: "2026-10-13",
      ends: "2026-10-27",
      status: "åpen",
      responses: 20,
      suppressed: false,
      channels: { web: 15, copilot: 5 },
    },
  ],
  questions: [
    {
      id: "verktoy",
      type: "multi",
      text: "Hvilke verktøy bruker du?",
      break: false,
      waves: [
        {
          excluded: 0,
          versions: [{ version: 1, n: 165, options: [{ value: "VS Code", count: 99, suppressed: false }] }],
        },
        {
          excluded: 0,
          versions: [{ version: 1, n: 20, options: [{ value: "VS Code", count: 14, suppressed: false }] }],
        },
      ],
    },
    {
      id: "pastander",
      type: "matrix",
      text: "Hvor enig er du?",
      min: 1,
      max: 5,
      labels: ["Helt uenig", "Uenig", "Nøytral", "Enig", "Helt enig"],
      items: [
        // Spring mean 3,25 / 50 % agree; autumn 4,0 / 75 %.
        {
          id: "tilfredshet",
          text: "Jeg er fornøyd",
          break: false,
          waves: [
            { excluded: 14, versions: [scale([10, 10, 20, 30, 10])] },
            { excluded: 0, versions: [scale([0, 0, 5, 10, 5])] },
          ],
        },
        {
          id: "code-review",
          text: "Review går fortere",
          break: true,
          waves: [
            { excluded: 14, versions: [scale([10, 10, 20, 30, 10])] },
            { excluded: 0, versions: [scale([0, 0, 5, 10, 5], 2)] },
          ],
        },
      ],
    },
  ],
};

function respond(body: unknown, status = 200) {
  const fetch = vi.fn(async (url: string) =>
    url.endsWith("/api/v1/admin/surveys")
      ? new Response(
          JSON.stringify({
            surveys: [
              ...comparison.surveys.map((s) => ({ ...s, series: "utviklerundersokelsen" })),
              { ...comparison.surveys[0], id: "q2-2027", starts: "2027-03-01", status: "kommende" },
              // Started between the two waves, but in another series: never a default.
              { ...comparison.surveys[0], id: "test", series: "annet", starts: "2026-09-28", status: "åpen" },
            ],
          })
        )
      : new Response(JSON.stringify(body), { status })
  );
  vi.stubGlobal("fetch", fetch);
  return fetch;
}

// Suspense children are async server components: render the resolved tree.
async function renderPage(ids?: string) {
  const page = await ComparePage({ searchParams: Promise.resolve(ids ? { ids } : {}) });
  const resolve = async (node: unknown): Promise<unknown> => {
    if (!node || typeof node !== "object") return node;
    if (Array.isArray(node)) return Promise.all(node.map(resolve));
    const el = node as { type: unknown; props: { children?: unknown } };
    if (typeof el.type === "function" && el.type.constructor.name === "AsyncFunction")
      return resolve(await (el.type as (p: unknown) => Promise<unknown>)(el.props));
    if (el.props?.children === undefined) return el;
    return { ...el, props: { ...el.props, children: await resolve(el.props.children) } };
  };
  return render((await resolve(page)) as React.ReactElement);
}

describe("/admin/undersokelser/sammenlign", () => {
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
    const fetch = respond(comparison);
    await expect(ComparePage({ searchParams: Promise.resolve({}) })).rejects.toThrow("NEXT_NOT_FOUND");
    expect(fetch).not.toHaveBeenCalled();
  });

  it("defaults to the two latest started waves, oldest first", async () => {
    const fetch = respond(comparison);
    await renderPage();
    expect(fetch).toHaveBeenCalledWith(
      "http://copilot-survey/api/v1/admin/surveys/compare?ids=q2-2026,q4-2026",
      expect.anything()
    );
  });

  it("picks no default when the newest survey has no series", async () => {
    const fetch = vi.fn(async () => new Response(JSON.stringify({ surveys: comparison.surveys })));
    vi.stubGlobal("fetch", fetch);
    await renderPage();
    expect(screen.getByText("Det trengs minst to runder for å sammenligne.")).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("shows n and channel per wave, deltas, and the exclusion", async () => {
    respond(comparison);
    await renderPage("q2-2026,q4-2026");
    expect(screen.getByRole("row", { name: /våren 2026/ })).toHaveTextContent("Microsoft Forms (radioknapper): 165");
    expect(screen.getByRole("row", { name: /høsten 2026/ })).toHaveTextContent(
      "Nettside (knapperad): 15, Terminal, nav-pilot (copilot): 5"
    );
    expect(screen.getByText("Δ snitt +0,8")).toBeInTheDocument();
    expect(screen.getByText("Δ enig +25 pp")).toBeInTheDocument();
    expect(screen.getByText("+10 pp")).toBeInTheDocument(); // VS Code 60 % → 70 %
    expect(screen.getAllByText(/Holdt utenfor fordi de ikke bruker KI-verktøy.*Våren 2026: 14/)).toHaveLength(2);
  });

  it("marks a version break: no deltas and no line in the overview", async () => {
    respond(comparison);
    await renderPage("q2-2026,q4-2026");
    expect(screen.getAllByText("Spørsmålet er endret: brudd i serien")).toHaveLength(1);
    expect(screen.getAllByText(/^Δ snitt/)).toHaveLength(1);
    expect(within(screen.getByTestId("dumbbell-tilfredshet")).getByTestId("dumbbell-line")).toBeInTheDocument();
    expect(within(screen.getByTestId("dumbbell-code-review")).queryByTestId("dumbbell-line")).not.toBeInTheDocument();
    expect(screen.getAllByRole("columnheader", { name: /Høsten 2026, versjon 2/ })).toHaveLength(1);
  });

  it("draws every version of a broken choice question, labelled, without Δ pp", async () => {
    const [spring, autumn] = comparison.questions[0].waves!;
    const v2 = { ...autumn.versions[0], version: 2 };
    respond({
      ...comparison,
      questions: [
        {
          ...comparison.questions[0],
          break: true,
          waves: [spring, { excluded: 0, versions: [autumn.versions[0], v2] }],
        },
      ],
    });
    await renderPage("q2-2026,q4-2026");
    expect(screen.getByText("Høsten 2026, versjon 2 · n 20")).toBeInTheDocument();
    expect(screen.getByText("Høsten 2026, versjon 1 · n 20")).toBeInTheDocument();
    expect(screen.queryByText(/pp$/)).not.toBeInTheDocument();
  });

  it("maps a bad id and 404 to not found", async () => {
    const fetch = respond({}, 404);
    await expect(renderPage("q2-2026,q4-2026")).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(renderPage("../x,q4-2026")).rejects.toThrow("NEXT_NOT_FOUND");
    expect(fetch).toHaveBeenCalledTimes(1);
  });
});
