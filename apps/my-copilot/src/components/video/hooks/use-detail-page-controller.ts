"use client";

import { useCallback, useState } from "react";
import type { HomepageVideo } from "@/lib/public-videos";
import { canPause, isCompleted, type PlaybackState, playbackTransition } from "@/lib/video-playback-machine";
import { useStorageAdapter } from "./use-shorts-feed-storage-adapter";
import { useTelemetryAdapter } from "./use-shorts-feed-telemetry-adapter";
import { useMediaAdapter, type ShortsFeedMediaHandlers } from "./use-shorts-feed-media-adapter";

export type { ShortsFeedMediaHandlers };

type UseDetailPageControllerArgs = {
  video: HomepageVideo;
};

export type DetailPageController = {
  playbackState: PlaybackState;
  mediaHandlers: ShortsFeedMediaHandlers;
  setVideoNode: (videoId: string, node: HTMLVideoElement | null) => void;
  onTogglePlayback: () => void;
  onSeekBackward: () => void;
  onSeekForward: () => void;
  onReplay: () => void;
  onFullscreen: () => void;
};

export function useDetailPageController({ video }: UseDetailPageControllerArgs): DetailPageController {
  // Detail page is always "open" — starts in paused/ready state.
  const [playbackState, setPlaybackState] = useState<PlaybackState>("paused");

  const dispatch = useCallback((event: Parameters<typeof playbackTransition>[1]) => {
    setPlaybackState((current) => playbackTransition(current, event));
  }, []);

  // Single video — always active.
  const isActiveEvent = useCallback(() => true, []);

  const { updateProgress, markComplete, flushProgress } = useStorageAdapter();
  const telemetry = useTelemetryAdapter({ videos: [video] });

  const media = useMediaAdapter({
    dispatch,
    isActiveEvent,
    telemetry,
    updateProgress,
    markComplete,
    flushProgress,
  });

  const onTogglePlayback = useCallback(() => {
    if (canPause(playbackState)) {
      media.pausePlayback(video.id);
    } else {
      media.resumePlayback(video.id);
    }
  }, [playbackState, media, video.id]);

  const onSeekBackward = useCallback(() => {
    media.seekPlayback(video.id, -5);
  }, [media, video.id]);

  const onSeekForward = useCallback(() => {
    media.seekPlayback(video.id, 5);
  }, [media, video.id]);

  const onReplay = useCallback(() => {
    if (isCompleted(playbackState)) {
      media.replayPlayback(video.id);
    }
  }, [playbackState, media, video.id]);

  const onFullscreen = useCallback(() => {
    media.toggleFullscreen(video.id);
  }, [media, video.id]);

  return {
    playbackState,
    mediaHandlers: media.mediaHandlers(video.id),
    setVideoNode: media.setVideoNode,
    onTogglePlayback,
    onSeekBackward,
    onSeekForward,
    onReplay,
    onFullscreen,
  };
}
