import { modelFamily } from "./model-family";
import type {
  AdminMonthlyOverview,
  BillingModelBreakdown,
  CopilotPRMonth,
  EnterpriseMetrics,
  RepositoryUsage,
  TeamAdoptionOverview,
  TeamGrossOverview,
  TeamNetOverview,
  TeamSpend,
  VolumeMonth,
} from "./types";

/** Groups smaller than five are never shown individually. */
export const MIN_VERDI_GROUP = 5;

export interface VerdiMonth {
  month: string;
  netUsd: number | null;
  preliminary: boolean;
  cloudAgentNetUsd: number | null;
  cloudAgentSharePct: number | null;
  codeReviewNetUsd: number | null;
  codeReviewSharePct: number | null;
  prTotalCreated: number;
  prTotalMerged: number;
  prCreatedByCopilot: number;
  prMergedCopilotAuthored: number;
  prReviewedByCopilot: number;
  prMergedCopilotReviewed: number;
  copilotSuggestions: number;
  copilotAppliedSuggestions: number;
  avgMedianMinutesToMerge: number | null;
  avgMedianMinutesToMergeCopilot: number | null;
  avgMedianMinutesToMergeReviewed: number | null;
  /** V2 (#1783): total net USD divided by all merged PRs (coarse enterprise ratio). */
  usdPerMergedPR: number | null;
  /** V2 (#1783) & V4 (#1785): total net USD divided by merged Copilot-authored PRs (coarse enterprise ratio). */
  usdPerAgentMergedPR: number | null;
  /** V2 (#1783) & V4 (#1785): direct Coding Agent SKU net USD divided by merged Copilot-authored PRs. */
  usdPerAgentMergedPRDirect: number | null;
  /** V2 (#1783): total net USD divided by PRs reviewed by Copilot (coarse enterprise ratio). */
  usdPerReviewGrossScope: number | null;
  /** V2 (#1783) & V8 (#1789): direct code-review-model net USD divided by PRs reviewed by Copilot. */
  usdPerReviewDirect: number | null;
  /** V2 (#1783) & V8 (#1789): direct code-review-model net USD divided by applied Copilot review suggestions. */
  usdPerAppliedSuggestion: number | null;
  /** V4 (#1785): monthly flow ratio of merged Copilot-authored PRs to Copilot-created PRs (may exceed 100% across month boundaries). */
  agentMergeSharePct: number | null;
  /** V8 (#1789): applied Copilot review suggestions as % of total Copilot suggestions. */
  reviewAppliedSharePct: number | null;
  /** V8 (#1789): monthly flow ratio of merged Copilot-reviewed PRs to PRs reviewed by Copilot. */
  reviewMergedSharePct: number | null;
  /** V3 (#1784): CLI prompt and output tokens and input:output ratio. */
  cliInputTokens: number;
  cliOutputTokens: number;
  cliRatio: number | null;
  /** V3 (#1784): Copilot app prompt and output tokens and input:output ratio. */
  appInputTokens: number;
  appOutputTokens: number;
  appRatio: number | null;
}

export interface VerdiRepoRow {
  repoName: string;
  totalCreated: number;
  totalMerged: number;
  createdByCopilot: number;
  mergedCopilotAuthored: number;
  reviewedByCopilot: number;
  mergedCopilotReviewed: number;
  suggestionApplyPct: number | null;
  avgMedianMinutesToMerge: number | null;
  avgMedianMinutesToMergeCopilot: number | null;
  avgMedianMinutesToMergeReviewed: number | null;
}

export interface VerdiTeamRow extends TeamSpend {
  activeRepos: number | null;
  reposWithCustomizations: number | null;
  adoptionPct: number | null;
}

export interface VerdiTeamSummary {
  month: string;
  basis: "net" | "gross";
  teams: VerdiTeamRow[];
  smallTeams: number;
}

type WeightedSample = { value: number; weight: number };

const usd = (v: number) => Math.round(v * 100) / 100;
const divUsd = (amount: number | null, count: number) =>
  amount !== null && amount >= 0 && count > 0 ? usd(amount / count) : null;
