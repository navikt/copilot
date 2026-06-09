import { notFound } from "next/navigation";
import { Box } from "@navikt/ds-react";
import { fetchVideoById } from "@/lib/public-videos";
import { ResponsiveVideoPlayer } from "@/components/responsive-video-player";
import { VideoMetadata } from "@/components/video-metadata";
import { VerticalVideoContainer } from "@/components/vertical-video-container";

type Props = {
  params: Promise<{ id: string }>;
};

export default async function VideoPage({ params }: Props) {
  const { id } = await params;
  const video = await fetchVideoById(id);

  if (!video) {
    notFound();
  }

  return (
    <main className="min-h-screen bg-white">
      {/* Main video + metadata section with responsive layout */}
      <VerticalVideoContainer>
        {/* Video player - takes up ~70% height on mobile, ~55% width on desktop */}
        <Box className="w-full lg:w-1/2 flex-shrink-0">
          <ResponsiveVideoPlayer video={video} autoplay={false} />
        </Box>

        {/* Metadata section - scrollable on mobile, alongside video on desktop */}
        <Box className="w-full lg:w-1/2 flex-1 lg:overflow-y-auto">
          <Box paddingBlock={{ xs: "space-16", md: "space-0" }} paddingInline={{ xs: "space-0", md: "space-16" }}>
            <VideoMetadata video={video} />
          </Box>
        </Box>
      </VerticalVideoContainer>

      {/* Related videos section - placeholder for future implementation */}
      <Box
        as="section"
        paddingBlock={{ xs: "space-16", md: "space-24" }}
        paddingInline={{ xs: "space-16", md: "space-40" }}
        className="max-w-7xl mx-auto border-t"
        style={{ paddingTop: "var(--ax-space-24)" }}
      >
        <Box paddingBlock="space-16">
          <h2 className="text-2xl font-bold">Relaterte videoer</h2>
        </Box>
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-6">
          {[1, 2, 3].map((i) => (
            <div key={i} className="bg-gray-100 rounded-lg overflow-hidden">
              <div className="aspect-video bg-gray-200" />
              <Box paddingInline="space-8" paddingBlock="space-8">
                <Box paddingBlock="space-4">
                  <div className="h-4 bg-gray-300 rounded" />
                </Box>
                <div className="h-3 bg-gray-300 rounded w-3/4" />
              </Box>
            </div>
          ))}
        </div>
      </Box>
    </main>
  );
}
