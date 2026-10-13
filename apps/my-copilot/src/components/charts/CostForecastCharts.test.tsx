import { render, screen } from "@testing-library/react";
import {
  CostForecastCharts,
  monthSeries,
  rangeLabel,
  yearEndLine,
  yearRangeLabel,
  yearSeries,
} from "./CostForecastCharts";
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
  year_forecast: [
    { month: "2026-05", cumulative_net_usd: 50, low: 50, high: 50, forecast: false },
    { month: "2026-09", cumulative_net_usd: 350, low: 350, high: 350, forecast: false },
    { month: "2026-10", cumulative_net_usd: 550, low: 520, high: 580, forecast: true },
  ],
  year_end: {
    mid: 550,
    low: 520,
    high: 580,
    range_pct: 30,
    backtest_error_pct: 12,
    label: "grovt anslag, få måneder å bygge på",
  },
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

  it("joins premium, actual and forecast in the cumulative year line", () => {
    const s = yearSeries(months, forecast);
    expect(s.premium).toEqual([50, 350, null]);
    expect(s.actual).toEqual([null, 350, null]);
    expect(s.forecast).toEqual([null, 350, 550]);
    expect(s.low).toEqual([null, 350, 520]);
    expect(s.high).toEqual([null, 350, 580]);
  });

  it("shows the year end on one line, with the label while it is rough", () => {
    render(<CostForecastCharts data={forecast} months={months} />);
    expect(
      screen.getByText(
        /^Forventet ved årets slutt: 550\sUSD \(520\sUSD–580\sUSD\), grovt anslag, få måneder å bygge på\.$/
      )
    ).toBeInTheDocument();
    expect(yearEndLine({ ...forecast.year_end!, label: undefined })).toMatch(/580\sUSD\)\.$/);
  });

  it("rounds the range to whole percent", () => {
    expect(rangeLabel(16.645)).toBe("Usikkerhet (±17 %)");
    expect(yearRangeLabel({ mid: 0, low: 341580, high: 440593, range_pct: 35, backtest_error_pct: 35 })).toMatch(
      /^Usikkerhet \(342\s000–441\s000\sUSD\)$/
    );
  });

  it("gives each chart a text summary", () => {
    render(<CostForecastCharts data={forecast} months={months} />);
    const [month, year] = screen
      .getAllByRole("img")
      .map((e) => document.getElementById(e.getAttribute("aria-labelledby")!)!.textContent);
    expect(month).toMatch(/Forventet faktura ved månedsslutt er 200\sUSD, mellom 170\sUSD og 230\sUSD/);
    expect(month).toMatch(/Kredittene i lisensene brukes etter prognosen opp 3\. oktober\./);
    expect(year).toMatch(/Fakturert 350\sUSD fra januar til og med september/);
    expect(year).toMatch(/Forventet ved årets slutt er 550\sUSD, mellom 520\sUSD og 580\sUSD/);
  });
});
