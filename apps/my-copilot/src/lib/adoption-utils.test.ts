import {
  extractCustomizationTypes,
  extractToolComparison,
  filterActiveTeams,
  filterTeamsWithAdoption,
  getTopTeams,
  sortTeamsByAdoption,
  getTopLanguagesByAdoptionRate,
  getTopLanguage,
  calculateTeamStats,
  calculateLanguageStats,
  formatAdoptionRate,
  formatScanDate,
  getTeamAdoptionRate,
  getTeamRepoCount,
  getLanguageAdoptionRate,
  getLanguageRepoCount,
  getCustomizationRepoCount,
  getTopTeamsForChart,
  getTopLanguagesForChart,
  sortCustomizationsByScope,
} from "./adoption-utils";
import type { AdoptionSummary, LanguageAdoption, TeamAdoption, CustomizationDetail } from "./types";

// Test fixtures
const mockSummary: AdoptionSummary = {
  scan_date: "2026-03-13",
  total_repos: 6361,
  active_repos: 3862,
  archived_repos: 2499,
  active_repos_with_recent_commits: 1200,
  dormant_repos: 2500,
  unknown_last_commit_repos: 162,
  repos_with_any_customization: 123,
  repos_without_customization: 3739,
  adoption_rate: 0.0318,
  adoption_rate_active_only: 0.0825,
  repos_with_copilot_instructions: 80,
  repos_with_agents_md: 15,
  repos_with_agents: 12,
  repos_with_instructions: 8,
  repos_with_prompts: 5,
  repos_with_skills: 3,
  repos_with_mcp_config: 2,
  repos_with_copilot_dir: 1,
  repos_with_copilot_review_instructions: 0,
  repos_with_cursorrules: 10,
  repos_with_cursor_rules_dir: 3,
  repos_with_claude_md: 7,
  repos_with_windsurfrules: 2,
  repos_with_cursorignore: 1,
  repos_with_claude_settings: 1,
  repos_with_copilot_setup_steps: 4,
  repos_with_agentic_workflows: 2,
  repos_with_agents_skills: 3,
  repos_with_nav_pilot_state: 6,
  repos_with_cplt_toml: 5,
  repos_with_any_non_copilot_ai: 18,
  avg_customization_count: 1.2,
  max_customization_count: 5,
};

const mockTeams: TeamAdoption[] = [
  {
    scan_date: "2026-03-13",
    team_slug: "team-alpha",
    team_name: "Team Alpha",
    team_repos: 15,
    active_repos: 12,
    recently_active_repos: 10,
    repos_with_customizations: 5,
    adoption_rate: 0.417,
    adoption_rate_active_only: 0.5,
    with_copilot_instructions: 4,
    with_agents_md: 2,
    with_agents: 1,
    with_instructions: 1,
    with_prompts: 0,
    with_skills: 0,
    with_mcp_config: 0,
    with_copilot_setup_steps: 1,
    with_agentic_workflows: 0,
    with_agents_skills: 0,
    with_nav_pilot_state: 1,
    with_cplt_toml: 1,
  },
  {
    scan_date: "2026-03-13",
    team_slug: "team-beta",
    team_name: "Team Beta",
    team_repos: 8,
    active_repos: 0, // Inactive team
    recently_active_repos: 0,
    repos_with_customizations: 0,
    adoption_rate: 0,
    adoption_rate_active_only: 0,
    with_copilot_instructions: 0,
    with_agents_md: 0,
    with_agents: 0,
    with_instructions: 0,
    with_prompts: 0,
    with_skills: 0,
    with_mcp_config: 0,
    with_copilot_setup_steps: 0,
    with_agentic_workflows: 0,
    with_agents_skills: 0,
    with_nav_pilot_state: 0,
    with_cplt_toml: 0,
  },
  {
    scan_date: "2026-03-13",
    team_slug: "team-gamma",
    team_name: "Team Gamma",
    team_repos: 20,
    active_repos: 18,
    recently_active_repos: 15,
    repos_with_customizations: 8,
    adoption_rate: 0.444,
    adoption_rate_active_only: 0.533,
    with_copilot_instructions: 6,
    with_agents_md: 3,
    with_agents: 2,
    with_instructions: 2,
    with_prompts: 1,
    with_skills: 1,
    with_mcp_config: 0,
    with_copilot_setup_steps: 2,
    with_agentic_workflows: 1,
    with_agents_skills: 1,
    with_nav_pilot_state: 2,
    with_cplt_toml: 1,
  },
  {
    scan_date: "2026-03-13",
    team_slug: "team-delta",
    team_name: "Team Delta",
    team_repos: 5,
    active_repos: 5,
    recently_active_repos: 3,
    repos_with_customizations: 0,
    adoption_rate: 0,
    adoption_rate_active_only: 0,
    with_copilot_instructions: 0,
    with_agents_md: 0,
    with_agents: 0,
    with_instructions: 0,
    with_prompts: 0,
    with_skills: 0,
    with_mcp_config: 0,
    with_copilot_setup_steps: 0,
    with_agentic_workflows: 0,
    with_agents_skills: 0,
    with_nav_pilot_state: 0,
    with_cplt_toml: 0,
  },
];

