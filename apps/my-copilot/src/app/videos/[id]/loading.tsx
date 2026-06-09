import { Skeleton, Box } from "@navikt/ds-react";

export default function Loading() {
  return (
    <main className="min-h-screen bg-white">
      <Box
        paddingBlock={{ xs: "space-16", md: "space-24" }}
        paddingInline={{ xs: "space-16", md: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <Box paddingBlock="space-8">
          <Skeleton variant="rectangle" width="50%" height="40px" />
        </Box>
        <Box paddingBlock="space-16">
          <Skeleton variant="text" width="80%" />
        </Box>

        <Box paddingBlock="space-16">
          <div className="aspect-video bg-gray-200 rounded-lg" />
        </Box>

        <Box paddingBlock="space-16">
          <div className="grid grid-cols-2 gap-4">
            {[1, 2, 3, 4].map((i) => (
              <div key={i}>
                <Box paddingBlock="space-4">
                  <Skeleton variant="text" width="40%" />
                </Box>
                <Skeleton variant="text" width="60%" />
              </div>
            ))}
          </div>
        </Box>
      </Box>
    </main>
  );
}
