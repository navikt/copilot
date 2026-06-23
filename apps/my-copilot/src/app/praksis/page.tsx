import { Box } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import { categories } from "./data";
import { PraksisHub } from "@/components/nav-pilot/praksis-hub";

export default async function BestPractices() {
  const clientCategories = categories.map((cat) => ({
    title: cat.title,
    description: cat.description,
    guides: cat.guides.map((g) => ({
      id: g.id,
      title: g.title,
      description: g.description,
      keywords: g.keywords,
      iconName: g.iconName,
    })),
  }));

  return (
    <main>
      <PageHero
        title="God praksis og guider"
        description="Lær å bruke GitHub Copilot effektivt og trygt. Finn oppskriften på din utfordring."
      />
      <div className="max-w-5xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20" }}
        >
          <PraksisHub categories={clientCategories} />
        </Box>
      </div>
    </main>
  );
}
