const COPILOT_API_URL = process.env.COPILOT_API_URL || "http://copilot-api";
const VIDEO_FETCH_TIMEOUT_MS = 8000;

export type PublicVideoFeedItem = {
  id: string;
  title: string;
  description: string;
  category: string;
  published_at: string;
  duration_sec: number;
  aspect_ratio: string;
  language: string;
  poster_url: string;
  play_url: string;
  mp4_url?: string;
  captions_url?: string;
  metadata?: {
    series?: string;
    season?: number;
    episode?: number;
    tags?: string[];
    overlay?: Array<{
      kind: string;
      anchor:
        | "top-left"
        | "top-right"
        | "center-left"
        | "center-right"
        | "center"
        | "bottom-left"
        | "bottom-right"
        | "bottom-full";
      labels: string[];
      highlight_index?: number;
      monospace?: boolean;
    }>;
  };
};

type VideoFeedResponse = {
  items: PublicVideoFeedItem[];
  next_cursor?: string;
};

export type OverlayComponent = {
  kind: "episode-number" | "badge" | "chip" | "counter" | "rule-pill" | string;
  anchor:
    | "top-left"
    | "top-right"
    | "center-left"
    | "center-right"
    | "center"
    | "bottom-left"
    | "bottom-right"
    | "bottom-full";
  labels: string[];
  highlightIndex?: number;
  monospace?: boolean;
};

export type HomepageVideo = {
  id: string;
  title: string;
  description: string;
  category: string;
  durationSec: number;
  language: string;
  posterUrl: string;
  playUrl: string;
  mp4Url?: string;
  captionsUrl?: string;
  metadata?: {
    series?: string;
    season?: number;
    episode?: number;
    tags?: string[];
    overlay?: OverlayComponent[];
  };
};

function normalizeOverlay(item: PublicVideoFeedItem): OverlayComponent[] | undefined {
  const overlays = item.metadata?.overlay;
  if (!overlays || overlays.length === 0) return undefined;
  return overlays.map((overlay) => ({
    kind: overlay.kind,
    anchor: overlay.anchor,
    labels: overlay.labels,
    highlightIndex: overlay.highlight_index,
    monospace: overlay.monospace,
  }));
}

function mapVideoItem(item: PublicVideoFeedItem): HomepageVideo {
  return {
    id: item.id,
    title: item.title,
    description: item.description,
    category: item.category,
    durationSec: item.duration_sec,
    language: item.language,
    posterUrl: item.poster_url,
    playUrl: item.play_url,
    mp4Url: item.mp4_url,
    captionsUrl: item.captions_url,
    metadata: item.metadata
      ? {
          series: item.metadata.series,
          season: item.metadata.season,
          episode: item.metadata.episode,
          tags: item.metadata.tags,
          overlay: normalizeOverlay(item),
        }
      : undefined,
  };
}

async function fetchWithTimeout(input: RequestInfo | URL, init: RequestInit, timeoutMs: number): Promise<Response> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(input, { ...init, signal: controller.signal });
  } finally {
    clearTimeout(timeoutId);
  }
}

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetchWithTimeout(
    `${COPILOT_API_URL}${path}`,
    {
      method: "GET",
      headers: { "Content-Type": "application/json" },
      next: { revalidate: 60 },
    },
    VIDEO_FETCH_TIMEOUT_MS
  );
  if (!response.ok) {
    throw new Error(`Video API request failed (${response.status})`);
  }
  return response.json() as Promise<T>;
}

export async function getPublicVideoFeed(limit: number = 5): Promise<HomepageVideo[]> {
  try {
    const feed = await fetchJSON<VideoFeedResponse>(`/public/v1/videos?limit=${limit}`);
    return feed.items.map(mapVideoItem);
  } catch (error) {
    console.error("Failed to fetch public video feed:", error);
    return [];
  }
}
