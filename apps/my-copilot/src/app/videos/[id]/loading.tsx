import { Skeleton, Box } from "@navikt/ds-react";

export default function Loading() {
  return (
    <main className="min-h-screen bg-black">
      {/* Nav bar skeleton */}
      <Box
        paddingBlock="space-12"
        paddingInline={{ xs: "space-16", md: "space-32" }}
        className="bg-black border-b border-white/10"
      >
        <Skeleton variant="rectangle" width="60px" height="20px" />
      </Box>

      {/* Two-column skeleton */}
      <div className="flex flex-col md:flex-row">
        {/* Video column */}
        <div className="md:w-[400px] md:flex-shrink-0 bg-black flex items-center justify-center">
          <Box paddingBlock={{ xs: "space-16", md: "space-0" }}>
            <div className="bg-[#1a1a1a] rounded" style={{ width: "360px", aspectRatio: "9 / 16" }} />
          </Box>
        </div>

        {/* Metadata skeleton */}
        <div className="flex-1 bg-[#111111]">
          <Box
            paddingBlock={{ xs: "space-16", md: "space-32" }}
            paddingInline={{ xs: "space-16", md: "space-32" }}
            className="flex flex-col gap-4"
          >
            <Skeleton variant="rectangle" width="80px" height="20px" />
            <Skeleton variant="rectangle" width="70%" height="32px" />
            <Skeleton variant="text" width="90%" />
            <Skeleton variant="text" width="75%" />
            <div className="flex gap-2">
              {[1, 2, 3].map((i) => (
                <Skeleton key={i} variant="rectangle" width="60px" height="24px" />
              ))}
            </div>
          </Box>
        </div>
      </div>
    </main>
  );
}
