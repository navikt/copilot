import Link from "next/link";
import { notFound } from "next/navigation";
import { Box } from "@navikt/ds-react";
import { ArrowLeftIcon } from "@navikt/aksel-icons";
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
    <main className="min-h-screen bg-black">
      {/* Top navigation bar */}
      <Box
        paddingBlock="space-12"
        paddingInline={{ xs: "space-16", md: "space-32" }}
        className="bg-black border-b border-white/10"
      >
        <Link
          href="/"
          className="inline-flex items-center gap-1 text-white/60 hover:text-white text-sm transition-colors"
        >
          <ArrowLeftIcon aria-hidden fontSize="1rem" />
          Tilbake
        </Link>
      </Box>

      {/* Video + metadata — two columns on desktop, stacked on mobile */}
      <VerticalVideoContainer>
        {/* Video column: black, centered, preserves 9:16 */}
        <div className="md:w-[400px] md:flex-shrink-0 bg-black flex items-start justify-center md:items-center md:min-h-[calc(100vh-52px)]">
          <Box paddingBlock={{ xs: "space-16", md: "space-0" }}>
            <ResponsiveVideoPlayer video={video} autoplay={false} />
          </Box>
        </div>

        {/* Metadata panel: dark surface, scrollable on desktop */}
        <div className="flex-1 bg-[#111111] md:overflow-y-auto">
          <Box paddingBlock={{ xs: "space-16", md: "space-32" }} paddingInline={{ xs: "space-16", md: "space-32" }}>
            <VideoMetadata video={video} />
          </Box>
        </div>
      </VerticalVideoContainer>
    </main>
  );
}
