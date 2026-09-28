import fs from "node:fs";
import path from "node:path";
import { slugify } from "@/components/linkable-heading";
import inventory from "@/lib/link-inventory.json";
import { SECTION } from "@/lib/nav-items";
import { getNewsItems } from "@/lib/news";
import type { SearchEntry } from "@/lib/site-search";

// Builds the search index from what already exists: the umbrella pages and
// their titles from the section menu, the anchors from link-inventory.json
// (so every hit is a link the link guard checks), and the news titles.
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
// ponytail: regex over the source, not a render. A heading whose text is an
// expression ({name}) gets no label and is left out of the index.
function headingLabels(dir: string): Map<string, string> {
  const labels = new Map<string, string>();
  const sources = fs
    .readdirSync(dir)
    .filter((f) => /\.tsx?$/.test(f) && !/\.(test|stories)\.tsx?$/.test(f))
    .map((f) => fs.readFileSync(path.join(dir, f), "utf-8"));
  for (const src of sources) {
    for (const m of src.matchAll(/<(LinkableHeading|Heading|h[1-6])\b((?:[^>"]|"[^"]*")*)>([\s\S]*?)<\/\1>/g)) {
      const children = m[3].replace(/\{" "\}/g, " ");
      if (children.includes("{")) continue;
      // Our own source, shown as text: split on tags, then drop any stray < or >.
      const text = children
        .split(/<[^>]*>/)
        .join("")
        .replace(/[<>]/g, "")
        .replace(/\s+/g, " ")
        .trim();
      const id = m[2].match(/\bid="([^"]+)"/)?.[1] ?? (m[1] === "LinkableHeading" ? slugify(text) : undefined);
      if (id && text) labels.set(id, text);
    }
    for (const m of src.matchAll(/\{\s*id:\s*"([^"]+)",\s*label:\s*"([^"]+)"/g)) labels.set(m[1], m[2]);
  }
  return labels;
}

export function buildSearchIndex(): SearchEntry[] {
  const pages: SearchEntry[] = SECTION.flatMap((g) => [
    ...(g.href ? [{ href: g.href, title: g.label, context: "nav-pilot" }] : []),
    ...(g.overview ? [{ href: g.overview, title: g.label, context: "nav-pilot" }] : []),
    ...(g.items ?? []).map((i) => ({ href: i.href, title: i.label, context: g.label })),
  ]).filter((p) => p.href in paths);

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
