import { Heading, Box, HGrid, VStack } from "@navikt/ds-react";
import { getAllCustomizations, getCountsByDomain } from "@/lib/customizations";
import type { Domain } from "@/lib/customization-types";
import { CustomizationCatalog } from "@/components/customization-catalog";
import { PageHeader } from "@/components/page-header";
import { DomainCards } from "./domain-cards";

export default function CustomizationsPage() {
  const items = getAllCustomizations();
  const counts = getCountsByDomain(items);

  const domains = Object.entries(counts)
    .filter(([, count]) => count > 0)
    .map(([domain]) => domain) as Domain[];

  return (
    <main className="max-w-7xl mx-auto">
      <Box
        paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
      >
        <VStack gap={{ xs: "space-24", md: "space-32" }}>
          <PageHeader
            title="Copilot-verktøy for Nav"
            description="Agenter, instruksjoner, prompts og ferdigheter som gjør GitHub Copilot smartere for Navs tekniske stack. Installer direkte i VS Code med ett klikk."
          />

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
    </main>
  );
}
