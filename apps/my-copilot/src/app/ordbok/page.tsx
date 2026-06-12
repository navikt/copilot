import type { Metadata } from "next";
import { PageHero } from "@/components/page-hero";
import { TrustBoundaryDiagram } from "@/components/trust-boundary-diagram";
import { BodyShort, Box, Heading, Link, VStack } from "@navikt/ds-react";
import { Glossary } from "../ordliste/glossary";
import { terms } from "../ordliste/terms";

export const metadata: Metadata = {
  title: "Ordbok",
  description: "Enkle forklaringer på begreper brukt i forbindelse med GitHub Copilot og AI-assistert utvikling.",
};

export default function OrdbokPage() {
  return (
    <main>
      <PageHero
        title="Ordbok"
        description="Enkle forklaringer på begreper brukt i forbindelse med GitHub Copilot og AI-assistert utvikling."
      />
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <VStack gap={{ xs: "space-24", md: "space-32" }}>
          <BodyShort>
            Trenger du kontekst? <Link href="#arkitektur-og-tillitsgrenser">Hopp til arkitektur og tillitsgrenser</Link>
            .
          </BodyShort>
          <Glossary terms={terms} />
          <section id="arkitektur-og-tillitsgrenser" aria-labelledby="arkitektur-heading" tabIndex={-1}>
            <VStack gap="space-16">
              <VStack gap="space-4">
                <Heading id="arkitektur-heading" size="medium" level="2">
                  Arkitektur og tillitsgrenser
                </Heading>
                <BodyShort>
                  Diagrammet viser hvordan data, verktøy og tillitsgrenser henger sammen i Copilot-oppsettet vårt.
                </BodyShort>
              </VStack>
              <TrustBoundaryDiagram />
            </VStack>
          </section>
        </VStack>
      </Box>
    </main>
  );
}