const pct = (part: number, whole: number) => (whole > 0 ? Math.round((part / whole) * 1000) / 10 : null);
const weightedAvg = (samples: WeightedSample[]) => {
  if (samples.length === 0) return null;
  let weightedSum = 0;
  let totalWeight = 0;
  for (const s of samples) {
    const w = s.weight > 0 ? s.weight : 1;
    weightedSum += s.value * w;
    totalWeight += w;
  }
  return totalWeight > 0 ? Math.round(weightedSum / totalWeight) : null;
};

export function buildVerdiMonths({
  admin,
  breakdown,
  usage,
  copilotPRs,
  volume,
}: {
  admin: AdminMonthlyOverview | null;
  breakdown: BillingModelBreakdown[] | null;
  usage: EnterpriseMetrics[] | null;
  copilotPRs?: CopilotPRMonth[] | null;
  volume: VolumeMonth[];
}): VerdiMonth[] {
  const adminByMonth = new Map((admin?.months ?? []).filter((m) => m.basis !== "none").map((m) => [m.month, m]));
  const volumeByMonth = new Map(volume.map((v) => [v.month, v]));

  const breakdownMonths = new Set<string>();
  const codeReviewByMonth = new Map<string, number>();
  const codingAgentModelByMonth = new Map<string, number>();
  for (const row of breakdown ?? []) {
    const m = row.year_month.slice(0, 7);
    breakdownMonths.add(m);
    if (modelFamily(row.model, true) === "kodegjennomgang") {
      codeReviewByMonth.set(m, (codeReviewByMonth.get(m) ?? 0) + row.net_amount);
    }
    if (row.model.toLowerCase().includes("coding agent")) {
      codingAgentModelByMonth.set(m, (codingAgentModelByMonth.get(m) ?? 0) + row.net_amount);
    }
  }

  type PRAcc = {
    totalCreated: number;
    totalMerged: number;
    createdByCopilot: number;
    mergedCopilotAuthored: number;
    reviewedByCopilot: number;
    mergedCopilotReviewed: number;
    copilotSuggestions: number;
    copilotAppliedSuggestions: number;
    medianAll: WeightedSample[];
    medianAgent: WeightedSample[];
    medianReviewed: WeightedSample[];
  };
  const prByMonth = new Map<string, PRAcc>();
  const usageMonths = new Set<string>();

  // Prefer enterprise-wide usage_metrics (includes private repos and true daily enterprise medians)
  // so PR denominators match the enterprise billing scope.
  for (const d of usage ?? []) {
    const pr = d.pull_requests;
    if (!pr) continue;
    const m = d.day.slice(0, 7);
    usageMonths.add(m);
    const acc = prByMonth.get(m) ?? {
      totalCreated: 0,
      totalMerged: 0,
      createdByCopilot: 0,
      mergedCopilotAuthored: 0,
      reviewedByCopilot: 0,
      mergedCopilotReviewed: 0,
      copilotSuggestions: 0,
      copilotAppliedSuggestions: 0,
      medianAll: [],
      medianAgent: [],
      medianReviewed: [],
    };
    acc.totalCreated += pr.total_created || 0;
    acc.totalMerged += pr.total_merged || 0;
    acc.createdByCopilot += pr.total_created_by_copilot || 0;
    acc.mergedCopilotAuthored += pr.total_merged_created_by_copilot || 0;
    acc.reviewedByCopilot += pr.total_reviewed_by_copilot || 0;
    acc.mergedCopilotReviewed += pr.total_merged_reviewed_by_copilot || 0;
    acc.copilotSuggestions += pr.total_copilot_suggestions || 0;
    acc.copilotAppliedSuggestions += pr.total_copilot_applied_suggestions || 0;
    if (pr.median_minutes_to_merge > 0) {
      acc.medianAll.push({ value: pr.median_minutes_to_merge, weight: pr.total_merged || 1 });
    }
    if (pr.median_minutes_to_merge_copilot_authored > 0) {
      acc.medianAgent.push({
        value: pr.median_minutes_to_merge_copilot_authored,
        weight: pr.total_merged_created_by_copilot || 1,
      });
    }
    if (pr.median_minutes_to_merge_copilot_reviewed > 0) {
      acc.medianReviewed.push({
        value: pr.median_minutes_to_merge_copilot_reviewed,
        weight: pr.total_merged_reviewed_by_copilot || 1,
      });
    }
    prByMonth.set(m, acc);
  }

  // Fall back to repository_metrics rollups only for months absent from enterprise usage_metrics.
  for (const row of copilotPRs ?? []) {
    const m = row.month.slice(0, 7);
    if (usageMonths.has(m)) continue;
    prByMonth.set(m, {
      totalCreated: row.total_created ?? row.created_by_copilot + row.reviewed_by_copilot,
      totalMerged: row.total_merged ?? 0,
      createdByCopilot: row.created_by_copilot ?? 0,
      mergedCopilotAuthored: row.merged_copilot_authored ?? 0,
      reviewedByCopilot: row.reviewed_by_copilot ?? 0,
      mergedCopilotReviewed: row.merged_copilot_reviewed ?? 0,
      copilotSuggestions: row.copilot_suggestions ?? 0,
      copilotAppliedSuggestions: row.copilot_applied_suggestions ?? 0,
      medianAll:
        row.avg_median_minutes_to_merge != null && row.avg_median_minutes_to_merge > 0
          ? [{ value: row.avg_median_minutes_to_merge, weight: 1 }]
          : [],
      medianAgent:
        row.avg_median_minutes_to_merge_copilot != null && row.avg_median_minutes_to_merge_copilot > 0
          ? [{ value: row.avg_median_minutes_to_merge_copilot, weight: 1 }]
          : [],
      medianReviewed:
        row.avg_median_minutes_to_merge_copilot_reviewed != null && row.avg_median_minutes_to_merge_copilot_reviewed > 0
          ? [{ value: row.avg_median_minutes_to_merge_copilot_reviewed, weight: 1 }]
          : [],
    });
  }

  const allMonths = [
    ...new Set([...adminByMonth.keys(), ...volumeByMonth.keys(), ...prByMonth.keys(), ...breakdownMonths]),
  ].sort();

  return allMonths
    .map((month): VerdiMonth => {
      const adm = adminByMonth.get(month);
      const netUsd = adm ? (adm.enterprise_net_usd ?? adm.estimated_net_usd ?? null) : null;
      const preliminary = adm?.basis === "gross";
      const hasBreakdownForMonth = breakdown !== null && breakdownMonths.has(month);
      const codeReviewNetUsd = hasBreakdownForMonth ? usd(codeReviewByMonth.get(month) ?? 0) : null;
      const pr = prByMonth.get(month);
      const vol = volumeByMonth.get(month);
      const cloudAgentNetUsd =
        vol?.cloud_agent_net_usd != null
          ? usd(vol.cloud_agent_net_usd)
          : hasBreakdownForMonth && codingAgentModelByMonth.has(month)
            ? usd(codingAgentModelByMonth.get(month) ?? 0)
            : null;

      const prTotalCreated = pr?.totalCreated ?? 0;
      const prTotalMerged = pr?.totalMerged ?? 0;
      const prCreatedByCopilot = pr?.createdByCopilot ?? 0;
      const prMergedCopilotAuthored = pr?.mergedCopilotAuthored ?? 0;
      const prReviewedByCopilot = pr?.reviewedByCopilot ?? 0;
      const prMergedCopilotReviewed = pr?.mergedCopilotReviewed ?? 0;
      const copilotSuggestions = pr?.copilotSuggestions ?? 0;
      const copilotAppliedSuggestions = pr?.copilotAppliedSuggestions ?? 0;

      const cliInputTokens = vol?.cli_input_tokens ?? 0;
      const cliOutputTokens = vol?.cli_output_tokens ?? 0;
      const appInputTokens = vol?.app_input_tokens ?? 0;
      const appOutputTokens = vol?.app_output_tokens ?? 0;

      return {
        month,
        netUsd,
        preliminary,
        cloudAgentNetUsd,
        cloudAgentSharePct:
          cloudAgentNetUsd !== null && netUsd !== null && netUsd > 0 ? pct(cloudAgentNetUsd, netUsd) : null,
        codeReviewNetUsd,
        codeReviewSharePct:
          codeReviewNetUsd !== null && netUsd !== null && netUsd > 0 ? pct(codeReviewNetUsd, netUsd) : null,
        prTotalCreated,
        prTotalMerged,
        prCreatedByCopilot,
        prMergedCopilotAuthored,
        prReviewedByCopilot,
        prMergedCopilotReviewed,
        copilotSuggestions,
        copilotAppliedSuggestions,
        avgMedianMinutesToMerge: weightedAvg(pr?.medianAll ?? []),
        avgMedianMinutesToMergeCopilot: weightedAvg(pr?.medianAgent ?? []),
        avgMedianMinutesToMergeReviewed: weightedAvg(pr?.medianReviewed ?? []),
        usdPerMergedPR: divUsd(netUsd, prTotalMerged),
        usdPerAgentMergedPR: divUsd(netUsd, prMergedCopilotAuthored),
        usdPerAgentMergedPRDirect: divUsd(cloudAgentNetUsd, prMergedCopilotAuthored),
        usdPerReviewGrossScope: divUsd(netUsd, prReviewedByCopilot),
        usdPerReviewDirect: divUsd(codeReviewNetUsd, prReviewedByCopilot),
        usdPerAppliedSuggestion: divUsd(codeReviewNetUsd, copilotAppliedSuggestions),
        agentMergeSharePct: pct(prMergedCopilotAuthored, prCreatedByCopilot),
        reviewAppliedSharePct: pct(copilotAppliedSuggestions, copilotSuggestions),
        reviewMergedSharePct: pct(prMergedCopilotReviewed, prReviewedByCopilot),
        cliInputTokens,
        cliOutputTokens,
        cliRatio: cliOutputTokens > 0 ? Math.round(cliInputTokens / cliOutputTokens) : null,
        appInputTokens,
        appOutputTokens,
        appRatio: appOutputTokens > 0 ? Math.round(appInputTokens / appOutputTokens) : null,
      };
    })
    .filter((m) => m.netUsd !== null || m.prTotalCreated > 0 || m.cliInputTokens > 0 || m.appInputTokens > 0);
}

