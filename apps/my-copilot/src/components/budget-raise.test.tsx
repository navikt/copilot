import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { BudgetRaisePage } from "./budget-raise";

const options = {
  roles: ["utvikler", "annet"],
  categories: ["koding", "annet"],
  expectedValues: ["spare_tid", "annet"],
  teams: [{ id: "annet", name: "Annet/ukjent" }],
  stepUsd: 150,
  capUsd: 750,
  defaultUsd: 150,
  freeTextMax: 500,
};

function mockApi(budget: object, raise?: { status: number; body: object }) {
  const fetchMock = vi.fn(async (url: string) => {
    const json = (status: number, body: object) => new Response(JSON.stringify(body), { status });
    if (url === "/api/budget") return json(200, budget);
    if (url === "/api/budget/options") return json(200, options);
    if (url === "/api/budget/profile") return json(200, { teamId: "annet", role: "utvikler", exists: true });
    if (url === "/api/budget/raises" && raise) return json(raise.status, raise.body);
    return json(404, {});
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

const defaultBudget = { budgetAmount: 150, consumedAmount: 120, isOverride: false, defaultBudget: 150, raises: [] };

afterEach(() => vi.unstubAllGlobals());

describe("BudgetRaisePage", () => {
  it("shows the summary, consumption and how many raises are left", async () => {
    mockApi(defaultBudget);
    render(<BudgetRaisePage />);
    expect(await screen.findByRole("heading", { name: "Kort fortalt" })).toBeInTheDocument();
    expect(screen.getByText(/Du har brukt 120 USD av 150 USD/)).toBeInTheDocument();
    expect(screen.getByText(/Du kan fylle på 4 ganger til/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Fyll på 150 USD" })).toBeInTheDocument();
  });

  it("lists the errors and sends nothing when required fields are empty", async () => {
    const fetchMock = mockApi(defaultBudget);
    render(<BudgetRaisePage />);
    fireEvent.click(await screen.findByRole("button", { name: "Fyll på 150 USD" }));
    expect(await screen.findByRole("link", { name: "Velg hva du skal bruke påfyllet til." })).toHaveAttribute(
      "href",
      "#pafyll-kategori"
    );
    expect(fetchMock).not.toHaveBeenCalledWith("/api/budget/raises", expect.anything());
  });

  it("explains the ceiling when the raise is refused", async () => {
    mockApi(defaultBudget, { status: 409, body: { error: "cap_reached" } });
    render(<BudgetRaisePage />);
    fireEvent.change(await screen.findByLabelText("Hva skal du bruke påfyllet til?"), { target: { value: "koding" } });
    fireEvent.change(screen.getByLabelText("Hva forventer du å få ut av det?"), { target: { value: "spare_tid" } });
    fireEvent.click(screen.getByRole("button", { name: "Fyll på 150 USD" }));
    expect(await screen.findByText(/Du har nådd taket for denne måneden/)).toBeInTheDocument();
  });

  it("confirms a raise with the new budget", async () => {
    mockApi(defaultBudget, {
      status: 201,
      body: { budgetUsd: 300, expiresAt: "2026-12-01", raisesLeft: 3, dryRun: false },
    });
    render(<BudgetRaisePage />);
    fireEvent.change(await screen.findByLabelText("Hva skal du bruke påfyllet til?"), { target: { value: "koding" } });
    fireEvent.change(screen.getByLabelText("Hva forventer du å få ut av det?"), { target: { value: "spare_tid" } });
    fireEvent.click(screen.getByRole("button", { name: "Fyll på 150 USD" }));
    await waitFor(() => expect(screen.getByText(/Budsjettet ditt er nå 300 USD ut måneden/)).toBeInTheDocument());
  });

  it("tells users with their own fixed budget that raises do not apply", async () => {
    mockApi({ ...defaultBudget, budgetAmount: 1000, isOverride: true });
    render(<BudgetRaisePage />);
    expect(await screen.findByText("Påfyll gjelder ikke for deg")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Fyll på 150 USD" })).not.toBeInTheDocument();
  });
});

describe("BudgetRaisePage «Kort fortalt»", () => {
  it("says the ceiling is reached instead of offering a raise", async () => {
    const r = {
      createdAt: "2026-12-01T00:00:00Z",
      amountUsd: 150,
      category: "koding",
      teamId: "annet",
      expectedValue: "laere",
      freeText: null,
      status: "applied",
    };
    mockApi({
      ...defaultBudget,
      budgetAmount: 750,
      isOverride: true,
      raises: ["1", "2", "3", "4"].map((id) => ({ ...r, id })),
    });
    render(<BudgetRaisePage />);
    await screen.findByRole("heading", { name: "Kort fortalt" });
    expect(screen.queryByText(/Går du tom/)).not.toBeInTheDocument();
    expect(screen.getAllByText(/Du har nådd taket/).length).toBeGreaterThan(0);
  });
});

async function fillAndSubmit() {
  fireEvent.change(await screen.findByLabelText("Hva skal du bruke påfyllet til?"), { target: { value: "koding" } });
  fireEvent.change(screen.getByLabelText("Hva forventer du å få ut av det?"), { target: { value: "spare_tid" } });
  fireEvent.click(screen.getByRole("button", { name: "Fyll på 150 USD" }));
}
const raiseCalls = (m: ReturnType<typeof mockApi>) =>
  m.mock.calls.filter(([u]) => u === "/api/budget/raises") as unknown as [string, RequestInit][];

describe("BudgetRaisePage review fixes", () => {
  it("survives raises: null from copilot-api", async () => {
    mockApi({ ...defaultBudget, raises: null });
    render(<BudgetRaisePage />);
    expect(await screen.findByText("Vi får ikke vist påfyllene dine akkurat nå.")).toBeInTheDocument();
  });

  it("sends no team unless the user picked one, so the profile team stays the default", async () => {
    const m = mockApi(defaultBudget, {
      status: 201,
      body: { raise: { status: "applied" }, budgetUsd: 300, raisesLeft: 3 },
    });
    render(<BudgetRaisePage />);
    await fillAndSubmit();
    await waitFor(() => expect(raiseCalls(m)).toHaveLength(1));
    expect(JSON.parse(raiseCalls(m)[0][1].body as string)).not.toHaveProperty("teamId");
  });

  it("counts raises from the default budget, not GitHub's current budget", async () => {
    mockApi({ ...defaultBudget, budgetAmount: 800, raises: [] });
    render(<BudgetRaisePage />);
    expect(await screen.findByText(/Du kan fylle på 4 ganger til/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Fyll på 150 USD" })).toBeInTheDocument();
  });

  it("shows a pending replay as pending and keeps the key for the retry", async () => {
    const m = mockApi(defaultBudget, { status: 200, body: { raise: { status: "pending" }, replayed: true } });
    render(<BudgetRaisePage />);
    await fillAndSubmit();
    expect(await screen.findByText("Påfyllet venter på GitHub")).toBeInTheDocument();
    expect(screen.queryByText(/allerede registrert|Du har fått påfyll/)).not.toBeInTheDocument();
    await fillAndSubmit();
    await waitFor(() => expect(raiseCalls(m)).toHaveLength(2));
    const key = (c: [string, RequestInit]) => (c[1].headers as Record<string, string>)["Idempotency-Key"];
    expect(key(raiseCalls(m)[1])).toBe(key(raiseCalls(m)[0]));
  });

  it("shows a failed replay as failed", async () => {
    mockApi(defaultBudget, { status: 200, body: { raise: { status: "failed" }, replayed: true } });
    render(<BudgetRaisePage />);
    await fillAndSubmit();
    expect(await screen.findByText("Påfyllet feilet")).toBeInTheDocument();
  });

  it("says a dry run changed nothing in GitHub", async () => {
    mockApi(defaultBudget, {
      status: 201,
      body: { raise: { status: "dry_run" }, budgetUsd: 300, raisesLeft: 3, dryRun: true },
    });
    render(<BudgetRaisePage />);
    await fillAndSubmit();
    expect(await screen.findByText("Testkjøring: ingenting er endret i GitHub")).toBeInTheDocument();
    expect(screen.queryByText(/Budsjettet ditt er nå/)).not.toBeInTheDocument();
  });
});
