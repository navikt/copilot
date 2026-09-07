import { Box, VStack, Heading, BodyShort, BodyLong, Tag, HStack } from "@navikt/ds-react";
import { CATEGORY_CONFIG, type NewsItem, type NewsLang } from "@/lib/news";
import NextLink from "next/link";
import Markdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { ArrowLeftIcon } from "@navikt/aksel-icons";
import { formatDate } from "@/lib/format";

const markdownComponents: Components = {
  // No article has used an image before this one, so react-markdown's bare <img>
  // was never styled: it would render at its intrinsic width and overflow a phone.
  // A figure in an article is data, so the alt text has to carry what the picture
  // says rather than name it, and the table beside it stays as the readable form
  // of the same numbers.
  img: ({ src, alt }) =>
    typeof src === "string" ? (
      // eslint-disable-next-line @next/next/no-img-element
      <img src={src} alt={alt ?? ""} style={{ maxWidth: "100%", height: "auto", margin: "1.5rem 0" }} />
    ) : null,
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
  ul: ({ children }) => <ul>{children}</ul>,
  ol: ({ children }) => <ol>{children}</ol>,
  li: ({ children }) => <li>{children}</li>,
  pre: ({ children }) => <pre>{children}</pre>,
  code: ({ children, className }) => {
    const isBlock = className?.startsWith("language-");
    return isBlock ? <code className={className}>{children}</code> : <code>{children}</code>;
  },
};

export interface ArticleLabels {
  backToIndex: string;
  backToIndexHref: string;
  backToAll: string;
  backToAllHref: string;
  backToAllLang?: string;
}

export function ArticleView({
  article,
  lang,
  labels,
}: {
  article: NewsItem & { content: string };
  lang: NewsLang;
  labels: ArticleLabels;
}) {
  const categoryConfig = CATEGORY_CONFIG[article.category] ?? { label: article.category, variant: "info" as const };

  return (
    <main>
      <div className="max-w-3xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32" }}
        >
          <VStack gap="space-16">
            <NextLink
              href={labels.backToIndexHref}
              className="inline-flex items-center gap-1.5 text-sm text-text-subtle no-underline hover:underline"
            >
              <ArrowLeftIcon aria-hidden fontSize="1rem" />
              {labels.backToIndex}
            </NextLink>

            <VStack gap="space-8">
              <HStack gap="space-4" align="center">
                <Tag size="small" variant={categoryConfig.variant}>
                  <span lang="nb">{categoryConfig.label}</span>
                </Tag>
                <BodyShort size="small" className="text-text-subtle">
                  {formatDate(article.date, lang === "en" ? "en-GB" : "nb-NO")}
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

            <NextLink
              href={labels.backToAllHref}
              hrefLang={labels.backToAllLang}
              className="inline-flex items-center gap-1.5 text-sm no-underline hover:underline py-2"
            >
              <ArrowLeftIcon aria-hidden fontSize="1rem" />
              {labels.backToAll}
            </NextLink>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
