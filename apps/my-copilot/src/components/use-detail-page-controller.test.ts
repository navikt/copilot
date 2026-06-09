import { renderHook, act } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import type { HomepageVideo } from "@/lib/public-videos";
import { useDetailPageController } from "./use-detail-page-controller";

// Mock adapters — same pattern as use-shorts-feed-controller.test.ts

vi.mock("./use-shorts-feed-storage-adapter", () => ({
  useStorageAdapter: vi.fn(() => ({
    watchState: {
      version: 1,
      updatedAt: new Date().toISOString(),
      videos: {},
    },
    updateProgress: vi.fn(),
    markComplete: vi.fn(),
    flushProgress: vi.fn(),
  })),
}));

vi.mock("./use-shorts-feed-telemetry-adapter", () => ({
  useTelemetryAdapter: vi.fn(() => ({
    emitVideoStarted: vi.fn(),
    emitVideoError: vi.fn(),
    addRebuffer: vi.fn(),
  })),
}));

vi.mock("./use-shorts-feed-media-adapter", () => ({
  useMediaAdapter: vi.fn((callbacks) => {
    const videoRefs = { current: new Map<string, HTMLVideoElement>() };

    const handlers = (videoId: string) => ({
      onPlay: () => callbacks.dispatch({ type: "PLAY" }),
      onPause: () => callbacks.dispatch({ type: "PAUSE" }),
      onTimeUpdate: vi.fn(),
      onEnded: () => {
        callbacks.dispatch({ type: "END" });
        callbacks.markComplete(videoId, undefined);
      },
      onError: vi.fn(),
      onWaiting: vi.fn(),
    });

    return {
      videoRefs,
      cardRefs: { current: new Map() },
      setVideoNode: vi.fn((videoId: string, node: HTMLVideoElement | null) => {
        if (node) videoRefs.current.set(videoId, node);
        else videoRefs.current.delete(videoId);
      }),
      setCardNode: vi.fn(),
      resumePlayback: vi.fn(() => {
        callbacks.dispatch({ type: "PLAY" });
      }),
      pausePlayback: vi.fn(() => {
        callbacks.dispatch({ type: "PAUSE" });
      }),
      replayPlayback: vi.fn(() => {
        callbacks.dispatch({ type: "REPLAY" });
      }),
      seekPlayback: vi.fn(),
      toggleFullscreen: vi.fn(),
      mediaHandlers: handlers,
    };
  }),
}));

vi.mock("@/lib/video-kpi-events", () => ({
  emitVideoKPIEvent: vi.fn(),
}));

function createTestVideo(id: string = "video-1"): HomepageVideo {
  return {
    id,
    title: `Test video ${id}`,
    description: "A test video",
    category: "copilot",
    durationSec: 120,
    language: "nb",
    aspectRatio: "9:16",
    posterUrl: `/poster-${id}.jpg`,
    playUrl: `/play-${id}.m3u8`,
    metadata: { overlay: [] },
  };
}

describe("useDetailPageController", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  // ============================================================================
  // 1. Initial state
  // ============================================================================

  it("starts in paused state (detail page is always open)", () => {
    const video = createTestVideo();
    const { result } = renderHook(() => useDetailPageController({ video }));
    expect(result.current.playbackState).toBe("paused");
  });

  // ============================================================================
  // 2. onTogglePlayback when paused → resumes playback
  // ============================================================================

  it("onTogglePlayback when paused transitions to playing", () => {
    const video = createTestVideo();
    const { result } = renderHook(() => useDetailPageController({ video }));

    expect(result.current.playbackState).toBe("paused");

    act(() => {
      result.current.onTogglePlayback();
    });

    // resumePlayback dispatches PLAY → "playing"
    expect(result.current.playbackState).toBe("playing");
  });

  // ============================================================================
  // 3. onTogglePlayback when playing → pauses
  // ============================================================================

  it("onTogglePlayback when playing transitions to paused", () => {
    const video = createTestVideo();
    const { result } = renderHook(() => useDetailPageController({ video }));

    // Get to playing state first
    act(() => {
      result.current.onTogglePlayback();
    });
    expect(result.current.playbackState).toBe("playing");

    act(() => {
      result.current.onTogglePlayback();
    });

    // pausePlayback dispatches PAUSE → "paused"
    expect(result.current.playbackState).toBe("paused");
  });

  // ============================================================================
  // 4. mediaHandlers.onEnded → state becomes "completed"
  // ============================================================================

  it("mediaHandlers.onEnded transitions to completed", () => {
    const video = createTestVideo();
    const { result } = renderHook(() => useDetailPageController({ video }));

    act(() => {
      result.current.mediaHandlers.onEnded();
    });

    expect(result.current.playbackState).toBe("completed");
  });

  // ============================================================================
  // 5. onReplay when completed → transitions back to playing
  // ============================================================================

  it("onReplay when completed transitions to playing", () => {
    const video = createTestVideo();
    const { result } = renderHook(() => useDetailPageController({ video }));

    // Get to completed state
    act(() => {
      result.current.mediaHandlers.onEnded();
    });
    expect(result.current.playbackState).toBe("completed");

    act(() => {
      result.current.onReplay();
    });

    // REPLAY → "playing"
    expect(result.current.playbackState).toBe("playing");
  });

  // ============================================================================
  // 6. onSeekBackward reduces currentTime by 5 seconds
  // ============================================================================

  it("onSeekBackward seeks the video element by -5 seconds", async () => {
    const { useMediaAdapter } = await import("./use-shorts-feed-media-adapter");
    const mockUseMediaAdapter = vi.mocked(useMediaAdapter);

    const video = createTestVideo("seek-test");
    const { result } = renderHook(() => useDetailPageController({ video }));

    act(() => {
      result.current.onSeekBackward();
    });

    const seekPlayback = mockUseMediaAdapter.mock.results[0]?.value?.seekPlayback as ReturnType<typeof vi.fn>;
    expect(seekPlayback).toHaveBeenCalledWith("seek-test", -5);
  });

  // ============================================================================
  // 7. Adapter wiring
  // ============================================================================

  it("isActiveEvent always returns true (single-video context)", async () => {
    const { useMediaAdapter } = await import("./use-shorts-feed-media-adapter");
    const mockUseMediaAdapter = vi.mocked(useMediaAdapter);

    const video = createTestVideo();
    renderHook(() => useDetailPageController({ video }));

    const callArgs = mockUseMediaAdapter.mock.calls[0]?.[0] as { isActiveEvent: (id: string) => boolean };
    expect(callArgs.isActiveEvent("anything")).toBe(true);
    expect(callArgs.isActiveEvent("other-id")).toBe(true);
  });

  it("returns setVideoNode from media adapter", () => {
    const video = createTestVideo();
    const { result } = renderHook(() => useDetailPageController({ video }));
    expect(typeof result.current.setVideoNode).toBe("function");
  });
});
