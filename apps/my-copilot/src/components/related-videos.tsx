import Link from "next/link";
import { Box } from "@navikt/ds-react";
import type { HomepageVideo } from "@/lib/public-videos";

const ACCENTS = ["#66d4cf", "#9af0a8", "#ffd485", "#c6a8ff", "#7cc7ff", "#ff9db1"] as const;

function accentForEpisode(episode: string | undefined): string {
  const n = Number.parseInt(episode ?? "", 10);
  if (Number.isFinite(n) && n > 0) {
    return ACCENTS[(n - 1) % ACCENTS.length];
  }
  return ACCENTS[0];
}

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
    <Box className="max-w-[240px]">
      <p className="text-white/60 text-[10px] uppercase tracking-wide mb-2">Flere videoer</p>
      <div className="grid grid-cols-3 gap-2">
        {videos.map((video) => {
          const marker = episodeMarkerFor(video);
          const accent = accentForEpisode(marker);

          return (
            <Link
              key={video.id}
              href={`/videos/${encodeURIComponent(video.id)}`}
              className="group block focus:outline-none focus-visible:ring-2 focus-visible:ring-white/60 rounded-md"
            >
              {/* Poster thumbnail — 9:16 aspect ratio */}
              <div
                style={{ aspectRatio: "9 / 16" }}
                className="relative w-full overflow-hidden rounded-md bg-[#1a1a1a]"
              >
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={video.posterUrl}
                  alt=""
                  className="absolute inset-0 w-full h-full object-cover transition-opacity group-hover:opacity-90"
                />

                {/* Duration badge */}
                <span className="absolute bottom-1 right-1 inline-flex h-4 items-center rounded px-1 text-[8px] font-medium bg-black/70 text-white/80 backdrop-blur-sm">
                  {formatDuration(video.durationSec)}
                </span>

                {/* Episode pill */}
                {marker && (
                  <span
                    className="absolute top-1 left-1 inline-flex h-4 items-center rounded px-1 text-[8px] font-semibold backdrop-blur-sm"
                    style={{ background: `${accent}22`, color: accent, border: `1px solid ${accent}55` }}
                  >
                    {marker}
                  </span>
                )}
              </div>

              {/* Title */}
              <Box paddingBlock="space-4">
                <p className="text-white/80 text-[10px] leading-tight line-clamp-1 group-hover:text-white transition-colors">
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
