import { Heading, Box, HGrid, VStack } from "@navikt/ds-react";
import { getAllCustomizations, getCountsByDomain } from "@/lib/customizations";
import type { Domain } from "@/lib/customization-types";
import { CustomizationCatalog } from "@/components/customization-catalog";
import { PageHero } from "@/components/page-hero";
import { getMcpServers } from "@/lib/mcp-registry";
import { DomainCards } from "./domain-cards";

export default async function CustomizationsPage() {
  const customizations = getAllCustomizations();
  const mcpServers = await getMcpServers();
  const items = [...customizations, ...mcpServers];
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
              <Heading size="medium" level="2" className="mb-4">
                Utforsk etter domene
              </Heading>
              <HGrid columns={{ xs: 2, sm: 3, md: 3, lg: 6 }} gap={{ xs: "space-8", md: "space-12" }}>
                {domains.map((domain) => (
                  <DomainCards key={domain} domain={domain} count={counts[domain]} />
                ))}
              </HGrid>
            </Box>

            <Box id="catalog">
              <Heading size="medium" level="2" className="mb-4">
                Alle tilpasninger
              </Heading>
              <CustomizationCatalog items={items} />
            </Box>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
