import { getNewsItems } from "@/lib/news";
import { Box, VStack, Heading, HGrid, BodyShort } from "@navikt/ds-react";
import { ExternalLinkIcon, PlayIcon, BookIcon } from "@navikt/aksel-icons";
import { NewsFeed } from "@/components/news-feed";
import { HighlightCards } from "@/components/pulse-strip";
import { HomeShortsFeed } from "@/components/video/home-shorts-feed";
import { Sidebar, SidebarCompact } from "@/components/sidebar";
import { NAV_ITEMS } from "@/lib/nav-items";
import { Greeting } from "@/components/greeting";
import { getUser } from "@/lib/auth";
import { getPublicVideoFeed } from "@/lib/public-videos";
import { NavCard } from "@/components/navigation/nav-card";
import { NavPill } from "@/components/navigation/nav-pill";

export default async function Home() {
  const [user, videos] = await Promise.all([getUser(false), getPublicVideoFeed(5)]);
  const news = getNewsItems({ frontPage: true });

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
                  icon={<PlayIcon aria-hidden fontSize="1.75rem" />}
                  title="Kom i gang"
                  description="Alt du trenger for å starte med Copilot"
                />
                <NavCard
                  href="/praksis"
                  icon={<BookIcon aria-hidden fontSize="1.75rem" />}
                  title="God praksis"
                  description="Mønstre og tips for effektiv AI-bruk"
                />
                <NavCard
                  href="https://docs.github.com/en/copilot"
                  icon={<ExternalLinkIcon aria-hidden fontSize="1.75rem" />}
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
