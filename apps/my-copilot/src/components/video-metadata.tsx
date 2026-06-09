"use client";

import { Box } from "@navikt/ds-react";
import type { HomepageVideo } from "@/lib/public-videos";

interface VideoMetadataProps {
  video: HomepageVideo;
}

export function VideoMetadata({ video }: VideoMetadataProps) {
  const formatDuration = (seconds: number) => {
    const minutes = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${minutes}m ${secs}s`;
  };

  return (
    <Box
      as="section"
      paddingBlock={{ xs: "space-16", md: "space-24" }}
      paddingInline={{ xs: "space-0", md: "space-0" }}
      className="flex flex-col gap-6"
    >
      {/* Title */}
      <Box paddingBlock="space-0">
        <h1 className="text-3xl md:text-4xl font-bold leading-tight">{video.title}</h1>
      </Box>

      {/* Description */}
      {video.description && (
        <Box paddingBlock="space-12">
          <p className="text-base md:text-lg text-gray-700 leading-relaxed">{video.description}</p>
        </Box>
      )}

      {/* Metadata Grid */}
      <Box paddingBlock="space-16">
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 md:gap-6">
          {/* Duration */}
          <Box className="flex flex-col">
            <p className="text-xs md:text-sm text-gray-600 font-semibold uppercase tracking-wide">Varighet</p>
            <p className="text-sm md:text-base font-semibold text-gray-900 mt-2">{formatDuration(video.durationSec)}</p>
          </Box>

          {/* Language */}
          <Box className="flex flex-col">
            <p className="text-xs md:text-sm text-gray-600 font-semibold uppercase tracking-wide">Språk</p>
            <p className="text-sm md:text-base font-semibold text-gray-900 mt-2 capitalize">{video.language}</p>
          </Box>

          {/* Series */}
          {video.metadata?.series && (
            <Box className="flex flex-col">
              <p className="text-xs md:text-sm text-gray-600 font-semibold uppercase tracking-wide">Serie</p>
              <p className="text-sm md:text-base font-semibold text-gray-900 mt-2">{video.metadata.series}</p>
            </Box>
          )}

          {/* Episode */}
          {video.metadata?.episode && (
            <Box className="flex flex-col">
              <p className="text-xs md:text-sm text-gray-600 font-semibold uppercase tracking-wide">Episode</p>
              <p className="text-sm md:text-base font-semibold text-gray-900 mt-2">
                {video.metadata.season && `S${video.metadata.season}E`}
                {video.metadata.episode}
              </p>
            </Box>
          )}

          {/* Category */}
          <Box className="flex flex-col">
            <p className="text-xs md:text-sm text-gray-600 font-semibold uppercase tracking-wide">Kategori</p>
            <p className="text-sm md:text-base font-semibold text-gray-900 mt-2 capitalize">{video.category}</p>
          </Box>
        </div>
      </Box>

      {/* Tags */}
      {video.metadata?.tags && video.metadata.tags.length > 0 && (
        <Box paddingBlock="space-16" paddingInline="space-0">
          <Box paddingBlock="space-8">
            <p className="text-xs md:text-sm text-gray-600 font-semibold uppercase tracking-wide">Merkelapper</p>
          </Box>
          <div className="flex flex-wrap gap-2">
            {video.metadata.tags.map((tag) => (
              <span
                key={tag}
                className="inline-block bg-blue-50 text-blue-900 rounded-full text-xs md:text-sm font-medium"
                style={{ padding: "var(--ax-space-6) var(--ax-space-12)" }}
              >
                {tag}
              </span>
            ))}
          </div>
        </Box>
      )}
    </Box>
  );
}
