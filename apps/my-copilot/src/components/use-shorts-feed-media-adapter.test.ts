import { renderHook } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { PlaybackEvent } from "@/lib/video-playback-machine";
import { useMediaAdapter } from "./use-shorts-feed-media-adapter";

// Reference test suite for the media adapter — the most security-critical
// module in the shorts feed. The adapter wires native <video> events to the
// playback machine and delegates KPI telemetry to the telemetry adapter. The
// single most important invariant is the `isActiveEvent(videoId)` guard: a
// background (off-screen) video must never be able to drive the active card's
// state or spam telemetry. Browsers routinely fire `pause`, `timeupdate`,
// `waiting`, `error` and `ended` on videos that are paused/buffering in the
// background, so without this guard a scrolled-away clip could corrupt progress,
// flood rebuffer/error KPIs, or desync the state machine.
//
// KPI deduplication (started/error/rebuffer bookkeeping) now lives in the
// telemetry adapter; this suite verifies the media adapter *delegates* to it
// behind the active-event guard. Dedup semantics themselves are covered by
// use-shorts-feed-telemetry-adapter.test.ts.
//
// The adapter exposes `mediaHandlers(videoId)` returning the per-card handlers
// that the JSX binds to `onPlay`/`onPause`/etc. We invoke those handlers
// directly (rather than dispatching synthetic DOM events) because that is the
// exact surface the controller wires up, and it lets each test target one
// handler in isolation.

const ACTIVE = "video-a";
const BACKGROUND = "video-b";

type Spies = {
  dispatch: ReturnType<typeof vi.fn<(event: PlaybackEvent) => void>>;
  telemetry: {
    emitVideoStarted: ReturnType<typeof vi.fn<(videoId: string) => void>>;
    emitVideoError: ReturnType<typeof vi.fn<(videoId: string, errorCode: number | string) => void>>;
    addRebuffer: ReturnType<typeof vi.fn<(videoId: string) => void>>;
  };
  updateProgress: ReturnType<
    typeof vi.fn<(videoId: string, currentSecond: number, duration: number | undefined) => void>
  >;
  markComplete: ReturnType<typeof vi.fn<(videoId: string, duration: number | undefined) => void>>;
  flushProgress: ReturnType<
    typeof vi.fn<(videoId: string, currentSecond: number, duration: number | undefined) => void>
  >;
};

function setVideoError(video: HTMLVideoElement, code: number) {
  // happy-dom exposes `error` as a read-only null; redefine it so the adapter
  // can read `video.error.code` the way a real failed media element would.
  Object.defineProperty(video, "error", {
    value: { code },
    configurable: true,
  });
}

