import { Box, VStack, Skeleton } from "@navikt/ds-react";

// Same frame as DocPage, while the page fetches the star count and the config keys.
export default function Loading() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <Box
        paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <VStack gap="space-24" className="max-w-3xl">
          <VStack gap="space-8">
            <Skeleton variant="text" width="40%" height={48} />
            <Skeleton variant="text" width="80%" height={20} />
          </VStack>
          <Skeleton variant="text" width="30%" height={28} />
          <Skeleton variant="rounded" width="100%" height={300} />
          <Skeleton variant="rounded" width="100%" height={40} />
        </VStack>
      </Box>
    </main>
  );
}
