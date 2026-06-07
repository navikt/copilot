import {
  getWatchStatus,
  isWatched,
  loadWatchState,
  markWatched,
  orderVideosByWatchStatus,
  saveWatchState,
  upsertProgress,
  type WatchStateV1,
} from "./video-watch-state";

describe("video-watch-state", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("loads default state when storage is empty", () => {
    const state = loadWatchState(new Date("2026-06-07T10:00:00.000Z"));
    expect(state.version).toBe(1);
    expect(state.videos).toEqual({});
  });

  it("recovers from invalid JSON in storage", () => {
    window.localStorage.setItem("my-copilot:shorts:watch-state:v1", "{oops");
    const state = loadWatchState();
    expect(state.videos).toEqual({});
    expect(window.localStorage.getItem("my-copilot:shorts:watch-state:v1")).toBeNull();
  });

  it("marks video watched at 80 percent progress", () => {
    const start = loadWatchState(new Date("2026-06-07T10:00:00.000Z"));
    const updated = upsertProgress({
      state: start,
      videoId: "video-1",
      currentTimeSec: 80,
      durationSec: 100,
      now: new Date("2026-06-07T10:01:00.000Z"),
    });

    const status = getWatchStatus(updated, "video-1");
    expect(status?.progressPct).toBe(80);
    expect(status?.watched).toBe(true);
    expect(status?.watchedAt).toBe("2026-06-07T10:01:00.000Z");
  });

  it("marks watched when ended", () => {
    const start = loadWatchState(new Date("2026-06-07T10:00:00.000Z"));
    const progressed = upsertProgress({
      state: start,
      videoId: "video-2",
      currentTimeSec: 40,
      durationSec: 100,
      now: new Date("2026-06-07T10:01:00.000Z"),
    });
    const ended = markWatched({
      state: progressed,
      videoId: "video-2",
      durationSec: 100,
      now: new Date("2026-06-07T10:02:00.000Z"),
    });

    expect(isWatched(getWatchStatus(ended, "video-2"))).toBe(true);
    expect(getWatchStatus(ended, "video-2")?.progressPct).toBe(100);
  });

  it("orders videos with unwatched first and keeps group order stable", () => {
    const base = loadWatchState(new Date("2026-06-07T10:00:00.000Z"));
    const watchedState = markWatched({ state: base, videoId: "b", now: new Date("2026-06-07T10:01:00.000Z") });
    const videos = [{ id: "a" }, { id: "b" }, { id: "c" }];

    expect(orderVideosByWatchStatus(videos, watchedState, "deprioritize").map((v) => v.id)).toEqual(["a", "c", "b"]);
    expect(orderVideosByWatchStatus(videos, watchedState, "hide").map((v) => v.id)).toEqual(["a", "c"]);
  });

  it("saves and reloads persisted state", () => {
    const base = loadWatchState(new Date("2026-06-07T10:00:00.000Z"));
    const watched = markWatched({ state: base, videoId: "video-3", now: new Date("2026-06-07T10:01:00.000Z") });
    saveWatchState(watched, new Date("2026-06-07T10:01:00.000Z"));

    const reloaded = loadWatchState(new Date("2026-06-07T10:02:00.000Z"));
    expect(reloaded.version).toBe(1);
    expect(isWatched(getWatchStatus(reloaded, "video-3"))).toBe(true);
  });

  it("keeps state unchanged when progress update contains no real change", () => {
    const state = loadWatchState(new Date("2026-06-07T10:00:00.000Z"));
    const first = upsertProgress({
      state,
      videoId: "video-4",
      currentTimeSec: 10,
      durationSec: 100,
      now: new Date("2026-06-07T10:01:00.000Z"),
    });
    const second = upsertProgress({
      state: first,
      videoId: "video-4",
      currentTimeSec: 10,
      durationSec: 100,
      now: new Date("2026-06-07T10:02:00.000Z"),
    });
    expect(second).toBe(first);
  });

  it("migrates unknown schema to default", () => {
    window.localStorage.setItem(
      "my-copilot:shorts:watch-state:v1",
      JSON.stringify({ version: 2, videos: { "video-1": { watched: true } } })
    );
    const state: WatchStateV1 = loadWatchState(new Date("2026-06-07T10:00:00.000Z"));
    expect(state.videos).toEqual({});
  });
});
