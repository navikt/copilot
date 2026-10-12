import { render, screen } from "@testing-library/react";
import { HighConsumersView } from "./high-consumers";
import type { HighConsumerMonth } from "@/lib/types";

vi.mock("react-chartjs-2", () => ({ Line: () => <canvas />, Bar: () => <canvas /> }));

const month = (m: Partial<HighConsumerMonth>): HighConsumerMonth => ({
  month: "2026-09",
  basis: "netto",
  users: 200,
  tiers: [
    { top_pct: 1, users: 0, net_share_pct: null },
    { top_pct: 5, users: 10, net_share_pct: 59.6 },
    { top_pct: 10, users: 20, net_share_pct: 61.7 },
  ],
  limit_usd: 800,
  limit_changed: true,
  budget_bands: [
    { first: 0, last: 0, label: "0–10 %", users: 190 },
    { first: 1, last: 6, label: "Over 10 %", users: 10 },
  ],
  ...m,
});

describe("HighConsumersView", () => {
  it("shows shares, hides small groups and spans merged bands", () => {
    render(
      <HighConsumersView months={[month({}), month({ month: "2026-01", basis: "premium", budget_bands: null })]} />
    );
    expect(screen.getAllByText("59,6 %").length).toBe(2);
    expect(screen.getByText(/jan.*2026\*/)).toBeInTheDocument();
    expect(screen.getByText("95 %")).toBeInTheDocument();
    const merged = screen.getByText("5 %", { selector: "td" });
    expect(merged.getAttribute("colspan")).toBe("6");
    expect(screen.getByText("800 USD (endret)")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: /brukte 90 % eller mer av grensen/ })).toBeInTheDocument();
    expect(screen.getByText("Vis tallene")).toBeInTheDocument();
  });

  it("shows month to date in its own panel, apart from full months", () => {
    const mtd = month({
      month: "2026-10",
      basis: "hittil",
      tiers: [],
      last_day: "2026-10-11",
      snapshots: 0,
      limit_changed: false,
    });
    render(<HighConsumersView months={[month({}), mtd]} />);
    expect(screen.getByRole("heading", { name: "Hittil i måneden (11 av 31 dager)" })).toBeInTheDocument();
    expect(screen.getByText(/kan ikke sammenlignes med hele/)).toBeInTheDocument();
    expect(screen.getByText("Egen grense")).toBeInTheDocument();
    // Not in the chart or the full-month table: only the panel's own row.
    expect(screen.getAllByRole("row").filter((r) => r.textContent?.includes("Hittil"))).toHaveLength(1);
    expect(screen.getByRole("img", { name: /^Andel brukere.*hele måneder\. sep.*2026/ })).toBeInTheDocument();
  });

  it("says so when no month is billed", () => {
    render(<HighConsumersView months={[]} />);
    expect(screen.getByText("Ingen fakturerte måneder ennå.")).toBeInTheDocument();
  });
});

describe("HighConsumersView unknown limit", () => {
  it("shows users outside the bands as «Ukjent grense», so each bar adds up", () => {
    render(<HighConsumersView months={[month({ users: 250 })]} />);
    expect(screen.getByRole("columnheader", { name: "Ukjent grense" })).toBeInTheDocument();
    expect(screen.getByText("20 %", { selector: "td" })).toBeInTheDocument();
  });
});
