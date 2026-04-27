import { Skeleton, Box, VStack, HGrid } from "@navikt/ds-react";

export default function Loading() {
  return (
    <main>
      <section className="hero-gradient-subtle text-white">
        <Box
          paddingBlock={{ xs: "space-16", md: "space-20" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
          className="max-w-7xl mx-auto"
        >
          <VStack gap="space-8">
            <Skeleton variant="text" width={200} height={36} />
            <Skeleton variant="text" width={450} height={20} />
          </VStack>
        </Box>
      </section>
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-24">
            <HGrid columns={{ xs: 1, md: 4 }} gap="space-16">
              <Skeleton variant="rectangle" height={100} />
              <Skeleton variant="rectangle" height={100} />
              <Skeleton variant="rectangle" height={100} />
              <Skeleton variant="rectangle" height={100} />
            </HGrid>
            <Skeleton variant="rectangle" height={300} />
            <Skeleton variant="rectangle" height={400} />
          </VStack>
        </Box>
      </div>
    </main>
  );
}
