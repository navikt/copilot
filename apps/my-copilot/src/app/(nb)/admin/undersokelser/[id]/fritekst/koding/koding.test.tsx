import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";

const refresh = vi.fn();
vi.mock("next/headers", () => ({ headers: vi.fn() }));
vi.mock("next/cache", () => ({ revalidatePath: vi.fn() }));
vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
  redirect: vi.fn(),
  useRouter: () => ({ refresh }),
}));

import SurveyCodingPage from "./page";
import { CodingPanel } from "./coding-panel";
import { POST as codingRoute } from "@/app/api/survey/[id]/freetext/coding/route";
import { POST as modelRoute } from "@/app/api/survey/[id]/freetext/coding/model/route";
import type { SurveyCoding } from "@/lib/survey";

const group = "11111111-2222-3333-4444-555555555555";
const tok = (c: string) => c.repeat(64);
const route = async (handler: typeof codingRoute, body: string) =>
  handler(new Request("http://x/", { method: "POST", body }), { params: Promise.resolve({ id: "q4-2026" }) });

const field = {
  id: "opplevelse",
  question: "Hva er din mest minneverdige opplevelse?",
  onske: "kodes" as const,
  max_codes: 2,
  categories: [
    {
      id: "raskere",
      name: "Raskere enn for hånd",
      kind: "nytte",
      definition: "Gikk fortere.",
      include: [],
      exclude: [],
      examples: [],
    },
    {
      id: "friksjon",
      name: "Friksjon",
      kind: "problem",
      definition: "Noe sto i veien.",
      include: [],
      exclude: [],
      examples: [],
    },
    { id: "annet", name: "Annet", kind: "annet", definition: "Resten.", include: [], exclude: [], examples: [] },
  ],
};

const coding: SurveyCoding = {
  id: "q4-2026",
  title: "Q4",
  codebook: { version: 1, rules: [], fields: [field] },
  progress: [{ field: "opplevelse", texts: 3, human: 1, model: 3, final: 1, disagreements: 1 }],
  next: { field: "opplevelse", token: tok("a"), text: "Syntetisk tekst til koding" },
  disagreements: [
    {
      field: "opplevelse",
      token: tok("b"),
      text: "Syntetisk uenighet",
      human: [{ category: "friksjon", onske: true }],
      model: [{ category: "raskere", onske: false }],
    },
  ],
  results: [
    {
      id: "opplevelse",
      question: "Hva er din mest minneverdige opplevelse?",
      written: 12,
      coded: 10,
      categories: [
        {
          id: "raskere",
          name: "Raskere enn for hånd",
          count: 7,
          onske: 1,
          share: 7 / 12,
          kappa: 0.8,
          status: "published",
        },
        { id: "friksjon", name: "Friksjon", count: 3, onske: 3, share: 0.25, kappa: 0.61, status: "published" },
        { id: "annet", name: "Annet", count: 0, onske: 0, share: 0, kappa: null, status: "hidden" },
      ],
      published: [
        {
          id: "raskere",
          name: "Raskere enn for hånd",
          count: 7,
          onske: 1,
          share: 7 / 12,
          kappa: 0.8,
          status: "published",
        },
        {
          id: "friksjon",
          name: "Friksjon",
          count: null,
          onske: null,
          share: null,
          small: true,
          kappa: 0.61,
          status: "published",
        },
      ],
    },
  ],
};

describe("/admin/undersokelser/[id]/fritekst/koding", () => {
  beforeEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "");
    vi.stubEnv("COPILOT_ADMIN_GROUP_ID", group);
    vi.stubEnv("DEV_USER_GROUPS", `other,${group}`);
    vi.stubEnv("COPILOT_SURVEY_URL", "http://copilot-survey");
  });

  it("gives a non-admin 404, and the routes refuse, without calling copilot-survey", async () => {
    vi.stubEnv("DEV_USER_GROUPS", "other");
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    await expect(SurveyCodingPage({ params: Promise.resolve({ id: "q4-2026" }) })).rejects.toThrow("NEXT_NOT_FOUND");
    expect((await route(codingRoute, "{}")).status).toBe(404);
    expect((await route(modelRoute, "{}")).status).toBe(404);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("forwards an admin's coding and model file to copilot-survey, with its status", async () => {
    const fetch = vi.fn(async () => Response.json({ error: "already coded" }, { status: 409 }));
    vi.stubGlobal("fetch", fetch);
    const body = JSON.stringify({ token: tok("a"), coder: "human", codes: [{ category: "annet", onske: false }] });
    const res = await route(codingRoute, body);
    expect(res.status).toBe(409);
    expect(await res.json()).toEqual({ error: "already coded" });
    expect(fetch).toHaveBeenCalledWith(
      "http://copilot-survey/api/v1/admin/surveys/q4-2026/freetext/coding",
      expect.objectContaining({ method: "POST", body })
    );
    expect((await route(modelRoute, '{"codebook_version":1,"codes":[]}')).status).toBe(409);
    expect(fetch).toHaveBeenLastCalledWith(
      "http://copilot-survey/api/v1/admin/surveys/q4-2026/freetext/coding/model",
      expect.anything()
    );
    expect((await route(codingRoute, "not json")).status).toBe(400);
    expect((await route(codingRoute, "x".repeat(5000))).status).toBe(413);
    // 3000 UTF-16 units but 6000 bytes: the limit is in bytes.
    expect((await route(codingRoute, JSON.stringify("æ".repeat(3000)))).status).toBe(413);
    expect(fetch).toHaveBeenCalledTimes(2);
  });
});

