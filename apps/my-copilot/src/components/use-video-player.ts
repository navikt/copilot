"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { HomepageVideo } from "@/lib/public-videos";

export type PlaybackState = "playing" | "paused" | "loading" | "error" | "ended";

interface UseVideoPlayerOptions {
  video: HomepageVideo;
  autoplay?: boolean;
  onPlaybackStateChange?: (state: PlaybackState) => void;
}

export function useVideoPlayer({ video, autoplay = false, onPlaybackStateChange }: UseVideoPlayerOptions) {
  const [playbackState, setPlaybackState] = useState<PlaybackState>("paused");
  const [isFullscreen, setIsFullscreen] = useState(false);

  const videoRef = useRef<HTMLVideoElement>(null);

  // Update playback state
  const updatePlaybackState = useCallback(
    (state: PlaybackState) => {
      setPlaybackState(state);
      onPlaybackStateChange?.(state);
    },
    [onPlaybackStateChange]
  );

  // Autoplay handling
  useEffect(() => {
    if (autoplay && videoRef.current && playbackState === "paused") {
      videoRef.current.play().catch((err) => {
        console.warn("Autoplay failed:", err);
      });
    }
  }, [autoplay, playbackState]);

  // Track watch state in localStorage
  useEffect(() => {
    if (!videoRef.current) return;

    const handleTimeUpdate = () => {
      const duration = videoRef.current?.duration || 0;
      const currentTime = videoRef.current?.currentTime || 0;

      if (duration > 0 && currentTime > 0) {
        const watchKey = `my-copilot:watch-state:${video.id}`;
        const progressPercent = Math.round((currentTime / duration) * 100);
        localStorage.setItem(
          watchKey,
          JSON.stringify({
            id: video.id,
            watchedAt: new Date().toISOString(),
            progressPercent,
          })
        );
      }
    };

    const videoElement = videoRef.current;
    videoElement.addEventListener("timeupdate", handleTimeUpdate);
    return () => videoElement.removeEventListener("timeupdate", handleTimeUpdate);
  }, [video.id]);

  const play = useCallback(async () => {
    if (videoRef.current) {
      try {
        await videoRef.current.play();
        updatePlaybackState("playing");
      } catch (err) {
        console.error("Play error:", err);
      }
    }
  }, [updatePlaybackState]);

  const pause = useCallback(() => {
    if (videoRef.current) {
      videoRef.current.pause();
      updatePlaybackState("paused");
    }
  }, [updatePlaybackState]);

  const toggleFullscreen = useCallback(async () => {
    if (!videoRef.current?.parentElement) return;

    try {
      if (!isFullscreen) {
        const container = videoRef.current.parentElement;
        await container.requestFullscreen?.();
        setIsFullscreen(true);
      } else {
        await document.exitFullscreen?.();
        setIsFullscreen(false);
      }
    } catch (err) {
      console.error("Fullscreen error:", err);
    }
  }, [isFullscreen]);

  return {
    videoRef,
    playbackState,
    isFullscreen,
    setIsFullscreen,
    play,
    pause,
    toggleFullscreen,
    updatePlaybackState,
  };
}