const mockLanguages: LanguageAdoption[] = [
  {
    scan_date: "2026-03-13",
    language: "TypeScript",
    total_repos: 500,
    recently_active_repos: 350,
    repos_with_customizations: 45,
    adoption_rate: 0.09,
    adoption_rate_active_only: 0.129,
    with_copilot_instructions: 40,
    with_agents: 5,
    with_instructions: 3,
    with_mcp_config: 2,
  },
  {
    scan_date: "2026-03-13",
    language: "Kotlin",
    total_repos: 300,
    recently_active_repos: 200,
    repos_with_customizations: 30,
    adoption_rate: 0.1,
    adoption_rate_active_only: 0.15,
    with_copilot_instructions: 25,
    with_agents: 8,
    with_instructions: 5,
    with_mcp_config: 1,
  },
  {
    scan_date: "2026-03-13",
    language: "Go",
    total_repos: 50,
    recently_active_repos: 40,
    repos_with_customizations: 10,
    adoption_rate: 0.2,
    adoption_rate_active_only: 0.25,
    with_copilot_instructions: 8,
    with_agents: 3,
    with_instructions: 2,
    with_mcp_config: 1,
  },
  {
    scan_date: "2026-03-13",
    language: "Java",
    total_repos: 200,
    recently_active_repos: 100,
    repos_with_customizations: 0,
    adoption_rate: 0,
    adoption_rate_active_only: 0,
    with_copilot_instructions: 0,
    with_agents: 0,
    with_instructions: 0,
    with_mcp_config: 0,
  },
];

describe("extractCustomizationTypes", () => {
  it("should extract and sort customization types by value descending", () => {
    const result = extractCustomizationTypes(mockSummary);

    expect(result.length).toBe(14);
    expect(result[0].label).toBe("copilot-instructions.md");
    expect(result[0].value).toBe(80);
    // Verify sorting
    for (let i = 1; i < result.length; i++) {
      expect(result[i - 1].value).toBeGreaterThanOrEqual(result[i].value);
    }
  });

  it("should include all customization type keys", () => {
    const result = extractCustomizationTypes(mockSummary);
    const keys = result.map((r) => r.key);

    expect(keys).toContain("copilot_instructions");
    expect(keys).toContain("agents_md");
    expect(keys).toContain("agents");
    expect(keys).toContain("instructions");
    expect(keys).toContain("prompts");
    expect(keys).toContain("skills");
    expect(keys).toContain("mcp_config");
    expect(keys).toContain("copilot_dir");
    expect(keys).toContain("copilot_review_instructions");
    expect(keys).toContain("copilot_setup_steps");
    expect(keys).toContain("agentic_workflows");
    expect(keys).toContain("agents_skills");
    expect(keys).toContain("nav_pilot_state");
    expect(keys).toContain("cplt_toml");
  });

  it("should assign correct groups", () => {
    const result = extractCustomizationTypes(mockSummary);

    const copilot = result.filter((t) => t.group === "copilot");
    const agentic = result.filter((t) => t.group === "agentic");
    const navPilot = result.filter((t) => t.group === "nav-pilot");

    expect(copilot.length).toBe(10);
    expect(agentic.length).toBe(3);
    expect(navPilot.length).toBe(1);
  });
});

