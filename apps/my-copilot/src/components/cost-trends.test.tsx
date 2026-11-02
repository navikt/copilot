import { render, screen, within } from "@testing-library/react";
import { CostTrendsView } from "./cost-trends";
import type { AdminMonth } from "@/lib/types";

vi.mock("react-chartjs-2", () => ({ Line: () => <canvas />, Bar: () => <canvas /> }));

describe("CostTrendsView", () => {
  it("divides net by billed users for billed months and withholds per-user amount when billed=false", () => {
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
        active={[
          { month: "2026-05", active_users: 4, billed: true },
          { month: "2026-10", active_users: 3, billed: false },
        ]}
        admin={[{ month: "2026-05", licence_usd_estimate: 250, total_usd: 1250 } as AdminMonth]}
        currentMonth="2026-10"
      />
    );
    expect(screen.queryByText(/des.*2025/)).not.toBeInTheDocument();
    expect(screen.getByText("250 USD")).toBeInTheDocument();
    expect(screen.getAllByText(/mai.*2026\*/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/okt.*2026 \(hittil\)/).length).toBeGreaterThan(0);
    expect(screen.getByText("Hvor mye koster Copilot per person?")).toBeInTheDocument();

    // Summary uses billed=true months only and mentions unbilled months missing denominator
    expect(
      screen.getByText(/Fakturert per fakturert bruker gikk fra 250 USD i mai 2026 til 250 USD/)
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /Måneder uten fullført faktureringsinnlesing viser totalbeløp, men mangler nevner for fakturerte brukere/
      )
    ).toBeInTheDocument();

    // October 2026 (billed=false) retains total 300 USD but does NOT divide by 3 to show 100 USD
    expect(screen.queryByText("100 USD")).not.toBeInTheDocument();
    expect(screen.getByText("300 USD")).toBeInTheDocument();

    // Footnote explains unbilled months
    expect(
      screen.getByText(
        /Måneder uten fullført faktureringsinnlesing viser faktisk eller estimert beløp, men ikke fakturerte brukere/
      )
    ).toBeInTheDocument();

    expect(screen.getByText("75 %", { selector: "td" })).toBeInTheDocument();
    expect(screen.getByText("100 %", { selector: "td" })).toBeInTheDocument();
    expect(screen.getByText("20 %")).toBeInTheDocument();
  });

  it("distinguishes observed zero from absent denominator without fallback zero", () => {
    const trend = [
      { year_month: "2026-06", total_gross_amount: 0, total_net_amount: 500, discount_rate_pct: 0, distinct_models: 1 },
      { year_month: "2026-07", total_gross_amount: 0, total_net_amount: 200, discount_rate_pct: 0, distinct_models: 1 },
      { year_month: "2026-08", total_gross_amount: 0, total_net_amount: 400, discount_rate_pct: 0, distinct_models: 1 },
    ];
    render(
      <CostTrendsView
        trend={trend}
        breakdown={[]}
        active={[
          // Observed 0 billed users: denominator is 0, not missing
          { month: "2026-06", active_users: 0, billed: true },
          // 2026-07 is absent from active array
          // 2026-08 has active users but billed is false
          { month: "2026-08", active_users: 5, billed: false },
        ]}
        currentMonth="2026-08"
      />
    );

    // Row 2026-06 (billed=true, users=0): observed "0" in user count cell, per-user is "–" (no division by zero / fallback zero)
    const rowJun = screen.getByText(/jun.*2026/).closest("tr")!;
    const cellsJun = within(rowJun).getAllByRole("cell");
    expect(cellsJun[0]).toHaveTextContent("500 USD");
    expect(cellsJun[1]).toHaveTextContent("0"); // observed 0
    expect(cellsJun[2]).toHaveTextContent("–"); // per-user undefined, not 0 USD

    // Row 2026-07 (absent from active array): user count is "–", per-user is "–"
    const rowJul = screen.getByText(/jul.*2026/).closest("tr")!;
    const cellsJul = within(rowJul).getAllByRole("cell");
    expect(cellsJul[0]).toHaveTextContent("200 USD");
    expect(cellsJul[1]).toHaveTextContent("–");
    expect(cellsJul[2]).toHaveTextContent("–");

    // Row 2026-08 (billed=false, users=5): user count is "–", per-user is "–" (withholds unbilled denominator)
    const rowAug = screen.getByText(/aug.*2026/).closest("tr")!;
    const cellsAug = within(rowAug).getAllByRole("cell");
    expect(cellsAug[0]).toHaveTextContent("400 USD");
    expect(cellsAug[1]).toHaveTextContent("–");
    expect(cellsAug[2]).toHaveTextContent("–");
  });
});