describe("CodingPanel", () => {
  beforeEach(() => vi.unstubAllGlobals());

  it("codes blind, and shows the model's codes only after saving", async () => {
    const fetch = vi.fn(async () => Response.json({ model: [{ category: "raskere", onske: false }] }));
    vi.stubGlobal("fetch", fetch);
    render(<CodingPanel coding={coding} />);
    expect(screen.getByText("Syntetisk tekst til koding")).toBeInTheDocument();
    expect(screen.queryByText(/Modellen kodet/)).not.toBeInTheDocument();
    const save = screen.getByRole("button", { name: "Lagre koding" });
    expect(save).toBeDisabled();
    fireEvent.click(screen.getAllByRole("checkbox", { name: "Friksjon: ønske" })[0]);
    fireEvent.click(save);
    await waitFor(() => expect(screen.getByText(/Modellen kodet: Raskere enn for hånd/)).toBeInTheDocument());
    expect(fetch).toHaveBeenCalledWith(
      "/api/survey/q4-2026/freetext/coding",
      expect.objectContaining({
        body: JSON.stringify({ token: tok("a"), coder: "human", codes: [{ category: "friksjon", onske: true }] }),
      })
    );
    fireEvent.click(screen.getByRole("button", { name: "Neste tekst" }));
    expect(refresh).toHaveBeenCalled();
  });

  it("refuses «Annet» with other categories", () => {
    render(<CodingPanel coding={coding} />);
    fireEvent.click(screen.getAllByRole("checkbox", { name: "Annet: erfaring" })[0]);
    fireEvent.click(screen.getAllByRole("checkbox", { name: "Friksjon: erfaring" })[0]);
    expect(screen.getByText("«Annet» brukes alene.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Lagre koding" })).toBeDisabled();
  });

  it("codes one category as both an experience and a wish, also in a decision", async () => {
    const fetch = vi.fn(async () => Response.json({ model: null }));
    vi.stubGlobal("fetch", fetch);
    render(<CodingPanel coding={coding} />);
    const [next, decision] = [0, 1].map((i) => ({
      exp: screen.getAllByRole("checkbox", { name: "Friksjon: erfaring" })[i],
      wish: screen.getAllByRole("checkbox", { name: "Friksjon: ønske" })[i],
    }));
    fireEvent.click(next.exp);
    fireEvent.click(next.wish);
    fireEvent.click(screen.getByRole("button", { name: "Lagre koding" }));
    const both = [
      { category: "friksjon", onske: false },
      { category: "friksjon", onske: true },
    ];
    await waitFor(() =>
      expect(fetch).toHaveBeenCalledWith(
        "/api/survey/q4-2026/freetext/coding",
        expect.objectContaining({ body: JSON.stringify({ token: tok("a"), coder: "human", codes: both }) })
      )
    );
    // The decision starts from the person's codes (friksjon as a wish) and adds the experience.
    expect(decision.wish).toBeChecked();
    fireEvent.click(decision.exp);
    fireEvent.click(screen.getByRole("button", { name: "Lagre avgjørelse" }));
    await waitFor(() =>
      expect(fetch).toHaveBeenLastCalledWith(
        "/api/survey/q4-2026/freetext/coding",
        expect.objectContaining({
          body: JSON.stringify({ token: tok("b"), coder: "final", codes: [both[1], both[0]] }),
        })
      )
    );
  });

  it("lists a disagreement with both coders, and shows kappa and what is published", () => {
    render(<CodingPanel coding={coding} />);
    expect(screen.getByText("Du: Friksjon (ønske)")).toBeInTheDocument();
    expect(screen.getByText("Modellen: Raskere enn for hånd")).toBeInTheDocument();
    expect(screen.getByText("0,80")).toBeInTheDocument();
    expect(screen.getByText(/Publiseres: Raskere enn for hånd 7 \(58\s%\), Friksjon <5/)).toBeInTheDocument();
  });
});
