"use client";

import { Box } from "@navikt/ds-react";
import type { HomepageVideo } from "@/lib/public-videos";
import { VideoPlayer } from "./video-player";

interface ResponsiveVideoPlayerProps {
  video: HomepageVideo;
  autoplay?: boolean;
}

/**
 * Responsive video player wrapper
 *
 * Handles aspect ratio based on video format:
 * - 9:16 (vertical): Uses full aspect-ratio, fills mobile height
 * - 16:9 (horizontal): Uses full aspect-ratio, maintains widescreen
 * - auto: Falls back to natural video dimensions
 *
 * Mobile: Full width, ~70% viewport height max
 * Desktop (lg+): Flex grow to fill available space
 */
export function ResponsiveVideoPlayer({ video, autoplay = false }: ResponsiveVideoPlayerProps) {
  // Use video's aspect ratio from API, fallback to 16:9
  const aspectRatio = video.aspectRatio || "16 / 9";

  return (
    <Box
      className="w-full flex flex-col flex-shrink-0 lg:flex-1"
      style={{
        // Mobile: constrain to max viewport height for proper layout
        maxHeight: "75vh",
      }}
    >
      <div
        className="w-full bg-black rounded-lg overflow-hidden flex-1"
        style={{
          aspectRatio: aspectRatio,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <VideoPlayer video={video} autoplay={autoplay} poster={video.posterUrl} />
      </div>
    </Box>
  );
}
