/**
 * Data transformation utilities for adoption metrics.
 *
 * These pure functions transform raw adoption data into chart-ready formats.
 * All functions are side-effect free and easily testable.
 */

import type { AdoptionSummary, LanguageAdoption, TeamAdoption, CustomizationDetail, AdoptionScope } from "./types";

/**
 * Customization type with label, count and group.
 */
export interface CustomizationType {
  label: string;
  value: number;
  key: string;
  group?: "copilot" | "agentic" | "nav-pilot";
}

/**
 * AI tool usage summary.
 */
export interface ToolUsage {
  label: string;
  value: number;
}

/**
 * Extract Copilot customization types from summary data.
 * Returns sorted array (highest value first).
 */
export function extractCustomizationTypes(summary: AdoptionSummary): CustomizationType[] {
  const types: CustomizationType[] = [
    {
      key: "copilot_instructions",
      label: "copilot-instructions.md",
      value: summary.repos_with_copilot_instructions,
      group: "copilot",
    },
    { key: "agents_md", label: "AGENTS.md", value: summary.repos_with_agents_md, group: "copilot" },
    { key: "agents", label: ".github/agents/", value: summary.repos_with_agents, group: "copilot" },
    { key: "instructions", label: ".github/instructions/", value: summary.repos_with_instructions, group: "copilot" },
    { key: "prompts", label: ".github/prompts/", value: summary.repos_with_prompts, group: "copilot" },
    { key: "skills", label: ".github/skills/", value: summary.repos_with_skills, group: "copilot" },
    { key: "mcp_config", label: "mcp.json", value: summary.repos_with_mcp_config, group: "copilot" },
    { key: "copilot_dir", label: ".copilot/", value: summary.repos_with_copilot_dir, group: "copilot" },
    {
      key: "copilot_setup_steps",
      label: "copilot-setup-steps.yml",
      value: summary.repos_with_copilot_setup_steps,
      group: "agentic",
    },
    { key: "agentic_workflows", label: ".github/aw/", value: summary.repos_with_agentic_workflows, group: "agentic" },
    { key: "agents_skills", label: ".agents/skills/", value: summary.repos_with_agents_skills, group: "agentic" },
    {
      key: "nav_pilot_state",
      label: "nav-pilot-state.json",
      value: summary.repos_with_nav_pilot_state,
      group: "nav-pilot",
    },
  ];

  return types.sort((a, b) => b.value - a.value);
}

/**
 * Extract AI tool comparison data from summary.
 * Returns non-zero values only, for cleaner charts.
 */
export function extractToolComparison(summary: AdoptionSummary): ToolUsage[] {
  // Copilot-only = repos with any customization minus those with non-Copilot tools
  const copilotOnly = Math.max(0, summary.repos_with_any_customization - summary.repos_with_any_non_copilot_ai);

  const tools: ToolUsage[] = [
    { label: "Kun Copilot", value: copilotOnly },
    { label: "Cursor", value: summary.repos_with_cursorrules + summary.repos_with_cursor_rules_dir },
    { label: "Claude", value: summary.repos_with_claude_md },
    { label: "Windsurf", value: summary.repos_with_windsurfrules },
  ];

  return tools.filter((t) => t.value > 0);
}

/**
 * Filter teams to only those with active repositories.
 */
export function filterActiveTeams(teams: TeamAdoption[]): TeamAdoption[] {
  return teams.filter((t) => t.active_repos > 0);
}

/**
 * Filter teams to only those with at least one repo with customizations.
 */
export function filterTeamsWithAdoption(teams: TeamAdoption[]): TeamAdoption[] {
  return teams.filter((t) => t.repos_with_customizations > 0);
}

/**
 * Get top N teams by number of repos with customizations.
 */
export function getTopTeams(teams: TeamAdoption[], maxTeams: number): TeamAdoption[] {
  return filterTeamsWithAdoption(teams)
    .sort((a, b) => b.repos_with_customizations - a.repos_with_customizations)
    .slice(0, maxTeams);
}

/**
 * Sort teams by repos with customizations (descending).
 */
export function sortTeamsByAdoption(teams: TeamAdoption[]): TeamAdoption[] {
  return [...teams].sort((a, b) => b.repos_with_customizations - a.repos_with_customizations);
}

/**
 * Get top N languages by adoption rate.
 * Only includes languages that have at least one repo with customizations.
 */
export function getTopLanguagesByAdoptionRate(languages: LanguageAdoption[], maxLanguages: number): LanguageAdoption[] {
  return languages
    .filter((l) => l.repos_with_customizations > 0)
    .sort((a, b) => b.adoption_rate - a.adoption_rate)
    .slice(0, maxLanguages);
}

/**
 * Find the language with the highest adoption rate.
 */
export function getTopLanguage(languages: LanguageAdoption[]): LanguageAdoption | null {
  if (languages.length === 0) return null;

  return languages.reduce((best, lang) => (lang.adoption_rate > best.adoption_rate ? lang : best));
}

