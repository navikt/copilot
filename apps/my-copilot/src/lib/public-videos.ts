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
    const seen = new Set<string>();
    const uniqueItems = feed.items.filter((item) => {
      if (seen.has(item.id)) return false;
      seen.add(item.id);
      return true;
    });
    const newestFirst = [...uniqueItems].sort((a, b) => {
      const aTime = Date.parse(a.published_at);
      const bTime = Date.parse(b.published_at);
      const aSort = Number.isFinite(aTime) ? aTime : Number.NEGATIVE_INFINITY;
      const bSort = Number.isFinite(bTime) ? bTime : Number.NEGATIVE_INFINITY;
      return bSort - aSort;
    });
    return newestFirst.map((item) => ({
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
      metadata: item.metadata,
    }));
  } catch (error) {
    console.error("Failed to fetch public video feed:", error);
    return [];
  }
}
