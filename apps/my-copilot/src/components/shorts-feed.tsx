"use client";

import { HStack, VStack } from "@navikt/ds-react";
import { useCallback, useEffect, useRef, useState } from "react";
import type { HomepageVideo } from "@/lib/public-videos";
import { isCompleted, type PlaybackState } from "@/lib/video-playback-machine";
import { accentForEpisode } from "./video-overlay-components";
import { CompletedOverlay, CornerFullscreenButton, IdleCaption } from "./video-card-chrome";
import { UnifiedVideoHUD } from "./unified-video-hud";
import {
  type ShortsFeedController,
  type ShortsFeedMediaHandlers,
  useShortsFeedController,
} from "./use-shorts-feed-controller";

type ShortsFeedProps = {
  videos: HomepageVideo[];
  initialVideoId?: string;
};

function formatDuration(durationSec: number): string {
  const min = Math.floor(durationSec / 60);
  const sec = durationSec % 60;
  return `${min}:${String(sec).padStart(2, "0")}`;
}

function episodeMarkerFor(video: HomepageVideo): string | undefined {
  return video.metadata?.overlay?.find((overlay) => overlay.kind === "episode-number")?.labels?.[0];
}

function episodeLabelFor(video: HomepageVideo): string | undefined {
  if (video.category === "cplt") {
    return "Bonus";
  }
  if (video.metadata?.season && video.metadata?.episode) {
    return `S${video.metadata.season}E${video.metadata.episode}`;
  }
  return undefined;
}

