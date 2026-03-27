import { PageHero } from "@/components/page-hero";
import { Box } from "@navikt/ds-react";
import { Glossary } from "./glossary";
import { terms } from "./terms";

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
        <Glossary terms={terms} />
      </Box>
    </main>
  );
}
