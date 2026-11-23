import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { BudgetRaisePage, BudgetSkeleton } from "./budget-raise";

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

function mockApi(
  budget: object,
  raise?: { status: number; body: object },
  profile: object = { teamId: "annet", role: "utvikler", exists: true }
) {
  const fetchMock = vi.fn(async (url: string) => {
    const json = (status: number, body: object) => new Response(JSON.stringify(body), { status });
    if (url === "/api/budget") return json(200, budget);
    if (url === "/api/budget/options") return json(200, options);
    if (url === "/api/budget/profile") return json(200, profile);
    if (url === "/api/budget/raises" && raise) return json(raise.status, raise.body);
    return json(404, {});
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

const defaultBudget = { budgetAmount: 150, consumedAmount: 120, isOverride: false, defaultBudget: 150, raises: [] };

afterEach(() => vi.unstubAllGlobals());

describe("BudgetSkeleton", () => {
  it("announces loading and marks the page busy", () => {
    const { container } = render(<BudgetSkeleton />);
    expect(screen.getByRole("status")).toHaveTextContent("Laster budsjett");
    expect(container.querySelector('[aria-busy="true"]')).toBeInTheDocument();
    expect(screen.getByRole("status").closest('[aria-busy="true"]')).toBeNull();
  });

  it("shows while the data loads", () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise(() => {}))
    );
    render(<BudgetRaisePage />);
    expect(screen.getByRole("status")).toHaveTextContent("Laster budsjett");
  });
});

