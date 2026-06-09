import Link from "next/link";
import { Box } from "@navikt/ds-react";
import type { HomepageVideo } from "@/lib/public-videos";
import { accentForEpisode } from "./video-overlay-components";

interface RelatedVideosProps {
  videos: HomepageVideo[];
}

function episodeMarkerFor(video: HomepageVideo): string | undefined {
  return video.metadata?.overlay?.find((o) => o.kind === "episode-number")?.labels?.[0];
}

function formatDuration(durationSec: number): string {
  const min = Math.floor(durationSec / 60);
  const sec = durationSec % 60;
  return `${min}:${String(sec).padStart(2, "0")}`;
}

export function RelatedVideos({ videos }: RelatedVideosProps) {
  if (videos.length === 0) return null;

  return (
    <Box>
      <p className="text-white/60 text-xs uppercase tracking-wide mb-3">Flere videoer</p>
      <div className="grid grid-cols-2 gap-x-3 gap-y-4">
        {videos.map((video) => {
          const marker = episodeMarkerFor(video);
          const accent = accentForEpisode(marker);

          return (
            <Link
              key={video.id}
              href={`/videos/${encodeURIComponent(video.id)}`}
              className="group block focus:outline-none focus-visible:ring-2 focus-visible:ring-white/60 rounded-lg"
            >
              {/* Poster thumbnail — 9:16 aspect ratio */}
              <div
                style={{ aspectRatio: "9 / 16" }}
                className="relative w-full overflow-hidden rounded-lg bg-[#1a1a1a]"
              >
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={video.posterUrl}
                  alt=""
                  className="absolute inset-0 w-full h-full object-cover transition-opacity group-hover:opacity-90"
                />

                {/* Duration badge */}
                <span className="absolute bottom-2 right-2 inline-flex h-5 items-center rounded px-1.5 text-[10px] font-medium bg-black/70 text-white/80 backdrop-blur-sm">
                  {formatDuration(video.durationSec)}
                </span>

                {/* Episode pill */}
                {marker && (
                  <span
                    className="absolute top-2 left-2 inline-flex h-5 items-center rounded px-1.5 text-[10px] font-semibold backdrop-blur-sm"
                    style={{ background: `${accent}22`, color: accent, border: `1px solid ${accent}55` }}
                  >
                    {marker}
                  </span>
                )}
              </div>

              {/* Title */}
              <Box paddingBlock="space-8">
                <p className="text-white/80 text-xs leading-snug line-clamp-2 group-hover:text-white transition-colors">
                  {video.title}
                </p>
              </Box>
            </Link>
          );
        })}
      </div>
    </Box>
  );
}
