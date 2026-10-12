import { render, screen } from "@testing-library/react";
import { CostTrendsView } from "./cost-trends";
import type { AdminMonth } from "@/lib/types";

vi.mock("react-chartjs-2", () => ({ Line: () => <canvas />, Bar: () => <canvas /> }));

describe("CostTrendsView", () => {
  it("divides net by active users and shows the model mix", () => {
    const trend = [
      { year_month: "2025-12", total_gross_amount: 0, total_net_amount: 99, discount_rate_pct: 0, distinct_models: 1 },
      {
        year_month: "2026-05",
        total_gross_amount: 0,
        total_net_amount: 1000,
        discount_rate_pct: 0,
        distinct_models: 2,
      },
      { year_month: "2026-10", total_gross_amount: 0, total_net_amount: 300, discount_rate_pct: 0, distinct_models: 1 },
    ];
    const breakdown = [
      { year_month: "2026-05", model: "Claude Opus 5", gross_amount: 0, net_amount: 750, pct_of_monthly_net: 75 },
      { year_month: "2026-05", model: "Auto: GPT-6 Astra", gross_amount: 0, net_amount: 250, pct_of_monthly_net: 25 },
      { year_month: "2026-10", model: "Code Review model", gross_amount: 0, net_amount: 300, pct_of_monthly_net: 100 },
    ];
    render(
      <CostTrendsView
        trend={trend}
        breakdown={breakdown}
        active={[{ month: "2026-05", active_users: 4 }]}
        admin={[{ month: "2026-05", licence_usd_estimate: 250, total_usd: 1250 } as AdminMonth]}
        currentMonth="2026-10"
      />
    );
    expect(screen.queryByText(/des.*2025/)).not.toBeInTheDocument();
    expect(screen.getByText("250 USD")).toBeInTheDocument();
    expect(screen.getAllByText(/mai.*2026\*/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/okt.*2026 \(hittil\)/).length).toBeGreaterThan(0);
    expect(screen.getByText("Blir Copilot dyrere per bruker?")).toBeInTheDocument();
    expect(screen.getByText(/Fakturert per aktiv bruker gikk fra 250 USD/)).toBeInTheDocument();
    expect(screen.getByText("75 %", { selector: "td" })).toBeInTheDocument();
    expect(screen.getByText("100 %", { selector: "td" })).toBeInTheDocument();
    expect(screen.getByText("20 %")).toBeInTheDocument();
  });
});
