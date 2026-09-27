import { buildSearchIndex } from "@/lib/search-index";
import inventory from "@/lib/link-inventory.json";
import { searchEntries, type SearchEntry } from "@/lib/site-search";

const index = buildSearchIndex();
const paths: Record<string, { anchors: Record<string, string[]> }> = inventory;

describe("search index", () => {
  it("has the umbrella pages, their headings and the news", () => {
    expect(index).toContainEqual({ href: "/nav-pilot/lokal", title: "Lokal modell på Mac", context: "Kom i gang" });
    expect(index).toContainEqual({
      href: "/nav-pilot/guider/lokal#bytte-lokal-modell",
      title: "Bytte lokal modell",
      context: "Lokal modell",
    });
    expect(index.some((e) => e.href === "/nyheter/lokale-modeller-i-nav-pilot")).toBe(true);
  });

  // src/link-inventory.test.ts checks that every inventoried path and anchor resolves.
  it("only links to paths and anchors in the link inventory", () => {
    const unknown = index.filter((e) => {
      const [p, anchor] = e.href.split("#");
      return !(p in paths) || (anchor !== undefined && !(anchor in paths[p].anchors));
    });
    expect(unknown).toEqual([]);
  });
});

describe("searchEntries", () => {
  const entry = (title: string, context = "nav-pilot"): SearchEntry => ({ href: "/", title, context });

  it("finds pages in the real index", () => {
    expect(searchEntries(index, "lokal modell")[0].title.toLowerCase()).toContain("lokal modell");
    expect(searchEntries(index, "lokal modell").map((e) => e.href)).toContain("/nav-pilot/lokal");
  });

  it("ignores case and diacritics", () => {
    expect(searchEntries([entry("Oppstart")], "oppstart")).toHaveLength(1);
    expect(searchEntries([entry("Målte grenser")], "malte")).toHaveLength(1);
    expect(searchEntries([entry("Kom i gang")], "  KOM   i ")).toHaveLength(1);
  });

  it("needs every word, in the title or the context", () => {
    expect(searchEntries([entry("Bytte modell", "Lokal modell")], "lokal bytte")).toHaveLength(1);
    expect(searchEntries([entry("Bytte modell")], "lokal bytte")).toEqual([]);
    expect(searchEntries([entry("Bytte modell")], " ")).toEqual([]);
  });

  it("ranks a title that starts with the query first", () => {
    const hits = searchEntries([entry("Om sync", "Guider"), entry("Sync", "Guider"), entry("x", "sync")], "sync");
    expect(hits.map((e) => e.title)).toEqual(["Sync", "Om sync", "x"]);
  });
});
