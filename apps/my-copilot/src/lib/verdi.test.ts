import { describe, expect, it } from "vitest";
import { buildVerdiMonths, buildVerdiRepos, buildVerdiTeams } from "./verdi";
import type {
  AdminMonthlyOverview,
  BillingModelBreakdown,
  EnterpriseMetrics,
  RepositoryUsage,
  TeamNetOverview,
  VolumeMonth,
} from "./types";

describe("verdi", () => {
  it("builds monthly cost-per-outcome, review cost, and context ratio series", () => {
    const admin: AdminMonthlyOverview = {
      year: 2026,
      coverage: {
        membership_from: "2025-11-01",
        last_gross_day: "2026-02-28",
        gross_stale: false,
        last_net_month: "2026-02",
        seats_snapshot: "2026-02-28",
        seats_from: "2026-01-01",
      },
      months: [
        {
          month: "2026-02",
          basis: "net",
          enterprise_net_usd: 2000,
          assigned_net_usd: 1900,
          residual_net_usd: 100,
          residual_pct: 5,
          no_team_net_usd: 0,
          gross_usd: 4000,
          estimated_net_usd: 2000,
          seats: 400,
          licence_usd_estimate: 7600,
          total_usd: 9600,
          loaded_at: "2026-03-01T00:00:00Z",
        },
      ],
    };

    const breakdown: BillingModelBreakdown[] = [
      {
        year_month: "2026-02",
        model: "Coding Agent model",
        gross_amount: 400,
        net_amount: 300,
        pct_of_monthly_net: 15,
      },
      {
        year_month: "2026-02",
        model: "Code Review model",
        gross_amount: 200,
        net_amount: 100,
        pct_of_monthly_net: 5,
      },
    ];

    const usage = [
      {
        day: "2026-02-10",
        pull_requests: {
          total_created: 500,
          total_merged: 400,
          total_created_by_copilot: 80,
          total_merged_created_by_copilot: 50,
          total_reviewed_by_copilot: 200,
          total_merged_reviewed_by_copilot: 180,
          total_copilot_suggestions: 400,
          total_copilot_applied_suggestions: 120,
          median_minutes_to_merge: 240,
          median_minutes_to_merge_copilot_authored: 180,
          median_minutes_to_merge_copilot_reviewed: 200,
        },
      },
    ] as unknown as EnterpriseMetrics[];

    const volume: VolumeMonth[] = [
      {
        month: "2026-02",
        ai_credits: 50000,
        premium_requests: 50000,
        cli_input_tokens: 1_200_000,
        cli_output_tokens: 10_000,
        app_input_tokens: 450_000,
        app_output_tokens: 15_000,
      },
    ];

    const months = buildVerdiMonths({ admin, breakdown, usage, volume });
    expect(months).toHaveLength(1);

    const feb = months[0];
    expect(feb.month).toBe("2026-02");
    expect(feb.netUsd).toBe(2000);
    expect(feb.codeReviewNetUsd).toBe(100);
    expect(feb.codeReviewSharePct).toBe(5);
    expect(feb.usdPerMergedPR).toBe(5);
    expect(feb.usdPerAgentMergedPR).toBe(40);
    expect(feb.usdPerReviewGrossScope).toBe(10);
    expect(feb.usdPerReviewDirect).toBe(0.5);
    expect(feb.agentMergeSharePct).toBe(62.5);
    expect(feb.reviewAppliedSharePct).toBe(30);
    expect(feb.reviewMergedSharePct).toBe(90);
    expect(feb.cliRatio).toBe(120);
    expect(feb.appRatio).toBe(30);
  });

  it("enforces k >= 5 pr_total_created on repo breakdown", () => {
    const repositories: RepositoryUsage[] = [
      {
        repo_id: 1,
        repo_owner: "navikt",
        repo_name: "active-repo",
        repo_visibility: "INTERNAL",
        scope_id: "navikt",
        days_with_data: 30,
        first_day: "2026-01-01",
        last_day: "2026-02-28",
        pr_total_created: 20,
        pr_total_merged: 18,
        pr_total_reviewed: 15,
        pr_created_by_copilot: 6,
        pr_merged_copilot_authored: 5,
        pr_reviewed_by_copilot: 10,
        pr_merged_copilot_reviewed: 9,
        pr_copilot_suggestions: 20,
        pr_copilot_applied_suggestions: 10,
        pr_avg_median_minutes_to_merge: 120,
        pr_avg_median_minutes_to_merge_copilot: 90,
        pr_avg_median_minutes_to_merge_copilot_reviewed: 95,
      },
      {
        repo_id: 2,
        repo_owner: "navikt",
        repo_name: "tiny-repo",
        repo_visibility: "INTERNAL",
        scope_id: "navikt",
        days_with_data: 10,
        first_day: "2026-02-01",
        last_day: "2026-02-28",
        pr_total_created: 4,
        pr_total_merged: 4,
        pr_total_reviewed: 3,
        pr_created_by_copilot: 2,
        pr_merged_copilot_authored: 2,
        pr_reviewed_by_copilot: 2,
        pr_merged_copilot_reviewed: 2,
        pr_copilot_suggestions: 4,
        pr_copilot_applied_suggestions: 2,
        pr_avg_median_minutes_to_merge: 60,
        pr_avg_median_minutes_to_merge_copilot: 45,
        pr_avg_median_minutes_to_merge_copilot_reviewed: 50,
      },
    ];

    const repos = buildVerdiRepos(repositories);
    expect(repos).toHaveLength(1);
    expect(repos[0].repoName).toBe("navikt/active-repo");
    expect(repos[0].copilotMergedTotal).toBe(14);
    expect(repos[0].suggestionApplyPct).toBe(50);
  });

  it("enforces k >= 5 users on team breakdown", () => {
    const net: TeamNetOverview = {
      month: "2026-02",
      small_teams: 3,
      loaded_at: "2026-03-01T00:00:00Z",
      comparison: "ok",
      teams: [
        {
          team_id: "t1",
          team_slug: "team-large",
          users: 8,
          amount_usd: 400,
          per_user_usd: 50,
          change_usd: 10,
          highlight: false,
        },
        {
          team_id: "t2",
          team_slug: "team-small",
          users: 4,
          amount_usd: 200,
          per_user_usd: 50,
          change_usd: 0,
          highlight: false,
        },
      ],
    };

    const summary = buildVerdiTeams(net, null);
    expect(summary).not.toBeNull();
    expect(summary!.teams).toHaveLength(1);
    expect(summary!.teams[0].team_slug).toBe("team-large");
    expect(summary!.smallTeams).toBe(3);
  });
});
