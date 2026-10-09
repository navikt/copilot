import { BodyLong, BodyShort, Box, VStack } from "@navikt/ds-react";
import type { ReactNode } from "react";
import { LinkableHeading } from "@/components/linkable-heading";
import { PageHero } from "@/components/page-hero";

// One layout for every page under /innsikt: hero, intro, «Sist oppdatert», a single column of
// sections and a «Kilde og metode» footer. Keep each section to at most four MetricCards and one chart.

export function InsightSection({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section>
      <VStack gap="space-16">
        <LinkableHeading id={id} size="medium" level="2">
          {title}
        </LinkableHeading>
        {children}
      </VStack>
    </section>
  );
}

export function InsightPage({
  title,
  description,
  intro,
  updated,
  source,
  children,
}: {
  title: string;
  description: string;
  intro: ReactNode;
  /** Shown after «Sist oppdatert:». */
  updated: ReactNode;
  /** Data view or endpoint and time window. */
  source: ReactNode;
  children: ReactNode;
}) {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero title={title} description={description} />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-40">
            <VStack gap="space-8">
              <BodyLong>{intro}</BodyLong>
              <BodyShort size="small" textColor="subtle">
                Sist oppdatert: {updated}
              </BodyShort>
            </VStack>
            {children}
            <InsightSection id="kilde-og-metode" title="Kilde og metode">
              <BodyLong>{source}</BodyLong>
            </InsightSection>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
