import fs from "node:fs";
import path from "node:path";
import { sourceHeadings } from "@/lib/page-headings";
import inventory from "@/lib/link-inventory.json";
import { SECTION } from "@/lib/nav-items";
import { getNewsItems } from "@/lib/news";
import type { SearchEntry } from "@/lib/site-search";

// Builds the search index from what already exists: the umbrella pages and
// their titles from the section menu, each page's meta description, the
// anchors from link-inventory.json (so every hit is a link the link guard
// checks), and the news titles.
// Run by scripts/build-search-index.ts.

// Run from the app directory, as news.ts assumes too.
const APP_DIR = path.join(process.cwd(), "src", "app");
const paths: Record<string, { anchors: Record<string, string[]> }> = inventory;

// URL path → directory of its page.tsx. Route groups like (nb) are not in the URL.
const pageDirs = new Map(
  fs.globSync("**/page.tsx", { cwd: APP_DIR }).map((f) => {
    const dir = path.dirname(f);
    const segments = dir.split(path.sep).filter((s) => s !== "." && !/^\(.*\)$/.test(s));
    return ["/" + segments.join("/"), path.join(APP_DIR, dir)];
  })
);

// Heading text by id, from the files next to page.tsx. The labels in a
// TableOfContents win, since someone wrote them to be read out of context.
// A heading whose text is an expression ({name}) gets no label and is left out
// of the index.
function headingLabels(dir: string): Map<string, string> {
  const labels = new Map<string, string>();
  const sources = fs
    .readdirSync(dir)
    .filter((f) => /\.tsx?$/.test(f) && !/\.(test|stories)\.tsx?$/.test(f))
    .map((f) => fs.readFileSync(path.join(dir, f), "utf-8"));
  for (const src of sources) {
    for (const h of sourceHeadings(src)) if (h.id && h.text) labels.set(h.id, h.text);
    for (const m of src.matchAll(/\{\s*id:\s*"([^"]+)",\s*label:\s*"([^"]+)"/g)) labels.set(m[1], m[2]);
  }
  return labels;
}

// The page's meta description, so «ollama» finds the page that mentions it (#1185).
// Only a string literal inside the metadata object counts; the object ends at
// the first "};" at the start of a line.
// ponytail: regex over page.tsx, so a description built from an expression
// (/cplt) is skipped. Render the metadata if more pages start doing that.
function pageDescription(dir: string): string | undefined {
  const src = fs.readFileSync(path.join(dir, "page.tsx"), "utf-8");
  const metadata = src.match(/export const metadata\b[\s\S]*?\n\};/)?.[0];
  return metadata?.match(/\bdescription:\s*"([^"]+)"/)?.[1];
}

export function buildSearchIndex(): SearchEntry[] {
  const pages: SearchEntry[] = SECTION.flatMap((g) => [
    ...(g.href ? [{ href: g.href, title: g.label, context: "nav-pilot" }] : []),
    ...(g.overview ? [{ href: g.overview, title: g.label, context: "nav-pilot" }] : []),
    ...(g.items ?? []).map((i) => ({ href: i.href, title: i.label, context: g.label })),
  ])
    .filter((p) => p.href in paths)
    .map((p) => {
      const dir = pageDirs.get(p.href);
      const text = dir && pageDescription(dir);
      return text ? { ...p, text } : p;
    });

  const headings = pages.flatMap((p) => {
    const dir = pageDirs.get(p.href);
    if (!dir) return [];
    const labels = headingLabels(dir);
    return Object.keys(paths[p.href].anchors)
      .filter((id) => labels.has(id))
      .map((id) => ({ href: `${p.href}#${id}`, title: labels.get(id)!, context: p.title }));
  });

  const news = getNewsItems({ lang: "nb" })
    .filter((n) => n.title && `/nyheter/${n.slug}` in paths)
    .map((n) => ({ href: `/nyheter/${n.slug}`, title: n.title, context: "Nyhet" }));

  return [...pages, ...headings, ...news];
}
