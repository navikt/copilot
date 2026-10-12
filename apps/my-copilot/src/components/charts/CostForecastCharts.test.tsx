import { render, screen } from "@testing-library/react";
import { CostForecastCharts, monthSeries, yearSeries } from "./CostForecastCharts";
import type { AdminMonth, CostForecast } from "@/lib/types";

vi.mock("react-chartjs-2", () => ({ Line: () => <canvas />, Bar: () => <canvas /> }));

const forecast: CostForecast = {
  month: "2026-10",
  days_elapsed: 2,
  days_in_month: 4,
  allowance_used_up: false,
  net_to_date_usd: 0,
  budget_status: "set",
  budget_usd: 500,
  days: [
    { day: "2026-10-01", net_usd: 0, forecast: false },
    { day: "2026-10-02", net_usd: 0, forecast: false },
    { day: "2026-10-03", net_usd: 100, forecast: true },
    { day: "2026-10-04", net_usd: 200, forecast: true },
  ],
  models: [
    {
      model: "trend",
      gross_mid_usd: 900,
      net_low_usd: 170,
      net_mid_usd: 200,
      net_high_usd: 230,
      range_pct: 15,
      allowance_runs_out: "2026-10-03",
      backtest_error_pct: 5,
      backtest_months: [],
      budget_pct: 40,
      over_budget: false,
    },
  ],
};

const month = (m: string, basis: AdminMonth["basis"], net: number | null) =>
  ({ month: m, basis, enterprise_net_usd: net }) as AdminMonth;
const months = [month("2026-05", "premium", 50), month("2026-09", "net", 300), month("2026-10", "gross", null)];

describe("CostForecastCharts", () => {
  it("joins actual and forecast and widens the range to ±15 % at month end", () => {
    const s = monthSeries(forecast);
    expect(s.actual).toEqual([0, 0, null, null]);
    expect(s.forecast).toEqual([null, 0, 100, 200]);
    expect(s.high[3]).toBeCloseTo(230);
    expect(s.low[3]).toBeCloseTo(170);
    expect(s.runOut).toEqual([null, null, 100, null]);
  });

  it("draws the forecast and range from daily_forecast when the API sends it", () => {
    const s = monthSeries({
      ...forecast,
      daily_forecast: [{ date: "2026-10-03", cumulative_net_usd: 120, low: 110, high: 130 }],
    });
    expect(s.forecast).toEqual([null, 0, 120, 200]);
    expect(s.low[2]).toBe(110);
    expect(s.high[2]).toBe(130);
    expect(s.high[3]).toBeCloseTo(230);
  });

  it("splits premium and net months and shows the current month as a range", () => {
    const s = yearSeries(months, forecast);
    expect(s.premium).toEqual([50, null, null]);
    expect(s.net).toEqual([null, 300, null]);
    expect(s.range).toEqual([null, null, [170, 230]]);
  });

  it("gives each chart a text summary", () => {
    render(<CostForecastCharts data={forecast} months={months} />);
    const [month, year] = screen
      .getAllByRole("img")
      .map((e) => document.getElementById(e.getAttribute("aria-labelledby")!)!.textContent);
    expect(month).toMatch(/Forventet faktura ved månedsslutt er 200\sUSD, mellom 170\sUSD og 230\sUSD/);
    expect(month).toMatch(/Kredittene i lisensene brukes etter prognosen opp 3\. oktober\./);
    expect(year).toMatch(/mai 50\sUSD, september 300\sUSD/);
    expect(screen.getAllByText(/Forventet faktura for oktober er 200\sUSD/)).toHaveLength(1);
  });
});
