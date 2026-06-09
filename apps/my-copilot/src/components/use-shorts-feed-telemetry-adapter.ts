"use client";

import { useCallback, useEffect, useRef } from "react";
import type { HomepageVideo } from "@/lib/public-videos";
import { emitVideoKPIEvent } from "@/lib/video-kpi-events";

// Telemetry adapter: owns all KPI state (dedup tracking) and emission for the
// shorts feed. The media adapter detects native <video> events and delegates to
// these functions; it never constructs KPI payloads or tracks dedup state
// itself. Keeping the started/error/rebuffer bookkeeping here gives each adapter
// a single responsibility (media = detection, telemetry = KPI).
export type TelemetryAdapter = {
  emitVideoStarted: (videoId: string) => void;
  emitVideoError: (videoId: string, errorCode: number | string) => void;
  addRebuffer: (videoId: string) => void;
};

export function useTelemetryAdapter({ videos }: { videos: HomepageVideo[] }): TelemetryAdapter {
  const feedImpressionSent = useRef(false);
  const startedIds = useRef<Set<string>>(new Set());
  const rebufferCountById = useRef<Map<string, number>>(new Map());
  const playErrorKeys = useRef<Set<string>>(new Set());

  // Feed impression KPI: emit once when videos first load
  useEffect(() => {
    if (videos.length > 0 && !feedImpressionSent.current) {
      feedImpressionSent.current = true;
      emitVideoKPIEvent("video_feed_impression", { videoCount: videos.length });
    }
  }, [videos]);

  // First play per video: emitted at most once (startedIds dedup).
  const emitVideoStarted = useCallback((videoId: string) => {
    if (startedIds.current.has(videoId)) return;
    startedIds.current.add(videoId);
    emitVideoKPIEvent("video_play_started", { videoId });
  }, []);

  // Play error per (video, errorCode): emitted at most once (playErrorKeys dedup).
  const emitVideoError = useCallback((videoId: string, errorCode: number | string) => {
    const key = `${videoId}:${errorCode}`;
    if (playErrorKeys.current.has(key)) return;
    playErrorKeys.current.add(key);
    emitVideoKPIEvent("video_play_error", { videoId, errorCode });
  }, []);

  // Rebuffer count: only counts once playback has started, and emits the running
  // total on every rebuffer so downstream sees the latest count per video.
  const addRebuffer = useCallback((videoId: string) => {
    if (!startedIds.current.has(videoId)) return;
    const current = rebufferCountById.current.get(videoId) ?? 0;
    const next = current + 1;
    rebufferCountById.current.set(videoId, next);
    emitVideoKPIEvent("video_rebuffer_count", { videoId, rebufferCount: next });
  }, []);

  return {
    emitVideoStarted,
    emitVideoError,
    addRebuffer,
  };
}
