"use client";

import { useCallback, useRef } from "react";
import type { PlaybackEvent } from "@/lib/video-playback-machine";
import type { TelemetryAdapter } from "./use-shorts-feed-telemetry-adapter";

export type ShortsFeedMediaHandlers = {
  onPlay: () => void;
  onPause: () => void;
  onTimeUpdate: () => void;
  onEnded: () => void;
  onError: () => void;
  onWaiting: () => void;
};

export type UseMediaAdapterReturn = {
  videoRefs: React.MutableRefObject<Map<string, HTMLVideoElement>>;
  cardRefs: React.MutableRefObject<Map<string, HTMLDivElement>>;
  setVideoNode: (videoId: string, node: HTMLVideoElement | null) => void;
  setCardNode: (videoId: string, node: HTMLDivElement | null) => void;
  resumePlayback: (videoId: string) => void;
  pausePlayback: (videoId: string) => void;
  replayPlayback: (videoId: string) => void;
  seekPlayback: (videoId: string, deltaSeconds: number) => void;
  toggleFullscreen: (videoId: string) => void;
  mediaHandlers: (videoId: string) => ShortsFeedMediaHandlers;
};

// Telemetry KPI dedup + emission is owned by the telemetry adapter; the media
// adapter only detects events and delegates.
type MediaAdapterCallbacks = {
  dispatch: (event: PlaybackEvent) => void;
  isActiveEvent: (videoId: string) => boolean;
  telemetry: TelemetryAdapter;
  updateProgress: (videoId: string, currentSecond: number, duration: number | undefined) => void;
  markComplete: (videoId: string, duration: number | undefined) => void;
  flushProgress: (videoId: string, currentSecond: number, duration: number | undefined) => void;
};

