import type { Metadata } from "next";
import { Box } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import { BudgetRaisePage } from "@/components/budget-raise";

export const metadata: Metadata = {
  title: "Budsjett og påfyll",
  description: "Se budsjettet og forbruket ditt i Copilot denne måneden, og fyll på selv når du trenger mer.",
};

// Private: guarded by src/proxy.ts and src/lib/private-paths.ts.
export default function BudsjettPage() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero
        title="Budsjett og påfyll"
        description="Ditt Copilot-budsjett denne måneden, og påfyll når du trenger mer."
      />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <BudgetRaisePage />
        </Box>
      </div>
    </main>
  );
}
