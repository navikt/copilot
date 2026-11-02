import { render, screen } from "@testing-library/react";
import type { MonthlyTrend } from "@/lib/types";
import MonthlyTrendsChart from "./MonthlyTrendsChart";

vi.mock("react-chartjs-2", () => ({
  Bar: ({ data }: { data: { labels: string[] } }) => <div>{data.labels.join(", ")}</div>,
}));

const month: MonthlyTrend = {
  month: "2026-09",
  days_in_month: 15,
  unique_users: 100,
  agent_users: 50,
  chat_users: 60,
  cli_users: 40,
  code_generations: 200,
  ide_interactions: 80,
  cli_requests: 90,
  prompt_tokens: 0,
  output_tokens: 0,
  lines_added: 300,
  lines_deleted: 0,
  acceptances: 0,
};

describe("MonthlyTrendsChart", () => {
  afterEach(() => vi.useRealTimers());

  it("keeps all-partial history visible with textual values and incomplete-month markers", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-11-02T00:00:00Z"));
    render(<MonthlyTrendsChart data={[month, { ...month, month: "2026-10", unique_users: 120 }]} />);

    expect(screen.getAllByText("2026-09 *, 2026-10 *")).toHaveLength(2);
    const descriptions = screen
      .getAllByRole("img")
      .map((chart) => document.getElementById(chart.getAttribute("aria-labelledby")!)!.textContent);
    expect(descriptions[0]).toContain("2026-10 *: 120 brukere");
    expect(descriptions[1]).toContain("2026-09 *: 200 kodeforslag");
    expect(screen.getByText(/Måneder merket med \* mangler data/)).toBeInTheDocument();
    expect(screen.queryByText(/^[+−-]?\d+.*fra forrige måned$/)).not.toBeInTheDocument();
  });
});
