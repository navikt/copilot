import { PageHero } from "@/components/page-hero";
import { Box } from "@navikt/ds-react";
import type { Metadata } from "next";
import { InteractiveSetupWizard } from "@/components/nav-pilot/interactive-setup-wizard";

export const metadata: Metadata = {
  title: "Kom i gang",
  description: "Fra null til produktiv med GitHub Copilot i Nav på under 10 minutter.",
};

export default function KomIGangPage() {
  return (
    <main>
      <PageHero title="Kom i gang" description="Fra null til produktiv med GitHub Copilot på under 10 minutter." />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
          marginInline="auto"
        >
          <div className="max-w-3xl mx-auto">
            <InteractiveSetupWizard />
          </div>
        </Box>
      </div>
    </main>
  );
}
