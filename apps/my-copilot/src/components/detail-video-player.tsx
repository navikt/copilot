"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { HomepageVideo } from "@/lib/public-videos";
import { isCompleted } from "@/lib/video-playback-machine";
import { accentForEpisode } from "./video-overlay-components";
import { CompletedOverlay, CornerFullscreenButton } from "./video-card-chrome";
import { UnifiedVideoHUD } from "./unified-video-hud";
import { useDetailPageController } from "./use-detail-page-controller";

interface DetailVideoPlayerProps {
  video: HomepageVideo;
}

function formatDuration(durationSec: number): string {
  const min = Math.floor(durationSec / 60);
  const sec = durationSec % 60;
  return `${min}:${String(sec).padStart(2, "0")}`;
}

function episodeMarkerFor(video: HomepageVideo): string | undefined {
  return video.metadata?.overlay?.find((o) => o.kind === "episode-number")?.labels?.[0];
}

function episodeLabelFor(video: HomepageVideo): string | undefined {
  if (video.category === "cplt") return "Bonus";
  if (video.metadata?.season && video.metadata?.episode) {
    return `S${video.metadata.season}E${video.metadata.episode}`;
  }
  return undefined;
}

export function DetailVideoPlayer({ video }: DetailVideoPlayerProps) {
  const {
    playbackState,
    mediaHandlers,
    setVideoNode,
    onTogglePlayback,
    onSeekBackward,
    onSeekForward,
    onReplay,
    onFullscreen,
  } = useDetailPageController({ video });

  const [hudVisible, setHudVisible] = useState(true);
  const hudHideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearHudTimer = useCallback(() => {
    if (hudHideTimerRef.current !== null) {
      clearTimeout(hudHideTimerRef.current);
      hudHideTimerRef.current = null;
    }
  }, []);

  const revealHud = useCallback(() => {
    setHudVisible(true);
    clearHudTimer();
    if (playbackState === "playing") {
      hudHideTimerRef.current = setTimeout(() => setHudVisible(false), 3000);
    }
  }, [clearHudTimer, playbackState]);

  const hideHud = useCallback(() => {
    if (playbackState === "playing") {
      clearHudTimer();
      hudHideTimerRef.current = setTimeout(() => setHudVisible(false), 500);
    }
  }, [clearHudTimer, playbackState]);

  useEffect(() => {
    if (playbackState !== "playing") {
      clearHudTimer();
      const frame = window.requestAnimationFrame(() => setHudVisible(true));
      return () => window.cancelAnimationFrame(frame);
    }
  }, [playbackState, clearHudTimer]);

  useEffect(() => () => clearHudTimer(), [clearHudTimer]);

  const showHud = playbackState !== "playing" || hudVisible;
  const showPlaybackSurface = playbackState === "playing" || playbackState === "paused";

  const marker = episodeMarkerFor(video);
  const accent = accentForEpisode(marker);
  const episodeLabel = episodeLabelFor(video);
  const shareHref = `/videos/${encodeURIComponent(video.id)}`;

  return (
    <div
      style={{ aspectRatio: "9 / 16" }}
      className="relative w-full bg-black overflow-hidden rounded-xl"
      onMouseMove={revealHud}
      onMouseLeave={hideHud}
      onTouchStart={revealHud}
    >
      {/* Poster — shown before playback starts */}
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={video.posterUrl}
        alt=""
        className={`absolute inset-0 w-full h-full object-cover transition-opacity ${showPlaybackSurface ? "opacity-0" : "opacity-100"}`}
      />

      {/* Video element */}
      <video
        ref={(node) => setVideoNode(video.id, node)}
        playsInline
        preload="metadata"
        poster={video.posterUrl}
        className={`w-full h-full object-contain transition-opacity ${showPlaybackSurface ? "opacity-100" : "opacity-0 pointer-events-none"}`}
        onPlay={mediaHandlers.onPlay}
        onPause={mediaHandlers.onPause}
        onTimeUpdate={mediaHandlers.onTimeUpdate}
        onEnded={mediaHandlers.onEnded}
        onError={mediaHandlers.onError}
        onWaiting={mediaHandlers.onWaiting}
      >
        <source src={video.playUrl} type="application/x-mpegURL" />
        {video.mp4Url && <source src={video.mp4Url} type="video/mp4" />}
        {video.captionsUrl && (
          <track kind="captions" src={video.captionsUrl} srcLang={video.language || "nb"} label="Teksting" />
        )}
      </video>

      {/* Pause gradient overlay */}
      {playbackState === "paused" && showPlaybackSurface && (
        <div className="absolute inset-0 bg-gradient-to-b from-black/40 via-transparent to-black/60 pointer-events-none" />
      )}

      {/* HUD — inert when hidden */}
      <div {...(showHud ? {} : { inert: "" as unknown as boolean })}>
        <UnifiedVideoHUD
          overlays={video.metadata?.overlay}
          episodeLabel={episodeLabel ?? video.category}
          accent={accent}
          durationLabel={formatDuration(video.durationSec)}
          shareHref={shareHref}
          shareTitle={video.title}
          playing={playbackState === "playing"}
          isActive={true}
          completed={isCompleted(playbackState)}
          showHud={showHud}
          playbackState={playbackState}
          onTogglePlayback={onTogglePlayback}
          onSeekBackward={onSeekBackward}
          onSeekForward={onSeekForward}
          title={video.title}
        />
      </div>

      <CornerFullscreenButton title={video.title} onClick={onFullscreen} />

      {isCompleted(playbackState) && (
        <CompletedOverlay
          title={video.title}
          shareHref={`/videos/${encodeURIComponent(video.id)}`}
          onReplay={onReplay}
        />
      )}
    </div>
  );
}
