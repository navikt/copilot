import { notFound } from "next/navigation";
import { Box } from "@navikt/ds-react";
import { fetchVideoById } from "@/lib/public-videos";
import { VideoPlayer } from "@/components/video-player";

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
      {/* Hero section with video player */}
      <Box
        as="section"
        paddingBlock={{ xs: "space-16", md: "space-24" }}
        paddingInline={{ xs: "space-16", md: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <VideoPlayer video={video} autoplay={false} />
      </Box>

      {/* Metadata section */}
      <Box
        as="section"
        paddingBlock={{ xs: "space-16", md: "space-24" }}
        paddingInline={{ xs: "space-16", md: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <Box paddingBlock="space-8">
          <h1 className="text-4xl font-bold">{video.title}</h1>
        </Box>
        <Box paddingBlock="space-16">
          <p className="text-lg text-gray-600 leading-relaxed">{video.description}</p>
        </Box>

        {/* Metadata grid */}
        <Box paddingBlock="space-24">
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-6">
            <div>
              <Box paddingBlock="space-2">
                <p className="text-sm text-gray-500 font-semibold">Duration</p>
              </Box>
              <p className="text-lg font-semibold">
                {Math.floor(video.durationSec / 60)}m {video.durationSec % 60}s
              </p>
            </div>
            <div>
              <Box paddingBlock="space-2">
                <p className="text-sm text-gray-500 font-semibold">Language</p>
              </Box>
              <p className="text-lg font-semibold capitalize">{video.language}</p>
            </div>
            {video.metadata?.series && (
              <div>
                <Box paddingBlock="space-2">
                  <p className="text-sm text-gray-500 font-semibold">Series</p>
                </Box>
                <p className="text-lg font-semibold">{video.metadata.series}</p>
              </div>
            )}
            {video.metadata?.episode && (
              <div>
                <Box paddingBlock="space-2">
                  <p className="text-sm text-gray-500 font-semibold">Episode</p>
                </Box>
                <p className="text-lg font-semibold">
                  {video.metadata.season && `S${video.metadata.season}E`}
                  {video.metadata.episode}
                </p>
              </div>
            )}
          </div>
        </Box>

        {/* Tags */}
        {video.metadata?.tags && video.metadata.tags.length > 0 && (
          <Box paddingBlock="space-16">
            <Box paddingBlock="space-6">
              <p className="text-sm text-gray-500 font-semibold">Tags</p>
            </Box>
            <div className="flex flex-wrap gap-2">
              {video.metadata.tags.map((tag) => (
                <span
                  key={tag}
                  className="bg-gray-100 text-gray-800 rounded-full text-sm"
                  style={{ padding: "var(--ax-space-4) var(--ax-space-8)" }}
                >
                  {tag}
                </span>
              ))}
            </div>
          </Box>
        )}
      </Box>

      {/* Related videos placeholder */}
      <Box
        as="section"
        paddingBlock={{ xs: "space-16", md: "space-24" }}
        paddingInline={{ xs: "space-16", md: "space-40" }}
        className="max-w-7xl mx-auto border-t"
        style={{ paddingTop: "var(--ax-space-24)" }}
      >
        <Box paddingBlock="space-16">
          <h2 className="text-2xl font-bold">Related Videos</h2>
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
