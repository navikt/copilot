import { Box, HGrid, HStack, Skeleton, VStack } from "@navikt/ds-react";

export default function Loading() {
  return (
    <main id="hovedinnhold" tabIndex={-1} className="verktoy-page">
      <div className="max-w-6xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-20", md: "space-32" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-24">
            <VStack gap="space-12" align="center">
              <Skeleton variant="text" width={100} />
              <Skeleton variant="text" width="60%" height={44} />
              <Skeleton variant="rounded" width="100%" height={52} className="max-w-2xl" />
              <HStack gap="space-8" justify="center" wrap>
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={i} variant="rounded" width={120} height={34} />
                ))}
              </HStack>
            </VStack>

            {Array.from({ length: 2 }).map((_, sectionIndex) => (
              <VStack key={sectionIndex} gap="space-12">
                <Skeleton variant="text" width={160} height={28} />
                <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-12">
                  {Array.from({ length: 4 }).map((_, i) => (
                    <Box key={i} background="default" padding="space-16" borderRadius="8">
                      <VStack gap="space-8">
                        <Skeleton variant="rounded" width={28} height={28} />
                        <Skeleton variant="text" width="70%" />
                        <Skeleton variant="text" width="100%" />
                      </VStack>
                    </Box>
                  ))}
                </HGrid>
              </VStack>
            ))}
          </VStack>
        </Box>
      </div>
    </main>
  );
}
