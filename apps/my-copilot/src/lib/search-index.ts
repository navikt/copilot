import fs from "node:fs";
import path from "node:path";
import { sourceHeadings } from "@/lib/page-headings";
import inventory from "@/lib/link-inventory.json";
import { activeTop, SECTION, TOP_LINKS } from "@/lib/nav-items";
import { allGuides } from "@/app/(nb)/praksis/data";
import { PRIVATE_ROUTES } from "../../scripts/auto-login-ignore-paths.mjs";
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
    // Section components that render their own heading: <InsightSection id="…" title="…">.
    for (const m of src.matchAll(/<\w*Section\s+id="([^"]+)"\s+title="([^"]+)"/g)) labels.set(m[1], m[2]);
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

// The page's title: the literal in its metadata, or the PageHero title.
// Anything after « — » or «:» is a subtitle and is left out.
function pageTitle(dir: string): string | undefined {
  const src = fs.readFileSync(path.join(dir, "page.tsx"), "utf-8");
  const metadata = src.match(/export const metadata\b[\s\S]*?\n\};/)?.[0];
  const title = metadata?.match(/\btitle:\s*"([^"]+)"/)?.[1] ?? src.match(/<PageHero[^>]*\btitle="([^"]+)"/)?.[1];
  return title?.split(/ — |: /)[0];
}

/** Routes behind a login. Only their titles and headings are indexed, never their data. */
const isPrivate = (href: string) => PRIVATE_ROUTES.some((r) => href === r || href.startsWith(r + "/"));

// /cplt/windows isn't in SECTION: it has no nav-pilot section menu of its own
// (AGENTS.md), and adding it as a SECTION item would put an English sub-link
// under "Sandkassen (cplt)" in every Norwegian section menu. Listed here
// instead, in the same shape a SECTION entry takes, so it's indexed like any
// other page (#1280).
const EXTRA_PAGES: { href: string; title: string; context: string }[] = [
  { href: "/cplt/windows", title: "cplt on Windows (WSL2)", context: "Sandkassen (cplt)" },
];

export function buildSearchIndex(): SearchEntry[] {
  const menuPages = SECTION.flatMap((g) => [
    ...(g.href ? [{ href: g.href, title: g.label, context: "nav-pilot" }] : []),
    ...(g.overview ? [{ href: g.overview, title: g.label, context: "nav-pilot" }] : []),
    ...(g.items ?? []).map((i) => ({ href: i.href, title: i.label, context: g.label })),
  ]).concat(EXTRA_PAGES);
  // Every other static page, so a new page is found without a list to update.
  // Its context is the header link it sits under.
  const otherPages = [...pageDirs.keys()]
    .filter((href) => !href.includes("[") && !href.startsWith("/nyheter/") && !menuPages.some((p) => p.href === href))
    .flatMap((href) => {
      const title = href === "/" ? "Forside" : pageTitle(pageDirs.get(href)!);
      const top = activeTop(href);
      const context = href.startsWith("/nyheter/")
        ? "Nyhet"
        : (TOP_LINKS.find((l) => l.href === top)?.nb ?? "ki-utvikling.nav.no");
      return title ? [{ href, title, context }] : [];
    });
  const guides = allGuides.map((g) => ({
    href: `/praksis/guide/${g.id}`,
    title: g.title,
    context: "Praksis og regler",
    text: g.description,
  }));

  const pages: SearchEntry[] = [...menuPages, ...otherPages]
    .filter((p) => p.href in paths)
    .map((p) => {
      const dir = pageDirs.get(p.href);
      const text = dir && pageDescription(dir);
      return { ...p, ...(text && { text }), ...(isPrivate(p.href) && { login: true as const }) };
    })
    .concat(guides.filter((g) => g.href in paths));

  const headings = pages.flatMap((p) => {
    const dir = pageDirs.get(p.href);
    if (!dir) return [];
    const labels = headingLabels(dir);
    return Object.keys(paths[p.href].anchors)
      .filter((id) => labels.has(id))
      .map((id) => ({
        href: `${p.href}#${id}`,
        title: labels.get(id)!,
        context: p.title,
        ...(p.login && { login: true as const }),
      }));
  });

  const news = getNewsItems({ lang: "nb" })
    .filter((n) => n.title && `/nyheter/${n.slug}` in paths)
    .map((n) => ({ href: `/nyheter/${n.slug}`, title: n.title, context: "Nyhet" }));

  return [...pages, ...headings, ...news.filter((n) => !pages.some((p) => p.href === n.href))];
}
