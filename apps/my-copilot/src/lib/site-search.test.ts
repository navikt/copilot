import { buildSearchIndex } from "@/lib/search-index";
import inventory from "@/lib/link-inventory.json";
import { searchEntries, type SearchEntry } from "@/lib/site-search";
import fs from "node:fs";
import path from "node:path";
import { PRIVATE_PAGE_PATHS } from "@/proxy";

const index = buildSearchIndex();
const paths: Record<string, { anchors: Record<string, string[]> }> = inventory;

describe("search index", () => {
  it("has the umbrella pages, their headings and the news", () => {
    expect(index).toContainEqual(
      expect.objectContaining({ href: "/nav-pilot/lokal", title: "Lokal modell på Mac", context: "Kom i gang" })
    );
    expect(index).toContainEqual({
      href: "/nav-pilot/guider/lokal#bytte-lokal-modell",
      title: "Bytte lokal modell",
      context: "Lokal modell",
    });
    expect(index.some((e) => e.href === "/nyheter/lokale-modeller-i-nav-pilot")).toBe(true);
    // No date in the context, or «2026» would find every news item.
    expect(index.filter((e) => e.href.startsWith("/nyheter/")).every((e) => e.context === "Nyhet")).toBe(true);
  });

  // Every static page route is searchable. A page whose title the index can't
  // read (no metadata title literal, no <PageHero title="…">) fails here.
  it("has every page route", () => {
    const appDir = path.join(__dirname, "..", "app");
    const files = fs.globSync("**/page.tsx", { cwd: appDir });
    const route = (f: string) =>
      "/" +
      path
        .dirname(f)
        .split(path.sep)
        .filter((s) => s !== "." && !/^\(.*\)$/.test(s))
        .join("/");
    const pageFile = (r: string) => files.find((f) => route(f) === r)!;
    const routes = files
      .map(route)
      .filter((r) => !r.includes("["))
      // A page that only redirects is found under the page it redirects to.
      .filter((r) => !/\bpermanentRedirect\(/.test(fs.readFileSync(path.join(appDir, pageFile(r)), "utf-8")));
    const hrefs = new Set(index.map((e) => e.href));
    expect(routes.filter((r) => !hrefs.has(r))).toEqual([]);
  });

  it("marks pages behind a login, headings included", () => {
    const under = (r: string) =>
      index.filter((e) => e.href === r || (/^[/#]/.test(e.href.slice(r.length)) && e.href.startsWith(r)));
    for (const r of PRIVATE_PAGE_PATHS) expect(under(r).filter((e) => !e.login)).toEqual([]);
    expect(index.find((e) => e.href === "/abonnement")?.login).toBe(true);
    expect(index.find((e) => e.href === "/innsikt")?.login).toBeUndefined();
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

  // #1185: the words are in egen-server's description, not in any title.
  it("finds a page by its description", () => {
    expect(searchEntries(index, "ollama")[0]?.href).toBe("/nav-pilot/lokal/egen-server");
    expect(searchEntries(index, "linux").map((e) => e.href)).toContain("/nav-pilot/lokal/egen-server");
  });

  // #1280: /cplt/windows has no nav-pilot section menu, so it isn't in SECTION
  // and reaches the index through EXTRA_PAGES in search-index.ts instead.
  it("finds /cplt/windows", () => {
    expect(searchEntries(index, "windows").map((e) => e.href)).toContain("/cplt/windows");
    expect(searchEntries(index, "wsl").map((e) => e.href)).toContain("/cplt/windows");
  });

  // /cplt's description is a constant; the first quoted description: further
  // down its page is body text and must not be indexed.
  it("only takes a description from the metadata object", () => {
    expect(index.find((e) => e.href === "/cplt")?.text).toBeUndefined();
  });

  it("ranks a text match below title and context matches", () => {
    const hits = searchEntries([{ ...entry("x"), text: "sync" }, entry("Sync"), entry("y", "sync")], "sync");
    expect(hits.map((e) => e.title)).toEqual(["Sync", "y", "x"]);
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
