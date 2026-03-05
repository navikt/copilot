import { Heading, BodyShort, Box, HGrid, VStack } from "@navikt/ds-react";
import { getAllCustomizations, getCountsByDomain } from "@/lib/customizations";
import type { Domain } from "@/lib/customization-types";
import { CustomizationCatalog } from "@/components/customization-catalog";
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
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          {/* Hero */}
          <div className="relative">
            <div className="absolute inset-0 overflow-hidden rounded-xl -z-10">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src="/images/agents-on-github-hero-mission-control.jpeg"
                alt=""
                className="w-full h-full object-cover opacity-15"
              />
              <div className="absolute inset-0 bg-linear-to-r from-white via-white/90 to-transparent" />
            </div>
            <div className="py-2 sm:py-4">
              <Heading size="xlarge" level="1" className="mb-2">
                Copilot-verktøy for Nav
              </Heading>
              <BodyShort className="text-gray-600 max-w-2xl">
                Agenter, instruksjoner, prompts og ferdigheter som gjør GitHub Copilot smartere for Navs tekniske stack.
                Installer direkte i VS Code med ett klikk.
              </BodyShort>
            </div>
          </div>

          {/* Domain overview */}
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

          {/* Catalog */}
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