describe("extractToolComparison", () => {
  it("should extract non-zero tool usage", () => {
    const result = extractToolComparison(mockSummary);

    // Should only include tools with value > 0
    expect(result.every((t) => t.value > 0)).toBe(true);
  });

  it("should calculate Copilot-only correctly", () => {
    const result = extractToolComparison(mockSummary);
    const copilotOnly = result.find((t) => t.label === "Kun Copilot");

    // repos_with_any_customization (123) - repos_with_any_non_copilot_ai (18) = 105
    expect(copilotOnly?.value).toBe(105);
  });

  it("should combine Cursor rules", () => {
    const result = extractToolComparison(mockSummary);
    const cursor = result.find((t) => t.label === "Cursor");

    // repos_with_cursorrules (10) + repos_with_cursor_rules_dir (3) = 13
    expect(cursor?.value).toBe(13);
  });
});

describe("filterActiveTeams", () => {
  it("should filter out teams with no active repos", () => {
    const result = filterActiveTeams(mockTeams);

    expect(result.length).toBe(3);
    expect(result.every((t) => t.active_repos > 0)).toBe(true);
    expect(result.find((t) => t.team_slug === "team-beta")).toBeUndefined();
  });
});

describe("filterTeamsWithAdoption", () => {
  it("should filter to only teams with customizations", () => {
    const result = filterTeamsWithAdoption(mockTeams);

    expect(result.length).toBe(2);
    expect(result.every((t) => t.repos_with_customizations > 0)).toBe(true);
  });
});

describe("getTopTeams", () => {
  it("should return top N teams sorted by customizations", () => {
    const result = getTopTeams(mockTeams, 2);

    expect(result.length).toBe(2);
    expect(result[0].team_slug).toBe("team-gamma"); // 8 repos
    expect(result[1].team_slug).toBe("team-alpha"); // 5 repos
  });

  it("should handle maxTeams larger than available", () => {
    const result = getTopTeams(mockTeams, 100);

    expect(result.length).toBe(2); // Only 2 teams have customizations
  });
});

describe("sortTeamsByAdoption", () => {
  it("should sort teams by repos_with_customizations descending", () => {
    const result = sortTeamsByAdoption(mockTeams);

    expect(result[0].repos_with_customizations).toBe(8);
    expect(result[1].repos_with_customizations).toBe(5);
  });

  it("should not mutate original array", () => {
    const original = [...mockTeams];
    sortTeamsByAdoption(mockTeams);

    expect(mockTeams).toEqual(original);
  });
});

describe("getTopLanguagesByAdoptionRate", () => {
  it("should return top N languages by adoption rate", () => {
    const result = getTopLanguagesByAdoptionRate(mockLanguages, 2);

    expect(result.length).toBe(2);
    expect(result[0].language).toBe("Go"); // 20% adoption rate
    expect(result[1].language).toBe("Kotlin"); // 10% adoption rate
  });

  it("should exclude languages with no customizations", () => {
    const result = getTopLanguagesByAdoptionRate(mockLanguages, 10);

    expect(result.find((l) => l.language === "Java")).toBeUndefined();
  });
});