describe("BudgetRaisePage", () => {
  it("shows the summary, consumption and how many raises are left", async () => {
    mockApi(defaultBudget);
    render(<BudgetRaisePage />);
    expect(await screen.findByRole("heading", { name: "Kort fortalt" })).toBeInTheDocument();
    expect(screen.getByText(/Du har brukt 120 USD av 150 USD/)).toBeInTheDocument();
    expect(screen.getByText(/Du kan fylle på 4 ganger til/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Fyll på 150 USD" })).toBeInTheDocument();
  });

  it("shows stat boxes and says when the saved team has been left", async () => {
    mockApi(defaultBudget, undefined, { teamId: "annet", role: "utvikler", exists: true, leftTeamId: "team-gammel" });
    render(<BudgetRaisePage />);
    expect(await screen.findByText("Igjen")).toBeInTheDocument();
    expect(screen.getByText("30 USD")).toBeInTheDocument();
    expect(screen.getByText(/Du er ikke lenger i teamet «team-gammel»/)).toBeInTheDocument();
  });

  it("hides the ceiling and raise count for an own budget", async () => {
    mockApi({ budgetAmount: 1000, consumedAmount: 120, isOverride: true, defaultBudget: 150, raises: [] });
    render(<BudgetRaisePage />);
    expect(await screen.findByText("Igjen")).toBeInTheDocument();
    expect(screen.queryByText(/^Tak /)).not.toBeInTheDocument();
    expect(screen.queryByText("Påfyll denne måneden")).not.toBeInTheDocument();
  });

  it("keeps the saved team selectable when the team list is down", async () => {
    mockApi(defaultBudget, undefined, { teamId: "team-b", role: "utvikler", exists: true });
    render(<BudgetRaisePage />);
    expect(await screen.findByRole("option", { name: "team-b (lagret)" })).toBeInTheDocument();
    expect(screen.getByLabelText(/^Team/, { selector: "#profil-team" })).toHaveValue("team-b");
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
    expect(screen.queryByText(/Trenger du mer, fyller/)).not.toBeInTheDocument();
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
  it("shows the form again with a fresh key once the pending raise resolves", async () => {
    let n = 0;
    const m = vi.fn(async (url: string) => {
      const json = (status: number, body: object) => new Response(JSON.stringify(body), { status });
      if (url === "/api/budget") return json(200, defaultBudget);
      if (url === "/api/budget/options") return json(200, options);
      if (url === "/api/budget/profile") return json(200, { teamId: "annet", role: "utvikler", exists: true });
      n++;
      return json(200, {
        raise: { status: n === 1 ? "pending" : "applied" },
        replayed: n === 2,
        budgetUsd: 300,
        raisesLeft: 3,
      });
    });
    vi.stubGlobal("fetch", m);
    render(<BudgetRaisePage />);
    await fillAndSubmit();
    fireEvent.click(await screen.findByRole("button", { name: "Prøv igjen" }));
    expect(await screen.findByRole("button", { name: "Fyll på 150 USD" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Prøv igjen" })).not.toBeInTheDocument();
    await fillAndSubmit();
    const raises = () => m.mock.calls.filter(([u]) => u === "/api/budget/raises") as unknown as [string, RequestInit][];
    await waitFor(() => expect(raises()).toHaveLength(3));
    const key = (c: [string, RequestInit]) => (c[1].headers as Record<string, string>)["Idempotency-Key"];
    expect(key(raises()[1])).toBe(key(raises()[0]));
    expect(key(raises()[2])).not.toBe(key(raises()[0]));
  });

  it("offers a same-key retry after github_error on the last raise, even when the reload hides the form", async () => {
    const row = (id: string, status: string) => ({
      id,
      createdAt: "2026-12-01T00:00:00Z",
      amountUsd: 150,
      category: "koding",
      expectedValue: "spare_tid",
      status,
    });
    const before = { ...defaultBudget, isOverride: true, raises: ["1", "2", "3"].map((id) => row(id, "applied")) };
    const after = { ...before, raises: [...before.raises, row("4", "pending")] };
    let posted = false;
    const m = vi.fn(async (url: string) => {
      const json = (status: number, body: object) => new Response(JSON.stringify(body), { status });
      if (url === "/api/budget") return json(200, posted ? after : before);
      if (url === "/api/budget/options") return json(200, options);
      if (url === "/api/budget/profile") return json(200, { teamId: "annet", role: "utvikler", exists: true });
      posted = true;
      return json(502, { error: "github_error" });
    });
    vi.stubGlobal("fetch", m);
    render(<BudgetRaisePage />);
    await fillAndSubmit();
    const retry = await screen.findByRole("button", { name: "Prøv igjen" });
    await waitFor(() => expect(screen.queryByRole("button", { name: "Fyll på 150 USD" })).not.toBeInTheDocument());
    fireEvent.click(retry);
    const raises = () => m.mock.calls.filter(([u]) => u === "/api/budget/raises") as unknown as [string, RequestInit][];
    await waitFor(() => expect(raises()).toHaveLength(2));
    const key = (c: [string, RequestInit]) => (c[1].headers as Record<string, string>)["Idempotency-Key"];
    // The key survives the github_error reload.
    expect(key(raises()[1])).toBe(key(raises()[0]));
    expect(raises()[1][1].body).toBe(raises()[0][1].body);
  });

  it("survives raises: null from copilot-api", async () => {
    mockApi({ ...defaultBudget, raises: null });
    render(<BudgetRaisePage />);
    expect(await screen.findByText("Vi får ikke vist påfyllene dine nå.")).toBeInTheDocument();
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
    expect(
      screen.getByText(/vet ikke ennå om endringen gikk gjennom[^]*uten å laste inn siden på nytt/)
    ).toBeInTheDocument();
    expect(screen.queryByText(/allerede registrert|Du har fått påfyll/)).not.toBeInTheDocument();
    // Raises are left, but the form is hidden until this raise resolves.
    expect(screen.queryByRole("button", { name: "Fyll på 150 USD" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Prøv igjen" }));
    await waitFor(() => expect(raiseCalls(m)).toHaveLength(2));
    const key = (c: [string, RequestInit]) => (c[1].headers as Record<string, string>)["Idempotency-Key"];
    expect(key(raiseCalls(m)[1])).toBe(key(raiseCalls(m)[0]));
  });

  it("does not claim a refusal when GitHub answered badly", async () => {
    const m = mockApi(defaultBudget, { status: 502, body: { error: "github_error" } });
    render(<BudgetRaisePage />);
    await fillAndSubmit();
    expect(await screen.findByText("Vi vet ikke om påfyllet gikk gjennom")).toBeInTheDocument();
    expect(screen.queryByText("Du fikk ikke påfyll")).not.toBeInTheDocument();
    expect(screen.getByText(/uten å laste inn siden på nytt/)).toBeInTheDocument();
    // The budget is fetched again so the table shows what GitHub really did.
    await waitFor(() => expect(m.mock.calls.filter(([u]) => u === "/api/budget")).toHaveLength(2));
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
