import { Box, VStack, Heading, BodyShort, BodyLong, HStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { getNewsItems } from "@/lib/news";
import { formatDate } from "@/lib/format";

export const metadata: Metadata = {
  title: "Articles in English",
  description: "Articles in English from Nav's AI-assisted development team.",
};

export default function EnglishNewsIndex() {
  // Link items point at someone else's page and have no body of their own,
  // so /en/news/<slug> would 404 for them.
  const items = getNewsItems({ lang: "en" }).filter((item) => item.type === "article");

  return (
    <main>
      <div className="max-w-3xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32" }}
        >
          <VStack gap="space-16">
            <VStack gap="space-8">
              <Heading size="xlarge" level="1">
                Articles in English
              </Heading>
              <BodyLong>
                Nav builds and runs public welfare systems, and these are notes from doing that with AI coding tools.
                Most of what we publish is in Norwegian.{" "}
                <NextLink href="/" hrefLang="nb" lang="nb">
                  Oh-My-Nav
                </NextLink>{" "}
                has the rest.
              </BodyLong>
            </VStack>

            {items.length === 0 ? (
              <BodyShort>Nothing here yet.</BodyShort>
            ) : (
              <VStack gap="space-16" as="ul" className="list-none p-0">
                {items.map((item) => (
                  <li key={item.slug}>
                    <VStack gap="space-4">
                      <HStack gap="space-8" align="center">
                        <BodyShort size="small" className="text-text-subtle">
                          {formatDate(item.date, "en-GB")}
                        </BodyShort>
                      </HStack>
                      <Heading size="small" level="2">
                        <NextLink href={`/en/news/${encodeURIComponent(item.slug)}`} className="text-text-action">
                          {item.title}
                        </NextLink>
                      </Heading>
                      {item.excerpt ? <BodyLong>{item.excerpt}</BodyLong> : null}
                    </VStack>
                  </li>
                ))}
              </VStack>
            )}
          </VStack>
        </Box>
      </div>
    </main>
  );
}
