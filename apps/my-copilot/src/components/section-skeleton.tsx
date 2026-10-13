import { Box, HGrid, Skeleton, VStack } from "@navikt/ds-react";

/** Placeholder for a streamed /innsikt or /admin section: heading, optional stat tiles, a chart and table rows. */
export function SectionSkeleton({
  cards = 0,
  chart = true,
  rows = 0,
}: {
  cards?: number;
  chart?: boolean;
  rows?: number;
}) {
  return (
    <div role="status" aria-busy="true">
      <span className="sr-only">Laster …</span>
      <VStack gap="space-16" aria-hidden="true">
        <Skeleton variant="text" width="30%" height={32} />
        {cards > 0 && (
          <HGrid columns={{ xs: 1, sm: 2, lg: Math.min(cards, 4) }} gap="space-16">
            {Array.from({ length: cards }, (_, i) => (
              <Skeleton key={i} variant="rounded" height={112} />
            ))}
          </HGrid>
        )}
        {chart && <Skeleton variant="rounded" height={300} />}
        {Array.from({ length: rows }, (_, i) => (
          <Skeleton key={i} variant="text" />
        ))}
      </VStack>
    </div>
  );
}

/** Route-level fallback for pages that cannot stream sections: hero band plus three sections. */
export function PageSkeleton() {
  return (
    <main id="hovedinnhold" tabIndex={-1} aria-busy="true">
      <Box
        background="neutral-soft"
        paddingBlock={{ xs: "space-16", md: "space-20" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        aria-hidden="true"
      >
        <div className="max-w-7xl mx-auto">
          <VStack gap="space-12">
            <Skeleton variant="text" width="40%" height={56} />
            <Skeleton variant="text" width="60%" />
          </VStack>
        </div>
      </Box>
      <div className="max-w-7xl mx-auto">
        <Box paddingBlock="space-24" paddingInline={{ xs: "space-16", md: "space-32", lg: "space-40" }}>
          <VStack gap="space-40">
            <SectionSkeleton cards={4} chart={false} />
            <SectionSkeleton />
            <SectionSkeleton chart={false} rows={6} />
          </VStack>
        </Box>
      </div>
    </main>
  );
}
