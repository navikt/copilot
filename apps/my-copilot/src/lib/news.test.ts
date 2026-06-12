import { CATEGORY_CONFIG, isExternalExcerptFresh, selectNewsItems, type NewsCategory } from "./news";
import type { NewsItem } from "./news-types";

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

  describe("isExternalExcerptFresh", () => {
    const now = new Date("2026-06-12T10:00:00Z");

    function createItem(overrides: Partial<NewsItem>): NewsItem {
      return {
        slug: "test-item",
        title: "Test item",
        date: "2026-06-12",
        draft: false,
        category: "copilot",
        excerpt: "",
        tags: [],
        type: "link",
        ...overrides,
      };
    }

    it("shows external excerpts that are five days old or newer", () => {
      const fresh = createItem({ date: "2026-06-07" });
      const newest = createItem({ date: "2026-06-12" });

      expect(isExternalExcerptFresh(fresh, now)).toBe(true);
      expect(isExternalExcerptFresh(newest, now)).toBe(true);
    });

    it("hides external excerpts older than five days", () => {
      const stale = createItem({ date: "2026-06-06" });
      expect(isExternalExcerptFresh(stale, now)).toBe(false);
    });

    it("keeps authored articles visible regardless of age", () => {
      const article = createItem({ type: "article", date: "2020-01-01" });
      expect(isExternalExcerptFresh(article, now)).toBe(true);
    });

    it("hides external excerpts with invalid date", () => {
      const invalid = createItem({ date: "" });
      expect(isExternalExcerptFresh(invalid, now)).toBe(false);
    });

    it("keeps future-dated external excerpts visible", () => {
      const future = createItem({ date: "2026-06-20" });
      expect(isExternalExcerptFresh(future, now)).toBe(true);
    });
  });

  describe("selectNewsItems", () => {
    const now = new Date("2026-06-12T10:00:00Z");

    const items: NewsItem[] = [
      {
        slug: "old-link",
        title: "Old external",
        date: "2026-05-01",
        draft: false,
        category: "nav",
        excerpt: "",
        tags: [],
        type: "link",
        url: "https://example.com/old",
      },
      {
        slug: "old-article",
        title: "Old article",
        date: "2026-01-01",
        draft: false,
        category: "copilot",
        excerpt: "",
        tags: [],
        type: "article",
      },
      {
        slug: "fresh-link",
        title: "Fresh external",
        date: "2026-06-11",
        draft: false,
        category: "nav-pilot",
        excerpt: "",
        tags: [],
        type: "link",
        url: "https://example.com/fresh",
      },
    ];

    it("keeps old external excerpts when not rendering front page", () => {
      const result = selectNewsItems(items, { now });
      expect(result.map((item) => item.slug)).toEqual(["fresh-link", "old-link", "old-article"]);
    });

    it("filters old external excerpts on front page while keeping authored articles", () => {
      const result = selectNewsItems(items, { frontPage: true, now });
      expect(result.map((item) => item.slug)).toEqual(["fresh-link", "old-article"]);
    });
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
