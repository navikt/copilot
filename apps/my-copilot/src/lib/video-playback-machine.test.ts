import { describe, expect, it } from "vitest";
import {
  canPause,
  INITIAL_PLAYBACK_STATE,
  isBodyContentVisible,
  isCompleted,
  type PlaybackState,
  playbackTransition,
} from "./video-playback-machine";

describe("video-playback-machine", () => {
  it("starts idle", () => {
    expect(INITIAL_PLAYBACK_STATE).toBe("idle");
  });

  it("OPEN moves an idle card to paused (ready, not playing)", () => {
    expect(playbackTransition("idle", { type: "OPEN" })).toBe("paused");
  });

  it("OPEN keeps a playing card playing (idempotent re-open)", () => {
    expect(playbackTransition("playing", { type: "OPEN" })).toBe("playing");
  });

  it("PLAY transitions any state to playing", () => {
    const states: PlaybackState[] = ["idle", "paused", "completed", "playing"];
    for (const state of states) {
      expect(playbackTransition(state, { type: "PLAY" })).toBe("playing");
    }
  });

  it("PAUSE from playing yields paused", () => {
    expect(playbackTransition("playing", { type: "PAUSE" })).toBe("paused");
  });

  it("PAUSE after completion does not drop out of completed", () => {
    // Some browsers emit a stray `pause` after `ended`; the completed overlay
    // must survive it.
    expect(playbackTransition("completed", { type: "PAUSE" })).toBe("completed");
  });

  it("END transitions to completed", () => {
    expect(playbackTransition("playing", { type: "END" })).toBe("completed");
  });

  it("REPLAY restarts playback from the completed state", () => {
    expect(playbackTransition("completed", { type: "REPLAY" })).toBe("playing");
  });

  it("CLOSE returns to idle", () => {
    expect(playbackTransition("paused", { type: "CLOSE" })).toBe("idle");
  });

  it("canPause is only true while playing", () => {
    expect(canPause("playing")).toBe(true);
    expect(canPause("paused")).toBe(false);
    expect(canPause("idle")).toBe(false);
    expect(canPause("completed")).toBe(false);
  });

  it("body content is visible only while browsing (idle)", () => {
    expect(isBodyContentVisible("idle")).toBe(true);
    expect(isBodyContentVisible("paused")).toBe(false);
    expect(isBodyContentVisible("playing")).toBe(false);
    expect(isBodyContentVisible("completed")).toBe(false);
  });

  it("isCompleted reflects the completed state", () => {
    expect(isCompleted("completed")).toBe(true);
    expect(isCompleted("playing")).toBe(false);
  });
});
