import { Box, VStack, HGrid, Skeleton } from "@navikt/ds-react";

export default function Loading() {
  return (
    <main>
      {/* Hero skeleton */}
      <section
        style={{
          background: "linear-gradient(165deg, #0a0a1a 0%, #1a1040 35%, #2d1b69 65%, #0f0a2a 100%)",
        }}
      >
        <Box
          paddingBlock={{ xs: "space-24", md: "space-40" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
          className="max-w-7xl mx-auto"
        >
          <VStack gap={{ xs: "space-20", md: "space-32" }} className="items-center">
            <VStack gap="space-12" className="items-center w-full">
              <Skeleton variant="text" width="40%" height={48} />
              <Skeleton variant="text" width="65%" height={20} />
            </VStack>
            <VStack gap="space-8" className="items-center">
              <Skeleton variant="rounded" width={320} height={40} />
              <Skeleton variant="text" width={200} height={16} />
            </VStack>
          </VStack>
        </Box>
      </section>

      {/* Collections skeleton */}
      <section style={{ background: "#f8fafc" }}>
        <Box
          paddingBlock={{ xs: "space-24", md: "space-40" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
          className="max-w-7xl mx-auto"
        >
          <VStack gap="space-24" className="items-center">
            <VStack gap="space-8" className="items-center w-full">
              <Skeleton variant="text" width="25%" height={28} />
              <Skeleton variant="text" width="45%" height={16} />
            </VStack>
            <HGrid columns={{ xs: 1, sm: 2, md: 3 }} gap="space-16" className="w-full">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} variant="rounded" height={160} />
              ))}
            </HGrid>
          </VStack>
        </Box>
      </section>
    </main>
  );
}
