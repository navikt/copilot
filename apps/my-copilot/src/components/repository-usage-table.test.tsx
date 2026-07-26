import { render, screen, within } from "@testing-library/react";
import RepositoryUsageTable from "./repository-usage-table";
import type { RepositoryUsage } from "@/lib/types";

function makeRepo(overrides: Partial<RepositoryUsage>): RepositoryUsage {
  return {
    repo_id: 1,
    repo_owner: "navikt",
    repo_name: "foo",
    repo_visibility: "PUBLIC",
    scope_id: "scope",
    days_with_data: 10,
    first_day: "2026-01-01",
    last_day: "2026-01-10",
    pr_total_created: 20,
    pr_total_merged: 15,
    pr_total_reviewed: 12,
    pr_created_by_copilot: 5,
    pr_reviewed_by_copilot: 4,
    pr_merged_copilot_authored: 3,
    pr_merged_copilot_reviewed: 2,
    pr_copilot_suggestions: 30,
    pr_copilot_applied_suggestions: 18,
    pr_avg_median_minutes_to_merge: 192,
    pr_avg_median_minutes_to_merge_copilot: 100,
    pr_avg_median_minutes_to_merge_copilot_reviewed: 80,
    ...overrides,
  };
}

describe("RepositoryUsageTable", () => {
  it("renders repository owner/name and a GitHub link", () => {
    render(<RepositoryUsageTable repositories={[makeRepo({ repo_id: 1 })]} />);
    const link = screen.getByRole("link", { name: "navikt/foo" });
    expect(link).toHaveAttribute("href", "https://github.com/navikt/foo");
  });

  it("formats the nullable median as an en-dash when null", () => {
    render(
      <RepositoryUsageTable
        repositories={[makeRepo({ repo_id: 1, repo_name: "nomedian", pr_avg_median_minutes_to_merge: null })]}
      />
    );
    const row = screen.getByRole("link", { name: "navikt/nomedian" }).closest("tr")!;
    expect(within(row).getByText("–")).toBeInTheDocument();
  });

  it("formats a non-null median duration", () => {
    render(<RepositoryUsageTable repositories={[makeRepo({ pr_avg_median_minutes_to_merge: 192 })]} />);
    expect(screen.getByText("3t 12m")).toBeInTheDocument();
  });

  it("defaults to sorting by Copilot-authored PRs descending", () => {
    render(
      <RepositoryUsageTable
        repositories={[
          makeRepo({ repo_id: 1, repo_name: "low", pr_created_by_copilot: 2 }),
          makeRepo({ repo_id: 2, repo_name: "high", pr_created_by_copilot: 99 }),
        ]}
      />
    );
    const links = screen.getAllByRole("link").map((a) => a.textContent);
    expect(links).toEqual(["navikt/high", "navikt/low"]);
  });

  it("shows an empty state when there are no repositories", () => {
    render(<RepositoryUsageTable repositories={[]} />);
    expect(screen.getByText("Ingen repositoriedata tilgjengelig ennå.")).toBeInTheDocument();
  });
});
