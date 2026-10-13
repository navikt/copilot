import { render, screen } from "@testing-library/react";
import { CostStatusCardsView } from "./cost-status-cards";
import type { CostForecast, CostForecastModel } from "@/lib/types";

function model(name: CostForecastModel["model"], mid: number, over_budget = false): CostForecastModel {
  return {
    model: name,
    gross_mid_usd: 1000,
    net_low_usd: mid - 100,
    net_mid_usd: mid,
    net_high_usd: mid + 100,
    range_pct: 20,
    backtest_error_pct: 10,
    backtest_months: ["2026-08", "2026-09"],
    budget_pct: mid / 10,
    over_budget,
  };
}

const base: CostForecast = {
  month: "2026-10",
  days_elapsed: 12,
  days_in_month: 31,
  allowance_used_up: false,
  budget_status: "set",
  budget_usd: 1000,
  net_to_date_usd: 250,
  allowance_run_out_date: "2026-10-20",
  models: [model("trend", 500), model("per_user", 550)],
};

describe("CostStatusCardsView", () => {
  it("shows to date, both forecasts, budget use and run-out date", () => {
    render(<CostStatusCardsView data={base} />);
    expect(screen.getByText(/Nav samlet: 500 USD \(400 USD–600 USD\)/)).toBeInTheDocument();
    expect(screen.getByText(/Hver person: 550 USD/)).toBeInTheDocument();
    expect(screen.getByText(/Brukt: 250 USD \(25 %\)/)).toBeInTheDocument();
    expect(screen.getByText(/Blir brukt opp rundt/)).toBeInTheDocument();
    expect(screen.queryByText("Månedsinnsikt")).not.toBeInTheDocument();
  });

  it("hides the per-person model until its backtest error is within the limit", () => {
    const off = { ...model("per_user", 550), backtest_error_pct: 16 };
    render(<CostStatusCardsView data={{ ...base, models: [model("trend", 500), off] }} />);
    expect(screen.queryByText(/Hver person/)).not.toBeInTheDocument();
  });

  it("says so when no budget is set", () => {
    render(<CostStatusCardsView data={{ ...base, budget_status: "missing", budget_usd: undefined }} />);
    expect(screen.getByText("Ikke satt budsjett i GitHub.")).toBeInTheDocument();
  });

  it("covers missing data and used-up credits", () => {
    render(<CostStatusCardsView data={{ ...base, models: [], budget_status: "error", allowance_used_up: true }} />);
    expect(screen.getByText("For lite data ennå.")).toBeInTheDocument();
    expect(screen.getByText("Kunne ikke hente budsjettet.")).toBeInTheDocument();
    expect(screen.getByText(/^Brukt opp/)).toBeInTheDocument();
  });

  it("links the insight count to the section", () => {
    render(<CostStatusCardsView data={{ ...base, models: [model("trend", 1200, true)] }} insights={3} />);
    expect(screen.getByText(/Forventet faktura er over budsjettet/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /månedsinnsikt/ })).toHaveAttribute("href", "#maanedsinnsikt");
  });
});
