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
  models: [model({}), model({ model: "per_user", net_mid_usd: 700, budget_pct: 70 })],
};

describe("CostForecastView", () => {
  it("shows one line with the days used and the past error of each shown forecast", () => {
    render(<CostForecastView data={base} />);
    expect(screen.getByText(/Anslag \(bygger på 12 av 31 dager\)/)).toHaveTextContent(
      /Nav samlet 4,9 % i snitt .*Hver person 4,9 % i snitt/
    );
    expect(screen.queryByText(/over budsjettet/)).not.toBeInTheDocument();
  });

  it("hides the per-person forecast without two closed months within the limit", () => {
    const models = [model({}), model({ model: "per_user", backtest_months: ["2026-09"] })];
    render(<CostForecastView data={{ ...base, models }} />);
    expect(screen.queryByText(/Hver person/)).not.toBeInTheDocument();
  });

  it("warns when the expected bill is over budget", () => {
    render(<CostForecastView data={{ ...base, models: [model({ over_budget: true })] }} />);
    expect(screen.getByText(/er over budsjettet i GitHub/)).toBeInTheDocument();
  });
});