export function useMediaAdapter({
  dispatch,
  isActiveEvent,
  telemetry,
  updateProgress,
  markComplete,
  flushProgress,
}: MediaAdapterCallbacks): UseMediaAdapterReturn {
  const videoRefs = useRef<Map<string, HTMLVideoElement>>(new Map());
  const cardRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  // --- ref registration -------------------------------------------------
  const setVideoNode = useCallback((videoId: string, node: HTMLVideoElement | null) => {
    if (!node) {
      videoRefs.current.delete(videoId);
      return;
    }
    node.dataset.videoId = videoId;
    videoRefs.current.set(videoId, node);
  }, []);

  const setCardNode = useCallback((videoId: string, node: HTMLDivElement | null) => {
    if (!node) {
      cardRefs.current.delete(videoId);
      return;
    }
    cardRefs.current.set(videoId, node);
  }, []);

  // --- media event handlers ---------------------------------------------
  const handlePlay = useCallback(
    (videoId: string) => {
      if (!isActiveEvent(videoId)) return;
      dispatch({ type: "PLAY" });
      telemetry.emitVideoStarted(videoId);
    },
    [dispatch, isActiveEvent, telemetry]
  );

  const handlePause = useCallback(
    (videoId: string) => {
      if (!isActiveEvent(videoId)) return;
      const video = videoRefs.current.get(videoId);
      if (video) {
        const currentSecond = Math.floor(video.currentTime);
        const duration = Number.isFinite(video.duration) ? video.duration : undefined;
        flushProgress(videoId, currentSecond, duration);
      }
      dispatch({ type: "PAUSE" });
    },
    [dispatch, isActiveEvent, flushProgress]
  );

  const handleError = useCallback(
    (videoId: string) => {
      if (!isActiveEvent(videoId)) return;
      const video = videoRefs.current.get(videoId);
      const errorCode = video?.error?.code;
      telemetry.emitVideoError(videoId, errorCode ?? "unknown");
    },
    [telemetry, isActiveEvent]
  );

  const handleWaiting = useCallback(
    (videoId: string) => {
      if (!isActiveEvent(videoId)) return;
      telemetry.addRebuffer(videoId);
    },
    [telemetry, isActiveEvent]
  );

  const handleTimeUpdate = useCallback(
    (videoId: string) => {
      if (!isActiveEvent(videoId)) return;
      const video = videoRefs.current.get(videoId);
      if (!video) return;

      const currentSecond = Math.floor(video.currentTime);
      const duration = Number.isFinite(video.duration) ? video.duration : undefined;
      updateProgress(videoId, currentSecond, duration);
    },
    [updateProgress, isActiveEvent]
  );

  const handleEnded = useCallback(
    (videoId: string) => {
      if (!isActiveEvent(videoId)) return;
      const video = videoRefs.current.get(videoId);
      const duration = video && Number.isFinite(video.duration) ? video.duration : undefined;
      const currentSecond = video ? Math.floor(video.currentTime) : 0;
      dispatch({ type: "END" });
      flushProgress(videoId, currentSecond, duration);
      markComplete(videoId, duration);
    },
    [dispatch, isActiveEvent, flushProgress, markComplete]
  );

  const mediaHandlers = useCallback(
    (videoId: string): ShortsFeedMediaHandlers => ({
      onPlay: () => handlePlay(videoId),
      onPause: () => handlePause(videoId),
      onTimeUpdate: () => handleTimeUpdate(videoId),
      onEnded: () => handleEnded(videoId),
      onError: () => handleError(videoId),
      onWaiting: () => handleWaiting(videoId),
    }),
    [handlePlay, handlePause, handleTimeUpdate, handleEnded, handleError, handleWaiting]
  );

  // --- imperative controls ----------------------------------------------
  const resumePlayback = useCallback((videoId: string) => {
    const video = videoRefs.current.get(videoId);
    if (!video) return;
    void video.play().catch(() => {
      // Native controls remain available if playback is blocked.
    });
  }, []);

  const pausePlayback = useCallback((videoId: string) => {
    const video = videoRefs.current.get(videoId);
    if (!video) return;
    video.pause();
  }, []);

  const replayPlayback = useCallback(
    (videoId: string) => {
      const video = videoRefs.current.get(videoId);
      if (!video) return;
      dispatch({ type: "REPLAY" });
      video.currentTime = 0;
      void video.play().catch(() => {
        // Native controls remain available if playback is blocked.
      });
    },
    [dispatch]
  );

  const seekPlayback = useCallback((videoId: string, deltaSeconds: number) => {
    const video = videoRefs.current.get(videoId);
    if (!video) return;

    const duration = Number.isFinite(video.duration) ? video.duration : undefined;
    const nextTime = duration
      ? Math.min(Math.max(video.currentTime + deltaSeconds, 0), duration)
      : Math.max(video.currentTime + deltaSeconds, 0);

    video.currentTime = nextTime;
  }, []);

  const toggleFullscreen = useCallback((videoId: string) => {
    const video = videoRefs.current.get(videoId);
    if (!video) return;

    const doc = document as Document & {
      webkitFullscreenElement?: Element;
      webkitExitFullscreen?: () => Promise<void> | void;
    };
    const webkitVideo = video as HTMLVideoElement & { webkitEnterFullscreen?: () => void };
    const isFullscreen = document.fullscreenElement === video || doc.webkitFullscreenElement === video;

    if (isFullscreen) {
      if (document.fullscreenElement && document.exitFullscreen) {
        document.exitFullscreen?.().catch(() => {
          // Exit fullscreen failed; continue normally
        });
        return;
      }
      if (doc.webkitExitFullscreen) {
        Promise.resolve(doc.webkitExitFullscreen()).catch(() => {
          // Exit fullscreen failed; continue normally
        });
      }
      return;
    }

    if (video.requestFullscreen) {
      video.requestFullscreen().catch(() => {
        // Fullscreen permission denied or request failed; video continues playing
      });
      return;
    }

    if (webkitVideo.webkitEnterFullscreen) {
      webkitVideo.webkitEnterFullscreen();
    }
  }, []);

  return {
    videoRefs,
    cardRefs,
    setVideoNode,
    setCardNode,
    resumePlayback,
    pausePlayback,
    replayPlayback,
    seekPlayback,
    toggleFullscreen,
    mediaHandlers,
  };
}
