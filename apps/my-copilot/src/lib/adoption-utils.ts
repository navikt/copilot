/**
 * Data transformation utilities for adoption metrics.
 *
 * These pure functions transform raw adoption data into chart-ready formats.
 * All functions are side-effect free and easily testable.
 */

import type { AdoptionSummary, LanguageAdoption, TeamAdoption } from "./types";

/**
 * Customization type with label and count.
 */
export interface CustomizationType {
  label: string;
  value: number;
  key: string;
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
    { key: "copilot_instructions", label: "copilot-instructions.md", value: summary.repos_with_copilot_instructions },
    { key: "agents_md", label: "AGENTS.md", value: summary.repos_with_agents_md },
    { key: "agents", label: ".github/agents/", value: summary.repos_with_agents },
    { key: "instructions", label: ".github/instructions/", value: summary.repos_with_instructions },
    { key: "prompts", label: ".github/prompts/", value: summary.repos_with_prompts },
    { key: "skills", label: ".github/skills/", value: summary.repos_with_skills },
    { key: "mcp_config", label: "mcp.json", value: summary.repos_with_mcp_config },
    { key: "copilot_dir", label: ".copilot/", value: summary.repos_with_copilot_dir },
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
  totalReposWithCustomizations: number;
}

export function calculateLanguageStats(languages: LanguageAdoption[]): LanguageAdoptionStats {
  return {
    totalLanguages: languages.length,
    topLanguage: getTopLanguage(languages),
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
