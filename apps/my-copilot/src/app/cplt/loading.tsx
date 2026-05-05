import { Box, VStack, Skeleton } from "@navikt/ds-react";

export default function Loading() {
  return (
    <main>
      {/* Hero skeleton */}
      <section
        style={{
          background: "linear-gradient(165deg, #0a0f0c 0%, #0d2118 35%, #143d2b 65%, #0a1f14 100%)",
        }}
      >
        <Box
          paddingBlock={{ xs: "space-24", md: "space-40" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
          className="max-w-7xl mx-auto"
        >
          <VStack gap={{ xs: "space-20", md: "space-32" }} className="items-center">
            <VStack gap="space-12" className="items-center w-full">
              <Skeleton variant="text" width="50%" height={48} />
              <Skeleton variant="text" width="70%" height={20} />
              <Skeleton variant="rounded" width={140} height={32} />
            </VStack>
            <Skeleton variant="rounded" width="100%" height={300} className="max-w-4xl" />
            <VStack gap="space-8" className="items-center">
              <Skeleton variant="rounded" width={320} height={40} />
              <Skeleton variant="text" width={260} height={16} />
            </VStack>
          </VStack>
        </Box>
      </section>

      {/* Security table skeleton */}
      <section style={{ background: "#f8fafc" }}>
        <Box
          paddingBlock={{ xs: "space-24", md: "space-40" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
          className="max-w-5xl mx-auto"
        >
          <VStack gap="space-24" className="items-center">
            <VStack gap="space-8" className="items-center w-full">
              <Skeleton variant="text" width="30%" height={28} />
              <Skeleton variant="text" width="50%" height={16} />
            </VStack>
            <Skeleton variant="rounded" width="100%" height={350} />
          </VStack>
        </Box>
      </section>
    </main>
  );
}
