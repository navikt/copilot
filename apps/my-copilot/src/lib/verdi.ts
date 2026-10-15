import { modelFamily } from "./model-family";
import type {
  AdminMonthlyOverview,
  BillingModelBreakdown,
  EnterpriseMetrics,
  RepositoryUsage,
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
  /** V2 (#1783): total net USD divided by PRs reviewed by Copilot (coarse enterprise ratio). */
  usdPerReviewGrossScope: number | null;
  /** V2 (#1783) & V8 (#1789): direct code-review-model net USD divided by PRs reviewed by Copilot. */
  usdPerReviewDirect: number | null;
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
  copilotMergedTotal: number;
  suggestionApplyPct: number | null;
  avgMedianMinutesToMergeCopilot: number | null;
}

export interface VerdiTeamSummary {
  month: string;
  basis: "net" | "gross";
  teams: TeamSpend[];
  smallTeams: number;
}

const usd = (v: number) => Math.round(v * 100) / 100;
const divUsd = (amount: number | null, count: number) =>
  amount !== null && amount >= 0 && count > 0 ? usd(amount / count) : null;
const pct = (part: number, whole: number) => (whole > 0 ? Math.round((part / whole) * 1000) / 10 : null);
const avg = (vals: number[]) => (vals.length > 0 ? Math.round(vals.reduce((a, b) => a + b, 0) / vals.length) : null);

export function buildVerdiMonths({
  admin,
  breakdown,
  usage,
  volume,
}: {
  admin: AdminMonthlyOverview | null;
  breakdown: BillingModelBreakdown[] | null;
  usage: EnterpriseMetrics[] | null;
  volume: VolumeMonth[];
}): VerdiMonth[] {
  const adminByMonth = new Map((admin?.months ?? []).filter((m) => m.basis !== "none").map((m) => [m.month, m]));
  const volumeByMonth = new Map(volume.map((v) => [v.month, v]));

  const breakdownMonths = new Set<string>();
  const codeReviewByMonth = new Map<string, number>();
  for (const row of breakdown ?? []) {
    const m = row.year_month.slice(0, 7);
    breakdownMonths.add(m);
    if (modelFamily(row.model, true) === "kodegjennomgang") {
      codeReviewByMonth.set(m, (codeReviewByMonth.get(m) ?? 0) + row.net_amount);
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
    medianAll: number[];
    medianAgent: number[];
    medianReviewed: number[];
  };
  const prByMonth = new Map<string, PRAcc>();
  for (const d of usage ?? []) {
    const pr = d.pull_requests;
    if (!pr) continue;
    const m = d.day.slice(0, 7);
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
    if (pr.median_minutes_to_merge > 0) acc.medianAll.push(pr.median_minutes_to_merge);
    if (pr.median_minutes_to_merge_copilot_authored > 0)
      acc.medianAgent.push(pr.median_minutes_to_merge_copilot_authored);
    if (pr.median_minutes_to_merge_copilot_reviewed > 0)
      acc.medianReviewed.push(pr.median_minutes_to_merge_copilot_reviewed);
    prByMonth.set(m, acc);
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
        avgMedianMinutesToMerge: avg(pr?.medianAll ?? []),
        avgMedianMinutesToMergeCopilot: avg(pr?.medianAgent ?? []),
        avgMedianMinutesToMergeReviewed: avg(pr?.medianReviewed ?? []),
        usdPerMergedPR: divUsd(netUsd, prTotalMerged),
        usdPerAgentMergedPR: divUsd(netUsd, prMergedCopilotAuthored),
        usdPerReviewGrossScope: divUsd(netUsd, prReviewedByCopilot),
        usdPerReviewDirect: divUsd(codeReviewNetUsd, prReviewedByCopilot),
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
      copilotMergedTotal: r.pr_merged_copilot_authored + r.pr_merged_copilot_reviewed,
      suggestionApplyPct: pct(r.pr_copilot_applied_suggestions, r.pr_copilot_suggestions),
      avgMedianMinutesToMergeCopilot:
        r.pr_avg_median_minutes_to_merge_copilot != null ? Math.round(r.pr_avg_median_minutes_to_merge_copilot) : null,
    }))
    .sort((a, b) => b.copilotMergedTotal - a.copilotMergedTotal || b.totalMerged - a.totalMerged);
}

export function buildVerdiTeams(net: TeamNetOverview | null, gross: TeamGrossOverview | null): VerdiTeamSummary | null {
  const source = net ?? gross;
  if (!source) return null;
  const teams = source.teams
    .filter((t) => t.users >= MIN_VERDI_GROUP)
    .slice()
    .sort((a, b) => b.amount_usd - a.amount_usd);
  return {
    month: source.month,
    basis: net ? "net" : "gross",
    teams,
    smallTeams: source.small_teams,
  };
}
