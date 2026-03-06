import fs from "fs";
import path from "path";
import matter from "gray-matter";

export type NewsCategory = "copilot" | "nav" | "praksis";

export interface NewsItem {
  slug: string;
  title: string;
  date: string;
  category: NewsCategory;
  excerpt: string;
  tags: string[];
  type: "link" | "article";
  url?: string;
  content?: string;
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
    title: data.title,
    date: data.date instanceof Date ? data.date.toISOString().split("T")[0] : data.date,
    category: data.category ?? "copilot",
    excerpt: data.excerpt ?? "",
    tags: data.tags ?? [],
    type: hasUrl && !hasContent ? "link" : "article",
    url: data.url,
  };
}

export function getNewsItems(): NewsItem[] {
  if (!fs.existsSync(articlesDir)) return [];

  const files = fs.readdirSync(articlesDir).filter((f) => f.endsWith(".md"));
  return files.map(parseNewsFile).sort((a, b) => b.date.localeCompare(a.date));
}

export function getArticle(slug: string): (NewsItem & { content: string }) | null {
  const filePath = path.join(articlesDir, `${slug}.md`);
  if (!fs.existsSync(filePath)) return null;

  const raw = fs.readFileSync(filePath, "utf-8");
  const { data, content } = matter(raw);

  if (content.trim().length === 0) return null;

  return {
    slug,
    title: data.title,
    date: data.date instanceof Date ? data.date.toISOString().split("T")[0] : data.date,
    category: data.category ?? "copilot",
    excerpt: data.excerpt ?? "",
    tags: data.tags ?? [],
    type: "article",
    url: data.url,
    content,
  };
}

export function getArticleSlugs(): string[] {
  if (!fs.existsSync(articlesDir)) return [];

  const files = fs.readdirSync(articlesDir).filter((f) => f.endsWith(".md"));
  return files
    .map((f) => {
      const slug = f.replace(/\.md$/, "");
      const filePath = path.join(articlesDir, f);
      const raw = fs.readFileSync(filePath, "utf-8");
      const { content } = matter(raw);
      return content.trim().length > 0 ? slug : null;
    })
    .filter((s): s is string => s !== null);
}

export const CATEGORY_CONFIG: Record<NewsCategory, { label: string; variant: "info" | "success" | "warning" }> = {
  copilot: { label: "Copilot", variant: "info" },
  nav: { label: "Nav", variant: "success" },
  praksis: { label: "Praksis", variant: "warning" },
};
