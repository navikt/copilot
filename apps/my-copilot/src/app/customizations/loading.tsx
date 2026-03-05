import { Skeleton, Box, HGrid, VStack } from "@navikt/ds-react";

export default function Loading() {
  return (
    <main className="max-w-7xl mx-auto">
      <Box
        paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          <div>
            <Skeleton variant="text" width="50%" height={40} />
            <Skeleton variant="text" width="70%" className="mt-2" />
          </div>

          <HGrid columns={{ xs: 2, sm: 3, md: 3, lg: 6 }} gap={{ xs: "space-8", md: "space-12" }}>
            {Array.from({ length: 6 }).map((_, i) => (
              <Box key={i} background="neutral-soft" padding="space-16" borderRadius="12">
                <Skeleton variant="rectangle" height={80} />
              </Box>
            ))}
          </HGrid>

          <div>
            <Skeleton variant="text" width="30%" height={32} />
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4">
              {Array.from({ length: 6 }).map((_, i) => (
                <Box key={i} background="neutral-soft" padding="space-16" borderRadius="12">
                  <Skeleton variant="rectangle" height={100} />
                </Box>
              ))}
            </div>
          </div>
        </VStack>
      </Box>
    </main>
  );
}
