import type { Metadata } from "next";
import { Box } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import { BudgetRaisePage } from "@/components/budget-raise";
import { getUser } from "@/lib/auth";

export const metadata: Metadata = {
  title: "Budsjett og påfyll",
  description: "Se budsjettet og forbruket ditt i Copilot denne måneden, og fyll på selv når du trenger mer.",
};

// Private: src/proxy.ts sends readers without a token to the login, and getUser() does the same here,
// so listing the path in autoLoginIgnorePaths does not make the page public.
export default async function BudsjettPage() {
  await getUser();
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero
        title="Budsjett og påfyll"
        description="Budsjettet ditt i Copilot denne måneden. Trenger du mer, fyller du på selv."
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
