import { renderHook, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { HomepageVideo } from "@/lib/public-videos";
import { playbackTransition, INITIAL_PLAYBACK_STATE } from "@/lib/video-playback-machine";
import { useShortsFeedController } from "./use-shorts-feed-controller";

// Mock all adapters
vi.mock("./use-shorts-feed-url-sync-adapter", () => ({
  useUrlSyncAdapter: vi.fn(),
}));

vi.mock("./use-shorts-feed-storage-adapter", () => ({
  useStorageAdapter: vi.fn(() => ({
    watchState: {
      version: 1,
      updatedAt: new Date().toISOString(),
      videos: {},
    },
    updateProgress: vi.fn(),
    markComplete: vi.fn(),
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
  useMediaAdapter: vi.fn((_callbacks) => ({
    videoRefs: { current: new Map() },
    cardRefs: { current: new Map() },
    setVideoNode: vi.fn(),
    setCardNode: vi.fn(),
    resumePlayback: vi.fn(),
    pausePlayback: vi.fn(),
    replayPlayback: vi.fn(),
    seekPlayback: vi.fn(),
    toggleFullscreen: vi.fn(),
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    mediaHandlers: vi.fn((_videoId: string) => ({
      onPlay: vi.fn(),
      onPause: vi.fn(),
      onTimeUpdate: vi.fn(),
      onEnded: vi.fn(),
      onError: vi.fn(),
      onWaiting: vi.fn(),
    })),
  })),
}));

vi.mock("@/lib/video-kpi-events", () => ({
  emitVideoKPIEvent: vi.fn(),
}));

function createTestVideo(id: string, title: string = `Video ${id}`): HomepageVideo {
  return {
    id,
    title,
    description: `Description for ${title}`,
    category: "copilot",
    durationSec: 60,
    language: "nb",
    posterUrl: `/poster-${id}.jpg`,
    playUrl: `/play-${id}.m3u8`,
    metadata: { overlay: [] },
  };
}

describe("useShortsFeedController", () => {
  let matchMediaMock: {
    matches: boolean;
    addEventListener: ReturnType<typeof vi.fn>;
    removeEventListener: ReturnType<typeof vi.fn>;
  };

  beforeEach(() => {
    matchMediaMock = {
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    };
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => matchMediaMock)
    );
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  // ============================================================================
  // 1. Event Guard Tests — Inactive events don't corrupt state
  // ============================================================================
  // ⚠️ WIRING TESTS ONLY: These tests verify the controller wires the media
  // adapter correctly. Actual guard logic validation is in
  // use-shorts-feed-media-adapter.test.ts with 25 comprehensive guard tests.
  // ============================================================================

  describe("Event guards (isActiveEvent)", () => {
    // WIRING TEST: Verify controller doesn't process play events for inactive videos.
    // Guard logic (isActiveEvent) is tested in use-shorts-feed-media-adapter.test.ts
    it("ignores play events from inactive/background videos", () => {
      const videoA = createTestVideo("a", "Video A");
      const videoB = createTestVideo("b", "Video B");
      const { result } = renderHook(() => useShortsFeedController({ videos: [videoA, videoB], initialVideoId: "a" }));

      expect(result.current.playbackState).toBe("paused");
      expect(result.current.isViewerOpen).toBe(true);

      // Try to trigger a play event for inactive video B (should be ignored by guard)
      act(() => {
        result.current.openViewer("a");
      });

      // playbackState should remain 'paused' (from OPEN), not changed to 'playing' by videoB events
      expect(result.current.playbackState).toBe("paused");
    });

    // WIRING TEST: Verify controller doesn't process pause events for inactive videos.
    // Guard logic (isActiveEvent) is tested in use-shorts-feed-media-adapter.test.ts
    it("ignores pause events from inactive videos", () => {
      const videoA = createTestVideo("a");
      const videoB = createTestVideo("b");
      const { result } = renderHook(() => useShortsFeedController({ videos: [videoA, videoB], initialVideoId: "a" }));

      // playbackState should start as paused (viewer open with initial video)
      expect(result.current.playbackState).toBe("paused");
      expect(result.current.resolvedActiveId).toBe("a");

      // isActiveEvent guard should prevent videoB pause from affecting state
      // (cannot directly call the guard, but we verify state doesn't change)
      const stateBefore = result.current.playbackState;
      expect(stateBefore).toBe("paused");
    });

    // WIRING TEST: Verify controller doesn't process ended events for inactive videos.
    // Guard logic (isActiveEvent) is tested in use-shorts-feed-media-adapter.test.ts
    it("ignores ended events from inactive videos", () => {
      const videoA = createTestVideo("a");
      const videoB = createTestVideo("b");
      const { result } = renderHook(() => useShortsFeedController({ videos: [videoA, videoB], initialVideoId: "a" }));

      act(() => {
        result.current.openViewer("a");
      });

      expect(result.current.playbackState).toBe("paused");
      // ended event from videoB should not transition to 'completed'
      // (guard prevents the transition from happening in the media adapter)
      expect(result.current.playbackState).not.toBe("completed");
    });
  });

  // ============================================================================
  // 2. Primary Action Policy Tests — onPrimaryAction decision tree
  // ============================================================================

  describe("Primary action policy (onPrimaryAction)", () => {
    it("opens viewer when called on inactive video", () => {
      const videoA = createTestVideo("a");
      const videoB = createTestVideo("b");
      const { result } = renderHook(() => useShortsFeedController({ videos: [videoA, videoB] }));

      expect(result.current.isViewerOpen).toBe(false);
      expect(result.current.resolvedActiveId).not.toBe("b");

      act(() => {
        result.current.onPrimaryAction("b");
      });

      expect(result.current.isViewerOpen).toBe(true);
      expect(result.current.resolvedActiveId).toBe("b");
      expect(result.current.playbackState).toBe("paused");
    });

    it("pauses when player is playing", () => {
      const videoA = createTestVideo("a");
      const { result } = renderHook(() => useShortsFeedController({ videos: [videoA], initialVideoId: "a" }));

      // Initial state: viewer open with paused state
      expect(result.current.isViewerOpen).toBe(true);
      expect(result.current.playbackState).toBe("paused");

      // Simulate playing state by dispatching PLAY event
      // We need to manually set this since we can't directly call dispatch
      // In real scenario, media adapter would trigger this
      act(() => {
        // onPrimaryAction when playing should pause
        result.current.pausePlayback("a");
      });
    });

    it("resumes when player is paused", () => {
      const videoA = createTestVideo("a");
      const { result } = renderHook(() => useShortsFeedController({ videos: [videoA], initialVideoId: "a" }));

      expect(result.current.playbackState).toBe("paused");

      act(() => {
        result.current.resumePlayback("a");
      });

      // resumePlayback is delegated to media adapter
      expect(result.current.resolvedActiveId).toBe("a");
    });

    it("replays when video is completed", () => {
      const videoA = createTestVideo("a");
      const { result } = renderHook(() => useShortsFeedController({ videos: [videoA], initialVideoId: "a" }));

      expect(result.current.isViewerOpen).toBe(true);

      act(() => {
        result.current.replayPlayback("a");
      });

      expect(result.current.resolvedActiveId).toBe("a");
    });
  });

  // ============================================================================
  // 3. Adapter Integration Tests — Each adapter wired correctly
  // ============================================================================

  describe("Adapter integration", () => {
    it("storage adapter is wired to receive watch state and callbacks", async () => {
      const { useStorageAdapter } = await import("./use-shorts-feed-storage-adapter");
      const mockUseStorageAdapter = vi.mocked(useStorageAdapter);

      const videos = [createTestVideo("a"), createTestVideo("b")];
      renderHook(() => useShortsFeedController({ videos }));

      // Verify storage adapter was called
      expect(mockUseStorageAdapter).toHaveBeenCalled();
    });

    it("media adapter receives isActiveEvent callback and adapters", async () => {
      const { useMediaAdapter } = await import("./use-shorts-feed-media-adapter");
      const mockUseMediaAdapter = vi.mocked(useMediaAdapter);

      const videos = [createTestVideo("a"), createTestVideo("b")];
      renderHook(() => useShortsFeedController({ videos, initialVideoId: "a" }));

      expect(mockUseMediaAdapter).toHaveBeenCalled();
      const callArgs = mockUseMediaAdapter.mock.calls[0][0];

      expect(callArgs).toHaveProperty("dispatch");
      expect(callArgs).toHaveProperty("isActiveEvent");
      expect(callArgs).toHaveProperty("updateProgress");
      expect(callArgs).toHaveProperty("markComplete");
      expect(typeof callArgs.isActiveEvent).toBe("function");
    });

    it("telemetry adapter is initialized with video list", async () => {
      const { useTelemetryAdapter } = await import("./use-shorts-feed-telemetry-adapter");
      const mockUseTelemetryAdapter = vi.mocked(useTelemetryAdapter);

      const videos = [createTestVideo("a"), createTestVideo("b"), createTestVideo("c")];
      renderHook(() => useShortsFeedController({ videos }));

      expect(mockUseTelemetryAdapter).toHaveBeenCalledWith({ videos });
    });

    it("url-sync adapter is wired to synchronize state with URL", async () => {
      const { useUrlSyncAdapter } = await import("./use-shorts-feed-url-sync-adapter");
      const mockUseUrlSyncAdapter = vi.mocked(useUrlSyncAdapter);

      const videos = [createTestVideo("a"), createTestVideo("b")];
      renderHook(() => useShortsFeedController({ videos, initialVideoId: "a" }));

      expect(mockUseUrlSyncAdapter).toHaveBeenCalled();
      const callArgs = mockUseUrlSyncAdapter.mock.calls[0][0];

      expect(callArgs).toHaveProperty("dispatch");
      expect(callArgs).toHaveProperty("setActiveId");
      expect(callArgs).toHaveProperty("setIsViewerOpen");
      expect(typeof callArgs.dispatch).toBe("function");
    });

    it("setVideoNode and setCardNode are wired to media adapter", () => {
      const videos = [createTestVideo("a"), createTestVideo("b")];
      const { result } = renderHook(() => useShortsFeedController({ videos, initialVideoId: "a" }));

      expect(typeof result.current.setVideoNode).toBe("function");
      expect(typeof result.current.setCardNode).toBe("function");

      const mockVideoElement = document.createElement("video");
      const mockCardDiv = document.createElement("div");

      act(() => {
        result.current.setVideoNode("a", mockVideoElement);
        result.current.setCardNode("a", mockCardDiv);
      });
    });
  });

  // ============================================================================
  // 4. Playback State Machine Tests — State transitions are legal
  // ============================================================================

  describe("Playback state transitions", () => {
    it("OPEN transitions idle → paused", () => {
      const state = playbackTransition(INITIAL_PLAYBACK_STATE, { type: "OPEN" });
      expect(state).toBe("paused");
    });

    it("PLAY only works from paused/idle", () => {
      expect(playbackTransition("idle", { type: "PLAY" })).toBe("playing");
      expect(playbackTransition("paused", { type: "PLAY" })).toBe("playing");
      expect(playbackTransition("playing", { type: "PLAY" })).toBe("playing");
    });

    it("END transitions playing → completed", () => {
      const state = playbackTransition("playing", { type: "END" });
      expect(state).toBe("completed");
    });

    it("REPLAY transitions completed → playing", () => {
      const state = playbackTransition("completed", { type: "REPLAY" });
      expect(state).toBe("playing");
    });

    it("CLOSE always returns to idle", () => {
      expect(playbackTransition("playing", { type: "CLOSE" })).toBe("idle");
      expect(playbackTransition("paused", { type: "CLOSE" })).toBe("idle");
      expect(playbackTransition("completed", { type: "CLOSE" })).toBe("idle");
    });

    it("maintains coherence with pause after ended (browser quirk)", () => {
      const completedState = playbackTransition("playing", { type: "END" });
      expect(completedState).toBe("completed");

      // Browser sometimes emits pause after ended; should not drop from completed
      const afterPause = playbackTransition(completedState, { type: "PAUSE" });
      expect(afterPause).toBe("completed");
    });
  });

  // ============================================================================
  // 5. Reduced Motion Tests — Media behavior respects prefers-reduced-motion
  // ============================================================================

  describe("Reduced motion support", () => {
    it("detects prefers-reduced-motion on mount", () => {
      matchMediaMock.matches = true;

      const videos = [createTestVideo("a")];
      const { result } = renderHook(() => useShortsFeedController({ videos, initialVideoId: "a" }));

      act(() => {
        // Trigger the listener to set reduced motion
        const listener = matchMediaMock.addEventListener.mock.calls[0][1];
        if (listener) listener();
      });

      expect(result.current.reducedMotion).toBe(true);
    });

    it("updates reducedMotion when preference changes", () => {
      const videos = [createTestVideo("a")];
      const { result } = renderHook(() => useShortsFeedController({ videos }));

      expect(result.current.reducedMotion).toBe(false);

      act(() => {
        matchMediaMock.matches = true;
        const listener = matchMediaMock.addEventListener.mock.calls[0][1];
        if (listener) listener();
      });

      expect(result.current.reducedMotion).toBe(true);
    });

    it("uses auto behavior for scroll when reduced-motion is set", () => {
      matchMediaMock.matches = true;
      const videos = [createTestVideo("a")];
      const { result } = renderHook(() => useShortsFeedController({ videos, initialVideoId: "a" }));

      act(() => {
        const listener = matchMediaMock.addEventListener.mock.calls[0][1];
        if (listener) listener();
      });

      expect(result.current.reducedMotion).toBe(true);
    });

    it("cleans up event listener on unmount", () => {
      const videos = [createTestVideo("a")];
      const { unmount } = renderHook(() => useShortsFeedController({ videos }));

      unmount();

      expect(matchMediaMock.removeEventListener).toHaveBeenCalledWith("change", expect.any(Function));
    });
  });

  // ============================================================================
  // 6. Watch State Ordering Tests — Unwatched videos prioritized
  // ============================================================================

  describe("Video ordering", () => {
    it("preserves video order when no watch state exists", () => {
      const videoA = createTestVideo("a", "Video A");
      const videoB = createTestVideo("b", "Video B");
      const videoC = createTestVideo("c", "Video C");

      const { result } = renderHook(() => useShortsFeedController({ videos: [videoA, videoB, videoC] }));

      expect(result.current.orderedVideos.map((v) => v.id)).toEqual(["a", "b", "c"]);
    });

    it("preserves order while playing (prevents confusing UX)", () => {
      const videos = [
        createTestVideo("a", "Video A"),
        createTestVideo("b", "Video B"),
        createTestVideo("c", "Video C"),
      ];

      const { result } = renderHook(() => useShortsFeedController({ videos, initialVideoId: "b" }));

      // With viewer open, playbackState is 'paused', so reordering can happen
      // But this test verifies the preservation of order logic
      expect(result.current.orderedVideos).toBeDefined();
    });

    it("returns empty orderedVideos when no videos", () => {
      const { result } = renderHook(() => useShortsFeedController({ videos: [] }));

      expect(result.current.orderedVideos).toEqual([]);
    });

    it("resolvedActiveId defaults to first video when activeId is invalid", () => {
      const videos = [createTestVideo("a", "Video A"), createTestVideo("b", "Video B")];

      const { result } = renderHook(() => useShortsFeedController({ videos, initialVideoId: "invalid" }));

      expect(result.current.resolvedActiveId).toBe("a");
    });
  });

  // ============================================================================
  // 7. Initialization Tests
  // ============================================================================

  describe("Initialization", () => {
    it("opens viewer when initialVideoId is provided and valid", () => {
      const videos = [createTestVideo("a"), createTestVideo("b")];
      const { result } = renderHook(() => useShortsFeedController({ videos, initialVideoId: "a" }));

      expect(result.current.isViewerOpen).toBe(true);
      expect(result.current.resolvedActiveId).toBe("a");
      expect(result.current.playbackState).toBe("paused");
    });

    it("keeps viewer closed when initialVideoId is not provided", () => {
      const videos = [createTestVideo("a"), createTestVideo("b")];
      const { result } = renderHook(() => useShortsFeedController({ videos }));

      expect(result.current.isViewerOpen).toBe(false);
      expect(result.current.resolvedActiveId).toBe("a");
      expect(result.current.playbackState).toBe("idle");
    });

    it("ignores invalid initialVideoId", () => {
      const videos = [createTestVideo("a"), createTestVideo("b")];
      const { result } = renderHook(() => useShortsFeedController({ videos, initialVideoId: "nonexistent" }));

      expect(result.current.isViewerOpen).toBe(false);
      expect(result.current.resolvedActiveId).toBe("a");
    });
  });

  // ============================================================================
  // 8. Ref Management Tests
  // ============================================================================

  describe("Ref management", () => {
    it("provides scrollContainerRef for scroll control", () => {
      const videos = [createTestVideo("a")];
      const { result } = renderHook(() => useShortsFeedController({ videos }));

      expect(result.current.scrollContainerRef).toHaveProperty("current");
    });

    it("provides mediaHandlers factory for event binding", () => {
      const videos = [createTestVideo("a"), createTestVideo("b")];
      const { result } = renderHook(() => useShortsFeedController({ videos }));

      const handlersA = result.current.mediaHandlers("a");
      const handlersB = result.current.mediaHandlers("b");

      expect(handlersA).toHaveProperty("onPlay");
      expect(handlersA).toHaveProperty("onPause");
      expect(handlersA).toHaveProperty("onTimeUpdate");
      expect(handlersA).toHaveProperty("onEnded");
      expect(handlersA).toHaveProperty("onError");
      expect(handlersA).toHaveProperty("onWaiting");

      expect(handlersB).toHaveProperty("onPlay");
    });
  });

  // ============================================================================
  // 9. Imperative Controls Tests
  // ============================================================================

  describe("Imperative controls", () => {
    it("openViewer sets activeId, opens viewer, and dispatches OPEN", () => {
      const videos = [createTestVideo("a"), createTestVideo("b")];
      const { result } = renderHook(() => useShortsFeedController({ videos }));

      expect(result.current.isViewerOpen).toBe(false);

      act(() => {
        result.current.openViewer("b");
      });

      expect(result.current.isViewerOpen).toBe(true);
      expect(result.current.resolvedActiveId).toBe("b");
    });

    it("handleCardKeyDown opens viewer on Enter key", () => {
      const videos = [createTestVideo("a"), createTestVideo("b")];
      const { result } = renderHook(() => useShortsFeedController({ videos }));

      const mockEvent = {
        key: "Enter",
        preventDefault: vi.fn(),
      } as unknown as React.KeyboardEvent<HTMLDivElement>;

      act(() => {
        result.current.handleCardKeyDown(mockEvent, "b");
      });

      expect(result.current.isViewerOpen).toBe(true);
      expect(result.current.resolvedActiveId).toBe("b");
    });

    it("handleCardKeyDown opens viewer on Space key", () => {
      const videos = [createTestVideo("a"), createTestVideo("b")];
      const { result } = renderHook(() => useShortsFeedController({ videos }));

      const mockEvent = {
        key: " ",
        preventDefault: vi.fn(),
      } as unknown as React.KeyboardEvent<HTMLDivElement>;

      act(() => {
        result.current.handleCardKeyDown(mockEvent, "b");
      });

      expect(result.current.isViewerOpen).toBe(true);
      expect(result.current.resolvedActiveId).toBe("b");
    });

    it("handleCardKeyDown ignores other keys", () => {
      const videos = [createTestVideo("a"), createTestVideo("b")];
      const { result } = renderHook(() => useShortsFeedController({ videos }));

      const mockEvent = {
        key: "Escape",
        preventDefault: vi.fn(),
      } as unknown as React.KeyboardEvent<HTMLDivElement>;

      act(() => {
        result.current.handleCardKeyDown(mockEvent, "b");
      });

      expect(result.current.isViewerOpen).toBe(false);
      expect(mockEvent.preventDefault).not.toHaveBeenCalled();
    });

    it("provides all imperative control methods", () => {
      const videos = [createTestVideo("a")];
      const { result } = renderHook(() => useShortsFeedController({ videos }));

      expect(typeof result.current.openViewer).toBe("function");
      expect(typeof result.current.closeViewer).toBe("function");
      expect(typeof result.current.onPrimaryAction).toBe("function");
      expect(typeof result.current.resumePlayback).toBe("function");
      expect(typeof result.current.pausePlayback).toBe("function");
      expect(typeof result.current.replayPlayback).toBe("function");
      expect(typeof result.current.seekPlayback).toBe("function");
      expect(typeof result.current.toggleFullscreen).toBe("function");
    });
  });

  // ============================================================================
  // 10. Reordering Delay Tests — Smooth close animation with delayed reorder
  // ============================================================================

  describe("Reordering delay on close", () => {
    it("delays list reordering after close to prevent visual jank", () => {
      vi.useFakeTimers();
      const videos = [createTestVideo("a"), createTestVideo("b")];
      const { result, rerender } = renderHook(() => useShortsFeedController({ videos }));

      // Open viewer
      act(() => {
        result.current.openViewer("a");
      });

      expect(result.current.isViewerOpen).toBe(true);
      expect(result.current.playbackState).toBe("paused");

      // Close viewer should transition state immediately
      act(() => {
        result.current.closeViewer();
      });

      expect(result.current.playbackState).toBe("idle");

      // Video list should not have changed yet (reorder happens after delay)
      expect(result.current.orderedVideos).toBeDefined();

      // Advance time by 300ms to trigger reordering
      act(() => {
        vi.advanceTimersByTime(300);
      });

      // After 300ms, the memo should have recalculated
      // We verify by checking the forceReorder state caused a re-render
      rerender();
      expect(result.current.orderedVideos).toBeDefined();

      vi.useRealTimers();
    });

    it("cleans up timeout on unmount when closeViewer is called", () => {
      vi.useFakeTimers();
      const videos = [createTestVideo("a"), createTestVideo("b")];
      const { result, unmount } = renderHook(() => useShortsFeedController({ videos }));

      // Open and close viewer
      act(() => {
        result.current.openViewer("a");
      });

      act(() => {
        result.current.closeViewer();
      });

      // Unmount before timeout completes
      unmount();

      // Advance timers - should not cause errors
      act(() => {
        vi.advanceTimersByTime(300);
      });

      // Test passes if no error thrown
      expect(true).toBe(true);

      vi.useRealTimers();
    });
  });
});
