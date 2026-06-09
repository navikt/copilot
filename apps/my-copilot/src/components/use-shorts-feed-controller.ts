"use client";

// Controller hook for the homepage shorts feed.
//
// Owns every imperative concern so the presentation layer can stay declarative:
//   - DOM refs (video + card elements, scroll container)
//   - URL <-> active-video synchronisation (?video=...)
//   - media side-effects (play/pause/scroll-into-view, reduced-motion)
//   - playback state transitions (via the pure playback machine)
//   - watch-state persistence and KPI telemetry
//
// It returns a small, explicit API. The component renders that API; it never
// reaches into refs or the media element directly.

import { useCallback, type KeyboardEvent } from "react";
import { useEffect, useMemo, useRef, useState } from "react";
import type { HomepageVideo } from "@/lib/public-videos";
import {
  canPause,
  isCompleted,
  INITIAL_PLAYBACK_STATE,
  type PlaybackEvent,
  type PlaybackState,
  playbackTransition,
} from "@/lib/video-playback-machine";
import { orderVideosByWatchStatus } from "@/lib/video-watch-state";
import { useUrlSyncAdapter } from "./use-shorts-feed-url-sync-adapter";
import { useStorageAdapter } from "./use-shorts-feed-storage-adapter";
import { useTelemetryAdapter } from "./use-shorts-feed-telemetry-adapter";
import { useMediaAdapter, type ShortsFeedMediaHandlers } from "./use-shorts-feed-media-adapter";

type UseShortsFeedControllerArgs = {
  videos: HomepageVideo[];
  initialVideoId?: string;
};

export type { ShortsFeedMediaHandlers };

export type ShortsFeedController = {
  orderedVideos: HomepageVideo[];
  resolvedActiveId: string;
  isViewerOpen: boolean;
  playbackState: PlaybackState;
  reducedMotion: boolean;
  scrollContainerRef: React.RefObject<HTMLDivElement | null>;
  setVideoNode: (videoId: string, node: HTMLVideoElement | null) => void;
  setCardNode: (videoId: string, node: HTMLDivElement | null) => void;
  mediaHandlers: (videoId: string) => ShortsFeedMediaHandlers;
  openViewer: (videoId: string) => void;
  closeViewer: () => void;
  onPrimaryAction: (videoId: string) => void;
  resumePlayback: (videoId: string) => void;
  pausePlayback: (videoId: string) => void;
  replayPlayback: (videoId: string) => void;
  seekPlayback: (videoId: string, deltaSeconds: number) => void;
  toggleFullscreen: (videoId: string) => void;
  handleCardKeyDown: (event: KeyboardEvent<HTMLDivElement>, videoId: string) => void;
};

