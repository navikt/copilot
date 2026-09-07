import type { Metadata } from "next";
import { notFound, redirect } from "next/navigation";
import { getArticle, getArticleSlugs, getLinkTarget } from "@/lib/news";
import { ArticleView } from "@/components/article-view";

interface Props {
  params: Promise<{ slug: string }>;
}

export function generateStaticParams() {
  return getArticleSlugs("en").map((slug) => ({ slug }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const article = getArticle(slug, "en");

  if (!article) {
    return { title: "Article not found" };
  }

  return {
    title: article.title,
    description: article.excerpt,
    openGraph: {
      title: article.title,
      description: article.excerpt,
      type: "article",
      locale: "en_GB",
      publishedTime: article.date,
      authors: article.author ? [article.author] : undefined,
      tags: article.tags,
    },
    twitter: {
      card: "summary_large_image",
      title: article.title,
      description: article.excerpt,
    },
  };
}

export default async function EnglishArticlePage({ params }: Props) {
  const { slug } = await params;
  const article = getArticle(slug, "en");

  if (!article) {
    const linkTarget = getLinkTarget(slug, "en");
    if (linkTarget) redirect(linkTarget);
    notFound();
  }

  return (
    <ArticleView
      article={article}
      lang="en"
      labels={{
        backToIndex: "Articles",
        backToIndexHref: "/en/news",
        backToAll: "All articles in Norwegian",
        backToAllHref: "/",
        backToAllLang: "nb",
      }}
    />
  );
}
