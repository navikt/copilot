import { Suspense } from "react";
import { Heading, Box, VStack } from "@navikt/ds-react";
import { getAllCustomizations, getCountsByDomain } from "@/lib/customizations";
import type { Domain } from "@/lib/customization-types";
import { CustomizationCatalog } from "@/components/customization-catalog";
import { PageHero } from "@/components/page-hero";
import { getMcpServers } from "@/lib/mcp-registry";
import { getCachedCustomizationUsage } from "@/lib/cached-bigquery";
import { enrichWithUsage } from "@/lib/enrich-customizations";
import { DomainCards } from "./domain-cards";

export default async function CustomizationsPage() {
  const customizations = getAllCustomizations();
  const [mcpServers, { usage }] = await Promise.all([getMcpServers(), getCachedCustomizationUsage()]);
  const items = [...customizations, ...mcpServers].sort((a, b) => a.name.localeCompare(b.name, "nb"));
  const enrichedItems = enrichWithUsage(items, usage);
  const counts = getCountsByDomain(items);

  const domains = Object.entries(counts)
    .filter(([, count]) => count > 0)
    .map(([domain]) => domain) as Domain[];

  return (
    <main>
      <PageHero
        title="Verktøy"
        description="Agenter, instruksjoner, ferdigheter og MCP-servere som gjør Copilot smartere for Navs stack."
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
