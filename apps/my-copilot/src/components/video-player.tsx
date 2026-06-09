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
  const { videoRef, playbackState, isFullscreen, play, pause, toggleFullscreen } = useVideoPlayer({
    video,
    autoplay,
  });

  const containerRef = useRef<HTMLDivElement>(null);

  // Handle fullscreen state changes
  useEffect(() => {
    const handleFullscreenChange = () => {
      if (!document.fullscreenElement && containerRef.current) {
        containerRef.current.classList.remove("fullscreen");
      }
    };

    document.addEventListener("fullscreenchange", handleFullscreenChange);
    return () => document.removeEventListener("fullscreenchange", handleFullscreenChange);
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
      <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black to-transparent p-4 opacity-0 hover:opacity-100 transition-opacity">
        <div className="flex items-center gap-2">
          <button
            onClick={() => (playbackState === "playing" ? pause() : play())}
            className="px-3 py-2 bg-white/20 hover:bg-white/30 rounded text-white text-sm font-semibold"
            aria-label={playbackState === "playing" ? "Pause" : "Play"}
          >
            {playbackState === "playing" ? "Pause" : "Play"}
          </button>

          {video.captionsUrl && (
            <button className="px-3 py-2 bg-white/20 hover:bg-white/30 rounded text-white text-sm font-semibold">
              CC
            </button>
          )}

          <button
            onClick={() => toggleFullscreen()}
            className="ml-auto px-3 py-2 bg-white/20 hover:bg-white/30 rounded text-white text-sm font-semibold"
            aria-label={isFullscreen ? "Exit fullscreen" : "Fullscreen"}
          >
            {isFullscreen ? "Exit FS" : "Fullscreen"}
          </button>
        </div>
      </div>
    </Box>
  );
}
