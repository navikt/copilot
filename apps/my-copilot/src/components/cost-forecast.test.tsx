import { render, screen } from "@testing-library/react";
import { CostForecastView } from "./cost-forecast";
import type { CostForecast, CostForecastModel } from "@/lib/types";

const model = (m: Partial<CostForecastModel>): CostForecastModel => ({
  model: "trend",
  gross_mid_usd: 1000,
  net_low_usd: 425,
  net_mid_usd: 500,
  net_high_usd: 575,
  range_pct: 15,
  allowance_runs_out: "2026-10-06",
  backtest_error_pct: 4.9,
  backtest_months: ["2026-08", "2026-09"],
  budget_pct: 50,
  over_budget: false,
  ...m,
});

const base: CostForecast = {
  month: "2026-10",
  days_elapsed: 12,
  days_in_month: 31,
  allowance_used_up: true,
  net_to_date_usd: 200,
  budget_status: "set",
  budget_usd: 1000,
  models: [model({}), model({ model: "per_user", net_mid_usd: 700, budget_pct: 70, backtest_months: [] })],
};

describe("CostForecastView", () => {
  it("shows both models with range, share of budget and accuracy", () => {
    render(<CostForecastView data={base} />);
    expect(screen.getByText("Trend")).toBeInTheDocument();
    expect(screen.getByText("Per person")).toBeInTheDocument();
    expect(screen.getAllByText(/425 USD–575 USD/)).toHaveLength(2);
    expect(screen.getByText("70 %")).toBeInTheDocument();
    expect(screen.getByText(/4,9 % i snitt/)).toBeInTheDocument();
    expect(screen.getByText("Ingen avsluttet måned ennå")).toBeInTheDocument();
    expect(screen.getByText(/Inkluderte kreditter brukt opp 6. oktober/)).toBeInTheDocument();
    expect(screen.getByText(/Usikkert anslag, kalibreres videre/)).toBeInTheDocument();
    expect(screen.queryByText(/over budsjettet/)).not.toBeInTheDocument();
  });

  it("shows the projected run-out date while credits are left", () => {
    render(<CostForecastView data={{ ...base, allowance_used_up: false }} />);
    expect(screen.getByText(/brukt opp ca. 6. oktober/)).toBeInTheDocument();
  });

  it("shows A4 when a midpoint is over budget", () => {
    render(<CostForecastView data={{ ...base, models: [model({ over_budget: true })] }} />);
    expect(screen.getByText(/er over budsjettet i GitHub/)).toBeInTheDocument();
  });

  it("says no budget is set and shows no A4", () => {
    render(<CostForecastView data={{ ...base, budget_status: "missing", budget_usd: undefined }} />);
    expect(screen.getByText(/Ikke satt budsjett i GitHub\./)).toBeInTheDocument();
    expect(screen.queryByText(/over budsjettet/)).not.toBeInTheDocument();
  });
});
