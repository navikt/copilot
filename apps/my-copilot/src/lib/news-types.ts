export type NewsCategory = "copilot" | "nav" | "nav-pilot" | "praksis" | "oppsummering";

export interface NewsItem {
  slug: string;
  title: string;
  date: string;
  draft: boolean;
  category: NewsCategory;
  excerpt: string;
  tags: string[];
  type: "link" | "article";
  url?: string;
  content?: string;
  author?: string;
}

export const CATEGORY_CONFIG: Record<
  NewsCategory,
  { label: string; variant: "info" | "success" | "warning" | "neutral" }
> = {
  copilot: { label: "Copilot", variant: "info" },
  nav: { label: "Nav", variant: "success" },
  "nav-pilot": { label: "Nav-pilot", variant: "info" },
  praksis: { label: "Praksis", variant: "warning" },
  oppsummering: { label: "Oppsummering", variant: "neutral" },
};
