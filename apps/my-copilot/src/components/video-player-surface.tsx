"use client";

import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from "react";
import type { HomepageVideo } from "@/lib/public-videos";
import { isCompleted, type PlaybackState } from "@/lib/video-playback-machine";
import type { ShortsFeedMediaHandlers } from "./use-shorts-feed-media-adapter";
import { accentForEpisode } from "./video-accent";
import { CompletedOverlay, CornerFullscreenButton, IdleCaption } from "./video-card-chrome";
import { UnifiedVideoHUD } from "./unified-video-hud";

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

function toCssAspectRatio(aspectRatio: string | undefined): string {
  if (!aspectRatio) return "9 / 16";
  if (aspectRatio.includes(":")) {
    return aspectRatio
      .split(":")
      .map((part) => part.trim())
      .join(" / ");
  }
  return aspectRatio;
}

type VideoPlayerSurfaceProps = {
  video: HomepageVideo;
  isActive: boolean;
  playbackState: PlaybackState;
  mediaHandlers: ShortsFeedMediaHandlers;
  setVideoNode: (videoId: string, node: HTMLVideoElement | null) => void;
  onPrimaryAction: () => void;
  onSeekBackward: () => void;
  onSeekForward: () => void;
  onReplay: () => void;
  onFullscreen: () => void;
  onOpen?: () => void;
  onKeyDown?: (event: KeyboardEvent<HTMLDivElement>) => void;
  aspectRatio?: string;
  hudHideDelayMs?: number;
  hudLeaveDelayMs?: number;
};

export function VideoPlayerSurface({
  video,
  isActive,
  playbackState,
  mediaHandlers,
  setVideoNode,
  onPrimaryAction,
  onSeekBackward,
  onSeekForward,
  onReplay,
  onFullscreen,
  onOpen,
  onKeyDown,
  aspectRatio,
  hudHideDelayMs = 1800,
  hudLeaveDelayMs = 500,
}: VideoPlayerSurfaceProps) {
  const marker = episodeMarkerFor(video);
  const accent = accentForEpisode(marker);
  const episodeLabel = episodeLabelFor(video);

  const playing = isActive && playbackState === "playing";
  const paused = isActive && playbackState === "paused";
  const completed = isActive && isCompleted(playbackState);
  const showIdleCaption = !isActive || playbackState === "idle";
  const showPlaybackSurface = playing || paused;
  const shareHref = `/videos/${encodeURIComponent(video.id)}`;

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
    clearHudTimer();
    hudHideTimerRef.current = window.setTimeout(() => {
      setHudVisible(false);
    }, hudLeaveDelayMs);
  }, [playing, clearHudTimer, hudLeaveDelayMs]);

  const revealHud = useCallback(() => {
    setHudVisible(true);
    clearHudTimer();
    if (playing) {
      hudHideTimerRef.current = window.setTimeout(() => {
        setHudVisible(false);
      }, hudHideDelayMs);
    }
  }, [playing, clearHudTimer, hudHideDelayMs]);

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
      }, hudHideDelayMs);
    });

    return () => {
      window.cancelAnimationFrame(frame);
      clearHudTimer();
    };
  }, [playing, clearHudTimer, hudHideDelayMs]);

  const showHud = !playing || hudVisible;
  const resolvedAspectRatio = toCssAspectRatio(aspectRatio ?? video.aspectRatio);

  return (
    <div
      role={!isActive && onOpen ? "button" : undefined}
      tabIndex={!isActive && onOpen ? 0 : -1}
      aria-label={!isActive && onOpen ? `Åpne video: ${video.title}` : undefined}
      onClick={!isActive && onOpen ? onOpen : undefined}
      onKeyDown={!isActive && onOpen ? onKeyDown : undefined}
      onMouseMove={isActive ? revealHud : undefined}
      onMouseLeave={isActive ? hideHud : undefined}
      onTouchStart={isActive ? revealHud : undefined}
      onFocusCapture={isActive ? revealHud : undefined}
      style={{ aspectRatio: resolvedAspectRatio }}
      className={`relative w-full overflow-hidden rounded-xl ${
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

      {paused ? <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-black/25 to-transparent" /> : null}
      {showIdleCaption ? (
        <div className="absolute inset-x-0 bottom-0 z-10 h-32 pointer-events-none bg-gradient-to-t from-black/75 via-black/35 to-transparent" />
      ) : null}

      <UnifiedVideoHUD
        overlays={video.metadata?.overlay}
        episodeLabel={episodeLabel}
        accent={accent}
        durationLabel={formatDuration(video.durationSec)}
        shareHref={shareHref}
        shareTitle={video.title}
        playing={playing}
        isActive={isActive}
        completed={completed}
        showHud={showHud}
        playbackState={playbackState}
        onTogglePlayback={onPrimaryAction}
        onSeekBackward={onSeekBackward}
        onSeekForward={onSeekForward}
        title={video.title}
      />

      {isActive ? <CornerFullscreenButton title={video.title} onClick={onFullscreen} /> : null}
      {showIdleCaption ? <IdleCaption title={video.title} /> : null}
      {completed ? <CompletedOverlay title={video.title} shareHref={shareHref} onReplay={onReplay} /> : null}
    </div>
  );
}
