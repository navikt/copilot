import {
  extractCustomizationTypes,
  extractToolComparison,
  getTopLanguagesByAdoptionRate,
  getTopLanguage,
  calculateLanguageStats,
  formatAdoptionRate,
  formatScanDate,
  getLanguageAdoptionRate,
  getLanguageRepoCount,
  getCustomizationRepoCount,
  getTopLanguagesForChart,
  sortCustomizationsByScope,
} from "./adoption-utils";
import type { AdoptionSummary, LanguageAdoption, CustomizationDetail } from "./types";

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
