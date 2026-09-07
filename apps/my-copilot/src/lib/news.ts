import fs from "fs";
import path from "path";
import matter from "gray-matter";

export type { NewsCategory, NewsItem, NewsLang } from "./news-types";
export { CATEGORY_CONFIG } from "./news-types";

import type { NewsCategory, NewsItem, NewsLang } from "./news-types";

const VALID_CATEGORIES: Set<string> = new Set<string>(["copilot", "nav", "nav-pilot", "praksis", "oppsummering"]);
const OSLO_TIME_ZONE = "Europe/Oslo";
const EXTERNAL_FRESHNESS_DAYS = 5;
const DAY_IN_MS = 24 * 60 * 60 * 1000;
const osloDateFormatter = new Intl.DateTimeFormat("en-CA", {
  timeZone: OSLO_TIME_ZONE,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});

export interface GetNewsItemsOptions {
  frontPage?: boolean;
  now?: Date;
  lang?: NewsLang;
}

function isValidCategory(value: unknown): value is NewsCategory {
  return typeof value === "string" && VALID_CATEGORIES.has(value);
}

function parseCategory(value: unknown, slug: string): NewsCategory {
  if (isValidCategory(value)) return value;
  if (value !== undefined) {
    console.warn(`Unknown news category "${value}" in ${slug}.md, falling back to "copilot"`);
  }
  return "copilot";
}

// Articles default to Norwegian: the site was Norwegian long before the first
// English one, and none of the existing files carry a lang field.
// A slug becomes a URL path segment, so it may only hold what a filename is
// allowed to contribute. Anything else is a file that could not be routed to
// anyway, and leaving it unchecked lets file content reach an href.
const SLUG_RE = /^[a-z0-9][a-z0-9-]*$/;

export function isValidSlug(slug: string): boolean {
  return SLUG_RE.test(slug);
}

function parseLang(value: unknown): NewsLang {
  return value === "en" ? "en" : "nb";
}

function toDateOnly(value: unknown): string {
  if (value instanceof Date) return value.toISOString().split("T")[0];
  return typeof value === "string" ? value : "";
}

function toUtcDayTimestamp(value: string): number | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const timestamp = Date.UTC(year, month - 1, day);
  const parsed = new Date(timestamp);

  if (
    Number.isNaN(timestamp) ||
    parsed.getUTCFullYear() !== year ||
    parsed.getUTCMonth() !== month - 1 ||
    parsed.getUTCDate() !== day
  ) {
    return null;
  }

  return timestamp;
}

export function getCurrentOsloDate(now: Date = new Date()): string {
  const parts = osloDateFormatter.formatToParts(now);
  const year = parts.find((part) => part.type === "year")?.value;
  const month = parts.find((part) => part.type === "month")?.value;
  const day = parts.find((part) => part.type === "day")?.value;
  if (!year || !month || !day) return "";
  return `${year}-${month}-${day}`;
}

export function isExternalExcerptFresh(item: NewsItem, now: Date = new Date()): boolean {
  if (!item.url) return true;

  const nowTimestamp = toUtcDayTimestamp(getCurrentOsloDate(now));
  const publishedTimestamp = toUtcDayTimestamp(item.date);
  if (nowTimestamp === null || publishedTimestamp === null) {
    console.warn(`Invalid news date "${item.date}" for external excerpt "${item.slug}". Hiding from front page.`);
    return false;
  }

  const ageInDays = Math.floor((nowTimestamp - publishedTimestamp) / DAY_IN_MS);
  return ageInDays <= EXTERNAL_FRESHNESS_DAYS;
}

export function selectNewsItems(items: NewsItem[], options: GetNewsItemsOptions = {}): NewsItem[] {
  const inLang = items.filter((item) => item.lang === (options.lang ?? "nb"));
  const visibleItems = options.frontPage ? inLang.filter((item) => isExternalExcerptFresh(item, options.now)) : inLang;
  // A festet sak outranks the date so an item can be lifted back to the top
  // without back-dating it. Only the flag is consulted; among festede saker,
  // and among the rest, the ordinary date order still applies.
  return visibleItems.sort(
    (a, b) => Number(b.featured ?? false) - Number(a.featured ?? false) || b.date.localeCompare(a.date)
  );
}

function resolveArticlesDir(): string {
  const local = path.join(process.cwd(), "docs", "news", "articles");
  if (fs.existsSync(local)) return local;
  return path.join(process.cwd(), "..", "..", "docs", "news", "articles");
}

const articlesDir = resolveArticlesDir();

function parseNewsFile(fileName: string): NewsItem {
  const slug = fileName.replace(/\.md$/, "");
  const filePath = path.join(articlesDir, fileName);
  const raw = fs.readFileSync(filePath, "utf-8");
  const { data, content } = matter(raw);

  const hasUrl = Boolean(data.url);
  const hasContent = content.trim().length > 0;

  return {
    slug,
    lang: parseLang(data.lang),
    title: data.title,
    date: toDateOnly(data.date),
    draft: data.draft === true,
    featured: data.featured === true,
    category: parseCategory(data.category, slug),
    excerpt: data.excerpt ?? "",
    tags: data.tags ?? [],
    type: hasUrl && !hasContent ? "link" : "article",
    url: data.url,
    author: data.author,
  };
}

export function getNewsItems(options: GetNewsItemsOptions = {}): NewsItem[] {
  if (!fs.existsSync(articlesDir)) return [];

  const files = fs
    .readdirSync(articlesDir)
    .filter((f) => f.endsWith(".md"))
    .filter((f) => isValidSlug(f.replace(/\.md$/, "")));
  return selectNewsItems(
    files.map(parseNewsFile).filter((item) => !item.draft),
    options
  );
}

// The language is checked here rather than at each call site: a route that
// forgets the check would serve an article under the wrong lang attribute, and
// a draft would be reachable by guessing its URL.
export function getArticle(slug: string, lang: NewsLang = "nb"): (NewsItem & { content: string }) | null {
  if (!isValidSlug(slug)) return null;

  const filePath = path.join(articlesDir, `${slug}.md`);
  if (!fs.existsSync(filePath)) return null;

  const raw = fs.readFileSync(filePath, "utf-8");
  const { data, content } = matter(raw);

  if (content.trim().length === 0) return null;
  if (data.draft === true) return null;
  if (parseLang(data.lang) !== lang) return null;

  return {
    slug,
    lang: parseLang(data.lang),
    title: data.title,
    date: toDateOnly(data.date),
    draft: data.draft === true,
    featured: data.featured === true,
    category: parseCategory(data.category, slug),
    excerpt: data.excerpt ?? "",
    tags: data.tags ?? [],
    type: "article",
    url: data.url,
    author: data.author,
    content,
  };
}

// A news item that is only a link has no page of its own. The URL still gets
// shared, so the route sends the reader to the source rather than answering
// 404 for a slug we can resolve.
export function getLinkTarget(slug: string, lang: NewsLang = "nb"): string | null {
  if (!isValidSlug(slug)) return null;

  const item = getNewsItems({ lang }).find((candidate) => candidate.slug === slug);
  return item?.type === "link" && item.url ? item.url : null;
}

export function getArticleSlugs(lang: NewsLang = "nb"): string[] {
  return getNewsItems({ lang })
    .filter((item) => item.type === "article")
    .map((item) => item.slug);
}
