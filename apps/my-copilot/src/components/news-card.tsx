import { Box, BodyShort, Heading, Tag, HStack } from "@navikt/ds-react";
import { ExternalLinkIcon } from "@navikt/aksel-icons";
import NextLink from "next/link";
import type { NewsItem } from "@/lib/news-types";
import { CATEGORY_CONFIG } from "@/lib/news-types";
import { formatDate } from "@/lib/format";

function safeHref(url: string): string {
  try {
    const parsed = new URL(url, "https://nav.no");
    if (parsed.protocol === "https:" || parsed.protocol === "http:") return url;
  } catch {}
  return "#";
}

const DEFAULT_CATEGORY_CONFIG = { label: "Annet", variant: "info" as const };

function AuthorAvatar({ author }: { author: string }) {
  return (
    <span className="flex items-center gap-1.5">
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={`https://github.com/${author}.png?size=32`} alt="" width={16} height={16} className="rounded-full" />
      <BodyShort size="small" className="text-text-subtle">
        {author}
      </BodyShort>
    </span>
  );
}

export function NewsCard({ item, span = 1 }: { item: NewsItem; span?: number }) {
  const categoryConfig = CATEGORY_CONFIG[item.category] ?? DEFAULT_CATEGORY_CONFIG;
  const isLink = item.type === "link";
  const href = isLink ? safeHref(item.url!) : `/nyheter/${item.slug}`;
  const isExternal = isLink && !href.startsWith("/");
  const linkProps = isExternal ? { target: "_blank" as const, rel: "noopener noreferrer" } : {};
  const isWide = span >= 2;

  // Static Tailwind classes so they survive purge; map span -> column span.
  const spanClass = span >= 3 ? "sm:col-span-2 md:col-span-3" : isWide ? "sm:col-span-2 md:col-span-2" : undefined;

  return (
    <Box
      borderColor="neutral"
      borderWidth="1"
      borderRadius={isWide ? "12" : "8"}
      padding={isWide ? { xs: "space-20", md: "space-24" } : "space-16"}
      className={spanClass}
      asChild
    >
      <NextLink
        href={href}
        {...linkProps}
        className={`no-underline hover:shadow-md transition-shadow news-card-${item.category}`}
      >
        <div className="flex flex-col gap-3 h-full">
          <HStack gap="space-4" align="center" wrap>
            <Tag size="small" variant={categoryConfig.variant}>
              {categoryConfig.label}
            </Tag>
            <BodyShort size="small" className="text-text-subtle">
              {formatDate(item.date)}
            </BodyShort>
            {item.author && <AuthorAvatar author={item.author} />}
          </HStack>
          <Heading size={isWide ? "small" : "xsmall"} level="3">
            <span className="flex items-center gap-2">
              {item.title}
              {isExternal && <ExternalLinkIcon aria-hidden fontSize="1rem" className="shrink-0" />}
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
  const categoryConfig = CATEGORY_CONFIG[item.category] ?? DEFAULT_CATEGORY_CONFIG;
  const isLink = item.type === "link";
  const href = isLink ? safeHref(item.url!) : `/nyheter/${item.slug}`;
  const isExternal = isLink && !href.startsWith("/");
  const linkProps = isExternal ? { target: "_blank" as const, rel: "noopener noreferrer" } : {};

  return (
    <Box background="neutral-soft" borderRadius="12" padding={{ xs: "space-20", md: "space-32" }} asChild>
      <NextLink href={href} {...linkProps} className="no-underline hover:shadow-lg transition-shadow featured-card">
        <div className="flex flex-col gap-5">
          <HStack gap="space-4" align="center" wrap>
            <Tag size="small" variant={categoryConfig.variant}>
              {categoryConfig.label}
            </Tag>
            <BodyShort size="small" className="text-text-subtle">
              {formatDate(item.date)}
            </BodyShort>
            {item.author && <AuthorAvatar author={item.author} />}
          </HStack>
          <Heading size="medium" level="2">
            <span className="flex items-center gap-2">
              {item.title}
              {isExternal && <ExternalLinkIcon aria-hidden fontSize="1.25rem" className="shrink-0" />}
            </span>
          </Heading>
          <BodyShort className="text-text-subtle">{item.excerpt}</BodyShort>
        </div>
      </NextLink>
    </Box>
  );
}
