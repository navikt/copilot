import { Box, HGrid, BodyShort } from "@navikt/ds-react";
import { getNewsItems } from "@/lib/news";
import { NewsCard } from "@/components/news-card";
import { PageHero } from "@/components/page-hero";

export default function NyheterPage() {
  const items = getNewsItems();

  return (
    <main>
      <PageHero title="Nyheter" description="Siste nytt om GitHub Copilot og AI-drevet utvikling i Nav." />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          {items.length === 0 ? (
            <BodyShort>Ingen nyheter ennå.</BodyShort>
          ) : (
            <HGrid columns={{ xs: 1, sm: 2, md: 3 }} gap="space-12">
              {items.map((item) => (
                <NewsCard key={item.slug} item={item} />
              ))}
            </HGrid>
          )}
        </Box>
      </div>
    </main>
  );
}
