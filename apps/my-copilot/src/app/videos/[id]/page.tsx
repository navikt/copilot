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
        <h1 className="text-4xl font-bold mb-4">{video.title}</h1>
        <p className="text-lg text-gray-600 mb-8 leading-relaxed">{video.description}</p>

        {/* Metadata grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-6 mb-12">
          <div>
            <p className="text-sm text-gray-500 font-semibold mb-1">Duration</p>
            <p className="text-lg font-semibold">
              {Math.floor(video.durationSec / 60)}m {video.durationSec % 60}s
            </p>
          </div>
          <div>
            <p className="text-sm text-gray-500 font-semibold mb-1">Language</p>
            <p className="text-lg font-semibold capitalize">{video.language}</p>
          </div>
          {video.metadata?.series && (
            <div>
              <p className="text-sm text-gray-500 font-semibold mb-1">Series</p>
              <p className="text-lg font-semibold">{video.metadata.series}</p>
            </div>
          )}
          {video.metadata?.episode && (
            <div>
              <p className="text-sm text-gray-500 font-semibold mb-1">Episode</p>
              <p className="text-lg font-semibold">
                {video.metadata.season && `S${video.metadata.season}E`}
                {video.metadata.episode}
              </p>
            </div>
          )}
        </div>

        {/* Tags */}
        {video.metadata?.tags && video.metadata.tags.length > 0 && (
          <div>
            <p className="text-sm text-gray-500 font-semibold mb-3">Tags</p>
            <div className="flex flex-wrap gap-2">
              {video.metadata.tags.map((tag) => (
                <span key={tag} className="px-3 py-1 bg-gray-100 text-gray-800 rounded-full text-sm">
                  {tag}
                </span>
              ))}
            </div>
          </div>
        )}
      </Box>

      {/* Related videos placeholder */}
      <Box
        as="section"
        paddingBlock={{ xs: "space-16", md: "space-24" }}
        paddingInline={{ xs: "space-16", md: "space-40" }}
        className="max-w-7xl mx-auto border-t pt-12"
      >
        <h2 className="text-2xl font-bold mb-8">Related Videos</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-6">
          {[1, 2, 3].map((i) => (
            <div key={i} className="bg-gray-100 rounded-lg overflow-hidden">
              <div className="aspect-video bg-gray-200" />
              <div className="p-4">
                <div className="h-4 bg-gray-300 rounded mb-2" />
                <div className="h-3 bg-gray-300 rounded w-3/4" />
              </div>
            </div>
          ))}
        </div>
      </Box>
    </main>
  );
}
