import { BodyLong, BodyShort, Box, VStack } from "@navikt/ds-react";
import { Suspense, type ReactNode } from "react";
import { formatDate } from "@/lib/format";
import { LinkableHeading } from "@/components/linkable-heading";
import { PageHero } from "@/components/page-hero";

// One layout for the pages under /innsikt, /priser and /reisen: hero, intro, «Sist oppdatert», a single column of
// sections and a «Kilde og metode» footer. Keep each section to at most four MetricCards and one chart.

export function InsightSection({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    // Named regions are deliberate: each section is a landmark screen readers can jump to.
    <section aria-labelledby={id} className="min-w-0">
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
  label,
  actions,
  intro,
  updated,
  hourly = false,
  source,
  width = "wide",
  className,
  printHeader,
  children,
}: {
  title: string;
  description: string;
  /** Passed to PageHero. */
  label?: string;
  /** Passed to PageHero, e.g. a print button. */
  actions?: ReactNode;
  intro?: ReactNode;
  /** Resolves to the date the numbers are from (ISO date or timestamp). Shown as «Sist oppdatert: 3. oktober 2026.» Leave out on pages without data. */
  updated?: () => Promise<string | null | undefined>;
  /** Adds «Tallene hentes på nytt hver time.» */
  hourly?: boolean;
  /** Data view or endpoint and time window, or where the text comes from. Always shown. */
  source: ReactNode;
  /** «prose» narrows the content column for long reading text; the hero keeps its width. */
  width?: "prose" | "wide";
  /** Classes on <main>, e.g. a print shell. */
  className?: string;
  /** Shown in print instead of the hero, which is then hidden in print. */
  printHeader?: ReactNode;
  children: ReactNode;
}) {
  const hero = <PageHero label={label} title={title} description={description} actions={actions} />;
  return (
    <main id="hovedinnhold" tabIndex={-1} className={className}>
      {printHeader ? (
        <>
          <div className="print-hidden">{hero}</div>
          {printHeader}
        </>
      ) : (
        hero
      )}
      <div className={width === "prose" ? "max-w-3xl mx-auto" : "max-w-7xl mx-auto"}>
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-40">
            {(intro || updated) && (
              <VStack gap="space-8">
                {intro && <BodyLong>{intro}</BodyLong>}
                {updated && (
                  <BodyShort size="small" textColor="subtle">
                    Sist oppdatert:{" "}
                    <Suspense fallback="henter …">
                      <UpdatedDate date={updated} />
                    </Suspense>
                    .{hourly && " Tallene hentes på nytt hver time."}
                  </BodyShort>
                )}
              </VStack>
            )}
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
