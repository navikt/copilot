import type { Metadata } from "next";
import { notFound, redirect } from "next/navigation";
import { getArticle, getArticleSlugs, getLinkTarget } from "@/lib/news";
import { ArticleView } from "@/components/article-view";

interface Props {
  params: Promise<{ slug: string }>;
}

export function generateStaticParams() {
  return getArticleSlugs().map((slug) => ({ slug }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const article = getArticle(slug);

  if (!article) {
    return {
      title: "Artikkel ikke funnet",
    };
  }

  return {
    title: article.title,
    description: article.excerpt,
    openGraph: {
      title: article.title,
      description: article.excerpt,
      type: "article",
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

export default async function ArticlePage({ params }: Props) {
  const { slug } = await params;
  const article = getArticle(slug);

  if (!article) {
    const linkTarget = getLinkTarget(slug);
    if (linkTarget) redirect(linkTarget);
    notFound();
  }

  return (
    <ArticleView
      article={article}
      lang="nb"
      labels={{
        backToIndex: "Nyheter",
        backToIndexHref: "/",
        backToAll: "Alle nyheter",
        backToAllHref: "/",
      }}
    />
  );
}
