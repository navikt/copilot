"use client";

import { BodyShort, Box, Button, Heading, VStack } from "@navikt/ds-react";
import { useEffect, useRef, useState } from "react";
import type { HomepageVideo } from "@/lib/public-videos";
import { emitVideoKPIEvent } from "@/lib/video-kpi-events";

type ShortsFeedProps = {
  videos: HomepageVideo[];
};

export function ShortsFeed({ videos }: ShortsFeedProps) {
  const [activeId, setActiveId] = useState<string>(videos[0]?.id ?? "");
  const [muted, setMuted] = useState(true);
  const [reducedMotion, setReducedMotion] = useState(false);
  const videoRefs = useRef<Map<string, HTMLVideoElement>>(new Map());
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
    if (!videos.length) return;
    if (!feedImpressionSent.current) {
      feedImpressionSent.current = true;
      emitVideoKPIEvent("video_feed_impression", {
        videoCount: videos.length,
      });
    }

    const observer = new IntersectionObserver(
      (entries) => {
        const best = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0];
        if (best?.target instanceof HTMLVideoElement) {
          const id = best.target.dataset.videoId;
          if (id) setActiveId(id);
        }
      },
      { threshold: [0.55, 0.7, 0.9] }
    );

    for (const video of videoRefs.current.values()) observer.observe(video);
    return () => observer.disconnect();
  }, [videos]);

  useEffect(() => {
    for (const [id, video] of videoRefs.current.entries()) {
      video.muted = muted;
      if (id !== activeId || reducedMotion) {
        video.pause();
        continue;
      }
      void video.play().catch(() => {
        // Browser autoplay policies may block; controls remain available.
      });
    }
  }, [activeId, muted, reducedMotion]);

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

  return (
    <VStack gap="space-12">
      <Box>
        <Button size="small" variant="secondary-neutral" onClick={() => setMuted((prev) => !prev)}>
          {muted ? "Skru på lyd" : "Skru av lyd"}
        </Button>
      </Box>

      <div className="max-h-[70vh] overflow-y-auto snap-y snap-mandatory">
        <VStack gap="space-16">
          {videos.map((video) => (
            <Box
              key={video.id}
              borderColor="neutral"
              borderWidth="1"
              borderRadius="12"
              padding="space-12"
              className="snap-start bg-bg-default"
            >
              <VStack gap="space-8">
                <div className="flex justify-center">
                  <div className="relative w-full max-w-[340px] overflow-hidden rounded-xl border border-gray-200 aspect-[9/16]">
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
                      muted={muted}
                      playsInline
                      preload="metadata"
                      poster={video.posterUrl}
                      className="h-full w-full object-cover"
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
                </div>

                <Heading size="xsmall" level="3">
                  {video.title}
                </Heading>
                <BodyShort size="small" className="text-text-subtle">
                  {video.description}
                </BodyShort>
              </VStack>
            </Box>
          ))}
        </VStack>
      </div>
    </VStack>
  );
}
