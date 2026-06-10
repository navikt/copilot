import Link from "next/link";
import { notFound } from "next/navigation";
import { Box } from "@navikt/ds-react";
import { ArrowLeftIcon } from "@navikt/aksel-icons";
import { fetchVideoById, getPublicVideoFeed, type HomepageVideo } from "@/lib/public-videos";
import { DetailVideoPlayer } from "@/components/detail-video-player";
import { VideoMetadata } from "@/components/video-metadata";
import { VerticalVideoContainer } from "@/components/vertical-video-container";
import { RelatedVideos } from "@/components/related-videos";

type Props = {
  params: Promise<{ id: string }>;
};

export default async function VideoPage({ params }: Props) {
  const { id } = await params;

  const [video, allVideos] = await Promise.all([fetchVideoById(id), getPublicVideoFeed(20)]);

  if (!video) {
    notFound();
  }

  // Related: same series first, then same category, exclude current
  const related = allVideos
    .filter((v) => v.id !== id)
    .sort((a, b) => {
      const score = (v: HomepageVideo) =>
        (video.metadata?.series && v.metadata?.series === video.metadata.series ? 2 : 0) +
        (v.category === video.category ? 1 : 0);
      return score(b) - score(a);
    })
    .slice(0, 6);

  return (
    <main className="video-detail-page bg-black h-full min-h-full md:flex md:min-h-0 md:flex-col">
      {/* Back navigation */}
      <Box className="bg-black border-b border-white/10">
        <Box
          paddingBlock="space-8"
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
          className="max-w-7xl mx-auto"
        >
          <Link
            href="/"
            className="inline-flex items-center gap-1 text-white/60 hover:text-white text-sm transition-colors"
          >
            <ArrowLeftIcon aria-hidden fontSize="1rem" />
            Tilbake
          </Link>
        </Box>
      </Box>

      {/* Video + right panel */}
      <VerticalVideoContainer>
        {/* Video column — narrow, tall, black */}
        <div className="flex items-start justify-center bg-black md:w-[400px] md:flex-shrink-0 md:items-center">
          <Box paddingBlock={{ xs: "space-16", md: "space-16" }} paddingInline={{ xs: "space-12", md: "space-16" }}>
            <DetailVideoPlayer video={video} />
          </Box>
        </div>

        {/* Right panel — metadata top, related bottom */}
        <div className="flex-1 bg-black border-t border-white/10 md:min-h-0 md:overflow-y-auto md:border-t-0 md:border-l">
          <Box
            paddingBlock={{ xs: "space-16", md: "space-32" }}
            paddingInline={{ xs: "space-16", md: "space-32" }}
            className="flex h-full flex-col md:justify-between"
          >
            <VideoMetadata video={video} />
            <Box paddingBlock={{ xs: "space-24", md: "space-16" }}>
              <RelatedVideos videos={related} />
            </Box>
          </Box>
        </div>
      </VerticalVideoContainer>
    </main>
  );
}
