import { BodyLong, BodyShort, Box, VStack } from "@navikt/ds-react";
import { Suspense, type ReactNode } from "react";
import { formatDate } from "@/lib/format";
import { LinkableHeading } from "@/components/linkable-heading";
import { PageHero } from "@/components/page-hero";

// One layout for every page under /innsikt: hero, intro, «Sist oppdatert», a single column of
// sections and a «Kilde og metode» footer. Keep each section to at most four MetricCards and one chart.

export function InsightSection({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section className="min-w-0">
      <VStack gap="space-16" className="min-w-0">
        <LinkableHeading id={id} size="medium" level="2">
          {title}
        </LinkableHeading>
        {children}
      </VStack>
    </section>
  );
}

async function UpdatedDate({ date }: { date: () => Promise<string | null | undefined> }) {
  const value = await date().catch(() => null);
  return value ? formatDate(value) : "kunne ikke hentes";
}

export function InsightPage({
  title,
  description,
  intro,
  updated,
  hourly = false,
  source,
  children,
}: {
  title: string;
  description: string;
  intro: ReactNode;
  /** Resolves to the date the numbers are from (ISO date or timestamp). Shown as «Sist oppdatert: 3. oktober 2026.» */
  updated: () => Promise<string | null | undefined>;
  /** Adds «Tallene hentes på nytt hver time.» */
  hourly?: boolean;
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
                Sist oppdatert:{" "}
                <Suspense fallback="henter …">
                  <UpdatedDate date={updated} />
                </Suspense>
                .{hourly && " Tallene hentes på nytt hver time."}
              </BodyShort>
            </VStack>
            {children}
            <InsightSection id="kilde-og-metode" title="Kilde og metode">
              <BodyLong as="div">{source}</BodyLong>
            </InsightSection>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
