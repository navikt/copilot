"use client";

export type VideoKPIEventName =
  | "video_feed_impression"
  | "video_play_started"
  | "video_play_error"
  | "video_rebuffer_count";

type VideoKPIEventPayload = {
  videoId?: string;
  [key: string]: string | number | boolean | undefined;
};

type VideoKPIEvent = {
  event: VideoKPIEventName;
  payload: VideoKPIEventPayload;
};

export function emitVideoKPIEvent(event: VideoKPIEventName, payload: VideoKPIEventPayload) {
  try {
    const entry: VideoKPIEvent = { event, payload };
    window.dispatchEvent(new CustomEvent<VideoKPIEvent>("video-kpi", { detail: entry }));
  } catch (error) {
    console.error("[KPI Event Error] Failed to emit video KPI event:", error, { event });
    // Intentionally don't re-throw; KPI telemetry failure should not crash playback
  }
}
