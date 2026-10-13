import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const root = join(__dirname, "../../..");
const src = (path: string) => readFileSync(join(root, path), "utf8");
const pages = readdirSync(__dirname, { withFileTypes: true })
  .filter((d) => d.isDirectory())
  .map((d) => `app/(nb)/innsikt/${d.name}/page.tsx`);

describe("/innsikt pages", () => {
  it("all use InsightPage and give «Sist oppdatert» as a date, not free text", () => {
    expect(pages.length).toBeGreaterThanOrEqual(5);
    for (const page of pages) {
      const text = src(page);
      expect(text, page).toContain("<InsightPage");
      expect(text, page).toMatch(/updated=\{async \(\) =>/);
      expect(text, page).not.toContain("Sist oppdatert:");
    }
    expect(src("components/insight-page.tsx")).toContain("Sist oppdatert:");
  });

  it("has no tab-like toggles on /innsikt/tilpasninger", () => {
    const files = [
      "app/(nb)/innsikt/tilpasninger/page.tsx",
      "components/team-table.tsx",
      ...readdirSync(join(root, "components/charts/adoption")).map((f) => `components/charts/adoption/${f}`),
    ];
    for (const file of files) expect(src(file), file).not.toMatch(/ToggleGroup|Tabs/);
  });

  it("formats numbers with the shared helpers, not toFixed or a bare %", () => {
    for (const file of [...pages, "components/team-gross-usage.tsx", "components/team-table.tsx"]) {
      const text = src(file);
      expect(text, file).not.toMatch(/toFixed\(|\}%|currency: "USD"/);
    }
  });
});
