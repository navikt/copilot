import { renderHook } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { HomepageVideo } from "@/lib/public-videos";
import { useTelemetryAdapter } from "./use-shorts-feed-telemetry-adapter";

// The telemetry adapter owns all KPI state (started/error/rebuffer bookkeeping)
// and emission for the shorts feed. KPIs are emitted as `video-kpi` CustomEvents
// on `window`, so we listen for those and assert on the emitted payloads. The
// dedup semantics here were previously embedded in the media adapter; this suite
// is the authoritative coverage for them after the Phase 4 boundary cleanup.

const ACTIVE = "video-a";
const BACKGROUND = "video-b";

type KPIEvent = { event: string; payload: Record<string, unknown> };

function makeVideos(count: number): HomepageVideo[] {
  return Array.from({ length: count }, (_, i) => ({ id: `video-${i}` }) as HomepageVideo);
}

describe("useTelemetryAdapter", () => {
  let events: KPIEvent[];
  let listener: (event: Event) => void;

  function callsFor(name: string) {
    return events.filter((entry) => entry.event === name);
  }

  beforeEach(() => {
    events = [];
    listener = (event: Event) => {
      const detail = (event as CustomEvent<KPIEvent>).detail;
      events.push(detail);
    };
    window.addEventListener("video-kpi", listener);
  });

  afterEach(() => {
    window.removeEventListener("video-kpi", listener);
    vi.clearAllMocks();
  });

  // ---------------------------------------------------------------------------
  // Feed impression KPI.
  // ---------------------------------------------------------------------------
  describe("feed impression", () => {
    it("emits video_feed_impression once when videos load", () => {
      renderHook(() => useTelemetryAdapter({ videos: makeVideos(3) }));
      const impressions = callsFor("video_feed_impression");
      expect(impressions).toHaveLength(1);
      expect(impressions[0].payload).toEqual({ videoCount: 3 });
    });

    it("does not emit video_feed_impression when there are no videos", () => {
      renderHook(() => useTelemetryAdapter({ videos: [] }));
      expect(callsFor("video_feed_impression")).toHaveLength(0);
    });

    it("does not re-emit video_feed_impression on re-render", () => {
      const { rerender } = renderHook(({ videos }) => useTelemetryAdapter({ videos }), {
        initialProps: { videos: makeVideos(2) },
      });
      rerender({ videos: makeVideos(2) });
      expect(callsFor("video_feed_impression")).toHaveLength(1);
    });
  });

  // ---------------------------------------------------------------------------
  // video_play_started dedup (startedIds).
  // ---------------------------------------------------------------------------
  describe("emitVideoStarted", () => {
    it("emits video_play_started for a freshly started video", () => {
      const { result } = renderHook(() => useTelemetryAdapter({ videos: makeVideos(1) }));
      result.current.emitVideoStarted(ACTIVE);
      const started = callsFor("video_play_started");
      expect(started).toHaveLength(1);
      expect(started[0].payload).toEqual({ videoId: ACTIVE });
    });

    it("emits video_play_started only once per video (startedIds dedup)", () => {
      const { result } = renderHook(() => useTelemetryAdapter({ videos: makeVideos(1) }));
      result.current.emitVideoStarted(ACTIVE);
      result.current.emitVideoStarted(ACTIVE);
      expect(callsFor("video_play_started")).toHaveLength(1);
    });

    it("tracks started state independently per video", () => {
      const { result } = renderHook(() => useTelemetryAdapter({ videos: makeVideos(2) }));
      result.current.emitVideoStarted(ACTIVE);
      result.current.emitVideoStarted(BACKGROUND);
      expect(callsFor("video_play_started").map((e) => e.payload.videoId)).toEqual([ACTIVE, BACKGROUND]);
    });
  });

  // ---------------------------------------------------------------------------
  // video_play_error dedup (playErrorKeys).
  // ---------------------------------------------------------------------------
  describe("emitVideoError", () => {
    it("emits video_play_error with the error code", () => {
      const { result } = renderHook(() => useTelemetryAdapter({ videos: makeVideos(1) }));
      result.current.emitVideoError(ACTIVE, 3);
      const errors = callsFor("video_play_error");
      expect(errors).toHaveLength(1);
      expect(errors[0].payload).toEqual({ videoId: ACTIVE, errorCode: 3 });
    });

    it("emits video_play_error only once per (video, errorCode) key", () => {
      const { result } = renderHook(() => useTelemetryAdapter({ videos: makeVideos(1) }));
      result.current.emitVideoError(ACTIVE, 2);
      result.current.emitVideoError(ACTIVE, 2);
      expect(callsFor("video_play_error")).toHaveLength(1);
    });

    it("emits separately for different error codes on the same video", () => {
      const { result } = renderHook(() => useTelemetryAdapter({ videos: makeVideos(1) }));
      result.current.emitVideoError(ACTIVE, 2);
      result.current.emitVideoError(ACTIVE, 3);
      expect(callsFor("video_play_error")).toHaveLength(2);
    });

    it("supports a non-numeric 'unknown' error code", () => {
      const { result } = renderHook(() => useTelemetryAdapter({ videos: makeVideos(1) }));
      result.current.emitVideoError(ACTIVE, "unknown");
      const errors = callsFor("video_play_error");
      expect(errors).toHaveLength(1);
      expect(errors[0].payload).toEqual({ videoId: ACTIVE, errorCode: "unknown" });
    });
  });

  // ---------------------------------------------------------------------------
  // video_rebuffer_count (startedIds guard + rebufferCountById accumulation).
  // ---------------------------------------------------------------------------
  describe("addRebuffer", () => {
    it("does not count a rebuffer before playback has started", () => {
      const { result } = renderHook(() => useTelemetryAdapter({ videos: makeVideos(1) }));
      result.current.addRebuffer(ACTIVE);
      expect(callsFor("video_rebuffer_count")).toHaveLength(0);
    });

    it("emits video_rebuffer_count after playback has started", () => {
      const { result } = renderHook(() => useTelemetryAdapter({ videos: makeVideos(1) }));
      result.current.emitVideoStarted(ACTIVE);
      result.current.addRebuffer(ACTIVE);
      const rebuffers = callsFor("video_rebuffer_count");
      expect(rebuffers).toHaveLength(1);
      expect(rebuffers[0].payload).toEqual({ videoId: ACTIVE, rebufferCount: 1 });
    });

    it("accumulates the running total across multiple rebuffers", () => {
      const { result } = renderHook(() => useTelemetryAdapter({ videos: makeVideos(1) }));
      result.current.emitVideoStarted(ACTIVE);
      result.current.addRebuffer(ACTIVE);
      result.current.addRebuffer(ACTIVE);
      result.current.addRebuffer(ACTIVE);
      const counts = callsFor("video_rebuffer_count").map(
        (e) => (e.payload as { rebufferCount: number }).rebufferCount
      );
      expect(counts).toEqual([1, 2, 3]);
    });

    it("keeps an independent rebuffer counter per video", () => {
      const { result } = renderHook(() => useTelemetryAdapter({ videos: makeVideos(2) }));
      result.current.emitVideoStarted(ACTIVE);
      result.current.addRebuffer(ACTIVE);
      result.current.addRebuffer(ACTIVE);

      result.current.emitVideoStarted(BACKGROUND);
      result.current.addRebuffer(BACKGROUND);
      const backgroundCounts = callsFor("video_rebuffer_count")
        .filter((e) => e.payload.videoId === BACKGROUND)
        .map((e) => (e.payload as { rebufferCount: number }).rebufferCount);
      // The new video's counter starts at 1, unaffected by the first video's 2.
      expect(backgroundCounts).toEqual([1]);
    });
  });
});