describe("getTopLanguage", () => {
  it("should return language with highest adoption rate", () => {
    const result = getTopLanguage(mockLanguages);

    expect(result?.language).toBe("Go");
    expect(result?.adoption_rate).toBe(0.2);
  });

  it("should return null for empty array", () => {
    const result = getTopLanguage([]);

    expect(result).toBeNull();
  });
});

describe("calculateTeamStats", () => {
  it("should calculate correct team statistics", () => {
    const result = calculateTeamStats(mockTeams);

    expect(result.totalTeams).toBe(3); // 3 active teams
    expect(result.teamsWithAdoption).toBe(2);
    expect(result.totalReposWithCustomizations).toBe(13); // 5 + 8
    expect(result.adoptionPercent).toBeCloseTo(66.67, 1);
  });

  it("should handle empty array", () => {
    const result = calculateTeamStats([]);

    expect(result.totalTeams).toBe(0);
    expect(result.teamsWithAdoption).toBe(0);
    expect(result.adoptionPercent).toBe(0);
  });
});

describe("calculateLanguageStats", () => {
  it("should calculate correct language statistics", () => {
    const result = calculateLanguageStats(mockLanguages);

    expect(result.totalLanguages).toBe(4);
    expect(result.topLanguage?.language).toBe("Go");
    expect(result.topActiveLanguage?.language).toBe("Go");
    expect(result.topActiveLanguage?.adoption_rate_active_only).toBe(0.25);
    expect(result.totalReposWithCustomizations).toBe(85); // 45 + 30 + 10 + 0
  });
});

describe("formatAdoptionRate", () => {
  it("should format rate as percentage", () => {
    expect(formatAdoptionRate(0.1)).toBe("10%");
    expect(formatAdoptionRate(0.0318, 1)).toBe("3.2%");
    expect(formatAdoptionRate(0.5, 2)).toBe("50.00%");
  });

  it("should default to 0 decimals", () => {
    expect(formatAdoptionRate(0.125)).toBe("13%"); // Rounded
  });
});

describe("formatScanDate", () => {
  it("should format date in Norwegian", () => {
    const result = formatScanDate("2026-03-13");

    expect(result).toContain("13");
    expect(result).toContain("mars");
    expect(result).toContain("2026");
  });
});

// --- Scope-aware helper tests ---

const mockCustomizationDetails: CustomizationDetail[] = [
  { category: "agents", file_name: "nais.agent.md", repo_count: 20, active_repo_count: 15 },
  { category: "agents", file_name: "auth.agent.md", repo_count: 10, active_repo_count: 8 },
  { category: "instructions", file_name: "kotlin.instructions.md", repo_count: 25, active_repo_count: 5 },
  { category: "instructions", file_name: "testing.instructions.md", repo_count: 15, active_repo_count: 12 },
];

describe("getTeamAdoptionRate", () => {
  const team = mockTeams[0]; // Team Alpha

  it("should return adoption_rate_active_only for active scope", () => {
    expect(getTeamAdoptionRate(team, "active")).toBe(0.5);
  });

  it("should return adoption_rate for all scope", () => {
    expect(getTeamAdoptionRate(team, "all")).toBe(0.417);
  });
});

describe("getTeamRepoCount", () => {
  const team = mockTeams[0];

  it("should return recently_active_repos for active scope", () => {
    expect(getTeamRepoCount(team, "active")).toBe(10);
  });

  it("should return active_repos for all scope", () => {
    expect(getTeamRepoCount(team, "all")).toBe(12);
  });
});

describe("getLanguageAdoptionRate", () => {
  const lang = mockLanguages[1]; // Kotlin

  it("should return adoption_rate_active_only for active scope", () => {
    expect(getLanguageAdoptionRate(lang, "active")).toBe(0.15);
  });

  it("should return adoption_rate for all scope", () => {
    expect(getLanguageAdoptionRate(lang, "all")).toBe(0.1);
  });
});