export function useShortsFeedController({ videos, initialVideoId }: UseShortsFeedControllerArgs): ShortsFeedController {
  const initialActiveId = useMemo(() => {
    if (initialVideoId && videos.some((video) => video.id === initialVideoId)) {
      return initialVideoId;
    }
    return videos[0]?.id ?? "";
  }, [initialVideoId, videos]);

  const initiallyOpen = Boolean(initialVideoId && videos.some((video) => video.id === initialVideoId));

  const [activeId, setActiveId] = useState<string>(initialActiveId);
  const [isViewerOpen, setIsViewerOpen] = useState(initiallyOpen);
  const [reducedMotion, setReducedMotion] = useState(false);
  const [playbackState, setPlaybackState] = useState<PlaybackState>(
    initiallyOpen ? playbackTransition(INITIAL_PLAYBACK_STATE, { type: "OPEN" }) : INITIAL_PLAYBACK_STATE
  );

  const { watchState, updateProgress, markComplete, flushProgress } = useStorageAdapter();
  const telemetry = useTelemetryAdapter({ videos });
  const pendingPlayId = useRef<string | null>(initialVideoId ?? null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  // Watch-status order (unwatched first). Recomputed only when inputs change.
  const watchOrder = useMemo(() => orderVideosByWatchStatus(videos, watchState, "deprioritize"), [videos, watchState]);
  const [orderedVideos, setOrderedVideos] = useState<HomepageVideo[]>(watchOrder);
  const [prevPlaybackState, setPrevPlaybackState] = useState<PlaybackState>(playbackState);

  // Re-sync the visible order only when playback returns to idle (e.g. on
  // close). Freezing the list across every non-idle state keeps the active
  // video from jumping the instant playback starts — previously the order
  // reverted to the raw `videos` order, which visibly reordered already-watched
  // (deprioritized) videos on first play. While idle the user is only browsing,
  // so the watch state cannot change until the viewer reopens; syncing on the
  // idle transition is sufficient and avoids re-render loops.
  if (playbackState !== prevPlaybackState) {
    setPrevPlaybackState(playbackState);
    if (playbackState === "idle") {
      setOrderedVideos(watchOrder);
    }
  }

  const resolvedActiveId =
    orderedVideos.length > 0 && orderedVideos.some((video) => video.id === activeId)
      ? activeId
      : (orderedVideos[0]?.id ?? "");

  // Single funnel for every state change. Keeps transitions legal and centralised
  // so the component can describe intent ("open", "pause") rather than spell out
  // which concrete state should result.
  const dispatch = useCallback((event: PlaybackEvent) => {
    setPlaybackState((current) => playbackTransition(current, event));
  }, []);

  // Single coordination point for autoplay intent ("play this video once its
  // element is mounted and active"). This is the *only* writer that sets
  // pendingPlayId; the autoplay effect below is the only reader/clearer. Routing
  // every open path (openViewer + url-sync) through here keeps the play handoff
  // in one place instead of scattering raw ref assignments.
  const requestAutoplay = useCallback((videoId: string) => {
    pendingPlayId.current = videoId;
  }, []);

  // Initialize media adapter with guard for active events
  const isActiveEvent = useCallback(
    (videoId: string) => isViewerOpen && videoId === resolvedActiveId,
    [isViewerOpen, resolvedActiveId]
  );

  const media = useMediaAdapter({
    dispatch,
    isActiveEvent,
    telemetry,
    updateProgress,
    markComplete,
    flushProgress,
  });

  // --- reduced motion -------------------------------------------------------
  useEffect(() => {
    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    const apply = () => setReducedMotion(media.matches);
    apply();
    media.addEventListener("change", apply);
    return () => media.removeEventListener("change", apply);
  }, []);

  // --- URL <-> active video sync (delegated to adapter) --------------------
  useUrlSyncAdapter({
    videos,
    initialActiveId,
    isViewerOpen,
    dispatch,
    setActiveId,
    setIsViewerOpen,
    onOpenViewer: requestAutoplay,
  });

  // --- autoplay / single-active enforcement --------------------------------
  useEffect(() => {
    if (!isViewerOpen) return;
    for (const [id, video] of media.videoRefs.current.entries()) {
      if (id !== resolvedActiveId || reducedMotion) {
        video.pause();
        continue;
      }
      if (pendingPlayId.current === id) {
        pendingPlayId.current = null;
        void video.play().catch(() => {
          // If autoplay is blocked, the unified HUD play control lets the user start playback.
        });
      }
    }
  }, [reducedMotion, resolvedActiveId, isViewerOpen, media.videoRefs]);

  // --- scroll the active card into view ------------------------------------
  useEffect(() => {
    if (!isViewerOpen || !resolvedActiveId) return;
    const card = media.cardRefs.current.get(resolvedActiveId);
    if (!card) return;

    card.scrollIntoView({
      block: "nearest",
      inline: "center",
      behavior: reducedMotion ? "auto" : "smooth",
    });
  }, [isViewerOpen, resolvedActiveId, reducedMotion, media.cardRefs]);

  // --- imperative controls delegated to adapter ----------------------------
  const openViewer = useCallback(
    (videoId: string) => {
      requestAutoplay(videoId);
      setActiveId(videoId);
      setIsViewerOpen(true);
      dispatch({ type: "OPEN" });
      // No direct resumePlayback here: opening flips isViewerOpen/resolvedActiveId,
      // which re-runs the autoplay effect and plays the video exactly once. A direct
      // play() call here would double-trigger playback.
    },
    [dispatch, requestAutoplay]
  );

  const closeViewer = useCallback(() => {
    dispatch({ type: "CLOSE" });
    // Returning to idle re-syncs `orderedVideos` to the latest watch-status
    // order during the next render, so a just-watched video is deprioritized.
  }, [dispatch]);

  const handleCardKeyDown = useCallback(
    (event: KeyboardEvent<HTMLDivElement>, videoId: string) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        openViewer(videoId);
      }
    },
    [openViewer]
  );

  const onPrimaryAction = useCallback(
    (videoId: string) => {
      if (videoId !== resolvedActiveId || !isViewerOpen) {
        openViewer(videoId);
        return;
      }
      if (canPause(playbackState)) {
        media.pausePlayback(videoId);
        return;
      }
      if (isCompleted(playbackState)) {
        media.replayPlayback(videoId);
        return;
      }
      media.resumePlayback(videoId);
    },
    [resolvedActiveId, isViewerOpen, playbackState, openViewer, media]
  );

  return {
    orderedVideos,
    resolvedActiveId,
    isViewerOpen,
    playbackState,
    reducedMotion,
    scrollContainerRef,
    setVideoNode: media.setVideoNode,
    setCardNode: media.setCardNode,
    mediaHandlers: media.mediaHandlers,
    openViewer,
    closeViewer,
    onPrimaryAction,
    resumePlayback: media.resumePlayback,
    pausePlayback: media.pausePlayback,
    replayPlayback: media.replayPlayback,
    seekPlayback: media.seekPlayback,
    toggleFullscreen: media.toggleFullscreen,
    handleCardKeyDown,
  };
}
