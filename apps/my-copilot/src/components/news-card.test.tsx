import { render, screen } from "@testing-library/react";
import { NewsCard, FeaturedNewsCard } from "./news-card";
import type { NewsItem, NewsCategory } from "@/lib/news";

function makeItem(overrides: Partial<NewsItem> = {}): NewsItem {
  return {
    lang: "nb",
    slug: "test-article",
    title: "Test Title",
    date: "2025-06-01",
    draft: false,
    category: "copilot",
    excerpt: "Test excerpt",
    tags: [],
    type: "article",
    ...overrides,
  };
}

describe("NewsCard", () => {
  it("renders title and category tag", () => {
    render(<NewsCard item={makeItem()} />);

    expect(screen.getByText("Test Title")).toBeInTheDocument();
    expect(screen.getByText("Copilot")).toBeInTheDocument();
  });

  it.each(["copilot", "nav", "praksis"] as const)("renders %s category", (category) => {
    render(<NewsCard item={makeItem({ category })} />);

    expect(screen.getByRole("heading", { level: 3 })).toBeInTheDocument();
  });

  it("does not crash with unknown category", () => {
    const item = makeItem({ category: "unknown-category" as NewsCategory });

    expect(() => render(<NewsCard item={item} />)).not.toThrow();
    expect(screen.getByText("Annet")).toBeInTheDocument();
  });

  it("renders external link icon for link type", () => {
    render(<NewsCard item={makeItem({ type: "link", url: "https://example.com" })} />);

    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("links to article page for article type", () => {
    render(<NewsCard item={makeItem({ slug: "my-article" })} />);

    expect(screen.getByRole("link")).toHaveAttribute("href", "/nyheter/my-article");
  });
});

describe("FeaturedNewsCard", () => {
  it("renders title and category tag", () => {
    render(<FeaturedNewsCard item={makeItem()} />);

    expect(screen.getByText("Test Title")).toBeInTheDocument();
    expect(screen.getByText("Copilot")).toBeInTheDocument();
  });

  it("does not crash with unknown category", () => {
    const item = makeItem({ category: "tips" as NewsCategory });

    expect(() => render(<FeaturedNewsCard item={item} />)).not.toThrow();
    expect(screen.getByText("Annet")).toBeInTheDocument();
  });

  it("renders heading at level 2", () => {
    render(<FeaturedNewsCard item={makeItem()} />);

    expect(screen.getByRole("heading", { level: 2 })).toBeInTheDocument();
  });
});
