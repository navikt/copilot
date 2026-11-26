import { Box, Skeleton, VStack } from "@navikt/ds-react";

/** Shown while a /fleet page waits for Fleet. */
export default function Loading() {
  return (
    <main id="hovedinnhold" tabIndex={-1} aria-busy="true">
      <div className="max-w-7xl mx-auto">
        <Box paddingBlock="space-24" paddingInline={{ xs: "space-16", md: "space-32", lg: "space-40" }}>
          <VStack gap="space-16">
            <Skeleton variant="text" width="30%" height={40} />
            <Box background="default" borderWidth="1" borderColor="neutral-subtle" borderRadius="12" padding="space-24">
              <VStack gap="space-16">
                {[0, 1, 2].map((i) => (
                  <VStack key={i} gap="space-4">
                    <Skeleton variant="text" width="40%" />
                    <Skeleton variant="text" width="60%" />
                  </VStack>
                ))}
              </VStack>
            </Box>
            <span className="sr-only">Laster</span>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
