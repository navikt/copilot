import { notFound } from "next/navigation";
import { fetchVideoById } from "@/lib/public-videos";

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
      <div className="max-w-7xl mx-auto px-4 py-8">
        <h1 className="text-3xl font-bold mb-4">{video.title}</h1>
        <p className="text-gray-600 mb-8">{video.description}</p>

        {/* Placeholder: Video player will be integrated in Phase 3 */}
        <div className="aspect-video bg-gray-200 rounded-lg mb-8 flex items-center justify-center">
          <span className="text-gray-500">Video player placeholder</span>
        </div>

        {/* Metadata */}
        <div className="grid grid-cols-2 gap-4 mb-8">
          <div>
            <p className="text-sm text-gray-500">Duration</p>
            <p className="font-semibold">
              {Math.floor(video.durationSec / 60)}m {video.durationSec % 60}s
            </p>
          </div>
          <div>
            <p className="text-sm text-gray-500">Language</p>
            <p className="font-semibold">{video.language}</p>
          </div>
          {video.metadata?.series && (
            <div>
              <p className="text-sm text-gray-500">Series</p>
              <p className="font-semibold">{video.metadata.series}</p>
            </div>
          )}
          {video.metadata?.episode && (
            <div>
              <p className="text-sm text-gray-500">Episode</p>
              <p className="font-semibold">
                {video.metadata.season && `S${video.metadata.season}E`}
                {video.metadata.episode}
              </p>
            </div>
          )}
        </div>
      </div>
    </main>
  );
}
