import { render, screen } from "@testing-library/react";
import { getMonthlyTrends } from "@/lib/cached-bigquery";
import type { MonthlyTrend } from "@/lib/types";
import UsagePage from "./page";

vi.mock("@/lib/auth", () => ({ getUser: vi.fn(), getUserToken: async () => "test-token" }));
vi.mock("@/lib/cached-bigquery", () => ({
  getMonthlyTrends: vi.fn(),
  getDailySummary: vi.fn(),
  getCopilotUsageMetrics: vi.fn(),
}));
vi.mock("react-chartjs-2", () => ({
  Bar: ({ data }: { data: { labels: string[] } }) => <div>{data.labels.join(", ")}</div>,
}));

const partial: MonthlyTrend = {
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

afterEach(() => vi.useRealTimers());

it("keeps incomplete history visible through KeyFigures without presenting complete-month figures", async () => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-11-02T00:00:00Z"));
  vi.mocked(getMonthlyTrends).mockResolvedValue({
    trends: [partial, { ...partial, month: "2026-10", unique_users: 120 }],
    error: null,
  });

  const page = await UsagePage();
  const section = page.props.children[0];
  expect(section.props.id).toBe("nokkeltall");
  // Resolve the page's actual async Suspense child, not a separately imported chart.
  const keyFigures = section.props.children.props.children;
  render(await keyFigures.type(keyFigures.props));

  expect(getMonthlyTrends).toHaveBeenCalledWith("test-token");
  expect(screen.getByText("Ingen hel måned med data ennå.")).toBeVisible();
  expect(screen.getByText(/Måneder merket med \* mangler data/)).toBeVisible();
  expect(screen.getByRole("heading", { name: "Brukere per funksjon" })).toBeVisible();
  expect(screen.getByRole("heading", { name: "Aktivitet per type" })).toBeVisible();
  expect(screen.getAllByText("2026-09 *, 2026-10 *")).toHaveLength(2);
  expect(screen.queryByText("Linjer lagt til")).not.toBeInTheDocument();
  expect(screen.queryByText("CLI-brukere")).not.toBeInTheDocument();
  expect(screen.queryByText("Aktive brukere")).not.toBeInTheDocument();
  expect(screen.queryByText(/fra forrige måned$|^Uendret$/)).not.toBeInTheDocument();
});
