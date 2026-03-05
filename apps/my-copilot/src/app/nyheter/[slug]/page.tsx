import { Box, VStack, Heading, BodyShort, BodyLong, Tag, HStack } from "@navikt/ds-react";
import { notFound } from "next/navigation";
import { getArticle, getArticleSlugs, CATEGORY_CONFIG } from "@/lib/news";
import NextLink from "next/link";
import Markdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { ArrowLeftIcon } from "@navikt/aksel-icons";
import { formatDate } from "@/lib/format";

const markdownComponents: Components = {
  h2: ({ children }) => (
    <Heading size="medium" level="2" spacing>
      {children}
    </Heading>
  ),
  h3: ({ children }) => (
    <Heading size="small" level="3" spacing>
      {children}
    </Heading>
  ),
  p: ({ children }) => <BodyLong spacing>{children}</BodyLong>,
};

interface Props {
  params: Promise<{ slug: string }>;
}

export function generateStaticParams() {
  return getArticleSlugs().map((slug) => ({ slug }));
}

export default async function ArticlePage({ params }: Props) {
  const { slug } = await params;
  const article = getArticle(slug);

  if (!article) notFound();

  const categoryConfig = CATEGORY_CONFIG[article.category];

  return (
    <main>
      <div className="max-w-3xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        >
          <VStack gap="space-16">
            <NextLink
              href="/"
              className="inline-flex items-center gap-1.5 text-sm text-text-subtle no-underline hover:underline"
            >
              <ArrowLeftIcon aria-hidden fontSize="1rem" />
              Nyheter
            </NextLink>

            <VStack gap="space-8">
              <HStack gap="space-4" align="center">
                <Tag size="small" variant={categoryConfig.variant}>
                  {categoryConfig.label}
                </Tag>
                <BodyShort size="small" className="text-text-subtle">
                  {formatDate(article.date)}
                </BodyShort>
              </HStack>
              <Heading size="xlarge" level="1">
                {article.title}
              </Heading>
            </VStack>

            <article className="prose max-w-none">
              <Markdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
                {article.content}
              </Markdown>
            </article>

            <NextLink href="/" className="inline-flex items-center gap-1.5 text-sm no-underline hover:underline py-2">
              <ArrowLeftIcon aria-hidden fontSize="1rem" />
              Alle nyheter
            </NextLink>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