export function buildVerdiRepos(repositories: RepositoryUsage[]): VerdiRepoRow[] {
  return repositories
    .filter((r) => r.pr_total_created >= MIN_VERDI_GROUP)
    .map((r) => ({
      repoName: `${r.repo_owner}/${r.repo_name}`,
      totalCreated: r.pr_total_created,
      totalMerged: r.pr_total_merged,
      createdByCopilot: r.pr_created_by_copilot,
      mergedCopilotAuthored: r.pr_merged_copilot_authored,
      reviewedByCopilot: r.pr_reviewed_by_copilot,
      mergedCopilotReviewed: r.pr_merged_copilot_reviewed,
      suggestionApplyPct: pct(r.pr_copilot_applied_suggestions, r.pr_copilot_suggestions),
      avgMedianMinutesToMerge:
        r.pr_avg_median_minutes_to_merge != null ? Math.round(r.pr_avg_median_minutes_to_merge) : null,
      avgMedianMinutesToMergeCopilot:
        r.pr_avg_median_minutes_to_merge_copilot != null ? Math.round(r.pr_avg_median_minutes_to_merge_copilot) : null,
      avgMedianMinutesToMergeReviewed:
        r.pr_avg_median_minutes_to_merge_copilot_reviewed != null
          ? Math.round(r.pr_avg_median_minutes_to_merge_copilot_reviewed)
          : null,
    }))
    .sort(
      (a, b) =>
        b.mergedCopilotAuthored + b.mergedCopilotReviewed - (a.mergedCopilotAuthored + a.mergedCopilotReviewed) ||
        b.totalMerged - a.totalMerged
    );
}

export function buildVerdiTeams(
  net: TeamNetOverview | null,
  gross: TeamGrossOverview | null,
  adoption?: TeamAdoptionOverview | null
): VerdiTeamSummary | null {
  const source = net ?? gross;
  if (!source) return null;
  const adoptionBySlug = new Map(
    (adoption?.teams ?? []).filter((t) => t.active_repos >= MIN_VERDI_GROUP).map((t) => [t.team_slug, t])
  );
  const teams = source.teams
    .filter((t) => t.users >= MIN_VERDI_GROUP)
    .map((t): VerdiTeamRow => {
      const ad = adoptionBySlug.get(t.team_slug);
      return {
        ...t,
        activeRepos: ad ? ad.active_repos : null,
        reposWithCustomizations: ad ? ad.repos_with_customizations : null,
        adoptionPct: ad ? ad.adoption_pct : null,
      };
    })
    .sort((a, b) => b.amount_usd - a.amount_usd);
  return {
    month: source.month,
    basis: net ? "net" : "gross",
    teams,
    smallTeams: source.small_teams,
  };
}
