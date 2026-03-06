import { Box, BodyShort, Heading, Tag, HStack } from "@navikt/ds-react";
import { ExternalLinkIcon } from "@navikt/aksel-icons";
import NextLink from "next/link";
import type { NewsItem } from "@/lib/news";
import { CATEGORY_CONFIG } from "@/lib/news";
import { formatDate } from "@/lib/format";

function safeHref(url: string): string {
  try {
    const parsed = new URL(url, "https://nav.no");
    if (parsed.protocol === "https:" || parsed.protocol === "http:") return url;
  } catch {}
  return "#";
}

export function NewsCard({ item }: { item: NewsItem }) {
  const categoryConfig = CATEGORY_CONFIG[item.category];
  const isLink = item.type === "link";
  const href = isLink ? safeHref(item.url!) : `/nyheter/${item.slug}`;
  const linkProps = isLink ? { target: "_blank" as const, rel: "noopener noreferrer" } : {};

  return (
    <Box borderColor="neutral" borderWidth="1" borderRadius="8" padding="space-16" asChild>
      <NextLink
        href={href}
        {...linkProps}
        className={`no-underline hover:shadow-md transition-shadow news-card-${item.category}`}
      >
        <div className="flex flex-col gap-3 h-full">
          <HStack gap="space-4" align="center">
            <Tag size="small" variant={categoryConfig.variant}>
              {categoryConfig.label}
            </Tag>
            <BodyShort size="small" className="text-text-subtle">
              {formatDate(item.date)}
            </BodyShort>
          </HStack>
          <Heading size="xsmall" level="3">
            <span className="flex items-center gap-2">
              {item.title}
              {isLink && <ExternalLinkIcon aria-hidden fontSize="1rem" className="shrink-0" />}
            </span>
          </Heading>
          <BodyShort size="small" className="text-text-subtle line-clamp-2">
            {item.excerpt}
          </BodyShort>
        </div>
      </NextLink>
    </Box>
  );
}

export function FeaturedNewsCard({ item }: { item: NewsItem }) {
  const categoryConfig = CATEGORY_CONFIG[item.category];
  const isLink = item.type === "link";
  const href = isLink ? safeHref(item.url!) : `/nyheter/${item.slug}`;
  const linkProps = isLink ? { target: "_blank" as const, rel: "noopener noreferrer" } : {};

  return (
    <Box borderRadius="12" padding={{ xs: "space-20", md: "space-32" }} asChild>
      <NextLink href={href} {...linkProps} className="no-underline hover:shadow-lg transition-shadow featured-card">
        <div className="flex flex-col gap-5">
          <HStack gap="space-4" align="center">
            <Tag size="small" variant={categoryConfig.variant}>
              {categoryConfig.label}
            </Tag>
            <BodyShort size="small" className="text-text-subtle">
              {formatDate(item.date)}
            </BodyShort>
          </HStack>
          <Heading size="medium" level="2">
            <span className="flex items-center gap-2">
              {item.title}
              {isLink && <ExternalLinkIcon aria-hidden fontSize="1.25rem" className="shrink-0" />}
            </span>
          </Heading>
          <BodyShort className="text-text-subtle">{item.excerpt}</BodyShort>
        </div>
      </NextLink>
    </Box>
  );
}