describe("useMediaAdapter", () => {
  let videoA: HTMLVideoElement;
  let videoB: HTMLVideoElement;
  let containerA: HTMLDivElement;
  let containerB: HTMLDivElement;
  let spies: Spies;
  // `activeId` is mutated by individual tests to flip which card is active.
  // `isActiveEvent` reads it lazily so we can switch the active card without
  // re-rendering the hook.
  let activeId: string;

  function renderAdapter() {
    const isActiveEvent = vi.fn((videoId: string) => videoId === activeId);
    const { result } = renderHook(() =>
      useMediaAdapter({
        dispatch: spies.dispatch,
        isActiveEvent,
        telemetry: spies.telemetry,
        updateProgress: spies.updateProgress,
        markComplete: spies.markComplete,
        flushProgress: spies.flushProgress,
      })
    );
    // Register both video nodes so handlers that read the DOM element work.
    result.current.setVideoNode(ACTIVE, videoA);
    result.current.setVideoNode(BACKGROUND, videoB);
    result.current.setCardNode(ACTIVE, containerA);
    result.current.setCardNode(BACKGROUND, containerB);
    return { result, isActiveEvent };
  }

  beforeEach(() => {
    activeId = ACTIVE;

    videoA = document.createElement("video");
    videoA.id = "video-a";
    videoB = document.createElement("video");
    videoB.id = "video-b";

    containerA = document.createElement("div");
    containerA.id = "card-a";
    containerB = document.createElement("div");
    containerB.id = "card-b";

    document.body.append(videoA, videoB, containerA, containerB);

    // Native play()/pause() are not implemented in happy-dom; stub them so the
    // imperative controls can be asserted without unhandled rejections.
    videoA.play = vi.fn(() => Promise.resolve());
    videoA.pause = vi.fn();
    videoB.play = vi.fn(() => Promise.resolve());
    videoB.pause = vi.fn();

    spies = {
      dispatch: vi.fn(),
      telemetry: {
        emitVideoStarted: vi.fn(),
        emitVideoError: vi.fn(),
        addRebuffer: vi.fn(),
      },
      updateProgress: vi.fn(),
      markComplete: vi.fn(),
      flushProgress: vi.fn(),
    };
  });

  afterEach(() => {
    videoA.remove();
    videoB.remove();
    containerA.remove();
    containerB.remove();
    vi.clearAllMocks();
  });

  // ---------------------------------------------------------------------------
  // Suite 1: Guard completeness — events from the BACKGROUND video are ignored.
  // Every handler must short-circuit on `!isActiveEvent(videoId)`. These are the
  // highest-risk paths flagged by the Phase 4 review.
  // ---------------------------------------------------------------------------
  describe("guard completeness (background video is inert)", () => {
    it("handlePlay does not dispatch or delegate telemetry when fired from a background video", () => {
      const { result } = renderAdapter();
      result.current.mediaHandlers(BACKGROUND).onPlay();
      // A background play must not start the state machine or emit a KPI.
      expect(spies.dispatch).not.toHaveBeenCalled();
      expect(spies.telemetry.emitVideoStarted).not.toHaveBeenCalled();
    });

    it("handlePause does not dispatch or flush when fired from a background video", () => {
      const { result } = renderAdapter();
      result.current.mediaHandlers(BACKGROUND).onPause();
      expect(spies.dispatch).not.toHaveBeenCalled();
      expect(spies.flushProgress).not.toHaveBeenCalled();
    });

    it("handleTimeUpdate does not update progress when fired from a background video", () => {
      const { result } = renderAdapter();
      videoB.currentTime = 30;
      // Without the guard this would overwrite the active card's progress with a
      // background clip's playhead — silent data corruption.
      result.current.mediaHandlers(BACKGROUND).onTimeUpdate();
      expect(spies.updateProgress).not.toHaveBeenCalled();
    });

    it("handleEnded does not emit ENDED or markComplete when fired from a background video", () => {
      const { result } = renderAdapter();
      result.current.mediaHandlers(BACKGROUND).onEnded();
      expect(spies.dispatch).not.toHaveBeenCalled();
      expect(spies.markComplete).not.toHaveBeenCalled();
      expect(spies.flushProgress).not.toHaveBeenCalled();
    });

    it("handleError does not delegate a play error when fired from a background video", () => {
      const { result } = renderAdapter();
      setVideoError(videoB, 4);
      // Background errors are common (network teardown on scroll) and must never
      // reach telemetry.
      result.current.mediaHandlers(BACKGROUND).onError();
      expect(spies.telemetry.emitVideoError).not.toHaveBeenCalled();
    });

    it("handleWaiting does not delegate a rebuffer when fired from a background video", () => {
      const { result } = renderAdapter();
      result.current.mediaHandlers(BACKGROUND).onWaiting();
      expect(spies.telemetry.addRebuffer).not.toHaveBeenCalled();
    });
  });

  // ---------------------------------------------------------------------------
  // Suite 2: Guard positive path — events from the ACTIVE video do take effect
  // and delegate to the telemetry adapter.
  // ---------------------------------------------------------------------------
  describe("guard positive path (active video drives state)", () => {
    it("handlePlay dispatches PLAY and delegates emitVideoStarted for the active video", () => {
      const { result } = renderAdapter();
      result.current.mediaHandlers(ACTIVE).onPlay();
      expect(spies.dispatch).toHaveBeenCalledWith({ type: "PLAY" });
      expect(spies.telemetry.emitVideoStarted).toHaveBeenCalledWith(ACTIVE);
    });

    it("handlePause dispatches PAUSE and flushes progress for the active video", () => {
      const { result } = renderAdapter();
      videoA.currentTime = 12;
      result.current.mediaHandlers(ACTIVE).onPause();
      expect(spies.flushProgress).toHaveBeenCalledWith(ACTIVE, 12, undefined);
      expect(spies.dispatch).toHaveBeenCalledWith({ type: "PAUSE" });
    });

    it("handleTimeUpdate updates progress for the active video", () => {
      const { result } = renderAdapter();
      videoA.currentTime = 7.9;
      result.current.mediaHandlers(ACTIVE).onTimeUpdate();
      // currentSecond is floored; duration is undefined while NaN.
      expect(spies.updateProgress).toHaveBeenCalledWith(ACTIVE, 7, undefined);
    });

    it("handleEnded dispatches END and marks complete for the active video", () => {
      const { result } = renderAdapter();
      videoA.currentTime = 60;
      result.current.mediaHandlers(ACTIVE).onEnded();
      expect(spies.dispatch).toHaveBeenCalledWith({ type: "END" });
      expect(spies.flushProgress).toHaveBeenCalledWith(ACTIVE, 60, undefined);
      expect(spies.markComplete).toHaveBeenCalledWith(ACTIVE, undefined);
    });

    it("handleError delegates emitVideoError with the element's error code for the active video", () => {
      const { result } = renderAdapter();
      setVideoError(videoA, 3);
      result.current.mediaHandlers(ACTIVE).onError();
      expect(spies.telemetry.emitVideoError).toHaveBeenCalledWith(ACTIVE, 3);
    });

    it("handleError delegates emitVideoError with 'unknown' when the element has no error code", () => {
      const { result } = renderAdapter();
      result.current.mediaHandlers(ACTIVE).onError();
      expect(spies.telemetry.emitVideoError).toHaveBeenCalledWith(ACTIVE, "unknown");
    });

    it("handleWaiting delegates addRebuffer for the active video", () => {
      const { result } = renderAdapter();
      result.current.mediaHandlers(ACTIVE).onWaiting();
      expect(spies.telemetry.addRebuffer).toHaveBeenCalledWith(ACTIVE);
    });
  });

  // ---------------------------------------------------------------------------
  // Suite 3: Telemetry delegation discipline — imperative controls never emit.
  // ---------------------------------------------------------------------------
  describe("telemetry delegation discipline", () => {
    it("resumePlayback does not delegate emitVideoStarted (only native play does)", () => {
      const { result } = renderAdapter();
      // Imperative resume drives the element directly; the KPI is emitted by the
      // resulting `play` event via handlePlay, never by the control itself.
      result.current.resumePlayback(ACTIVE);
      expect(spies.telemetry.emitVideoStarted).not.toHaveBeenCalled();
    });

    it("does not delegate telemetry from pause or ended handlers", () => {
      const { result } = renderAdapter();
      result.current.mediaHandlers(ACTIVE).onPause();
      result.current.mediaHandlers(ACTIVE).onEnded();
      expect(spies.telemetry.emitVideoStarted).not.toHaveBeenCalled();
      expect(spies.telemetry.emitVideoError).not.toHaveBeenCalled();
      expect(spies.telemetry.addRebuffer).not.toHaveBeenCalled();
    });
  });

  // ---------------------------------------------------------------------------
  // Suite 4: Imperative playback controls.
  // ---------------------------------------------------------------------------
  describe("playback controls", () => {
    it("resumePlayback calls video.play()", () => {
      const { result } = renderAdapter();
      result.current.resumePlayback(ACTIVE);
      expect(videoA.play).toHaveBeenCalledTimes(1);
    });

    it("pausePlayback calls video.pause()", () => {
      const { result } = renderAdapter();
      result.current.pausePlayback(ACTIVE);
      expect(videoA.pause).toHaveBeenCalledTimes(1);
    });

    it("replayPlayback resets currentTime, dispatches REPLAY and plays", () => {
      const { result } = renderAdapter();
      videoA.currentTime = 42;
      result.current.replayPlayback(ACTIVE);
      expect(spies.dispatch).toHaveBeenCalledWith({ type: "REPLAY" });
      expect(videoA.currentTime).toBe(0);
      expect(videoA.play).toHaveBeenCalledTimes(1);
    });
  });
});
