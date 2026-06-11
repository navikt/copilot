"use client";

import { Box } from "@navikt/ds-react";
import type { HomepageVideo } from "@/lib/public-videos";

interface VideoMetadataProps {
  video: HomepageVideo;
}

function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}

function buildEpisodeLabel(video: HomepageVideo): string | null {
  if (video.metadata?.season && video.metadata?.episode) {
    return `S${video.metadata.season}E${video.metadata.episode}`;
  }
  if (video.metadata?.episode) {
    return `Episode ${video.metadata.episode}`;
  }
  return null;
}

/**
 * Compact dark-themed metadata panel for the video detail page.
 *
 * Layout (top → bottom):
 *   episode pill + duration · language
 *   title (large, white)
 *   series name (if present)
 *   description (muted white)
 *   tags (pill row)
 *   category (subtle footer label)
 */
export function VideoMetadata({ video }: VideoMetadataProps) {
  const episodeLabel = buildEpisodeLabel(video);

  return (
    <Box as="section" className="flex flex-col gap-4">
      {/* Episode + duration row */}
      <div className="flex items-center gap-2 flex-wrap">
        {episodeLabel && (
          <span
            className="rounded text-xs font-semibold text-white/80 bg-white/10"
            style={{ paddingInline: "var(--ax-space-8)", paddingBlock: "var(--ax-space-4)" }}
          >
            {episodeLabel}
          </span>
        )}
        <span className="text-white/60 text-sm font-medium">{formatDuration(video.durationSec)}</span>
        <span className="text-white/40 text-xs capitalize">{video.language}</span>
      </div>

      {/* Title */}
      <h1 className="text-xl md:text-2xl font-bold leading-tight text-white">{video.title}</h1>

      {/* Series */}
      {video.metadata?.series && <p className="text-white/50 text-sm -mt-2">{video.metadata.series}</p>}

      {/* Description */}
      {video.description && (
        <Box paddingBlock="space-4">
          <p className="text-white/75 text-sm leading-relaxed">{video.description}</p>
        </Box>
      )}

      {/* Tags */}
      {video.metadata?.tags && video.metadata.tags.length > 0 && (
        <Box paddingBlock="space-4">
          <div className="flex flex-wrap gap-2">
            {video.metadata.tags.map((tag) => (
              <span
                key={tag}
                className="rounded-full text-xs font-medium text-white/70 bg-white/10"
                style={{ paddingInline: "var(--ax-space-12)", paddingBlock: "var(--ax-space-4)" }}
              >
                {tag}
              </span>
            ))}
          </div>
        </Box>
      )}

      {/* Category */}
      <Box paddingBlock="space-8">
        <p className="text-white/30 text-xs uppercase tracking-widest">{video.category}</p>
      </Box>
    </Box>
  );
}
