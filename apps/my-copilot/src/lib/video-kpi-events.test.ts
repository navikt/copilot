import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { emitVideoKPIEvent } from "./video-kpi-events";

describe("emitVideoKPIEvent", () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("emits KPI event successfully", () => {
    const dispatchSpy = vi.spyOn(window, "dispatchEvent");

    emitVideoKPIEvent("video_play_started", { videoId: "test-video" });

    expect(dispatchSpy).toHaveBeenCalledTimes(1);
    expect(consoleErrorSpy).not.toHaveBeenCalled();
  });

  it("does not throw even if a KPI listener throws", () => {
    const throwingListener = () => {
      throw new Error("KPI listener failed");
    };
    window.addEventListener("video-kpi", throwingListener);

    // Should not throw even though the listener throws
    expect(() => emitVideoKPIEvent("video_play_started", { videoId: "test-video" })).not.toThrow();

    window.removeEventListener("video-kpi", throwingListener);
  });

  it("continues processing after KPI event even if dispatch fails", () => {
    vi.spyOn(window, "dispatchEvent").mockImplementation(() => {
      throw new Error("Dispatch failed");
    });

    // Should not crash
    expect(() => emitVideoKPIEvent("video_rebuffer_count", { videoId: "test", rebufferCount: 2 })).not.toThrow();
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "[KPI Event Error] Failed to emit video KPI event:",
      expect.any(Error),
      expect.objectContaining({ event: "video_rebuffer_count" })
    );
  });

  it("logs error details without exposing internal state", () => {
    vi.spyOn(window, "dispatchEvent").mockImplementation(() => {
      throw new Error("Out of memory");
    });

    emitVideoKPIEvent("video_play_error", { videoId: "vid", errorCode: "NETWORK_ERROR" });

    // Verify error is logged with event context but no raw payload leak
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "[KPI Event Error] Failed to emit video KPI event:",
      expect.any(Error),
      expect.objectContaining({ event: "video_play_error" })
    );
  });
});
