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
  const entry: VideoKPIEvent = { event, payload };
  console.info("[video-kpi]", entry);
  window.dispatchEvent(new CustomEvent<VideoKPIEvent>("video-kpi", { detail: entry }));
}