describe("getLanguageRepoCount", () => {
  const lang = mockLanguages[0]; // TypeScript

  it("should return recently_active_repos for active scope", () => {
    expect(getLanguageRepoCount(lang, "active")).toBe(350);
  });

  it("should return total_repos for all scope", () => {
    expect(getLanguageRepoCount(lang, "all")).toBe(500);
  });
});

describe("getCustomizationRepoCount", () => {
  const detail = mockCustomizationDetails[0];

  it("should return active_repo_count for active scope", () => {
    expect(getCustomizationRepoCount(detail, "active")).toBe(15);
  });

  it("should return repo_count for all scope", () => {
    expect(getCustomizationRepoCount(detail, "all")).toBe(20);
  });
});

describe("getTopTeamsForChart", () => {
  it("should sort by adoption_rate_active_only in percentage mode with active scope", () => {
    const result = getTopTeamsForChart(mockTeams, "active", "percentage", 10);

    expect(result.length).toBe(2); // Only alpha and gamma have customizations + active repos
    expect(result[0].team_slug).toBe("team-gamma"); // 0.533 > 0.5
    expect(result[1].team_slug).toBe("team-alpha");
  });

  it("should sort by adoption_rate in percentage mode with all scope", () => {
    const result = getTopTeamsForChart(mockTeams, "all", "percentage", 10);

    expect(result[0].team_slug).toBe("team-gamma"); // 0.444 > 0.417
  });

  it("should sort by repos_with_customizations in absolute mode", () => {
    const result = getTopTeamsForChart(mockTeams, "active", "absolute", 10);

    expect(result[0].team_slug).toBe("team-gamma"); // 8 > 5
    expect(result[1].team_slug).toBe("team-alpha");
  });

  it("should exclude teams with zero repos in scope for percentage mode", () => {
    const result = getTopTeamsForChart(mockTeams, "active", "percentage", 10);

    expect(result.find((t) => t.team_slug === "team-beta")).toBeUndefined();
  });

  it("should handle empty input", () => {
    expect(getTopTeamsForChart([], "active", "percentage", 10)).toEqual([]);
  });
});

describe("getTopLanguagesForChart", () => {
  it("should sort by adoption_rate_active_only with active scope", () => {
    const result = getTopLanguagesForChart(mockLanguages, "active", 10);

    expect(result[0].language).toBe("Go"); // 0.25
    expect(result[1].language).toBe("Kotlin"); // 0.15
    expect(result[2].language).toBe("TypeScript"); // 0.129
  });

  it("should sort by adoption_rate with all scope", () => {
    const result = getTopLanguagesForChart(mockLanguages, "all", 10);

    expect(result[0].language).toBe("Go"); // 0.2
    expect(result[1].language).toBe("Kotlin"); // 0.1
  });

  it("should exclude languages with zero customizations", () => {
    const result = getTopLanguagesForChart(mockLanguages, "active", 10);

    expect(result.find((l) => l.language === "Java")).toBeUndefined();
  });

  it("should handle empty input", () => {
    expect(getTopLanguagesForChart([], "active", 10)).toEqual([]);
  });
});

describe("sortCustomizationsByScope", () => {
  it("should sort by active_repo_count with active scope", () => {
    const result = sortCustomizationsByScope(mockCustomizationDetails, "active");

    expect(result[0].file_name).toBe("nais.agent.md"); // 15
    expect(result[1].file_name).toBe("testing.instructions.md"); // 12
  });

  it("should sort by repo_count with all scope", () => {
    const result = sortCustomizationsByScope(mockCustomizationDetails, "all");

    expect(result[0].file_name).toBe("kotlin.instructions.md"); // 25
    expect(result[1].file_name).toBe("nais.agent.md"); // 20
  });

  it("should not mutate original array", () => {
    const original = [...mockCustomizationDetails];
    sortCustomizationsByScope(mockCustomizationDetails, "active");

    expect(mockCustomizationDetails).toEqual(original);
  });
});
