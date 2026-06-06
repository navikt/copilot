"use client";

import { PlayIcon } from "@navikt/aksel-icons";
import { BodyShort, Box, Heading, HStack, VStack } from "@navikt/ds-react";
import { useEffect, useRef, useState } from "react";
import type { HomepageVideo } from "@/lib/public-videos";
import { emitVideoKPIEvent } from "@/lib/video-kpi-events";
import { VideoOverlayRenderer } from "./video-overlay-renderer";

type ShortsFeedProps = {
  videos: HomepageVideo[];
};

export function ShortsFeed({ videos }: ShortsFeedProps) {
  const [activeId, setActiveId] = useState<string>(videos[0]?.id ?? "");
  const [isViewerOpen, setIsViewerOpen] = useState(false);
  const [reducedMotion, setReducedMotion] = useState(false);
  const videoRefs = useRef<Map<string, HTMLVideoElement>>(new Map());
  const pendingPlayId = useRef<string | null>(null);
  const feedImpressionSent = useRef(false);
  const startedIds = useRef<Set<string>>(new Set());
  const rebufferCountById = useRef<Map<string, number>>(new Map());
  const playErrorKeys = useRef<Set<string>>(new Set());

  useEffect(() => {
    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    const apply = () => setReducedMotion(media.matches);
    apply();
    media.addEventListener("change", apply);
    return () => media.removeEventListener("change", apply);
  }, []);

  useEffect(() => {
    if (!videos.length || !feedImpressionSent.current) {
      feedImpressionSent.current = true;
      emitVideoKPIEvent("video_feed_impression", { videoCount: videos.length });
    }
  }, [videos]);

  useEffect(() => {
    if (!isViewerOpen) return;
    for (const [id, video] of videoRefs.current.entries()) {
      if (id !== activeId || reducedMotion) {
        video.pause();
        continue;
      }
      if (pendingPlayId.current === id) {
        pendingPlayId.current = null;
        void video.play().catch(() => {
          // If autoplay is blocked, native controls let the user start playback.
        });
      }
    }
  }, [activeId, reducedMotion, isViewerOpen]);

  const handlePlay = (videoId: string) => {
    if (startedIds.current.has(videoId)) return;
    startedIds.current.add(videoId);
    emitVideoKPIEvent("video_play_started", { videoId });
  };

  const handleError = (videoId: string) => {
    const video = videoRefs.current.get(videoId);
    const errorCode = video?.error?.code;
    const key = `${videoId}:${errorCode ?? "unknown"}`;
    if (playErrorKeys.current.has(key)) return;
    playErrorKeys.current.add(key);
    emitVideoKPIEvent("video_play_error", {
      videoId,
      errorCode: errorCode ?? "unknown",
    });
  };

  const handleWaiting = (videoId: string) => {
    if (!startedIds.current.has(videoId)) return;
    const current = rebufferCountById.current.get(videoId) ?? 0;
    const next = current + 1;
    rebufferCountById.current.set(videoId, next);
    emitVideoKPIEvent("video_rebuffer_count", {
      videoId,
      rebufferCount: next,
    });
  };

  const openViewer = (videoId: string) => {
    pendingPlayId.current = videoId;
    setActiveId(videoId);
    setIsViewerOpen(true);
  };

  const formatDuration = (durationSec: number): string => {
    const min = Math.floor(durationSec / 60);
    const sec = durationSec % 60;
    return `${min}:${String(sec).padStart(2, "0")}`;
  };

  return (
    <VStack gap="space-12">
      <div className="overflow-x-auto overscroll-x-contain snap-x snap-mandatory">
        <HStack gap="space-16" wrap={false} align="start">
          {videos.map((video, index) => {
            const episodeLabel =
              video.metadata?.season && video.metadata?.episode
                ? `S${video.metadata.season}E${video.metadata.episode}`
                : undefined;
            const isActive = isViewerOpen && activeId === video.id;
            return (
              <div key={`${video.id}-${index}`} className="snap-start shrink-0 w-[240px] sm:w-[260px]">
                {isActive ? (
                  <div className="relative w-full overflow-hidden rounded-xl aspect-[9/16] bg-black">
                    <video
                      ref={(node) => {
                        if (!node) {
                          videoRefs.current.delete(video.id);
                          return;
                        }
                        node.dataset.videoId = video.id;
                        videoRefs.current.set(video.id, node);
                      }}
                      controls
                      playsInline
                      preload="metadata"
                      poster={video.posterUrl}
                      className="h-full w-full object-contain"
                      onPlay={() => handlePlay(video.id)}
                      onError={() => handleError(video.id)}
                      onWaiting={() => handleWaiting(video.id)}
                    >
                      <source src={video.playUrl} type="application/x-mpegURL" />
                      {video.mp4Url ? <source src={video.mp4Url} type="video/mp4" /> : null}
                      {video.captionsUrl ? (
                        <track
                          src={video.captionsUrl}
                          kind="captions"
                          srcLang={video.language || "nb"}
                          label="Teksting"
                        />
                      ) : null}
                    </video>
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={() => openViewer(video.id)}
                    className="relative w-full overflow-hidden rounded-xl aspect-[9/16] text-left"
                    aria-label={`Åpne video: ${video.title}`}
                  >
                    <img src={video.posterUrl} alt="" className="h-full w-full object-cover" />
                    <VideoOverlayRenderer overlays={video.metadata?.overlay} />
                    <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-black/25 to-transparent" />
                    <div className="absolute inset-0 flex items-center justify-center">
                      <span className="inline-flex items-center justify-center rounded-full bg-black/60 text-white p-3">
                        <PlayIcon aria-hidden fontSize="1.5rem" />
                      </span>
                    </div>
                    <Box
                      as="span"
                      borderRadius="8"
                      paddingInline="space-8"
                      paddingBlock="space-4"
                      className="absolute top-2 right-2 bg-black/70 text-xs text-white"
                    >
                      {formatDuration(video.durationSec)}
                    </Box>
                    <div className="absolute inset-x-0 bottom-0 p-3 text-white">
                      <Heading size="xsmall" level="3" className="text-white">
                        {video.title}
                      </Heading>
                      <BodyShort size="small" className="text-white/80">
                        {episodeLabel ?? video.category}
                      </BodyShort>
                    </div>
                  </button>
                )}
              </div>
            );
          })}
        </HStack>
      </div>
    </VStack>
  );
}
