import { getNewsItems } from "@/lib/news";
import React from "react";
import { Box, VStack, Heading, HGrid, BodyShort } from "@navikt/ds-react";
import { ExternalLinkIcon, PadlockLockedIcon, PlayIcon, BookIcon } from "@navikt/aksel-icons";
import NextLink from "next/link";
import { NewsFeed } from "@/components/news-feed";
import { HighlightCards } from "@/components/pulse-strip";
import { HomeShortsFeed } from "@/components/home-shorts-feed";
import { Sidebar, SidebarCompact } from "@/components/sidebar";
import { NAV_ITEMS } from "@/lib/nav-items";
import { Greeting } from "@/components/greeting";
import { getUser } from "@/lib/auth";
import { getPublicVideoFeed } from "@/lib/public-videos";

export default async function Home() {
  const [user, videos] = await Promise.all([getUser(false), getPublicVideoFeed(5)]);
  const news = getNewsItems();

  return (
    <main>
      <section className="hero-gradient text-white">
        <Box
          paddingBlock={{ xs: "space-32", md: "space-40" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
          className="max-w-7xl mx-auto"
        >
          <VStack gap="space-16">
            <VStack gap="space-8">
              <Heading size="xlarge" level="1" className="hero-title hero-animate">
                Copilot i Nav
              </Heading>
              <BodyShort className="max-w-md opacity-70 hero-animate-d1">
                {user && <Greeting />}
                Nyheter, beste praksis og verktøy for AI-drevet utvikling i Nav.
              </BodyShort>
            </VStack>
            <div className="flex flex-wrap gap-2 hero-animate-d2">
              {NAV_ITEMS.map(({ href, icon: Icon, label, requiresAuth }) => (
                <NavPill
                  key={href}
                  href={href}
                  icon={<Icon aria-hidden fontSize="1rem" />}
                  label={label}
                  locked={requiresAuth}
                />
              ))}
            </div>
          </VStack>
        </Box>
      </section>

      <section className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap={{ xs: "space-24", md: "space-32" }}>
            <Box className="reveal-section">
              <HighlightCards />
            </Box>

            <Box className="reveal-section">
              <SidebarCompact />
              <div className="flex gap-8 lg:gap-10">
                <div className="flex-1 min-w-0">
                  <NewsFeed
                    items={news}
                    compact
                    afterFeatured={
                      videos.length > 0 ? (
                        <Box key="after-featured-shorts" className="reveal-section">
                          <HomeShortsFeed videos={videos} />
                        </Box>
                      ) : undefined
                    }
                  />
                </div>
                <div className="hidden lg:block w-64 shrink-0">
                  <Sidebar />
                </div>
              </div>
            </Box>

            <Box className="reveal-section">
              <Heading size="small" level="2" className="mb-4">
                Ressurser
              </Heading>
              <HGrid columns={{ xs: 1, sm: 2, md: 3 }} gap="space-12">
                <NavCard
                  href="/kom-i-gang"
                  icon={<PlayIcon aria-hidden />}
                  title="Kom i gang"
                  description="Alt du trenger for å starte med Copilot"
                />
                <NavCard
                  href="/praksis"
                  icon={<BookIcon aria-hidden />}
                  title="God praksis"
                  description="Mønstre og tips for effektiv AI-bruk"
                />
                <NavCard
                  href="https://docs.github.com/en/copilot"
                  icon={<ExternalLinkIcon aria-hidden />}
                  title="Dokumentasjon"
                  description="Offisiell dokumentasjon fra GitHub"
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

function NavPill({
  href,
  icon,
  label,
  locked,
}: {
  href: string;
  icon: React.ReactNode;
  label: string;
  locked?: boolean;
}) {
  return (
    <NextLink
      href={href}
      className="inline-flex items-center gap-1.5 px-4 py-2 rounded-full text-sm no-underline bg-white/10 text-white hover:bg-white/20 transition-colors"
    >
      {icon}
      {label}
      {locked && <PadlockLockedIcon aria-label="Krever innlogging" fontSize="0.75rem" className="opacity-60" />}
    </NextLink>
  );
}
