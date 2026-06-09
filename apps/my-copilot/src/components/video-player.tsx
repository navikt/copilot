"use client";

import { useRef, useEffect } from "react";
import { Box } from "@navikt/ds-react";
import type { HomepageVideo } from "@/lib/public-videos";
import { useVideoPlayer } from "./use-video-player";

interface VideoPlayerProps {
  video: HomepageVideo;
  autoplay?: boolean;
}

export function VideoPlayer({ video, autoplay = false }: VideoPlayerProps) {
  const { videoRef, playbackState, isFullscreen, setIsFullscreen, play, pause, toggleFullscreen } = useVideoPlayer({
    video,
    autoplay,
  });

  const containerRef = useRef<HTMLDivElement>(null);

  // Handle fullscreen state changes
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const handleFullscreenChange = () => {
      if (document.fullscreenElement === null) {
        setIsFullscreen(false);
      }
    };

    container.addEventListener("fullscreenchange", handleFullscreenChange);
    return () => container.removeEventListener("fullscreenchange", handleFullscreenChange);
  }, []);

  return (
    <Box
      ref={containerRef}
      as="div"
      className={`relative w-full bg-black rounded-lg overflow-hidden ${
        isFullscreen ? "fixed inset-0 z-50 rounded-none" : ""
      }`}
      style={{ aspectRatio: video.metadata?.overlay?.[0]?.anchor ? "auto" : "16 / 9" }}
    >
      <video
        ref={videoRef}
        className="w-full h-full object-contain"
        controls={false}
        onPlay={() => play()}
        onPause={() => pause()}
        crossOrigin="anonymous"
      >
        <source src={video.playUrl} type="application/x-mpegURL" />
        {video.mp4Url && <source src={video.mp4Url} type="video/mp4" />}
        {video.captionsUrl && <track kind="subtitles" src={video.captionsUrl} label="Norwegian" />}
        Your browser does not support the video tag.
      </video>

      {/* Custom playback controls */}
      <Box
        as="div"
        className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black to-transparent opacity-0 hover:opacity-100 transition-opacity"
        paddingInline="space-8"
        paddingBlock="space-8"
      >
        <div className="flex items-center gap-2">
          <button
            onClick={() => (playbackState === "playing" ? pause() : play())}
            className="bg-white/20 hover:bg-white/30 rounded text-white text-sm font-semibold"
            style={{ padding: "var(--ax-space-6) var(--ax-space-8)" }}
            aria-label={playbackState === "playing" ? "Pause" : "Play"}
          >
            {playbackState === "playing" ? "Pause" : "Play"}
          </button>

          {video.captionsUrl && (
            <button
              className="bg-white/20 hover:bg-white/30 rounded text-white text-sm font-semibold"
              style={{ padding: "var(--ax-space-6) var(--ax-space-8)" }}
            >
              CC
            </button>
          )}

          <button
            onClick={() => toggleFullscreen()}
            className="flex-1 bg-white/20 hover:bg-white/30 rounded text-white text-sm font-semibold text-right"
            style={{ padding: "var(--ax-space-6) var(--ax-space-8)" }}
            aria-label={isFullscreen ? "Exit fullscreen" : "Fullscreen"}
          >
            {isFullscreen ? "Exit FS" : "Fullscreen"}
          </button>
        </div>
      </Box>
    </Box>
  );
}
