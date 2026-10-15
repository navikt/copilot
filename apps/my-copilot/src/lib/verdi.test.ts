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
    expect(feb.cloudAgentNetUsd).toBe(300);
    expect(feb.cloudAgentSharePct).toBe(15);
    expect(feb.codeReviewNetUsd).toBe(100);
    expect(feb.codeReviewSharePct).toBe(5);
    expect(feb.usdPerMergedPR).toBe(5);
    expect(feb.usdPerAgentMergedPR).toBe(40);
    expect(feb.usdPerAgentMergedPRDirect).toBe(6);
    expect(feb.usdPerReviewGrossScope).toBe(10);
    expect(feb.usdPerReviewDirect).toBe(0.5);
    expect(feb.usdPerAppliedSuggestion).toBe(0.83);
    expect(feb.agentMergeSharePct).toBe(62.5);
    expect(feb.reviewAppliedSharePct).toBe(30);
    expect(feb.reviewMergedSharePct).toBe(90);
    expect(feb.cliRatio).toBe(120);
    expect(feb.appRatio).toBe(30);
  });

  it("prefers enterprise-scope usage over public/internal copilotPRs and weights daily medians by merged PR count", () => {
    const months = buildVerdiMonths({
      admin: null,
      breakdown: [
        { year_month: "2026-01", model: "Code Review model", gross_amount: 120, net_amount: 60, pct_of_monthly_net: 6 },
        { year_month: "2026-02", model: "Code Review model", gross_amount: 120, net_amount: 60, pct_of_monthly_net: 6 },
      ],
      usage: [
        {
          day: "2026-02-10",
          pull_requests: {
            total_created: 180,
            total_merged: 90,
            total_created_by_copilot: 30,
            total_merged_created_by_copilot: 9,
            total_reviewed_by_copilot: 80,
            total_merged_reviewed_by_copilot: 70,
            total_copilot_suggestions: 120,
            total_copilot_applied_suggestions: 50,
            median_minutes_to_merge: 100,
            median_minutes_to_merge_copilot_authored: 80,
            median_minutes_to_merge_copilot_reviewed: 90,
          },
        },
        {
          day: "2026-02-11",
          pull_requests: {
            total_created: 20,
            total_merged: 10,
            total_created_by_copilot: 10,
            total_merged_created_by_copilot: 1,
            total_reviewed_by_copilot: 20,
            total_merged_reviewed_by_copilot: 10,
            total_copilot_suggestions: 30,
            total_copilot_applied_suggestions: 10,
            median_minutes_to_merge: 200,
            median_minutes_to_merge_copilot_authored: 180,
            median_minutes_to_merge_copilot_reviewed: 170,
          },
        },
      ] as unknown as EnterpriseMetrics[],
      copilotPRs: [
        {
          month: "2026-01",
          total_created: 150,
          created_by_copilot: 25,
          reviewed_by_copilot: 60,
          total_merged: 120,
          merged_copilot_authored: 20,
          merged_copilot_reviewed: 50,
          copilot_suggestions: 100,
          copilot_applied_suggestions: 40,
          avg_median_minutes_to_merge: 140,
          avg_median_minutes_to_merge_copilot: 110,
          avg_median_minutes_to_merge_copilot_reviewed: 125,
          days: 31,
        },
        {
          month: "2026-02",
          total_created: 999,
          created_by_copilot: 999,
          reviewed_by_copilot: 999,
          total_merged: 999,
          merged_copilot_authored: 999,
          merged_copilot_reviewed: 999,
          copilot_suggestions: 999,
          copilot_applied_suggestions: 999,
          avg_median_minutes_to_merge: 999,
          avg_median_minutes_to_merge_copilot: 999,
          avg_median_minutes_to_merge_copilot_reviewed: 999,
          days: 28,
        },
      ],
      volume: [
        {
          month: "2026-02",
          ai_credits: 1000,
          premium_requests: 0,
          cloud_agent_net_usd: 25,
          cli_input_tokens: 0,
          cli_output_tokens: 0,
          app_input_tokens: 0,
          app_output_tokens: 0,
        },
      ],
    });

    expect(months).toHaveLength(2);
    // 2026-01 is absent from usage, so it falls back to copilotPRs
    expect(months[0].month).toBe("2026-01");
    expect(months[0].prTotalCreated).toBe(150);
    expect(months[0].avgMedianMinutesToMerge).toBe(140);

    // 2026-02 is present in enterprise usage, so it uses usage and PR-weights daily medians
    expect(months[1].month).toBe("2026-02");
    expect(months[1].prTotalCreated).toBe(200);
    expect(months[1].prTotalMerged).toBe(100);
    expect(months[1].cloudAgentNetUsd).toBe(25);
    expect(months[1].usdPerAgentMergedPRDirect).toBe(2.5);
    expect(months[1].usdPerReviewDirect).toBe(0.6);
    expect(months[1].usdPerAppliedSuggestion).toBe(1);
    // (100 * 90 + 200 * 10) / 100 = 110 (vs unweighted 150)
    expect(months[1].avgMedianMinutesToMerge).toBe(110);
    // (80 * 9 + 180 * 1) / 10 = 90 (vs unweighted 130)
    expect(months[1].avgMedianMinutesToMergeCopilot).toBe(90);
    // (90 * 70 + 170 * 10) / 80 = 100 (vs unweighted 130)
    expect(months[1].avgMedianMinutesToMergeReviewed).toBe(100);
  });

  it("preserves 0 USD unit costs when credits cover usage and returns null when breakdown is unavailable", () => {
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
          enterprise_net_usd: 0,
          assigned_net_usd: 0,
          residual_net_usd: 0,
          residual_pct: 0,
          no_team_net_usd: 0,
          gross_usd: 1500,
          estimated_net_usd: 0,
          seats: 400,
          licence_usd_estimate: 7600,
          total_usd: 7600,
          loaded_at: "2026-03-01T00:00:00Z",
        },
      ],
    };
    const usage = [
      {
        day: "2026-02-10",
        pull_requests: {
          total_created: 100,
          total_merged: 80,
          total_created_by_copilot: 20,
          total_merged_created_by_copilot: 10,
          total_reviewed_by_copilot: 50,
          total_merged_reviewed_by_copilot: 45,
          total_copilot_suggestions: 100,
          total_copilot_applied_suggestions: 30,
          median_minutes_to_merge: 120,
          median_minutes_to_merge_copilot_authored: 90,
          median_minutes_to_merge_copilot_reviewed: 100,
        },
      },
    ] as unknown as EnterpriseMetrics[];

    const [withZeroCost] = buildVerdiMonths({
      admin,
      breakdown: [
        { year_month: "2026-02", model: "Code Review model", gross_amount: 100, net_amount: 0, pct_of_monthly_net: 0 },
      ],
      usage,
      volume: [],
    });
    expect(withZeroCost.netUsd).toBe(0);
    expect(withZeroCost.usdPerMergedPR).toBe(0);
    expect(withZeroCost.usdPerAgentMergedPR).toBe(0);
    expect(withZeroCost.codeReviewNetUsd).toBe(0);
    expect(withZeroCost.usdPerReviewDirect).toBe(0);
    expect(withZeroCost.usdPerAppliedSuggestion).toBe(0);

    const [withoutBreakdown] = buildVerdiMonths({
      admin,
      breakdown: null,
      usage,
      volume: [],
    });
    expect(withoutBreakdown.codeReviewNetUsd).toBeNull();
    expect(withoutBreakdown.codeReviewSharePct).toBeNull();
    expect(withoutBreakdown.usdPerReviewDirect).toBeNull();
    expect(withoutBreakdown.usdPerAppliedSuggestion).toBeNull();
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
    expect(repos[0].mergedCopilotAuthored).toBe(5);
    expect(repos[0].mergedCopilotReviewed).toBe(9);
    expect(repos[0].suggestionApplyPct).toBe(50);
    expect(repos[0].avgMedianMinutesToMerge).toBe(120);
    expect(repos[0].avgMedianMinutesToMergeCopilot).toBe(90);
    expect(repos[0].avgMedianMinutesToMergeReviewed).toBe(95);
  });

  it("enforces k >= 5 users on team breakdown and joins team adoption", () => {
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

    const summary = buildVerdiTeams(net, null, {
      total_teams: 2,
      teams_with_adoption: 1,
      adoption_pct: 50,
      small_teams: 1,
      teams: [
        {
          team_slug: "team-large",
          team_name: "Team Large",
          active_repos: 10,
          recently_active_repos: 8,
          repos_with_customizations: 7,
          adoption_pct: 70,
          adoption_active_pct: 88,
          prs_merged_30d: 24,
          prs_reviewed_by_copilot_30d: 15,
          avg_median_minutes_to_merge_30d: 112.4,
          commits_30d: 90,
          open_issues_count: 14,
        },
      ],
    });
    expect(summary).not.toBeNull();
    expect(summary!.teams).toHaveLength(1);
    expect(summary!.teams[0].team_slug).toBe("team-large");
    expect(summary!.teams[0].activeRepos).toBe(10);
    expect(summary!.teams[0].reposWithCustomizations).toBe(7);
    expect(summary!.teams[0].adoptionPct).toBe(70);
    expect(summary!.teams[0].prsMerged30d).toBe(24);
    expect(summary!.teams[0].prsReviewedByCopilot30d).toBe(15);
    expect(summary!.teams[0].avgMedianMinutesToMerge30d).toBe(112);
    expect(summary!.teams[0].commits30d).toBe(90);
    expect(summary!.teams[0].openIssuesCount).toBe(14);
    expect(summary!.smallTeams).toBe(3);
  });
});