// Presentational card. All interaction is delegated to the controller; this
// component only maps the resolved playback state onto chrome.
function ShortsFeedCard({
  video,
  isActive,
  playbackState,
  mediaHandlers,
  setVideoNode,
  setCardNode,
  onOpen,
  onKeyDown,
  onCenterAction,
  onSeekBackward,
  onSeekForward,
  onReplay,
  onFullscreen,
}: {
  video: HomepageVideo;
  isActive: boolean;
  playbackState: PlaybackState;
  mediaHandlers: ShortsFeedMediaHandlers;
  setVideoNode: ShortsFeedController["setVideoNode"];
  setCardNode: ShortsFeedController["setCardNode"];
  onOpen: () => void;
  onKeyDown: (event: React.KeyboardEvent<HTMLDivElement>) => void;
  onCenterAction: () => void;
  onSeekBackward: () => void;
  onSeekForward: () => void;
  onReplay: () => void;
  onFullscreen: () => void;
}) {
  const marker = episodeMarkerFor(video);
  const accent = accentForEpisode(marker);
  const episodeLabel = episodeLabelFor(video);

  // A non-active card always renders in its idle browsing state.
  const playing = isActive && playbackState === "playing";
  const paused = isActive && playbackState === "paused";
  const completed = isActive && isCompleted(playbackState);
  const showIdleCaption = !isActive || playbackState === "idle";
  const showPlaybackSurface = playing || paused;

  const shareHref = `${typeof window !== "undefined" ? window.location.origin : ""}/videos/${encodeURIComponent(video.id)}`;
  const headerEpisodeLabel = episodeLabel ?? video.category;

  const [hudVisible, setHudVisible] = useState(true);
  const hudHideTimerRef = useRef<number | null>(null);

  const clearHudTimer = useCallback(() => {
    if (hudHideTimerRef.current !== null) {
      window.clearTimeout(hudHideTimerRef.current);
      hudHideTimerRef.current = null;
    }
  }, []);

  const hideHud = useCallback(() => {
    if (!playing) return;
    setHudVisible(false);
    clearHudTimer();
  }, [playing, clearHudTimer]);

  const revealHud = useCallback(() => {
    setHudVisible(true);
    clearHudTimer();
    if (playing) {
      hudHideTimerRef.current = window.setTimeout(() => {
        setHudVisible(false);
      }, 1800);
    }
  }, [playing, clearHudTimer]);

  useEffect(() => {
    clearHudTimer();
    if (!playing) {
      const frame = window.requestAnimationFrame(() => {
        setHudVisible(true);
      });
      return () => window.cancelAnimationFrame(frame);
    }
    const frame = window.requestAnimationFrame(() => {
      setHudVisible(true);
      hudHideTimerRef.current = window.setTimeout(() => {
        setHudVisible(false);
      }, 1800);
    });
    return () => {
      window.cancelAnimationFrame(frame);
      clearHudTimer();
    };
  }, [playing, clearHudTimer]);

  const showHud = !playing || hudVisible;

  return (
    <div ref={(node) => setCardNode(video.id, node)} className="group snap-start shrink-0 w-[240px] sm:w-[260px]">
      <div
        role={isActive ? undefined : "button"}
        tabIndex={isActive ? -1 : 0}
        aria-label={isActive ? undefined : `Åpne video: ${video.title}`}
        onClick={isActive ? undefined : onOpen}
        onKeyDown={isActive ? undefined : onKeyDown}
        onMouseMove={isActive ? revealHud : undefined}
        onMouseLeave={isActive ? hideHud : undefined}
        onTouchStart={isActive ? revealHud : undefined}
        className={`relative w-full overflow-hidden rounded-xl aspect-[9/16] ${
          isActive ? "bg-black" : "text-left outline-none focus:ring-2 focus:ring-blue-500"
        }`}
      >
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={video.posterUrl}
          alt=""
          className={`absolute inset-0 h-full w-full object-cover transition-opacity duration-200 ${
            showPlaybackSurface ? "opacity-0" : "opacity-100"
          }`}
        />
        <video
          ref={(node) => setVideoNode(video.id, node)}
          playsInline
          preload="metadata"
          poster={video.posterUrl}
          className={`absolute inset-0 h-full w-full object-contain transition-opacity duration-200 ${
            showPlaybackSurface ? "opacity-100" : "opacity-0 pointer-events-none"
          }`}
          onPlay={mediaHandlers.onPlay}
          onPause={mediaHandlers.onPause}
          onTimeUpdate={mediaHandlers.onTimeUpdate}
          onEnded={mediaHandlers.onEnded}
          onError={mediaHandlers.onError}
          onWaiting={mediaHandlers.onWaiting}
        >
          <source key={`${video.id}-hls`} src={video.playUrl} type="application/x-mpegURL" />
          {video.mp4Url ? <source key={`${video.id}-mp4`} src={video.mp4Url} type="video/mp4" /> : null}
          {video.captionsUrl ? (
            <track
              key={`${video.id}-captions`}
              src={video.captionsUrl}
              kind="captions"
              srcLang={video.language || "nb"}
              label="Teksting"
            />
          ) : null}
        </video>

        {paused && <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-black/25 to-transparent" />}
        {showIdleCaption && (
          <div className="absolute inset-x-0 bottom-0 z-10 h-32 pointer-events-none bg-gradient-to-t from-black/75 via-black/35 to-transparent" />
        )}

        {/* Unified HUD: episode pill, badges, duration, share, content panel, playback controls */}
        {/* Wrap HUD in inert div to prevent keyboard focus when hidden (a11y) */}
        <div inert={!showHud}>
          <UnifiedVideoHUD
            overlays={video.metadata?.overlay}
            episodeLabel={headerEpisodeLabel}
            accent={accent}
            durationLabel={formatDuration(video.durationSec)}
            shareHref={shareHref}
            shareTitle={video.title}
            playing={playing}
            isActive={isActive}
            completed={completed}
            showHud={showHud}
            playbackState={playbackState}
            onTogglePlayback={onCenterAction}
            onSeekBackward={onSeekBackward}
            onSeekForward={onSeekForward}
            title={video.title}
          />
        </div>

        {isActive ? <CornerFullscreenButton title={video.title} onClick={onFullscreen} /> : null}

        {showIdleCaption && <IdleCaption title={video.title} />}

        {completed && <CompletedOverlay title={video.title} shareHref={shareHref} onReplay={onReplay} />}
      </div>
    </div>
  );
}

export function ShortsFeed({ videos, initialVideoId }: ShortsFeedProps) {
  const controller = useShortsFeedController({ videos, initialVideoId });
  const {
    orderedVideos,
    resolvedActiveId,
    isViewerOpen,
    playbackState,
    scrollContainerRef,
    setVideoNode,
    setCardNode,
    mediaHandlers,
    openViewer,
    onPrimaryAction,
    replayPlayback,
    seekPlayback,
    toggleFullscreen,
    handleCardKeyDown,
  } = controller;

  return (
    <VStack gap="space-12">
      <div ref={scrollContainerRef} className="overflow-x-auto overscroll-x-contain snap-x snap-mandatory">
        <HStack gap="space-16" wrap={false} align="start">
          {orderedVideos.map((video) => {
            const isActive = isViewerOpen && resolvedActiveId === video.id;

            return (
              <ShortsFeedCard
                key={video.id}
                video={video}
                isActive={isActive}
                playbackState={playbackState}
                mediaHandlers={mediaHandlers(video.id)}
                setVideoNode={setVideoNode}
                setCardNode={setCardNode}
                onOpen={() => openViewer(video.id)}
                onKeyDown={(event) => handleCardKeyDown(event, video.id)}
                onCenterAction={() => onPrimaryAction(video.id)}
                onSeekBackward={() => seekPlayback(video.id, -5)}
                onSeekForward={() => seekPlayback(video.id, 5)}
                onReplay={() => replayPlayback(video.id)}
                onFullscreen={() => toggleFullscreen(video.id)}
              />
            );
          })}
        </HStack>
      </div>
    </VStack>
  );
}
