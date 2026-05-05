import { CATEGORY_CONFIG, type NewsCategory } from "./news";

describe("CATEGORY_CONFIG", () => {
  it("should have config for all known categories", () => {
    const categories: NewsCategory[] = ["copilot", "nav", "nav-pilot", "praksis"];

    for (const category of categories) {
      const config = CATEGORY_CONFIG[category];
      expect(config).toBeDefined();
      expect(config.label).toBeTruthy();
      expect(config.variant).toBeTruthy();
    }
  });

  it("should return undefined for unknown categories", () => {
    const config = CATEGORY_CONFIG["unknown" as NewsCategory];
    expect(config).toBeUndefined();
  });

  it("should use valid Tag variants", () => {
    const validVariants = ["info", "success", "warning", "error", "neutral", "alt1", "alt2", "alt3"];

    for (const config of Object.values(CATEGORY_CONFIG)) {
      expect(validVariants).toContain(config.variant);
    }
  });

  it("should have an entry for every value in NewsCategory", () => {
    const configKeys = Object.keys(CATEGORY_CONFIG);
    expect(configKeys).toContain("copilot");
    expect(configKeys).toContain("nav");
    expect(configKeys).toContain("nav-pilot");
    expect(configKeys).toContain("praksis");
    expect(configKeys).toContain("oppsummering");
    expect(configKeys).toHaveLength(5);
  });
});
