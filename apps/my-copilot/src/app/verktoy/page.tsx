import { Suspense } from "react";
import type { Metadata } from "next";
import { Heading, Box, VStack } from "@navikt/ds-react";
import { getAllCustomizations, getCountsByDomain, getCustomizationById } from "@/lib/customizations";
import type { Domain } from "@/lib/customization-types";
import { TYPE_LABELS } from "@/lib/customization-types";
import { CustomizationCatalog } from "@/components/customization-catalog";
import { PageHero } from "@/components/page-hero";
import { getMcpServers } from "@/lib/mcp-registry";
import { getCustomizationUsage } from "@/lib/cached-bigquery";
import { getUserToken, getUser } from "@/lib/auth";
import { enrichWithUsage } from "@/lib/enrich-customizations";
import { DomainCards } from "./domain-cards";
import type { CustomizationUsage } from "@/lib/types";

interface Props {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}

export async function generateMetadata({ searchParams }: Props): Promise<Metadata> {
  const params = await searchParams;
  const itemId = typeof params.item === "string" ? params.item : undefined;

  if (itemId) {
    const item = getCustomizationById(itemId);
    if (item) {
      const typeLabel = TYPE_LABELS[item.type];
      const title = `${item.name} — ${typeLabel}`;
      const description = item.description;

      return {
        title,
        description,
        openGraph: {
          title: `${item.name} — ${typeLabel} for GitHub Copilot`,
          description,
          type: "website",
        },
        twitter: {
          card: "summary_large_image",
          title: `${item.name} — ${typeLabel} for GitHub Copilot`,
          description,
        },
      };
    }
  }

  return {
    title: "Verktøy — Copilot-tilpasninger for Nav",
    description: "Agenter, instruksjoner, skills og MCP-servere som gjør GitHub Copilot smartere for Navs stack.",
    openGraph: {
      title: "Verktøy — Copilot-tilpasninger for Nav",
      description: "Agenter, instruksjoner, skills og MCP-servere som gjør GitHub Copilot smartere for Navs stack.",
      type: "website",
    },
    twitter: {
      card: "summary_large_image",
      title: "Verktøy — Copilot-tilpasninger for Nav",
      description: "Agenter, instruksjoner, skills og MCP-servere som gjør GitHub Copilot smartere for Navs stack.",
    },
  };
}

export default async function CustomizationsPage() {
  const user = await getUser(false);
  const customizations = getAllCustomizations();
  const token = user ? await getUserToken() : null;
  const [mcpServers, usageResult] = await Promise.all([
    getMcpServers(),
    token ? getCustomizationUsage(token) : Promise.resolve({ usage: [] as CustomizationUsage[], error: null }),
  ]);
  const items = [...customizations, ...mcpServers].sort((a, b) => a.name.localeCompare(b.name, "nb"));
  const enrichedItems = enrichWithUsage(items, usageResult.usage);
  const counts = getCountsByDomain(items);

  const domains = Object.entries(counts)
    .filter(([, count]) => count > 0)
    .map(([domain]) => domain) as Domain[];

  return (
    <main>
      <PageHero
        title="Verktøy"
        description="Agenter, instruksjoner, skills og MCP-servere som gjør Copilot smartere for Navs stack."
      />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap={{ xs: "space-24", md: "space-32" }}>
            <Box>
              <Heading size="small" level="2" className="mb-4">
                Utforsk etter domene
              </Heading>
              <div className="flex gap-3 overflow-x-auto pb-2 scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent">
                {domains.map((domain) => (
                  <div key={domain} className="shrink-0 w-44">
                    <DomainCards domain={domain} count={counts[domain]} />
                  </div>
                ))}
              </div>
            </Box>

            <Box id="catalog">
              <Heading size="small" level="2" className="mb-4">
                Alle tilpasninger
              </Heading>
              <Suspense>
                <CustomizationCatalog items={enrichedItems} />
              </Suspense>
            </Box>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
