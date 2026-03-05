import { getLatestNews } from "@/lib/news";
import React from "react";
import { Box, VStack, Heading, HGrid, BodyShort } from "@navikt/ds-react";
import { ExternalLinkIcon } from "@navikt/aksel-icons";
import NextLink from "next/link";
import { NewsCard, FeaturedNewsCard } from "@/components/news-card";
import { NAV_ITEMS } from "@/lib/nav-items";

export default function Home() {
  const news = getLatestNews(7);
  const featured = news[0];
  const rest = news.slice(1, 7);

  return (
    <main>
      <section className="hero-gradient text-white">
        <Box
          paddingBlock={{ xs: "space-24", md: "space-32" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
          className="max-w-7xl mx-auto"
        >
          <VStack gap="space-12">
            <VStack gap="space-8">
              <Heading size="xlarge" level="1">
                GitHub Copilot
              </Heading>
              <BodyShort className="max-w-2xl opacity-90">
                Nyheter, beste praksis og verktøy for AI-drevet utvikling i Nav.
              </BodyShort>
            </VStack>
            <div className="flex flex-wrap gap-2">
              {NAV_ITEMS.map(({ href, icon: Icon, label }) => (
                <NavPill key={href} href={href} icon={<Icon aria-hidden fontSize="1rem" />} label={label} />
              ))}
            </div>
          </VStack>
        </Box>
      </section>

      <section className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-24", md: "space-32" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap={{ xs: "space-24", md: "space-32" }}>
            <Box>
              <div className="flex items-center justify-between mb-4">
                <Heading size="medium" level="2">
                  Siste nytt
                </Heading>
                <NextLink href="/nyheter" className="text-sm hover:underline">
                  Se alle →
                </NextLink>
              </div>
              <VStack gap="space-12">
                {featured && <FeaturedNewsCard item={featured} />}
                {rest.length > 0 && (
                  <HGrid columns={{ xs: 1, sm: 2, md: 3 }} gap="space-12">
                    {rest.map((item) => (
                      <NewsCard key={item.slug} item={item} />
                    ))}
                  </HGrid>
                )}
              </VStack>
            </Box>

            <Box>
              <Heading size="medium" level="2" className="mb-4">
                Ressurser
              </Heading>
              <HGrid columns={{ xs: 1, sm: 2 }} gap="space-12">
                <NavCard
                  href="https://docs.github.com/en/copilot"
                  icon={<ExternalLinkIcon aria-hidden />}
                  title="GitHub Copilot Dokumentasjon"
                  description="Offisiell dokumentasjon fra GitHub"
                  external
                />
                <NavCard
                  href="https://utvikling.intern.nav.no/teknisk/github-copilot.html"
                  icon={<ExternalLinkIcon aria-hidden />}
                  title="Om GitHub Copilot i Nav"
                  description="Navs retningslinjer og oppsett"
                  external
                />
              </HGrid>
            </Box>
          </VStack>
        </Box>
      </section>
    </main>
  );
}

function NavCard({
  href,
  icon,
  title,
  description,
  external,
}: {
  href: string;
  icon: React.ReactNode;
  title: string;
  description: string;
  external?: boolean;
}) {
  const linkProps = external ? { target: "_blank", rel: "noopener noreferrer" } : {};
  return (
    <Box borderColor="neutral" borderWidth="1" borderRadius="8" padding="space-16" asChild>
      <NextLink href={href} {...linkProps} className="no-underline hover:shadow-md transition-shadow">
        <VStack gap="space-8">
          <Heading size="xsmall" level="3">
            <span className="flex items-center gap-2">
              {icon}
              {title}
            </span>
          </Heading>
          <span className="text-text-subtle text-sm">{description}</span>
        </VStack>
      </NextLink>
    </Box>
  );
}

function NavPill({ href, icon, label }: { href: string; icon: React.ReactNode; label: string }) {
  return (
    <NextLink
      href={href}
      className="inline-flex items-center gap-1.5 px-3 py-2 rounded-full text-sm no-underline border border-white/30 text-white hover:bg-white/20 transition-colors"
    >
      {icon}
      {label}
    </NextLink>
  );
}
