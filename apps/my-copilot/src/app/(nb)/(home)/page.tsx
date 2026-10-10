import { getNewsItems } from "@/lib/news";
import { Box, VStack, Heading, HGrid, BodyShort } from "@navikt/ds-react";
import { ExternalLinkIcon, PlayIcon, BookIcon } from "@navikt/aksel-icons";
import { NewsFeed } from "@/components/news-feed";
import { InsightStrip } from "@/components/insight-strip";
import { WeeklyTip } from "@/components/weekly-tip";
import { HomeShortsFeed } from "@/components/video/home-shorts-feed";
import { Sidebar, SidebarCompact } from "@/components/sidebar";
import { Greeting } from "@/components/greeting";
import { getUser } from "@/lib/auth";
import { getPublicVideoFeed } from "@/lib/public-videos";
import { NavCard } from "@/components/navigation/nav-card";
import { HeroNetwork } from "@/components/hero-network/hero-network";
import { getAllCustomizations } from "@/lib/customizations";
import { getMcpServers } from "@/lib/mcp-registry";

export default async function Home() {
  const [user, videos, mcpServers] = await Promise.all([getUser(false), getPublicVideoFeed(5), getMcpServers()]);
  const items = getAllCustomizations();
  const count = (type: string) => items.filter((i) => i.type === type).length;
  const clusters = [
    { name: "Agenter", count: count("agent") },
    { name: "Skills", count: count("skill") },
    { name: "Instruksjoner", count: count("instruction") },
    { name: "MCP-servere", count: mcpServers.length },
  ];
  // English articles sit in the same feed as the Norwegian ones, sorted by date
  // like everything else. A separate link beside the category chips read as a
  // filter that did nothing, and the piece most worth reading was the one the
  // front page did not show.
  const news = [...getNewsItems({ frontPage: true }), ...getNewsItems({ lang: "en", frontPage: true })].sort(
    (a, b) => Number(b.featured ?? false) - Number(a.featured ?? false) || b.date.localeCompare(a.date)
  );

  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <section className="hero-gradient text-white overflow-hidden max-md:min-h-[19rem]">
        <HeroNetwork clusters={clusters} />
        <Box
          paddingBlock={{ xs: "space-32", md: "space-40" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
          className="max-w-7xl mx-auto relative"
        >
          <VStack gap="space-8">
            <Heading size="xlarge" level="1" className="hero-title hero-animate">
              KI-utvikling i Nav
            </Heading>
            <BodyShort className="max-w-md opacity-70 hero-animate-d1">
              {user && <Greeting />}
              Nyheter, beste praksis og verktøy for KI-drevet utvikling i Nav.
            </BodyShort>
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
              <InsightStrip />
            </Box>

            <Box className="reveal-section">
              <WeeklyTip />
            </Box>

            <Box className="reveal-section">
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
              <SidebarCompact />
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
                  description="Start i terminalen med Copilot CLI, nav-pilot og cplt"
                />
                <NavCard
                  href="/praksis"
                  icon={<BookIcon aria-hidden fontSize="1.75rem" />}
                  title="God praksis"
                  description="Mønstre og tips for effektiv KI-bruk"
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
