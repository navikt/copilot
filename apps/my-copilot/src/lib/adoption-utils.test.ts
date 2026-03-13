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
} from "./adoption-utils";
import type { AdoptionSummary, LanguageAdoption, TeamAdoption } from "./types";

// Test fixtures
const mockSummary: AdoptionSummary = {
  scan_date: "2026-03-13",
  total_repos: 6361,
  active_repos: 3862,
  archived_repos: 2499,
  repos_with_any_customization: 123,
  repos_without_customization: 3739,
  adoption_rate: 0.0318,
  repos_with_copilot_instructions: 80,
  repos_with_agents_md: 15,
  repos_with_agents: 12,
  repos_with_instructions: 8,
  repos_with_prompts: 5,
  repos_with_skills: 3,
  repos_with_mcp_config: 2,
  repos_with_copilot_dir: 1,
  repos_with_cursorrules: 10,
  repos_with_cursor_rules_dir: 3,
  repos_with_claude_md: 7,
  repos_with_windsurfrules: 2,
  repos_with_cursorignore: 1,
  repos_with_claude_settings: 1,
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
    repos_with_customizations: 5,
    adoption_rate: 0.417,
    with_copilot_instructions: 4,
    with_agents_md: 2,
    with_agents: 1,
    with_instructions: 1,
    with_prompts: 0,
    with_skills: 0,
    with_mcp_config: 0,
  },
  {
    scan_date: "2026-03-13",
    team_slug: "team-beta",
    team_name: "Team Beta",
    team_repos: 8,
    active_repos: 0, // Inactive team
    repos_with_customizations: 0,
    adoption_rate: 0,
    with_copilot_instructions: 0,
    with_agents_md: 0,
    with_agents: 0,
    with_instructions: 0,
    with_prompts: 0,
    with_skills: 0,
    with_mcp_config: 0,
  },
  {
    scan_date: "2026-03-13",
    team_slug: "team-gamma",
    team_name: "Team Gamma",
    team_repos: 20,
    active_repos: 18,
    repos_with_customizations: 8,
    adoption_rate: 0.444,
    with_copilot_instructions: 6,
    with_agents_md: 3,
    with_agents: 2,
    with_instructions: 2,
    with_prompts: 1,
    with_skills: 1,
    with_mcp_config: 0,
  },
  {
    scan_date: "2026-03-13",
    team_slug: "team-delta",
    team_name: "Team Delta",
    team_repos: 5,
    active_repos: 5,
    repos_with_customizations: 0,
    adoption_rate: 0,
    with_copilot_instructions: 0,
    with_agents_md: 0,
    with_agents: 0,
    with_instructions: 0,
    with_prompts: 0,
    with_skills: 0,
    with_mcp_config: 0,
  },
];

const mockLanguages: LanguageAdoption[] = [
  {
    scan_date: "2026-03-13",
    language: "TypeScript",
    total_repos: 500,
    repos_with_customizations: 45,
    adoption_rate: 0.09,
    with_copilot_instructions: 40,
    with_agents: 5,
    with_instructions: 3,
    with_mcp_config: 2,
  },
  {
    scan_date: "2026-03-13",
    language: "Kotlin",
    total_repos: 300,
    repos_with_customizations: 30,
    adoption_rate: 0.1,
    with_copilot_instructions: 25,
    with_agents: 8,
    with_instructions: 5,
    with_mcp_config: 1,
  },
  {
    scan_date: "2026-03-13",
    language: "Go",
    total_repos: 50,
    repos_with_customizations: 10,
    adoption_rate: 0.2,
    with_copilot_instructions: 8,
    with_agents: 3,
    with_instructions: 2,
    with_mcp_config: 1,
  },
  {
    scan_date: "2026-03-13",
    language: "Java",
    total_repos: 200,
    repos_with_customizations: 0,
    adoption_rate: 0,
    with_copilot_instructions: 0,
    with_agents: 0,
    with_instructions: 0,
    with_mcp_config: 0,
  },
];

describe("extractCustomizationTypes", () => {
  it("should extract and sort customization types by value descending", () => {
    const result = extractCustomizationTypes(mockSummary);

    expect(result.length).toBe(8);
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
