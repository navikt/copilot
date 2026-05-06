import { PageHero } from "@/components/page-hero";
import { Box, VStack, Heading } from "@navikt/ds-react";
import { Glossary } from "./glossary";
import { terms } from "./terms";
import { TrustBoundaryDiagram } from "@/components/trust-boundary-diagram";

export default function OrdlistePage() {
  return (
    <main>
      <PageHero
        title="Ordliste"
        description="Enkle forklaringer på begreper brukt i forbindelse med GitHub Copilot og AI-assistert utvikling."
      />
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <VStack gap={{ xs: "space-24", md: "space-32" }}>
          <section>
            <VStack gap="space-16">
              <Heading size="medium" level="2">
                Arkitektur og tillitsgrenser
              </Heading>
              <TrustBoundaryDiagram />
            </VStack>
          </section>
          <Glossary terms={terms} />
        </VStack>
      </Box>
    </main>
  );
}
