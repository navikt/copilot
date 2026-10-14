import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";

vi.mock("./hide", () => ({ hideFreetext: vi.fn(async () => true) }));
vi.mock("next/headers", () => ({ headers: vi.fn() }));
vi.mock("next/cache", () => ({ revalidatePath: vi.fn() }));
vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
  redirect: vi.fn(),
}));

import SurveyFreetextPage from "./page";
import { FreetextList } from "./freetext-list";
import { hideFreetext } from "./hide";
import { POST as hideRoute } from "@/app/api/survey/[id]/freetext/hide/route";
import type { FreetextField } from "@/lib/survey";

const group = "11111111-2222-3333-4444-555555555555";
const tok = (c: string) => c.repeat(64);
const realHide = async (id: string, t: string) =>
  (
    await hideRoute(new Request("http://x/", { method: "POST", body: JSON.stringify({ token: t }) }), {
      params: Promise.resolve({ id }),
    })
  ).status;

const fields: FreetextField[] = [
  {
    id: "verktoy.other",
    question: "Hvilke verktøy bruker du?",
    other: "Annet",
    texts: [{ token: tok("a"), text: "Cursor" }],
  },
  {
    id: "opplevelse",
    question: "Hva er din mest minneverdige opplevelse?",
    texts: [
      { token: tok("b"), text: "Agenten fant en feil jeg hadde lett etter i to dager" },
      { token: tok("c"), text: "Skriv til [e-post]" },
    ],
  },
];

describe("/admin/undersokelser/[id]/fritekst", () => {
  beforeEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "");
    vi.stubEnv("COPILOT_ADMIN_GROUP_ID", group);
    vi.stubEnv("DEV_USER_GROUPS", `other,${group}`);
    vi.stubEnv("COPILOT_SURVEY_URL", "http://copilot-survey");
  });

  it("gives a non-admin 404, and the hide route refuses, without calling copilot-survey", async () => {
    vi.stubEnv("DEV_USER_GROUPS", "other");
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    await expect(SurveyFreetextPage({ params: Promise.resolve({ id: "q4-2026" }) })).rejects.toThrow("NEXT_NOT_FOUND");
    expect(await realHide("q4-2026", tok("a"))).toBe(404);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("hides through copilot-survey for an admin", async () => {
    const fetch = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetch);
    expect(await realHide("q4-2026", tok("a"))).toBe(204);
    expect(fetch).toHaveBeenCalledWith(
      "http://copilot-survey/api/v1/admin/surveys/q4-2026/freetext/hide",
      expect.objectContaining({ method: "POST", body: JSON.stringify({ token: tok("a") }) })
    );
    expect(await realHide("q4-2026", "not-a-token")).toBe(502);
    expect(fetch).toHaveBeenCalledTimes(1);
  });
});

describe("FreetextList", () => {
  it("lists each field with its count, «Annet» under its question, and searches", () => {
    render(<FreetextList surveyId="q4-2026" fields={fields} />);
    expect(screen.getByRole("heading", { name: "Hvilke verktøy bruker du?" })).toBeInTheDocument();
    expect(screen.getByText("Svar under «Annet»: 1 svar")).toBeInTheDocument();
    expect(screen.getByText("Fritekst: 2 svar")).toBeInTheDocument();
    fireEvent.change(screen.getByRole("searchbox", { name: /opplevelse/ }), { target: { value: "AGENTEN" } });
    expect(screen.getByText(/Agenten fant en feil/)).toBeInTheDocument();
    expect(screen.queryByText("Skriv til [e-post]")).not.toBeInTheDocument();
  });

  it("hides a text after confirm, and not without", async () => {
    vi.stubGlobal(
      "confirm",
      vi.fn(() => false)
    );
    render(<FreetextList surveyId="q4-2026" fields={fields} />);
    fireEvent.click(screen.getAllByRole("button", { name: "Skjul" })[0]);
    expect(hideFreetext).not.toHaveBeenCalled();
    vi.stubGlobal(
      "confirm",
      vi.fn(() => true)
    );
    fireEvent.click(screen.getAllByRole("button", { name: "Skjul" })[0]);
    await waitFor(() => expect(screen.queryByText("Cursor")).not.toBeInTheDocument());
    expect(hideFreetext).toHaveBeenCalledWith("q4-2026", tok("a"));
  });
});