/**
 * Calculate team adoption statistics.
 */
export interface TeamAdoptionStats {
  totalTeams: number;
  teamsWithAdoption: number;
  adoptionPercent: number;
  totalReposWithCustomizations: number;
}

export function calculateTeamStats(teams: TeamAdoption[]): TeamAdoptionStats {
  const activeTeams = filterActiveTeams(teams);
  const teamsWithAdoption = filterTeamsWithAdoption(activeTeams);

  return {
    totalTeams: activeTeams.length,
    teamsWithAdoption: teamsWithAdoption.length,
    adoptionPercent: activeTeams.length > 0 ? (teamsWithAdoption.length / activeTeams.length) * 100 : 0,
    totalReposWithCustomizations: teamsWithAdoption.reduce((sum, t) => sum + t.repos_with_customizations, 0),
  };
}

/**
 * Calculate language adoption statistics.
 */
export interface LanguageAdoptionStats {
  totalLanguages: number;
  topLanguage: LanguageAdoption | null;
  topActiveLanguage: LanguageAdoption | null;
  totalReposWithCustomizations: number;
}

export function calculateLanguageStats(languages: LanguageAdoption[]): LanguageAdoptionStats {
  const activeLanguages = languages.filter((l) => l.recently_active_repos > 0 && l.repos_with_customizations > 0);
  const topActiveLanguage =
    activeLanguages.length > 0
      ? activeLanguages.reduce((best, lang) =>
          lang.adoption_rate_active_only > best.adoption_rate_active_only ? lang : best
        )
      : null;

  return {
    totalLanguages: languages.length,
    topLanguage: getTopLanguage(languages),
    topActiveLanguage,
    totalReposWithCustomizations: languages.reduce((sum, l) => sum + l.repos_with_customizations, 0),
  };
}

/**
 * Format adoption rate as percentage string.
 */
export function formatAdoptionRate(rate: number, decimals: number = 0): string {
  return `${(rate * 100).toFixed(decimals)}%`;
}

/**
 * Format scan date for display.
 */
export function formatScanDate(scanDate: string): string {
  return new Date(scanDate).toLocaleDateString("nb-NO", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}

// --- Scope-aware helpers ---

/**
 * Get the adoption rate for a team based on scope.
 */
export function getTeamAdoptionRate(team: TeamAdoption, scope: AdoptionScope): number {
  return scope === "active" ? team.adoption_rate_active_only : team.adoption_rate;
}

/**
 * Get the repo count denominator for a team based on scope.
 */
export function getTeamRepoCount(team: TeamAdoption, scope: AdoptionScope): number {
  return scope === "active" ? team.recently_active_repos : team.active_repos;
}

/**
 * Get the adoption rate for a language based on scope.
 */
export function getLanguageAdoptionRate(lang: LanguageAdoption, scope: AdoptionScope): number {
  return scope === "active" ? lang.adoption_rate_active_only : lang.adoption_rate;
}

/**
 * Get the repo count denominator for a language based on scope.
 */
export function getLanguageRepoCount(lang: LanguageAdoption, scope: AdoptionScope): number {
  return scope === "active" ? lang.recently_active_repos : lang.total_repos;
}

/**
 * Get the repo count for a customization detail based on scope.
 */
export function getCustomizationRepoCount(detail: CustomizationDetail, scope: AdoptionScope): number {
  return scope === "active" ? detail.active_repo_count : detail.repo_count;
}

/**
 * Get top teams for chart display, sorted by scope-appropriate metric.
 */
export function getTopTeamsForChart(
  teams: TeamAdoption[],
  scope: AdoptionScope,
  viewMode: "absolute" | "percentage",
  maxTeams: number
): TeamAdoption[] {
  const withCustomizations = teams.filter((t) => t.repos_with_customizations > 0);
  if (viewMode === "percentage") {
    return withCustomizations
      .filter((t) => getTeamRepoCount(t, scope) > 0)
      .sort((a, b) => getTeamAdoptionRate(b, scope) - getTeamAdoptionRate(a, scope))
      .slice(0, maxTeams);
  }
  return withCustomizations
    .sort((a, b) => b.repos_with_customizations - a.repos_with_customizations)
    .slice(0, maxTeams);
}

/**
 * Get top languages for chart display, sorted by scope-appropriate adoption rate.
 */
export function getTopLanguagesForChart(
  languages: LanguageAdoption[],
  scope: AdoptionScope,
  maxLanguages: number
): LanguageAdoption[] {
  return languages
    .filter((l) => l.repos_with_customizations > 0)
    .sort((a, b) => getLanguageAdoptionRate(b, scope) - getLanguageAdoptionRate(a, scope))
    .slice(0, maxLanguages);
}

/**
 * Sort customization details by scope-appropriate repo count (descending).
 */
export function sortCustomizationsByScope(details: CustomizationDetail[], scope: AdoptionScope): CustomizationDetail[] {
  return [...details].sort((a, b) => getCustomizationRepoCount(b, scope) - getCustomizationRepoCount(a, scope));
}
